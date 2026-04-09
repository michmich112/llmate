package proxy

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strings"
	"sync"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
)

// ErrNoAvailableProvider is returned when no healthy, circuit-open provider
// can serve the requested model and endpoint.
var ErrNoAvailableProvider = errors.New("no available provider for model")

// SmartRouter implements Router using alias resolution, circuit breaking,
// endpoint filtering, and weighted priority-based provider selection.
type SmartRouter struct {
	store    db.Store
	breakers map[string]*CircuitBreaker // keyed by provider ID
	mu       sync.RWMutex
}

// NewSmartRouter creates a SmartRouter backed by the given Store.
func NewSmartRouter(store db.Store) *SmartRouter {
	return &SmartRouter{
		store:    store,
		breakers: make(map[string]*CircuitBreaker),
	}
}

// getBreaker returns the CircuitBreaker for providerID, creating one if absent.
// Uses double-checked locking to minimize lock contention.
func (r *SmartRouter) getBreaker(providerID string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[providerID]
	r.mu.RUnlock()
	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if cb, ok = r.breakers[providerID]; ok {
		return cb
	}
	cb = NewCircuitBreaker()
	r.breakers[providerID] = cb
	return cb
}

// candidate is an internal routing candidate with selection metadata.
type candidate struct {
	provider models.Provider
	modelID  string
	weight   int
	priority int
}

// Route selects a backend provider for the given model and endpoint path.
//
// Algorithm:
//  1. Resolve alias → alias candidates or direct GetHealthyProvidersForModel
//  2. Filter by circuit breaker Allow()
//  3. Filter by enabled endpoint
//  4. Group by priority (highest first), take top group
//  5. Weighted random within top group
func (r *SmartRouter) Route(ctx context.Context, modelID string, endpointPath string) (*RouteResult, error) {
	candidates, requestedViaAlias, err := r.resolveCandidates(ctx, modelID)
	if err != nil {
		return nil, err
	}

	// Filter: circuit breaker
	candidates = r.filterByCircuitBreaker(candidates)

	// Filter: enabled endpoint
	candidates, err = r.filterByEndpoint(ctx, candidates, endpointPath)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, ErrNoAvailableProvider
	}

	selected, err := selectWeightedPriority(candidates)
	if err != nil {
		return nil, err
	}

	targetURL := strings.TrimRight(selected.provider.BaseURL, "/") + "/" + strings.TrimLeft(endpointPath, "/")
	return &RouteResult{
		Provider:          selected.provider,
		ModelID:           selected.modelID,
		TargetURL:         targetURL,
		RequestedViaAlias: requestedViaAlias,
	}, nil
}

// resolveCandidates builds the initial candidate list via alias or direct lookup.
// The second return value is true when ResolveAlias returned at least one enabled alias row
// (routing is alias-based for this client model name).
func (r *SmartRouter) resolveCandidates(ctx context.Context, modelID string) ([]candidate, bool, error) {
	aliases, err := r.store.ResolveAlias(ctx, modelID)
	if err != nil {
		return nil, false, err
	}

	if len(aliases) > 0 {
		var candidates []candidate
		for _, a := range aliases {
			provider, lookupErr := r.store.GetProvider(ctx, a.ProviderID)
			if lookupErr != nil || provider == nil {
				continue
			}
			candidates = append(candidates, candidate{
				provider: *provider,
				modelID:  a.ModelID,
				weight:   a.Weight,
				priority: a.Priority,
			})
		}
		return candidates, true, nil
	}

	providers, err := r.store.GetHealthyProvidersForModel(ctx, modelID)
	if err != nil {
		return nil, false, err
	}
	candidates := make([]candidate, 0, len(providers))
	for _, p := range providers {
		candidates = append(candidates, candidate{
			provider: p,
			modelID:  modelID,
			weight:   1,
			priority: 0,
		})
	}
	return candidates, false, nil
}

// filterByCircuitBreaker removes candidates whose breaker does not allow requests.
func (r *SmartRouter) filterByCircuitBreaker(candidates []candidate) []candidate {
	out := candidates[:0]
	for _, c := range candidates {
		if r.getBreaker(c.provider.ID).Allow() {
			out = append(out, c)
		}
	}
	return out
}

// filterByEndpoint removes candidates that lack an enabled endpoint for the path.
func (r *SmartRouter) filterByEndpoint(ctx context.Context, candidates []candidate, endpointPath string) ([]candidate, error) {
	out := candidates[:0]
	for _, c := range candidates {
		ep, err := r.store.GetEnabledEndpoint(ctx, c.provider.ID, endpointPath)
		if err != nil {
			return nil, err
		}
		if ep != nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// selectWeightedPriority picks one candidate from the highest-priority group
// using weighted random selection.
func selectWeightedPriority(candidates []candidate) (candidate, error) {
	// Sort descending by priority
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})

	topPriority := candidates[0].priority
	var topGroup []candidate
	for _, c := range candidates {
		if c.priority == topPriority {
			topGroup = append(topGroup, c)
		}
	}

	// Collect positive-weight candidates and sum weights
	var pool []candidate
	totalWeight := 0
	for _, c := range topGroup {
		if c.weight > 0 {
			pool = append(pool, c)
			totalWeight += c.weight
		}
	}
	if len(pool) == 0 || totalWeight == 0 {
		return candidate{}, ErrNoAvailableProvider
	}

	pick := rand.Intn(totalWeight)
	cumulative := 0
	for _, c := range pool {
		cumulative += c.weight
		if pick < cumulative {
			return c, nil
		}
	}

	// Unreachable under correct arithmetic, but safe fallback
	return pool[len(pool)-1], nil
}

// ReportSuccess records a successful request for the given provider.
func (r *SmartRouter) ReportSuccess(providerID string) {
	r.getBreaker(providerID).RecordSuccess()
}

// ReportFailure records a failed request for the given provider.
func (r *SmartRouter) ReportFailure(providerID string) {
	r.getBreaker(providerID).RecordFailure()
}

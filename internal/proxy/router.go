package proxy

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/llmate/gateway/internal/models"
)

var ErrNoAvailableProvider = errors.New("no available provider for model")

type SmartRouter struct {
	catalog  *RoutingCatalog
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

func NewSmartRouter(catalog *RoutingCatalog) *SmartRouter {
	return &SmartRouter{
		catalog:  catalog,
		breakers: make(map[string]*CircuitBreaker),
	}
}

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

func (r *SmartRouter) syncBreakerConfig(providerID string, p *models.Provider) *CircuitBreaker {
	cb := r.getBreaker(providerID)
	if p == nil {
		p = r.catalog.ProviderByID(providerID)
	}
	if p == nil {
		return cb
	}
	cb.Configure(
		p.CircuitBreakerErrorThreshold,
		time.Duration(p.CircuitBreakerWindowSeconds)*time.Second,
		time.Duration(p.CircuitBreakerCooldownSeconds)*time.Second,
	)
	return cb
}

func (r *SmartRouter) circuitBreakerEnabled(providerID string, p *models.Provider) bool {
	if p != nil {
		return p.CircuitBreakerEnabled
	}
	if cached := r.catalog.ProviderByID(providerID); cached != nil {
		return cached.CircuitBreakerEnabled
	}
	// Unknown provider: keep legacy behavior (breaker active).
	return true
}

type candidate struct {
	provider models.Provider
	modelID  string
	weight   int
	priority int
}

func (r *SmartRouter) Route(ctx context.Context, modelID string, endpointPath string) (*RouteResult, error) {
	_ = ctx
	candidates, requestedViaAlias, err := r.resolveCandidates(modelID)
	if err != nil {
		return nil, err
	}
	candidates = r.filterByCircuitBreaker(candidates)
	candidates = r.filterByEndpoint(candidates, endpointPath)
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

func (r *SmartRouter) resolveCandidates(modelID string) ([]candidate, bool, error) {
	if aliasCands, ok := r.catalog.AliasCandidates(modelID); ok {
		out := make([]candidate, 0, len(aliasCands))
		for _, c := range aliasCands {
			out = append(out, candidate{provider: c.Provider, modelID: c.ModelID, weight: c.Weight, priority: c.Priority})
		}
		return out, true, nil
	}
	direct := r.catalog.DirectCandidates(modelID)
	out := make([]candidate, 0, len(direct))
	for _, c := range direct {
		out = append(out, candidate{provider: c.Provider, modelID: c.ModelID, weight: c.Weight, priority: c.Priority})
	}
	return out, false, nil
}

func (r *SmartRouter) filterByCircuitBreaker(candidates []candidate) []candidate {
	out := candidates[:0]
	for _, c := range candidates {
		p := c.provider
		if !r.circuitBreakerEnabled(p.ID, &p) {
			out = append(out, c)
			continue
		}
		cb := r.syncBreakerConfig(p.ID, &p)
		if cb.Allow() {
			out = append(out, c)
		}
	}
	return out
}

func (r *SmartRouter) filterByEndpoint(candidates []candidate, endpointPath string) []candidate {
	out := candidates[:0]
	for _, c := range candidates {
		if r.catalog.HasEnabledEndpoint(c.provider.ID, endpointPath) {
			out = append(out, c)
		}
	}
	return out
}

func selectWeightedPriority(candidates []candidate) (candidate, error) {
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
	return pool[len(pool)-1], nil
}

func (r *SmartRouter) ReportSuccess(providerID string) {
	if !r.circuitBreakerEnabled(providerID, nil) {
		return
	}
	r.syncBreakerConfig(providerID, nil).RecordSuccess()
}

func (r *SmartRouter) ReportFailure(providerID string) {
	if !r.circuitBreakerEnabled(providerID, nil) {
		return
	}
	r.syncBreakerConfig(providerID, nil).RecordFailure()
}

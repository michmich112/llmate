package proxy

import (
	"context"
	"log/slog"
	"sort"
	"sync/atomic"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
)

// RoutingChangeNotifier triggers an async routing catalog reload.
type RoutingChangeNotifier func()

// RouteCandidate is a routing target with selection metadata.
type RouteCandidate struct {
	Provider models.Provider
	ModelID  string
	Weight   int
	Priority int
}

type routingSnapshot struct {
	aliases         map[string][]RouteCandidate
	directModels    map[string][]RouteCandidate
	endpoints       map[string]map[string]struct{}
	publicModelIDs  []string
	providerModels  map[string]map[string]models.ProviderModel
	providers       map[string]models.Provider
}

// RoutingCatalog holds an in-memory routing snapshot refreshed on routing-relevant events.
type RoutingCatalog struct {
	store    db.Store
	snap     atomic.Pointer[routingSnapshot]
	reloadCh chan struct{}
	logger   *slog.Logger
}


// NewRoutingCatalogFromData builds a catalog from static data (tests and bootstrapping).
func NewRoutingCatalogFromData(data *models.RoutingData) *RoutingCatalog {
	c := &RoutingCatalog{
		reloadCh: make(chan struct{}, 1),
		logger:   slog.Default(),
	}
	c.snap.Store(buildRoutingSnapshot(data))
	return c
}

func NewRoutingCatalog(store db.Store) *RoutingCatalog {
	c := &RoutingCatalog{
		store:    store,
		reloadCh: make(chan struct{}, 1),
		logger:   slog.Default(),
	}
	c.snap.Store(&routingSnapshot{
		aliases:        make(map[string][]RouteCandidate),
		directModels:   make(map[string][]RouteCandidate),
		endpoints:      make(map[string]map[string]struct{}),
		providerModels: make(map[string]map[string]models.ProviderModel),
		providers:      make(map[string]models.Provider),
	})
	return c
}

func (c *RoutingCatalog) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.reloadCh:
				rctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := c.Reload(rctx); err != nil {
					c.logger.Warn("routing catalog reload failed", "error", err)
				}
				cancel()
			}
		}
	}()
}

func (c *RoutingCatalog) ReloadAsync() {
	select {
	case c.reloadCh <- struct{}{}:
	default:
	}
}

func (c *RoutingCatalog) Reload(ctx context.Context) error {
	data, err := c.store.LoadRoutingData(ctx)
	if err != nil {
		return err
	}
	c.snap.Store(buildRoutingSnapshot(data))
	return nil
}

func (c *RoutingCatalog) snapshot() *routingSnapshot {
	return c.snap.Load()
}

func (c *RoutingCatalog) AliasCandidates(alias string) ([]RouteCandidate, bool) {
	s := c.snapshot()
	cands, ok := s.aliases[alias]
	if !ok || len(cands) == 0 {
		return nil, false
	}
	out := make([]RouteCandidate, len(cands))
	copy(out, cands)
	return out, true
}

func (c *RoutingCatalog) DirectCandidates(modelID string) []RouteCandidate {
	s := c.snapshot()
	cands := s.directModels[modelID]
	out := make([]RouteCandidate, len(cands))
	copy(out, cands)
	return out
}

func normalizeEndpointPath(path string) string {
	if path == "" {
		return path
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

func (c *RoutingCatalog) HasEnabledEndpoint(providerID, path string) bool {
	s := c.snapshot()
	paths, ok := s.endpoints[providerID]
	if !ok {
		return false
	}
	_, ok = paths[normalizeEndpointPath(path)]
	return ok
}

func (c *RoutingCatalog) PublicModelIDs() []string {
	s := c.snapshot()
	out := make([]string, len(s.publicModelIDs))
	copy(out, s.publicModelIDs)
	return out
}

func (c *RoutingCatalog) ProviderModel(providerID, modelID string) *models.ProviderModel {
	s := c.snapshot()
	if byModel, ok := s.providerModels[providerID]; ok {
		if pm, ok := byModel[modelID]; ok {
			copy := pm
			return &copy
		}
	}
	return nil
}

// ProviderByID returns a copy of the provider from the routing snapshot, or nil.
func (c *RoutingCatalog) ProviderByID(providerID string) *models.Provider {
	s := c.snapshot()
	if p, ok := s.providers[providerID]; ok {
		copy := p
		return &copy
	}
	return nil
}

func buildRoutingSnapshot(data *models.RoutingData) *routingSnapshot {
	providers := make(map[string]models.Provider, len(data.Providers))
	for _, p := range data.Providers {
		providers[p.ID] = p
	}

	endpoints := make(map[string]map[string]struct{})
	for _, ep := range data.Endpoints {
		if !ep.IsSupported || !ep.IsEnabled {
			continue
		}
		if endpoints[ep.ProviderID] == nil {
			endpoints[ep.ProviderID] = make(map[string]struct{})
		}
		endpoints[ep.ProviderID][ep.Path] = struct{}{}
	}

	providerModels := make(map[string]map[string]models.ProviderModel)
	directModels := make(map[string][]RouteCandidate)
	seen := make(map[string]struct{})
	for _, m := range data.Models {
		if providerModels[m.ProviderID] == nil {
			providerModels[m.ProviderID] = make(map[string]models.ProviderModel)
		}
		providerModels[m.ProviderID][m.ModelID] = m
		if !m.IsAvailable {
			continue
		}
		seen[m.ModelID] = struct{}{}
		p, ok := providers[m.ProviderID]
		if !ok || !p.IsHealthy {
			continue
		}
		directModels[m.ModelID] = append(directModels[m.ModelID], RouteCandidate{
			Provider: p,
			ModelID:  m.ModelID,
			Weight:   1,
			Priority: 0,
		})
	}

	aliases := make(map[string][]RouteCandidate)
	for _, a := range data.Aliases {
		if !a.IsEnabled {
			continue
		}
		p, ok := providers[a.ProviderID]
		if !ok || !p.IsHealthy {
			continue
		}
		if byModel, ok := providerModels[a.ProviderID]; ok {
			if pm, ok := byModel[a.ModelID]; ok && !pm.IsAvailable {
				continue
			}
		}
		aliases[a.Alias] = append(aliases[a.Alias], RouteCandidate{
			Provider: p,
			ModelID:  a.ModelID,
			Weight:   a.Weight,
			Priority: a.Priority,
		})
		seen[a.Alias] = struct{}{}
	}

	publicModelIDs := make([]string, 0, len(seen))
	for id := range seen {
		publicModelIDs = append(publicModelIDs, id)
	}
	sort.Strings(publicModelIDs)

	return &routingSnapshot{
		aliases:        aliases,
		directModels:   directModels,
		endpoints:      endpoints,
		publicModelIDs: publicModelIDs,
		providerModels: providerModels,
		providers:      providers,
	}
}

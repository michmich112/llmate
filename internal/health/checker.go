package health

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
)

// CircuitBreakerReporter defines the interface for feeding health check results
// into the circuit breaker. This avoids importing the concrete circuit breaker
// implementation.
type CircuitBreakerReporter interface {
	ReportSuccess(providerID string)
	ReportFailure(providerID string)
}

// Checker runs background health checks on all registered providers.
type Checker struct {
	store    db.Store
	breaker  CircuitBreakerReporter
	client   *http.Client
	interval time.Duration
	logger   *slog.Logger
}

// NewChecker creates a new health checker instance.
// If client is nil, http.DefaultClient is used.
func NewChecker(store db.Store, breaker CircuitBreakerReporter, client *http.Client, interval time.Duration, logger *slog.Logger) *Checker {
	if client == nil {
		client = http.DefaultClient // use default client if none provided
	}
	return &Checker{
		store:    store,
		breaker:  breaker,
		client:   client,
		interval: interval,
		logger:   logger,
	}
}

// Start runs the background health check goroutine.
// If ctx is already cancelled, returns immediately.
// If interval <= 0, logs a warning and returns without starting.
func (c *Checker) Start(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if c.interval <= 0 {
		c.logger.Warn("health checker interval is <= 0, not starting")
		return
	}

	c.logger.Info("starting health checker goroutine")

	go func() {
		defer c.logger.Info("stopping health checker goroutine")

		// Run immediately on start
		c.checkAll(ctx)

		// Then run on ticker
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.checkAll(ctx)
			}
		}
	}()
}

// checkAll retrieves all providers and checks each one concurrently.
func (c *Checker) checkAll(ctx context.Context) {
	providers, err := c.store.ListProviders(ctx)
	if err != nil {
		c.logger.Warn("failed to list providers for health check", "error", err)
		return
	}

	var wg sync.WaitGroup
	for _, p := range providers {
		wg.Add(1)
		go func(provider models.Provider) {
			defer wg.Done()
			c.checkProvider(ctx, provider)
		}(p)
	}

	wg.Wait()
}

// checkProvider performs a health check on a single provider.
func (c *Checker) checkProvider(ctx context.Context, p models.Provider) {
	// Build request URL with proper handling of trailing slashes
	baseURL := strings.TrimSuffix(p.BaseURL, "/")
	reqURL := baseURL + "/v1/models"

	// Create request with 10 second timeout
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		c.logger.Warn("failed to create health check request",
			"name", p.Name,
			"provider_id", p.ID,
			"error", err)
		c.updateHealth(p.ID, false)
		return
	}

	// Add API key header if present
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Warn("provider unhealthy - network error",
			"name", p.Name,
			"provider_id", p.ID,
			"error", err)
		c.updateHealth(p.ID, false)
		return
	}
	defer resp.Body.Close()

	// Ensure response body is fully consumed for connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	// Check if status code is 2xx
	isHealthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	if isHealthy {
		c.logger.Debug("provider healthy",
			"name", p.Name,
			"provider_id", p.ID)
		c.updateHealth(p.ID, true)
	} else {
		c.logger.Warn("provider unhealthy - non-2xx status",
			"name", p.Name,
			"provider_id", p.ID,
			"status", resp.StatusCode)
		c.updateHealth(p.ID, false)
	}
}

// updateHealth updates the provider's health in the store and reports to circuit breaker.
func (c *Checker) updateHealth(providerID string, healthy bool) {
	// Update store
	if err := c.store.UpdateProviderHealth(context.Background(), providerID, healthy); err != nil {
		c.logger.Warn("failed to update provider health in store",
			"provider_id", providerID,
			"error", err)
	}

	// Report to circuit breaker
	if healthy {
		c.breaker.ReportSuccess(providerID)
	} else {
		c.breaker.ReportFailure(providerID)
	}
}

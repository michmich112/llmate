package proxy

import (
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	StateClosed   CircuitBreakerState = iota // normal operation
	StateOpen                                // tripped, rejecting requests
	StateHalfOpen                            // probing after cooldown
)

// CircuitBreaker implements a sliding-window error-rate circuit breaker
// with Closed → Open → HalfOpen → Closed state transitions.
type CircuitBreaker struct {
	mu              sync.Mutex
	state           CircuitBreakerState
	failures        []time.Time // timestamps of failures within the window
	successes       []time.Time // timestamps of successes within the window
	lastStateChange time.Time

	errorThreshold float64       // fraction of errors to trip (e.g. 0.5 = 50%)
	windowSize     time.Duration // sliding window duration
	cooldownPeriod time.Duration // time to wait in Open before probing
}

// NewCircuitBreaker creates a circuit breaker with production defaults:
// errorThreshold=0.5, windowSize=60s, cooldownPeriod=30s.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:           StateClosed,
		failures:        []time.Time{},
		successes:       []time.Time{},
		lastStateChange: time.Now(),
		errorThreshold:  0.5,
		windowSize:      60 * time.Second,
		cooldownPeriod:  30 * time.Second,
	}
}

// prune removes entries older than now-windowSize from both slices.
// Must be called with mu held.
func (cb *CircuitBreaker) prune(now time.Time) {
	cutoff := now.Add(-cb.windowSize)

	kept := cb.failures[:0]
	for _, t := range cb.failures {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	cb.failures = kept

	kept = cb.successes[:0]
	for _, t := range cb.successes {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	cb.successes = kept
}

// Allow reports whether a request should be forwarded to the provider.
// Open → HalfOpen transition happens here after cooldown elapses.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastStateChange) >= cb.cooldownPeriod {
			cb.state = StateHalfOpen
			cb.lastStateChange = time.Now()
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful request. Transitions HalfOpen → Closed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.prune(now)
	cb.successes = append(cb.successes, now)

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failures = cb.failures[:0]
		cb.lastStateChange = now
	}
}

// RecordFailure records a failed request. May trip Closed → Open or HalfOpen → Open.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.prune(now)
	cb.failures = append(cb.failures, now)

	switch cb.state {
	case StateHalfOpen:
		cb.state = StateOpen
		cb.lastStateChange = now
	case StateClosed:
		f := len(cb.failures)
		s := len(cb.successes)
		if total := f + s; total > 0 {
			rate := float64(f) / float64(total)
			if rate > cb.errorThreshold {
				cb.state = StateOpen
				cb.lastStateChange = now
			}
		}
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

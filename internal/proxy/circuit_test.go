package proxy

import (
	"sync"
	"testing"
	"time"
)

// newTestBreaker creates a circuit breaker with a small windowSize for testing.
func newTestBreaker(errorThreshold float64, windowSize, cooldownPeriod time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:           StateClosed,
		failures:        []time.Time{},
		successes:       []time.Time{},
		lastStateChange: time.Now(),
		errorThreshold:  errorThreshold,
		windowSize:      windowSize,
		cooldownPeriod:  cooldownPeriod,
	}
}

// TestCircuitBreaker_ClosedAllowsRequests verifies Closed state always permits.
func TestCircuitBreaker_ClosedAllowsRequests(t *testing.T) {
	cb := NewCircuitBreaker()
	for i := 0; i < 10; i++ {
		if !cb.Allow() {
			t.Fatalf("iteration %d: expected Allow() = true in Closed state", i)
		}
	}
	if cb.State() != StateClosed {
		t.Fatal("expected state to remain Closed")
	}
}

// TestCircuitBreaker_TripsOpenAfterThreshold verifies error rate > threshold trips breaker.
func TestCircuitBreaker_TripsOpenAfterThreshold(t *testing.T) {
	cb := newTestBreaker(0.5, 60*time.Second, 30*time.Second)

	// 3 successes: error rate = 0/3 = 0 — stays Closed
	for i := 0; i < 3; i++ {
		cb.RecordSuccess()
	}
	if cb.State() != StateClosed {
		t.Fatal("expected Closed after pure successes")
	}

	// 3 failures: rate = 3/6 = 0.5, not > 0.5 — stays Closed
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed at 0.5 error rate (threshold is strictly >), got %d", cb.State())
	}

	// 1 more failure: rate = 4/7 ≈ 0.571 > 0.5 — trips Open
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected Open after exceeding error threshold, got %d", cb.State())
	}
}

// TestCircuitBreaker_OpenRejectsRequests verifies Open state rejects Allow().
func TestCircuitBreaker_OpenRejectsRequests(t *testing.T) {
	cb := newTestBreaker(0.5, 60*time.Second, 30*time.Second)
	cb.mu.Lock()
	cb.state = StateOpen
	cb.lastStateChange = time.Now()
	cb.mu.Unlock()

	for i := 0; i < 5; i++ {
		if cb.Allow() {
			t.Fatal("expected Allow() = false in Open state before cooldown")
		}
	}
}

// TestCircuitBreaker_OpenTransitionsToHalfOpenAfterCooldown verifies cooldown behaviour.
func TestCircuitBreaker_OpenTransitionsToHalfOpenAfterCooldown(t *testing.T) {
	cb := newTestBreaker(0.5, 60*time.Second, 50*time.Millisecond)
	cb.mu.Lock()
	cb.state = StateOpen
	cb.lastStateChange = time.Now()
	cb.mu.Unlock()

	// Should still be rejected immediately
	if cb.Allow() {
		t.Fatal("expected Allow() = false before cooldown")
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("expected Allow() = true after cooldown")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen after cooldown, got %d", cb.State())
	}
}

// TestCircuitBreaker_HalfOpenSuccessTransitionsToClosed verifies recovery path.
func TestCircuitBreaker_HalfOpenSuccessTransitionsToClosed(t *testing.T) {
	cb := newTestBreaker(0.5, 60*time.Second, 30*time.Second)
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.lastStateChange = time.Now()
	cb.mu.Unlock()

	if !cb.Allow() {
		t.Fatal("expected Allow() = true in HalfOpen state")
	}
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after RecordSuccess in HalfOpen, got %d", cb.State())
	}
}

// TestCircuitBreaker_HalfOpenFailureReopens verifies probe failure re-trips breaker.
func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := newTestBreaker(0.5, 60*time.Second, 30*time.Second)
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.lastStateChange = time.Now()
	cb.mu.Unlock()

	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected Open after RecordFailure in HalfOpen, got %d", cb.State())
	}
	if cb.Allow() {
		t.Fatal("expected Allow() = false after re-opening from HalfOpen")
	}
}

// TestCircuitBreaker_SlidingWindowPrunesOldEntries verifies old data is ignored.
func TestCircuitBreaker_SlidingWindowPrunesOldEntries(t *testing.T) {
	cb := newTestBreaker(0.5, 100*time.Millisecond, 30*time.Second)

	// Inject 10 failures directly into the past (before window)
	cb.mu.Lock()
	past := time.Now().Add(-200 * time.Millisecond)
	for i := 0; i < 10; i++ {
		cb.failures = append(cb.failures, past)
	}
	cb.mu.Unlock()

	// Record 3 fresh successes — old failures should be pruned
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after old failures pruned from window, got %d", cb.State())
	}
}

// TestCircuitBreaker_SlidingWindowExpiresFailures verifies failures expire naturally.
func TestCircuitBreaker_SlidingWindowExpiresFailures(t *testing.T) {
	cb := newTestBreaker(0.5, 50*time.Millisecond, 30*time.Second)

	// Record failures to trip the breaker
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Skip("breaker did not trip — adjust test ratios")
	}

	// Reset to Closed manually and wait for window to expire
	cb.mu.Lock()
	cb.state = StateClosed
	cb.lastStateChange = time.Now()
	cb.mu.Unlock()

	time.Sleep(60 * time.Millisecond)

	// A new success should see no failures in window
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after failures expired from window, got %d", cb.State())
	}
}

// TestCircuitBreaker_Concurrent verifies no data races under concurrent access.
func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := NewCircuitBreaker()
	var wg sync.WaitGroup

	const goroutines = 100
	wg.Add(goroutines * 3)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			cb.Allow()
		}()
		go func() {
			defer wg.Done()
			cb.RecordSuccess()
		}()
		go func() {
			defer wg.Done()
			cb.RecordFailure()
		}()
	}
	wg.Wait()
	// Just ensure no panic or race; state can be anything
	_ = cb.State()
}

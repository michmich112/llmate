package proxy

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ActiveRequest describes one in-flight proxy request for admin observability.
type ActiveRequest struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	Endpoint  string    `json:"endpoint"`
	StartedAt time.Time `json:"started_at"`
	Remote    string    `json:"remote,omitempty"`
}

// ActiveRegistry is an in-memory registry of in-flight proxy requests.
// It is safe for concurrent use.
type ActiveRegistry struct {
	mu     sync.RWMutex
	active map[string]ActiveRequest
}

// NewActiveRegistry returns an empty registry.
func NewActiveRegistry() *ActiveRegistry {
	return &ActiveRegistry{active: make(map[string]ActiveRequest)}
}

// Begin registers a new in-flight request and returns its tracking ID.
func (r *ActiveRegistry) Begin(model, endpoint, remote string) string {
	id := uuid.New().String()
	r.mu.Lock()
	r.active[id] = ActiveRequest{
		ID: id, Model: model, Endpoint: endpoint, StartedAt: time.Now().UTC(), Remote: remote,
	}
	r.mu.Unlock()
	return id
}

// End deregisters the in-flight request with the given tracking ID.
func (r *ActiveRegistry) End(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	delete(r.active, id)
	r.mu.Unlock()
}

// Count returns the number of in-flight requests.
func (r *ActiveRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.active)
}

// Snapshot returns a copy of all in-flight requests.
func (r *ActiveRegistry) Snapshot() []ActiveRequest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ActiveRequest, 0, len(r.active))
	for _, a := range r.active {
		out = append(out, a)
	}
	return out
}

package httpx

import (
	"net/http"
	"sync"
	"time"
)

// PooledClient owns a shared *http.Client whose Transport can be replaced when
// idle-connection policy changes. Swapping does not interrupt active RoundTrips;
// CloseIdleConnections on the previous transport only drops pooled idle sockets.
type PooledClient struct {
	mu     sync.Mutex
	client *http.Client
}

func newTransport(idleConnTimeout time.Duration) *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     idleConnTimeout,
	}
}

// NewPooledClient returns a client backed by a transport with the given IdleConnTimeout.
func NewPooledClient(idleConnTimeout time.Duration) *PooledClient {
	return &PooledClient{
		client: &http.Client{
			Transport: newTransport(idleConnTimeout),
		},
	}
}

// Client returns the shared HTTP client used for outbound requests.
func (p *PooledClient) Client() *http.Client {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.client
}

// ApplyIdleConnTimeout replaces the transport with one using the new IdleConnTimeout.
// In-flight requests continue on their existing connections; only idle pooled
// connections on the old transport are closed.
func (p *PooledClient) ApplyIdleConnTimeout(idleConnTimeout time.Duration) {
	next := newTransport(idleConnTimeout)
	p.mu.Lock()
	prev := p.client.Transport
	p.client.Transport = next
	p.mu.Unlock()
	if t, ok := prev.(*http.Transport); ok && t != nil {
		t.CloseIdleConnections()
	}
}

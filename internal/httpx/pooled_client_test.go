package httpx

import (
	"net/http"
	"testing"
	"time"
)

func TestPooledClient_ApplyIdleConnTimeout(t *testing.T) {
	p := NewPooledClient(90 * time.Second)
	c := p.Client()
	if c == nil {
		t.Fatal("nil client")
	}
	t1 := c.Transport.(*http.Transport)
	if t1.IdleConnTimeout != 90*time.Second {
		t.Fatalf("initial idle: got %v", t1.IdleConnTimeout)
	}

	p.ApplyIdleConnTimeout(120 * time.Second)
	t2 := c.Transport.(*http.Transport)
	if t2 == t1 {
		t.Fatal("expected new transport instance")
	}
	if t2.IdleConnTimeout != 120*time.Second {
		t.Fatalf("after apply: got %v", t2.IdleConnTimeout)
	}
}

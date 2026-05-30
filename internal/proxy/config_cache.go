package proxy

import (
	"context"
	"sync/atomic"

	"github.com/llmate/gateway/internal/db"
)

type ConfigSnapshot struct {
	store db.Store
	val   atomic.Pointer[map[string]string]
}

func NewConfigSnapshot(store db.Store) *ConfigSnapshot {
	c := &ConfigSnapshot{store: store}
	empty := map[string]string{}
	c.val.Store(&empty)
	return c
}

func (c *ConfigSnapshot) Reload(ctx context.Context) error {
	cfg, err := c.store.GetAllConfig(ctx)
	if err != nil {
		return err
	}
	c.val.Store(&cfg)
	return nil
}

func (c *ConfigSnapshot) Get() map[string]string {
	m := c.val.Load()
	if m == nil {
		return map[string]string{}
	}
	return *m
}

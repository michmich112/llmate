package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/llmate/gateway/internal/models"
)

func newFileTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteStore_PragmasApplyToAllPooledConnections(t *testing.T) {
	store := newFileTestStore(t)
	ctx := context.Background()

	const n = 4
	conns := make([]*sql.Conn, n)
	t.Cleanup(func() {
		for _, c := range conns {
			if c != nil {
				c.Close()
			}
		}
	})

	for i := 0; i < n; i++ {
		c, err := store.db.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn %d: %v", i, err)
		}
		conns[i] = c

		var timeout int
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout); err != nil {
			t.Fatalf("PRAGMA busy_timeout conn %d: %v", i, err)
		}
		if timeout != 5000 {
			t.Errorf("conn %d busy_timeout=%d, want 5000", i, timeout)
		}

		var fk int
		if err := c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatalf("PRAGMA foreign_keys conn %d: %v", i, err)
		}
		if fk != 1 {
			t.Errorf("conn %d foreign_keys=%d, want 1", i, fk)
		}
	}
}

func TestSQLiteStore_ConcurrentConfigAndRequestLogWrites(t *testing.T) {
	store := newFileTestStore(t)
	ctx := context.Background()

	const n = 40
	var wg sync.WaitGroup
	errCh := make(chan error, n*2)

	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := store.SetConfig(ctx, "require_api_keys", "true"); err != nil {
				errCh <- err
			}
		}()
		go func() {
			defer wg.Done()
			now := time.Now().UTC()
			log := &models.RequestLog{
				ID: uuid.NewString(), Timestamp: now, ClientIP: "127.0.0.1",
				Method: "POST", Path: "/v1/chat/completions", StatusCode: 502,
				CreatedAt: now,
			}
			if err := store.InsertRequestLog(ctx, log); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent write: %v", err)
	}
}

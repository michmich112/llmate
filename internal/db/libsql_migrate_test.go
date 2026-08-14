package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/llmate/gateway/internal/models"
)


func seedLegacyProvider(t *testing.T, path, id, name string) {
	t.Helper()
	st, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	p := &models.Provider{
		ID:          id,
		Name:        name,
		BaseURL:     "https://api.example.com",
		IsHealthy:   true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := st.CreateProvider(t.Context(), p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
}

// Verifies NewLibSQLStore migrates a legacy sqlite db into a fresh empty target.
func TestNewLibSQLStoreMigratesLegacy(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.db")
	target := filepath.Join(dir, "target.db")

	seedLegacyProvider(t, legacy, "p1", "alpha")

	st, err := NewLibSQLStore(target, legacy)
	if err != nil {
		t.Fatalf("NewLibSQLStore: %v", err)
	}
	defer st.Close()

	providers, err := st.ListProviders(t.Context())
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(providers) != 1 || providers[0].Name != "alpha" {
		t.Fatalf("expected migrated provider alpha, got %+v", providers)
	}
}

// Verifies opening the libsql driver at the SAME path as an existing LLMate
// sqlite file preserves data (a local libsql file IS the sqlite file).
func TestNewLibSQLStoreSamePathPreservesData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.db")

	seedLegacyProvider(t, path, "p1", "alpha")

	// Legacy path == same file -> migration is a no-op; data must survive.
	st, err := NewLibSQLStore(path, path)
	if err != nil {
		t.Fatalf("NewLibSQLStore: %v", err)
	}
	defer st.Close()

	providers, err := st.ListProviders(t.Context())
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(providers) != 1 || providers[0].Name != "alpha" {
		t.Fatalf("expected provider alpha preserved, got %+v", providers)
	}
}

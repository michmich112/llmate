package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateLegacySQLite(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.db")
	target := filepath.Join(dir, "target.db")

	// Seed a legacy sqlite database with schema + data.
	src, err := sql.Open("sqlite", "file:"+legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	mustExec(t, src, `CREATE TABLE IF NOT EXISTS providers (id TEXT PRIMARY KEY, name TEXT NOT NULL)`)
	mustExec(t, src, `CREATE TABLE IF NOT EXISTS requests (id TEXT PRIMARY KEY, total_time_ms INTEGER)`)
	mustExec(t, src, `INSERT INTO providers (id, name) VALUES ('p1', 'alpha')`)
	mustExec(t, src, `INSERT INTO providers (id, name) VALUES ('p2', 'beta')`)
	mustExec(t, src, `INSERT INTO requests (id, total_time_ms) VALUES ('r1', 123)`)

	// Open an empty target libsql db and migrate.
	tgt, err := sql.Open("sqlite", "file:"+target)
	if err != nil {
		t.Fatal(err)
	}
	defer tgt.Close()

	if err := migrateLegacySQLite(tgt, legacy); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify data landed in target.
	var n int
	if err := tgt.QueryRow(`SELECT COUNT(*) FROM providers`).Scan(&n); err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 providers, got %d", n)
	}
	var name string
	if err := tgt.QueryRow(`SELECT name FROM providers WHERE id='p2'`).Scan(&name); err != nil {
		t.Fatalf("select provider: %v", err)
	}
	if name != "beta" {
		t.Fatalf("expected beta, got %q", name)
	}
	var ms int
	if err := tgt.QueryRow(`SELECT total_time_ms FROM requests WHERE id='r1'`).Scan(&ms); err != nil {
		t.Fatalf("select request: %v", err)
	}
	if ms != 123 {
		t.Fatalf("expected 123, got %d", ms)
	}

	// Running again must be a no-op (target already has tables).
	if err := migrateLegacySQLite(tgt, legacy); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestMigrateLegacySQLiteNoSource(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.db")
	tgt, err := sql.Open("sqlite", "file:"+target)
	if err != nil {
		t.Fatal(err)
	}
	defer tgt.Close()

	// Missing legacy file -> no-op.
	if err := migrateLegacySQLite(tgt, filepath.Join(dir, "missing.db")); err != nil {
		t.Fatalf("migrate with missing source: %v", err)
	}

	// Empty legacyPath -> no-op.
	if err := migrateLegacySQLite(tgt, ""); err != nil {
		t.Fatalf("migrate with empty legacy: %v", err)
	}
}

func mustExec(t *testing.T, db *sql.DB, sql string) {
	t.Helper()
	if _, err := db.Exec(sql); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

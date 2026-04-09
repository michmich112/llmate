package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/llmate/gateway/internal/models"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// SQLiteStore is the SQLite-backed implementation of Store.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens a SQLite database at dbPath, applies PRAGMAs, and runs migrations.
// Use ":memory:" for an in-memory database (e.g. tests).
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// _time_format=sqlite instructs modernc.org/sqlite to store time.Time values
	// in "2006-01-02 15:04:05.999999999-07:00" format, which SQLite's strftime()
	// and datetime() functions can parse natively without any substr workarounds.
	dsn := dbPath
	if dbPath == ":memory:" {
		dsn = "file::memory:?_time_format=sqlite"
	} else {
		dsn = "file:" + dbPath + "?_time_format=sqlite"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite allows only one concurrent writer; cap connections to prevent locking.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// WAL mode is not meaningful for in-memory databases.
	if dbPath != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("set WAL journal mode: %w", err)
		}
	}

	if err := runMigrationsWithLog(db, func(msg, name string) {
		switch msg {
		case "apply":
			slog.Info("migration applied", "name", name)
		case "skip":
			slog.Debug("migration already applied", "name", name)
		}
	}); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

// runMigrations applies any unapplied *.up.sql migration files in sorted order.
// Applied migrations are tracked in the _migrations table so each file runs exactly once.
// Migration 0001 uses CREATE TABLE IF NOT EXISTS throughout, making it safe to re-run
// on existing databases that predate the tracking table.
func runMigrations(db *sql.DB) error { return runMigrationsWithLog(db, nil) }

func runMigrationsWithLog(db *sql.DB, logf func(msg, name string)) error {
	if logf == nil {
		logf = func(_, _ string) {}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		name       TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Collect *.up.sql files and sort them to guarantee order.
	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		var cnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM _migrations WHERE name = ?`, name).Scan(&cnt); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if cnt > 0 {
			logf("skip", name)
			continue
		}

		data, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		for _, stmt := range strings.Split(string(data), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			// Skip statement segments that are pure comments (all lines start with --).
			allComment := true
			for _, line := range strings.Split(stmt, "\n") {
				if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "--") {
					allComment = false
					break
				}
			}
			if allComment {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback() //nolint:errcheck
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO _migrations (name, applied_at) VALUES (?, ?)`, name, time.Now().UTC()); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
		logf("apply", name)
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- helpers ---

// nullStr returns nil for empty strings so they're stored as NULL.
func nullStr(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// nullInt converts *int to a SQL-compatible nullable int64.
func nullInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return int64(*v)
}

// nullFloat64 converts *float64 to a SQL-compatible nullable value.
func nullFloat64(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// nullTime converts *time.Time to a SQL-compatible nullable value.
func nullTime(v *time.Time) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// nullTimeScanner scans a nullable SQLite time column into *time.Time.
// sql.NullTime cannot be used here because modernc.org/sqlite returns string
// driver values for columns it did not write itself (e.g. data migrated from
// older formats), and sql.NullTime.Scan delegates to convertAssign which does
// not convert strings to time.Time.
type nullTimeScanner struct {
	Time  time.Time
	Valid bool
}

// timeFormats lists every format that may appear in the database, newest first.
var timeFormats = []string{
	"2006-01-02 15:04:05.999999999 -07:00", // space before tz offset (written by modernc with _time_format=sqlite)
	"2006-01-02 15:04:05.999999999-07:00",  // no space before tz offset
	"2006-01-02 15:04:05 -07:00",           // no fractional seconds, space before tz
	"2006-01-02 15:04:05-07:00",            // no fractional seconds, no space
	"2006-01-02 15:04:05",                  // no tz at all
	time.RFC3339Nano,
	time.RFC3339,
}

func (n *nullTimeScanner) Scan(value any) error {
	if value == nil {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		n.Time, n.Valid = v, true
		return nil
	case string:
		for _, f := range timeFormats {
			t, err := time.Parse(f, v)
			if err == nil {
				n.Time, n.Valid = t, true
				return nil
			}
		}
		return fmt.Errorf("cannot parse time %q", v)
	case []byte:
		return n.Scan(string(v))
	default:
		return fmt.Errorf("unsupported driver type for time scan: %T", value)
	}
}

// timeScanner is like nullTimeScanner but for non-nullable time columns.
// It exists for the same reason: modernc.org/sqlite returns historical rows
// (written before _time_format=sqlite was in the DSN) as plain strings, and
// database/sql's convertAssign cannot convert strings to time.Time.
type timeScanner struct {
	Time time.Time
}

func (t *timeScanner) Scan(value any) error {
	var n nullTimeScanner
	if err := n.Scan(value); err != nil {
		return err
	}
	t.Time = n.Time
	return nil
}

// scanProvider reads a Provider from any Scan func (row.Scan or rows.Scan).
func scanProvider(scan func(...any) error) (models.Provider, error) {
	var p models.Provider
	var apiKey sql.NullString
	var healthCheckedAt nullTimeScanner
	var createdAt, updatedAt timeScanner
	err := scan(
		&p.ID, &p.Name, &p.BaseURL, &apiKey,
		&p.IsHealthy, &healthCheckedAt,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return models.Provider{}, err
	}
	if apiKey.Valid {
		p.APIKey = apiKey.String
	}
	if healthCheckedAt.Valid {
		t := healthCheckedAt.Time
		p.HealthCheckedAt = &t
	}
	p.CreatedAt = createdAt.Time
	p.UpdatedAt = updatedAt.Time
	return p, nil
}


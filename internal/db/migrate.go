package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// migrateLegacySQLite copies all tables, indexes, and data from a legacy SQLite
// database file into target. It is used when switching the default storage from
// sqlite to libsql/Turso: on the first boot against an empty Turso database, an
// existing sqlite file is migrated so no data is lost.
//
// It is a no-op when target already has user tables (already migrated) or the
// legacy file does not exist.
func migrateLegacySQLite(target *sql.DB, legacyPath string) error {
	if legacyPath == "" {
		return nil
	}
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat legacy db %q: %w", legacyPath, err)
	}

	has, err := hasUserTables(target)
	if err != nil {
		return err
	}
	if has {
		return nil
	}

	src, err := sql.Open("sqlite", "file:"+legacyPath)
	if err != nil {
		return fmt.Errorf("open legacy db %q: %w", legacyPath, err)
	}
	defer src.Close()

	tables, err := listTables(src)
	if err != nil {
		return err
	}
	for _, t := range tables {
		if err := copyTable(target, src, t.name, t.createSQL); err != nil {
			return err
		}
	}

	indexes, err := listIndexes(src)
	if err != nil {
		return err
	}
	for _, sql := range indexes {
		if _, err := target.Exec(sql); err != nil {
			return fmt.Errorf("create index from legacy db: %w", err)
		}
	}

	return nil
}

type tableInfo struct {
	name      string
	createSQL string
}

func hasUserTables(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check target tables: %w", err)
	}
	return n > 0, nil
}

func listTables(src *sql.DB) ([]tableInfo, error) {
	rows, err := src.Query(`SELECT name, sql FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list legacy tables: %w", err)
	}
	defer rows.Close()

	var out []tableInfo
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			return nil, fmt.Errorf("scan legacy table: %w", err)
		}
		out = append(out, tableInfo{name: name, createSQL: sql})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate legacy tables: %w", err)
	}
	return out, nil
}

func listIndexes(src *sql.DB) ([]string, error) {
	rows, err := src.Query(`SELECT sql FROM sqlite_master
		WHERE type = 'index' AND sql IS NOT NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list legacy indexes: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			return nil, fmt.Errorf("scan legacy index: %w", err)
		}
		out = append(out, sql)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate legacy indexes: %w", err)
	}
	return out, nil
}

// copyTable recreates a table in target and copies all of its rows.
func copyTable(target, src *sql.DB, name, createSQL string) error {
	if _, err := target.Exec(createSQL); err != nil {
		return fmt.Errorf("create table %s in target: %w", name, err)
	}

	cols, err := src.Query("SELECT * FROM " + quoteIdent(name))
	if err != nil {
		return fmt.Errorf("select %s from legacy db: %w", name, err)
	}
	defer cols.Close()

	colNames, err := cols.Columns()
	if err != nil {
		return fmt.Errorf("columns of %s: %w", name, err)
	}
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(name),
		strings.Join(quoteAll(colNames), ", "),
		strings.TrimSuffix(strings.Repeat("?, ", len(colNames)), ", "),
	)

	values := make([]any, len(colNames))
	scanArgs := make([]any, len(colNames))
	for i := range scanArgs {
		scanArgs[i] = &values[i]
	}
	for cols.Next() {
		if err := cols.Scan(scanArgs...); err != nil {
			return fmt.Errorf("scan row of %s: %w", name, err)
		}
		if _, err := target.Exec(insertSQL, values...); err != nil {
			return fmt.Errorf("insert row into %s: %w", name, err)
		}
	}
	if err := cols.Err(); err != nil {
		return fmt.Errorf("iterate rows of %s: %w", name, err)
	}
	return nil
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = quoteIdent(n)
	}
	return out
}

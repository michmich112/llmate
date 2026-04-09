package db

import (
    "context"
    "fmt"
    "time"
)

func (s *SQLiteStore) GetAllConfig(ctx context.Context) (map[string]string, error) {
    rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM config`)
    if err != nil {
        return nil, fmt.Errorf("get all config: %w", err)
    }
    defer rows.Close()
    result := make(map[string]string)
    for rows.Next() {
        var k, v string
        if err := rows.Scan(&k, &v); err != nil {
            return nil, fmt.Errorf("scan config row: %w", err)
        }
        result[k] = v
    }
    return result, rows.Err()
}

func (s *SQLiteStore) SetConfig(ctx context.Context, key, value string) error {
    _, err := s.db.ExecContext(ctx,
        `INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
         ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
        key, value, time.Now().UTC(),
    )
    if err != nil {
        return fmt.Errorf("set config %s: %w", key, err)
    }
    return nil
}

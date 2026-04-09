package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/llmate/gateway/internal/models"
)

const providerCols = `id, name, base_url, api_key, is_healthy, health_checked_at, created_at, updated_at`

func (s *SQLiteStore) CreateProvider(ctx context.Context, p *models.Provider) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO providers (id, name, base_url, api_key, is_healthy, health_checked_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.BaseURL, nullStr(p.APIKey),
		p.IsHealthy, nullTime(p.HealthCheckedAt), p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetProvider(ctx context.Context, id string) (*models.Provider, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+providerCols+` FROM providers WHERE id = ?`, id,
	)
	p, err := scanProvider(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get provider: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return &p, nil
}

func (s *SQLiteStore) ListProviders(ctx context.Context) ([]models.Provider, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+providerCols+` FROM providers ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var providers []models.Provider
	for rows.Next() {
		p, err := scanProvider(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list providers scan: %w", err)
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list providers rows: %w", err)
	}
	return providers, nil
}

func (s *SQLiteStore) UpdateProvider(ctx context.Context, p *models.Provider) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE providers SET name = ?, base_url = ?, api_key = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.BaseURL, nullStr(p.APIKey), p.UpdatedAt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update provider: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update provider rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update provider: no rows for id %s", p.ID)
	}
	return nil
}

func (s *SQLiteStore) DeleteProvider(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete provider begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `DELETE FROM providers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete provider rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete provider: no rows for id %s", id)
	}
	return tx.Commit()
}

func (s *SQLiteStore) UpdateProviderHealth(ctx context.Context, id string, healthy bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE providers SET is_healthy = ?, health_checked_at = ? WHERE id = ?`,
		healthy, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update provider health: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update provider health rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update provider health: no rows for id %s", id)
	}
	return nil
}

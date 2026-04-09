package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/llmate/gateway/internal/models"
)

const endpointCols = `id, provider_id, path, method, is_supported, is_enabled, created_at`

func (s *SQLiteStore) UpsertProviderEndpoints(ctx context.Context, providerID string, eps []models.ProviderEndpoint) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert provider endpoints begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM provider_endpoints WHERE provider_id = ?`, providerID); err != nil {
		return fmt.Errorf("upsert provider endpoints delete: %w", err)
	}
	for _, ep := range eps {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO provider_endpoints (id, provider_id, path, method, is_supported, is_enabled, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			ep.ID, providerID, ep.Path, ep.Method, ep.IsSupported, ep.IsEnabled, ep.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("upsert provider endpoints insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("upsert provider endpoints commit: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListProviderEndpoints(ctx context.Context, providerID string) ([]models.ProviderEndpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+endpointCols+`
		 FROM provider_endpoints
		 WHERE provider_id = ?
		 ORDER BY path ASC, method ASC`,
		providerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list provider endpoints: %w", err)
	}
	defer rows.Close()

	var eps []models.ProviderEndpoint
	for rows.Next() {
		var ep models.ProviderEndpoint
		var createdAt timeScanner
		if err := rows.Scan(&ep.ID, &ep.ProviderID, &ep.Path, &ep.Method, &ep.IsSupported, &ep.IsEnabled, &createdAt); err != nil {
			return nil, fmt.Errorf("list provider endpoints scan: %w", err)
		}
		ep.CreatedAt = createdAt.Time
		eps = append(eps, ep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list provider endpoints rows: %w", err)
	}
	return eps, nil
}

func (s *SQLiteStore) UpdateProviderEndpoint(ctx context.Context, ep *models.ProviderEndpoint) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE provider_endpoints SET is_enabled = ? WHERE id = ?`,
		ep.IsEnabled, ep.ID,
	)
	if err != nil {
		return fmt.Errorf("update provider endpoint: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update provider endpoint rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update provider endpoint: no rows for id %s", ep.ID)
	}
	return nil
}

func (s *SQLiteStore) GetEnabledEndpoint(ctx context.Context, providerID string, path string) (*models.ProviderEndpoint, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+endpointCols+`
		 FROM provider_endpoints
		 WHERE provider_id = ? AND path = ? AND is_supported = 1 AND is_enabled = 1
		 ORDER BY method ASC LIMIT 1`,
		providerID, path,
	)
	var ep models.ProviderEndpoint
	var createdAt timeScanner
	err := row.Scan(&ep.ID, &ep.ProviderID, &ep.Path, &ep.Method, &ep.IsSupported, &ep.IsEnabled, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get enabled endpoint: %w", err)
	}
	ep.CreatedAt = createdAt.Time
	return &ep, nil
}

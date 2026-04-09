package db

import (
	"context"
	"fmt"

	"github.com/llmate/gateway/internal/models"
)

const aliasCols = `id, alias, provider_id, model_id, weight, priority, is_enabled, created_at, updated_at`

func (s *SQLiteStore) CreateAlias(ctx context.Context, a *models.ModelAlias) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO model_aliases (id, alias, provider_id, model_id, weight, priority, is_enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Alias, a.ProviderID, a.ModelID, a.Weight, a.Priority, a.IsEnabled, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create alias: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListAliases(ctx context.Context) ([]models.ModelAlias, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+aliasCols+` FROM model_aliases ORDER BY alias ASC, priority DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	defer rows.Close()

	var aliases []models.ModelAlias
	for rows.Next() {
		var a models.ModelAlias
		var createdAt, updatedAt timeScanner
		if err := rows.Scan(&a.ID, &a.Alias, &a.ProviderID, &a.ModelID, &a.Weight, &a.Priority, &a.IsEnabled, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("list aliases scan: %w", err)
		}
		a.CreatedAt = createdAt.Time
		a.UpdatedAt = updatedAt.Time
		aliases = append(aliases, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list aliases rows: %w", err)
	}
	return aliases, nil
}

func (s *SQLiteStore) UpdateAlias(ctx context.Context, a *models.ModelAlias) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE model_aliases SET weight = ?, priority = ?, is_enabled = ?, updated_at = ? WHERE id = ?`,
		a.Weight, a.Priority, a.IsEnabled, a.UpdatedAt, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update alias: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update alias rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update alias: no rows for id %s", a.ID)
	}
	return nil
}

func (s *SQLiteStore) DeleteAlias(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM model_aliases WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alias: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ResolveAlias(ctx context.Context, alias string) ([]models.ModelAlias, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+aliasCols+`
		 FROM model_aliases
		 WHERE alias = ? AND is_enabled = 1
		 ORDER BY priority DESC`,
		alias,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve alias: %w", err)
	}
	defer rows.Close()

	aliases := []models.ModelAlias{}
	for rows.Next() {
		var a models.ModelAlias
		var createdAt, updatedAt timeScanner
		if err := rows.Scan(&a.ID, &a.Alias, &a.ProviderID, &a.ModelID, &a.Weight, &a.Priority, &a.IsEnabled, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("resolve alias scan: %w", err)
		}
		a.CreatedAt = createdAt.Time
		a.UpdatedAt = updatedAt.Time
		aliases = append(aliases, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve alias rows: %w", err)
	}
	return aliases, nil
}

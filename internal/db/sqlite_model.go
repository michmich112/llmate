package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/llmate/gateway/internal/models"
)

const providerModelCols = `id, provider_id, model_id, created_at,
	cost_per_million_input, cost_per_million_output,
	cost_per_million_cache_read, cost_per_million_cache_write`

func scanProviderModel(scan func(...any) error) (models.ProviderModel, error) {
	var m models.ProviderModel
	var costIn, costOut, costCacheRead, costCacheWrite sql.NullFloat64
	var createdAt timeScanner
	err := scan(
		&m.ID, &m.ProviderID, &m.ModelID, &createdAt,
		&costIn, &costOut, &costCacheRead, &costCacheWrite,
	)
	if err != nil {
		return models.ProviderModel{}, err
	}
	m.CreatedAt = createdAt.Time
	if costIn.Valid {
		m.CostPerMillionInput = &costIn.Float64
	}
	if costOut.Valid {
		m.CostPerMillionOutput = &costOut.Float64
	}
	if costCacheRead.Valid {
		m.CostPerMillionCacheRead = &costCacheRead.Float64
	}
	if costCacheWrite.Valid {
		m.CostPerMillionCacheWrite = &costCacheWrite.Float64
	}
	return m, nil
}

// SyncProviderModels reconciles the provider's model list without touching cost columns.
// New models are inserted; models absent from modelIDs are removed.
// Existing records are left untouched so configured pricing is preserved across re-onboarding.
func (s *SQLiteStore) SyncProviderModels(ctx context.Context, providerID string, modelIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sync provider models begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().UTC()
	for _, modelID := range modelIDs {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO provider_models (id, provider_id, model_id, created_at) VALUES (?, ?, ?, ?)`,
			uuid.NewString(), providerID, modelID, now,
		)
		if err != nil {
			return fmt.Errorf("sync provider models insert: %w", err)
		}
	}

	// Build a placeholder list to delete models no longer in the set.
	if len(modelIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM provider_models WHERE provider_id = ?`, providerID); err != nil {
			return fmt.Errorf("sync provider models delete all: %w", err)
		}
	} else {
		placeholders := strings.Repeat("?,", len(modelIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]interface{}, 0, len(modelIDs)+1)
		args = append(args, providerID)
		for _, id := range modelIDs {
			args = append(args, id)
		}
		q := `DELETE FROM provider_models WHERE provider_id = ? AND model_id NOT IN (` + placeholders + `)`
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("sync provider models delete stale: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sync provider models commit: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListProviderModels(ctx context.Context, providerID string) ([]models.ProviderModel, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+providerModelCols+` FROM provider_models WHERE provider_id = ? ORDER BY model_id ASC`,
		providerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list provider models: %w", err)
	}
	defer rows.Close()

	var ms []models.ProviderModel
	for rows.Next() {
		m, err := scanProviderModel(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list provider models scan: %w", err)
		}
		ms = append(ms, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list provider models rows: %w", err)
	}
	return ms, nil
}

func (s *SQLiteStore) ListAllModels(ctx context.Context) ([]models.ProviderModel, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+providerModelCols+` FROM provider_models ORDER BY provider_id ASC, model_id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all models: %w", err)
	}
	defer rows.Close()

	var ms []models.ProviderModel
	for rows.Next() {
		m, err := scanProviderModel(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list all models scan: %w", err)
		}
		ms = append(ms, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list all models rows: %w", err)
	}
	return ms, nil
}

func (s *SQLiteStore) UpdateProviderModelCosts(ctx context.Context, id string, m *models.ProviderModel) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE provider_models
		 SET cost_per_million_input = ?, cost_per_million_output = ?,
		     cost_per_million_cache_read = ?, cost_per_million_cache_write = ?
		 WHERE id = ?`,
		nullFloat64(m.CostPerMillionInput), nullFloat64(m.CostPerMillionOutput),
		nullFloat64(m.CostPerMillionCacheRead), nullFloat64(m.CostPerMillionCacheWrite),
		id,
	)
	if err != nil {
		return fmt.Errorf("update provider model costs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update provider model costs rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update provider model costs: no rows for id %s: %w", id, sql.ErrNoRows)
	}
	return nil
}

func (s *SQLiteStore) GetProviderModelCosts(ctx context.Context, providerID, modelID string) (*models.ProviderModel, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+providerModelCols+` FROM provider_models WHERE provider_id = ? AND model_id = ?`,
		providerID, modelID,
	)
	m, err := scanProviderModel(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get provider model costs: %w", err)
	}
	return &m, nil
}

func (s *SQLiteStore) GetHealthyProvidersForModel(ctx context.Context, modelID string) ([]models.Provider, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT p.id, p.name, p.base_url, p.api_key, p.is_healthy, p.health_checked_at, p.created_at, p.updated_at
		 FROM providers p
		 INNER JOIN provider_models m ON m.provider_id = p.id
		 WHERE m.model_id = ? AND p.is_healthy = 1
		 ORDER BY p.name ASC`,
		modelID,
	)
	if err != nil {
		return nil, fmt.Errorf("get healthy providers for model: %w", err)
	}
	defer rows.Close()

	var providers []models.Provider
	for rows.Next() {
		p, err := scanProvider(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("get healthy providers for model scan: %w", err)
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get healthy providers for model rows: %w", err)
	}
	return providers, nil
}

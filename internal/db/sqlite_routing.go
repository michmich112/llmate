package db

import (
	"context"
	"fmt"

	"github.com/llmate/gateway/internal/models"
)

func (s *SQLiteStore) LoadRoutingData(ctx context.Context) (*models.RoutingData, error) {
	providers, err := s.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("load routing data providers: %w", err)
	}
	modelsList, err := s.ListAllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("load routing data models: %w", err)
	}
	aliases, err := s.ListAliases(ctx)
	if err != nil {
		return nil, fmt.Errorf("load routing data aliases: %w", err)
	}
	endpoints, err := s.listAllEndpoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("load routing data endpoints: %w", err)
	}
	return &models.RoutingData{
		Providers: providers,
		Models:    modelsList,
		Aliases:   aliases,
		Endpoints: endpoints,
	}, nil
}

func (s *SQLiteStore) listAllEndpoints(ctx context.Context) ([]models.ProviderEndpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+endpointCols+` FROM provider_endpoints ORDER BY provider_id ASC, path ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all endpoints: %w", err)
	}
	defer rows.Close()

	var eps []models.ProviderEndpoint
	for rows.Next() {
		var ep models.ProviderEndpoint
		var createdAt timeScanner
		if err := rows.Scan(&ep.ID, &ep.ProviderID, &ep.Path, &ep.Method, &ep.IsSupported, &ep.IsEnabled, &createdAt); err != nil {
			return nil, fmt.Errorf("list all endpoints scan: %w", err)
		}
		ep.CreatedAt = createdAt.Time
		eps = append(eps, ep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list all endpoints rows: %w", err)
	}
	return eps, nil
}

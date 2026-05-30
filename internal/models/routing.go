package models

// RoutingData holds a consistent snapshot of all tables needed for in-memory routing.
type RoutingData struct {
	Providers []Provider
	Models    []ProviderModel
	Aliases   []ModelAlias
	Endpoints []ProviderEndpoint
}

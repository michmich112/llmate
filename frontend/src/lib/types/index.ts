export interface Provider {
  id: string;
  name: string;
  base_url: string;
  api_key?: string;
  is_healthy: boolean;
  health_checked_at?: string;
  created_at: string;
  updated_at: string;
  /** Model IDs registered for this provider. Present in list responses. */
  models?: string[];
}

export interface ProviderEndpoint {
  id: string;
  provider_id: string;
  path: string;
  method: string;
  is_supported: boolean;
  is_enabled: boolean;
  created_at: string;
}

export interface ProviderModel {
  id: string;
  provider_id: string;
  model_id: string;
  created_at: string;
  cost_per_million_input?: number;
  cost_per_million_output?: number;
  cost_per_million_cache_read?: number;
  cost_per_million_cache_write?: number;
}

export interface ModelAlias {
  id: string;
  alias: string;
  provider_id: string;
  model_id: string;
  weight: number;
  priority: number;
  is_enabled: boolean;
  created_at: string;
  updated_at: string;
}

/** Populated on GET /admin/logs/:id when provider model pricing exists */
export interface RequestLogCostBreakdown {
  input_usd: number;
  output_usd: number;
  cached_read_usd: number;
  total_usd: number;
}

export interface RequestLog {
  id: string;
  timestamp: string;
  client_ip: string;
  method: string;
  path: string;
  requested_model?: string;
  resolved_model?: string;
  provider_id?: string;
  provider_name?: string;
  status_code: number;
  is_streamed: boolean;
  ttft_ms?: number;
  total_time_ms: number;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  cached_tokens?: number;
  error_message?: string;
  created_at: string;
  estimated_cost_usd?: number;
  cost_breakdown?: RequestLogCostBreakdown;
  request_body?: string;
  response_body?: string;
}

/** Query filter for GET /admin/logs — use ISO strings for since/until */
export interface LogFilter {
  model?: string;
  provider_id?: string;
  since?: string;
  until?: string;
  /** status: "2xx" | "4xx" | "5xx" | "error" | "success" */
  status?: string;
  limit?: number;
  offset?: number;
}

export interface DashboardStats {
  total_requests: number;
  avg_latency_ms: number;
  error_rate: number;
  by_model: ModelStats[];
  by_provider: ProviderStats[];
}

export interface ModelStats {
  model: string;
  request_count: number;
  avg_latency_ms: number;
  error_count: number;
  total_tokens: number;
}

export interface ProviderStats {
  provider_id: string;
  provider_name: string;
  request_count: number;
  avg_latency_ms: number;
  error_count: number;
}

export interface DiscoveryResult {
  models: string[];
  endpoints: DiscoveredEndpoint[];
}

export interface DiscoveredEndpoint {
  path: string;
  method: string;
  is_supported: boolean | null;
}

/** Body for POST /admin/providers/{id}/confirm */
export interface ConfirmProviderBody {
  endpoints: ConfirmEndpointInput[];
  models: string[];
}

export interface ConfirmEndpointInput {
  path: string;
  method: string;
  is_supported: boolean;
  is_enabled: boolean;
}

export interface ProviderDetailResponse {
  provider: Provider;
  endpoints: ProviderEndpoint[];
  models: ProviderModel[];
}

export interface TimeSeriesPoint {
  bucket: string;
  requests: number;
  success_count: number;
  error_count: number;
  input_tokens: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cached_tokens: number;
  total_cost_usd: number;
  input_cost_usd: number;
  output_cost_usd: number;
  cached_cost_usd: number;
}

export interface Configuration {
  request_body_max_bytes: number;
  response_body_max_bytes: number;
  track_streaming: boolean;
  streaming_buffer_size: number;
  /** Days to keep full streaming chunk bodies (raw SSE line + text delta). Min 1. */
  streaming_log_body_retention_days: number;
  /** Days to keep request_body text on each request log row. Independent of other retention settings. */
  request_log_body_retention_days: number;
  /** Days to keep response_body text on each request log row. Independent of other retention settings. */
  response_log_body_retention_days: number;
}

export interface ConfigField {
  type: 'integer' | 'boolean';
  default: number | boolean;
  min?: number;
  max?: number;
  description: string;
}

export interface ConfigDefinition {
  request_body_max_bytes: ConfigField;
  response_body_max_bytes: ConfigField;
  track_streaming: ConfigField;
  streaming_buffer_size: ConfigField;
  streaming_log_body_retention_days: ConfigField;
  request_log_body_retention_days: ConfigField;
  response_log_body_retention_days: ConfigField;
}

export interface StreamingLog {
  id: string;
  request_log_id: string;
  chunk_index: number;
  data: string;
  /** Assistant text delta parsed from this SSE payload (OpenAI-style); may be empty. */
  content_delta: string;
  /** True when chunk bodies were cleared by the retention policy. */
  body_purged: boolean;
  is_truncated: boolean;
  timestamp: string;
  created_at: string;
  /** Running total of content_delta after this chunk (computed by the API). */
  cumulative_body: string;
}

import type {
  ConfirmProviderBody,
  Configuration,
  ConfigDefinition,
  DashboardStats,
  DiscoveryResult,
  LogFilter,
  ModelAlias,
  Provider,
  ProviderDetailResponse,
  ProviderEndpoint,
  ProviderModel,
  RequestLog,
  StreamingLog,
  TimeSeriesPoint
} from '$lib/types';

class ApiClient {
  private accessKey: string | null;

  constructor() {
    if (typeof localStorage !== 'undefined') {
      this.accessKey = localStorage.getItem('access_key');
    } else {
      this.accessKey = null;
    }
  }

  setAccessKey(key: string): void {
    this.accessKey = key;
    localStorage.setItem('access_key', key);
  }

  clearAccessKey(): void {
    this.accessKey = null;
    localStorage.removeItem('access_key');
  }

  isAuthenticated(): boolean {
    return !!this.accessKey;
  }

  private headers(key?: string): Record<string, string> {
    const k = key ?? this.accessKey;
    const h: Record<string, string> = {};
    if (k) h['Authorization'] = `Bearer ${k}`;
    return h;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    authKey?: string
  ): Promise<T> {
    const headers: Record<string, string> = this.headers(authKey);
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
    }

    const res = await fetch(`/admin${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined
    });

    if (res.status === 401) {
      this.clearAccessKey();
      window.location.href = '/login';
      throw new Error('Unauthorized');
    }

    if (res.status === 204) {
      return undefined as T;
    }

    const data = await res.json().catch(() => ({}));

    if (!res.ok) {
      const msg = (data as { error?: string }).error ?? `Request failed: ${res.status}`;
      throw new Error(msg);
    }

    return data as T;
  }

  /** POST /admin/auth — returns true if response is 200 and JSON valid:true */
  async validateKey(key: string): Promise<boolean> {
    try {
      const res = await fetch('/admin/auth', {
        method: 'POST',
        headers: { Authorization: `Bearer ${key}` }
      });
      if (res.status !== 200) return false;
      const data = (await res.json()) as { valid?: boolean };
      return data.valid === true;
    } catch {
      return false;
    }
  }

  async listProviders(): Promise<Provider[]> {
    const data = await this.request<{ providers: Provider[] }>('GET', '/providers');
    return data.providers;
  }

  async createProvider(data: {
    name: string;
    base_url: string;
    api_key?: string;
  }): Promise<Provider> {
    const res = await this.request<{ provider: Provider }>('POST', '/providers', data);
    return res.provider;
  }

  async getProvider(id: string): Promise<ProviderDetailResponse> {
    return this.request<ProviderDetailResponse>('GET', `/providers/${id}`);
  }

  async updateProvider(id: string, data: Partial<Provider>): Promise<Provider> {
    const res = await this.request<{ provider: Provider }>('PUT', `/providers/${id}`, data);
    return res.provider;
  }

  async deleteProvider(id: string): Promise<void> {
    await this.request<void>('DELETE', `/providers/${id}`);
  }

  async discoverProvider(id: string): Promise<DiscoveryResult> {
    return this.request<DiscoveryResult>('POST', `/providers/${id}/discover`);
  }

  async confirmProvider(id: string, data: ConfirmProviderBody): Promise<ProviderDetailResponse> {
    return this.request<ProviderDetailResponse>('POST', `/providers/${id}/confirm`, data);
  }

  async updateEndpoint(
    providerId: string,
    endpointId: string,
    data: { is_enabled: boolean }
  ): Promise<ProviderEndpoint> {
    const res = await this.request<{ endpoint: ProviderEndpoint }>(
      'PUT',
      `/providers/${providerId}/endpoints/${endpointId}`,
      data
    );
    return res.endpoint;
  }

  async listAliases(): Promise<ModelAlias[]> {
    const data = await this.request<{ aliases: ModelAlias[] }>('GET', '/aliases');
    return data.aliases;
  }

  async createAlias(
    data: Omit<ModelAlias, 'id' | 'created_at' | 'updated_at'>
  ): Promise<ModelAlias> {
    const res = await this.request<{ alias: ModelAlias }>('POST', '/aliases', data);
    return res.alias;
  }

  async updateAlias(id: string, data: Partial<ModelAlias>): Promise<ModelAlias> {
    const res = await this.request<{ alias: ModelAlias }>('PUT', `/aliases/${id}`, data);
    return res.alias;
  }

  async deleteAlias(id: string): Promise<void> {
    await this.request<void>('DELETE', `/aliases/${id}`);
  }

  async queryLogs(filter?: Partial<LogFilter>): Promise<{ logs: RequestLog[]; total: number }> {
    const params = new URLSearchParams();
    if (filter) {
      for (const [k, v] of Object.entries(filter)) {
        if (v !== undefined) params.set(k, String(v));
      }
    }
    const qs = params.toString() ? `?${params.toString()}` : '';
    return this.request<{ logs: RequestLog[]; total: number }>('GET', `/logs${qs}`);
  }

  /** since: duration string e.g. 24h, 7d — passed as query param */
  async getStats(since?: string): Promise<DashboardStats> {
    const qs = since ? `?since=${encodeURIComponent(since)}` : '';
    return this.request<DashboardStats>('GET', `/stats${qs}`);
  }

  /** Returns time-bucketed usage metrics. since e.g. "24h", granularity "hour"|"day" */
  async getTimeSeries(
    since: string,
    granularity: 'hour' | 'day'
  ): Promise<{ points: TimeSeriesPoint[] }> {
    const params = new URLSearchParams({ since, granularity });
    return this.request<{ points: TimeSeriesPoint[] }>('GET', `/stats/timeseries?${params}`);
  }

  /** Returns a single request log including request/response bodies. */
  async getLog(id: string): Promise<{ log: RequestLog }> {
    return this.request<{ log: RequestLog }>('GET', `/logs/${encodeURIComponent(id)}`);
  }

  /** Updates per-million-token cost fields for a provider model record. */
  async updateProviderModel(
    providerId: string,
    modelRecordId: string,
    costs: Pick<
      ProviderModel,
      | 'cost_per_million_input'
      | 'cost_per_million_output'
      | 'cost_per_million_cache_read'
      | 'cost_per_million_cache_write'
    >
  ): Promise<{ models: ProviderModel[] }> {
    return this.request<{ models: ProviderModel[] }>(
      'PUT',
      `/providers/${providerId}/models/${modelRecordId}`,
      costs
    );
  }

  async getConfig(): Promise<Configuration> {
    return this.request<Configuration>('GET', '/config');
  }

  async updateConfig(config: Partial<Configuration>): Promise<Configuration> {
    return this.request<Configuration>('PUT', '/config', config);
  }

  async getConfigDefinition(): Promise<ConfigDefinition> {
    return this.request<ConfigDefinition>('GET', '/config/definition');
  }

  async getStreamingLogs(requestLogId: string): Promise<StreamingLog[]> {
    const data = await this.request<{ streaming_logs: StreamingLog[] }>(
      'GET',
      `/logs/${encodeURIComponent(requestLogId)}/streaming`
    );
    return data.streaming_logs;
  }
}

export const api = new ApiClient();

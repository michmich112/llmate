# Spec 02: Proxy Handlers (Phase 1B)

## Goal

Implement OpenAI-compatible HTTP handlers that receive requests, delegate routing to the smart router (from Phase 1C), forward requests to the selected backend, and capture metrics. Includes full Server-Sent Events (SSE) streaming passthrough with **TTFT** (time to first token) measurement.

The implementing agent must follow this document only. Do not depend on reading `Context.md`, other specs, or external design notes.

---

## Files to Create

| # | Path | Purpose |
|---|------|---------|
| 1 | `internal/proxy/handler.go` | Handler struct, constructor, all eight endpoint handlers, JSON/multipart helpers, non-streaming proxy with failover, response helpers |
| 2 | `internal/proxy/streaming.go` | SSE streaming passthrough, `UsageInfo`, `proxyStreaming`, stream option injection |
| 3 | `internal/proxy/handler_test.go` | Table-driven and integration-style tests with mocked Router, MetricsCollector, and Store |

---

## Package

```go
package proxy
```

Imports may include: `bufio`, `bytes`, `context`, `encoding/json`, `errors`, `fmt`, `io`, `mime/multipart`, `net/http`, `net/http/httptest`, `strings`, `time`, `github.com/google/uuid`, and project packages `internal/db`, `internal/models`.

---

## Dependencies (interfaces and types this package uses)

This package depends on **interfaces**, not concrete implementations. Define the following **locally** in `handler.go` (or a small `types.go` in the same package if you prefer—still only the three files listed above).

### Router (implemented by Phase 1C: `internal/proxy/router.go`)

```go
// Router selects a healthy provider and builds the backend URL for a given model and OpenAI endpoint path.
type Router interface {
    Route(ctx context.Context, modelID string, endpointPath string) (*RouteResult, error)
    ReportSuccess(providerID string)
    ReportFailure(providerID string)
}

type RouteResult struct {
    Provider  models.Provider
    ModelID   string // resolved model ID after alias resolution
    TargetURL string // full URL: provider.BaseURL + endpointPath (see URL construction below)
}
```

**URL construction:** `TargetURL` must be an absolute URL. Normalize `provider.BaseURL` (trim trailing `/`) and `endpointPath` (ensure leading `/`), then concatenate: `base + path`. Example: `https://api.openai.com` + `/v1/chat/completions` → `https://api.openai.com/v1/chat/completions`.

**Circuit reporting:** Call `ReportSuccess(providerID)` after a **successful** completion of a request (non-streaming: 2xx after body read; streaming: after stream completes normally). Call `ReportFailure(providerID)` on backend errors, timeouts, or non-retryable failures as specified below.

### MetricsCollector (async request logging)

```go
// MetricsCollector persists request logs asynchronously (e.g. channel + worker). Must not block the proxy hot path for long.
type MetricsCollector interface {
    Record(log *models.RequestLog)
}
```

The handler builds a `*models.RequestLog` per request and calls `Record` **once** per logical client request (after the final attempt for non-streaming, or after stream end / error for streaming).

### Store (subset used by this package)

Use the real `db.Store` type from `internal/db`. The handler only needs:

```go
// Required Store methods for proxy handlers:
//   ListAllModels(ctx context.Context) ([]models.ProviderModel, error)
//   ListAliases(ctx context.Context) ([]models.ModelAlias, error)
//   GetProvider(ctx context.Context, id string) (*models.Provider, error)  // optional if you resolve provider names for logs only via RouteResult
```

If `GetProvider` is unavailable, populate `ProviderName` in logs from `RouteResult.Provider.Name` when available.

---

## Domain Types (inline reference — copy into mental model; actual structs live in `internal/models`)

These definitions match the contracts phase. **Use the types from `internal/models` in code**, not re-declared duplicates.

### `models.Provider`

```go
type Provider struct {
    ID              string     `json:"id"`
    Name            string     `json:"name"`
    BaseURL         string     `json:"base_url"`
    APIKey          string     `json:"api_key,omitempty"`
    IsHealthy       bool       `json:"is_healthy"`
    HealthCheckedAt *time.Time `json:"health_checked_at,omitempty"`
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
}
```

### `models.ProviderModel`

```go
type ProviderModel struct {
    ID         string    `json:"id"`
    ProviderID string    `json:"provider_id"`
    ModelID    string    `json:"model_id"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### `models.ModelAlias`

```go
type ModelAlias struct {
    ID         string    `json:"id"`
    Alias      string    `json:"alias"`
    ProviderID string    `json:"provider_id"`
    ModelID    string    `json:"model_id"`
    Weight     int       `json:"weight"`
    Priority   int       `json:"priority"`
    IsEnabled  bool      `json:"is_enabled"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

### `models.RequestLog`

```go
type RequestLog struct {
    ID               string    `json:"id"`
    Timestamp        time.Time `json:"timestamp"`
    ClientIP         string    `json:"client_ip"`
    Method           string    `json:"method"`
    Path             string    `json:"path"`
    RequestedModel   string    `json:"requested_model,omitempty"`
    ResolvedModel    string    `json:"resolved_model,omitempty"`
    ProviderID       string    `json:"provider_id,omitempty"`
    ProviderName     string    `json:"provider_name,omitempty"`
    StatusCode       int       `json:"status_code"`
    IsStreamed       bool      `json:"is_streamed"`
    TTFTMs           *int      `json:"ttft_ms,omitempty"`
    TotalTimeMs      int       `json:"total_time_ms"`
    PromptTokens     *int      `json:"prompt_tokens,omitempty"`
    CompletionTokens *int      `json:"completion_tokens,omitempty"`
    TotalTokens      *int      `json:"total_tokens,omitempty"`
    CachedTokens     *int      `json:"cached_tokens,omitempty"`
    ErrorMessage     string    `json:"error_message,omitempty"`
    CreatedAt        time.Time `json:"created_at"`
}
```

**Log population rules:**

- Generate `ID` with `uuid.New().String()`.
- `Timestamp` and `CreatedAt`: use `time.Now().UTC()` (or consistent single instant at end of request).
- `ClientIP`: from `r.RemoteAddr` or `X-Forwarded-For` first hop if you implement trusted proxy parsing; minimum is `r.RemoteAddr`.
- `RequestedModel`: model string from the incoming request (alias or id as sent by client).
- `ResolvedModel`: `RouteResult.ModelID` from the successful route.
- `ProviderID` / `ProviderName`: from `RouteResult.Provider`.
- `IsStreamed`: `true` for SSE chat/completions streams.
- `TTFTMs`: set only for streaming when first non-done `data:` line is observed; see Streaming section.
- Token fields: pointers; set when usage is parsed (`nil` if unknown).
- `ErrorMessage`: non-empty on handler/gateway errors; optional on upstream 4xx/5xx if you copy upstream body snippet (keep short).

---

## Proxy Endpoints to Handle

| Method | Path | Handler Function | Streaming | Body Type |
|--------|------|------------------|-----------|-----------|
| POST | `/v1/chat/completions` | `HandleChatCompletions` | Yes, if JSON body contains `"stream": true` | JSON |
| POST | `/v1/completions` | `HandleCompletions` | Yes, if `"stream": true` | JSON |
| POST | `/v1/embeddings` | `HandleEmbeddings` | No | JSON |
| POST | `/v1/images/generations` | `HandleImageGenerations` | No | JSON |
| POST | `/v1/audio/speech` | `HandleAudioSpeech` | No | JSON; **binary response** |
| POST | `/v1/audio/transcriptions` | `HandleAudioTranscriptions` | No | `multipart/form-data` |
| GET | `/v1/models` | `HandleListModels` | No | N/A |
| GET | `/v1/models/{model}` | `HandleGetModel` | No | N/A |

**Path parameter:** Chi router style `{model}` may include URL-encoded characters; decode with `path.Unescape` or chi’s URLParam after routing.

Register these on the application router in `cmd/gateway` (outside this spec), but **export** handler methods so wiring is trivial.

---

## Handler Struct

```go
type Handler struct {
    router  Router
    metrics MetricsCollector
    store   db.Store
    client  *http.Client
}

func NewHandler(router Router, metrics MetricsCollector, store db.Store, client *http.Client) *Handler
```

**HTTP client:** If `client == nil`, use `http.DefaultClient` or a sensible default with timeouts (e.g. no global zero-timeout for production; for tests, injected client is fine). Document in code that production wiring should set timeouts.

---

## Constants

```go
const maxProxyAttempts = 3 // includes the first attempt; i.e. up to 2 failovers after failure
```

Apply **only** to non-streaming JSON and multipart flows where failover is specified. **Do not** retry after streaming has started.

---

## Helper Functions (required signatures)

```go
func respondJSON(w http.ResponseWriter, status int, data interface{})
func respondError(w http.ResponseWriter, status int, msg string)
func extractModelFromJSON(body []byte) (string, error)
func extractModelFromMultipart(r *http.Request) (string, error)
func injectStreamOptions(body []byte) ([]byte, error)
```

### Behavior

- **`respondJSON`:** `Content-Type: application/json`, `json.NewEncoder(w).Encode(data)`, ignore encode error or log internally; prefer Encode error not to panic.
- **`respondError`:** JSON body `{"error":"<msg>"}` (match existing project convention if `respondError` exists elsewhere; this spec uses a string field `error`).
- **`extractModelFromJSON`:** Decode `body` as JSON into a minimal struct `struct { Model string \`json:"model"\` }` or `map[string]json.RawMessage` + check `model` key. Return error if `model` missing or empty.
- **`extractModelFromMultipart`:** Call `r.ParseMultipartForm` with a reasonable max memory (e.g. 32 << 20), read form field `model`. Return error if missing or empty. Do not consume the body in a way that prevents rebuilding the outbound request—see Multipart section.
- **`injectStreamOptions`:** Parse `body` as JSON object, set nested key `stream_options` to `{"include_usage": true}`. If `stream_options` already exists as object, merge: ensure `include_usage` is true without removing other keys. Return minified or stable JSON bytes. If body is not a JSON object, return error.

---

## JSON Proxy Flow (Non-Streaming)

Applies to: chat/completions with `stream: false` or absent, completions, embeddings, images/generations.

1. **Read body:** `io.ReadAll(r.Body)`; close original body.
2. **Extract model:** `extractModelFromJSON(body)`.
3. **Detect stream:** For chat and completions only, parse `stream` boolean from body (if `"stream": true`, delegate to Streaming flow—do not use this subsection).
4. **Loop `attempt` from 1 to `maxProxyAttempts`:**
   - `route, err := router.Route(ctx, model, endpointPath)` where `endpointPath` is the OpenAI path for this handler (e.g. `/v1/chat/completions`).
   - On `Route` error: if last attempt, return `500` with error JSON; else continue (or return if non-retryable—router may return fatal errors; treat router errors as non-failover if they are “no provider” vs transient—**minimal rule:** if `Route` fails every time, respond `400` or `503` with message).
   - Build backend request: `http.NewRequestWithContext(r.Context(), http.MethodPost, route.TargetURL, bytes.NewReader(body))`.
   - **Headers:** Copy safe headers from incoming request: `Content-Type`, `Accept`, `User-Agent` (optional), `OpenAI-*` if present. **Do not** copy `Host`, `Connection`, `Content-Length` (set from body). Set `Content-Length` automatically via `NewRequest` + body or use `GetBody` for retries—**for retries, buffer body in memory** so each attempt gets a fresh reader.
   - **Authorization:** If `route.Provider.APIKey != ""`, set `Authorization: Bearer <APIKey>`. If the client sent `Authorization` and provider has no key, forward client’s header (optional product choice); **this spec:** prefer provider key when set, else forward incoming `Authorization` if present.
   - Execute `h.client.Do(req)`.
   - **On transport error / timeout:** `router.ReportFailure(route.Provider.ID)`; if `attempt < maxProxyAttempts`, continue loop; else return `502` with message.
   - **On HTTP response:** Read resp body into buffer for metrics parsing when JSON; for large responses still need full body to forward—stream through for memory: you may `io.Copy` to client while teeing for usage parse, or read all then write (acceptable for Phase 1B).
   - **Status:** Forward upstream `status code` to client unless you normalize only for gateway bugs.
   - **Headers:** Copy `Content-Type` and relevant headers from backend; omit hop-by-hop.
   - **Body:** Write response body to client.
   - **Success criteria:** HTTP status `< 500` and not context canceled → `ReportSuccess(providerID)`.
   - **Failure criteria:** Status `>= 500` or network error → `ReportFailure`, retry if attempts remain.
   - **4xx from backend:** Do **not** failover (client error); `ReportFailure` optional (product: can ReportSuccess for “upstream rejected”—**this spec:** `ReportFailure` on 5xx only, neither on 4xx for circuit—simplest: `ReportSuccess` only on 2xx, `ReportFailure` on 5xx and network errors).
   - **Clarified reporting rule:**
     - `ReportSuccess(providerID)` when backend returns **2xx**.
     - `ReportFailure(providerID)` when **5xx**, **timeout**, **connection error**, or **stream error** mid-flight (streaming).
     - **4xx:** do not count as success for circuit if your router treats failures—minimal approach: **no ReportSuccess/Failure for 4xx** (only 2xx success, 5xx/network failure) to avoid skewing circuit.

5. **Usage parsing (2xx JSON):** After reading body, `json.Unmarshal` into a struct with nested `usage` if present:

```go
type usageOnly struct {
    Usage *struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
        // Cached tokens: OpenAI uses prompt_tokens_details.cached_tokens in some responses; accept optional:
        PromptTokensDetails *struct {
            CachedTokens int `json:"cached_tokens"`
        } `json:"prompt_tokens_details,omitempty"`
    } `json:"usage"`
}
```

Map `CachedTokens` from `usage` field if present in your OpenAI sample; if only `prompt_tokens_details.cached_tokens` exists, map that to `RequestLog.CachedTokens`.

6. **Metrics:** Build `RequestLog` with timing: `start := time.Now()` at handler entry, `totalTimeMs := int(time.Since(start).Milliseconds())`. Call `metrics.Record(log)`.

7. **Failover:** Only on **5xx** or **network/timeout** from backend. **Not** on 4xx. **Not** after bytes written to client (non-streaming: only failover before `WriteHeader` on client—so read backend fully before writing to client when possible).

**Order of operations (non-streaming):** Complete backend request fully, then write status/headers/body to client once, then `Record` metrics. For failover, do not write anything to client until success or final failure.

---

## SSE Streaming Flow

Applies when `stream: true` for `HandleChatCompletions` or `HandleCompletions`.

1. Read body buffer; extract model via `extractModelFromJSON`.
2. `body = injectStreamOptions(body)`; on error respond `400`.
3. **Single routing attempt for stream:** Call `router.Route`. If error, respond `503`/`400` appropriately—**no multi-attempt loop for stream** in Phase 1B unless first `Route` fails before any write (you may retry `Route` only if no response started—acceptable to reuse same `maxProxyAttempts` **only until** `WriteHeader` on client; once streaming starts, **no failover**).

   **Recommended:** Use up to `maxProxyAttempts` **only** until a successful `http.Response` from backend with `2xx` and body opened. If backend returns 5xx, retry new `Route` + new request. Once `proxyStreaming` begins copying lines, **do not** failover.

4. Build POST to `TargetURL` with modified body; set headers as non-streaming; include SSE-related `Accept: text/event-stream` if client sent it.
5. `client.Do`; if failure before writing to client, `ReportFailure` and retry per above.
6. Validate: `resp.StatusCode == 200` (or accept 2xx—OpenAI uses 200). If not 2xx, read body snippet, `ReportFailure`, try next attempt or return error.
7. Set client response headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no` (optional, helps nginx).
8. `Flush` headers via `http.Flusher`.
9. Call `proxyStreaming(w, resp, startTime)` which copies SSE data and returns usage + TTFT.

10. **Metrics:** `IsStreamed: true`, set `TTFTMs` from return value, token fields from parsed usage, `ReportSuccess` on normal completion, `ReportFailure` on mid-stream error.

### TTFT (Time To First Token)

- **Start clock:** same `startTime` as passed into `proxyStreaming` (handler start or first byte read—**spec:** use handler entry `startTime` for consistency with non-streaming `TotalTimeMs`).
- **First token event:** The first SSE line that begins with `data:` whose content is not `[DONE]` after trim. Parse line as `data: {...}`; if JSON object with `choices` and delta content, or any non-empty payload, record `TTFTMs = int(time.Since(startTime).Milliseconds())` **once**.
- **Ignore:** comment lines (`:`), empty lines, `event:` lines until first qualifying `data:` line.

### Usage in Streams

OpenAI sends `usage` in the **final** JSON chunk before `data: [DONE]` when `include_usage` is true. Parse each `data:` line as JSON; if object contains `"usage":{...}`, extract token counts. The last chunk with usage wins.

After `data: [DONE]` line, stop reading; finalize metrics.

### Mid-Stream Errors

If `Read` fails or context canceled: close client connection if possible; `ReportFailure`; do not send second response.

---

## streaming.go

### Types

```go
type UsageInfo struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
    CachedTokens     int `json:"cached_tokens"`
}
```

### Method

```go
func (h *Handler) proxyStreaming(w http.ResponseWriter, backendResp *http.Response, startTime time.Time) (usage *UsageInfo, ttftMs *int, err error)
```

**Requirements:**

- Assert `backendResp.Body != nil`; defer close body.
- Obtain `http.Flusher` from `w`; if not flusher, return error (streaming impossible).
- Use `bufio.Scanner` on `backendResp.Body` with **increased buffer** (`bufio.MaxScanTokenSize` default may be too small). Set `scanner.Buffer(make([]byte, 64*1024), 1024*1024)` or similar.
- **Line splitting:** Split on `\n`; handle `\r\n` by trimming `\r`.
- For each line, write to `w` exactly as read (preserve SSE format), then `Flush()`.
- **TTFT:** On first `data:` line where trimmed payload is not `[DONE]` and not empty, set `ttftMs` pointer to elapsed ms from `startTime`.
- **Usage:** When a `data:` line parses as JSON and contains `usage`, set `usage` struct (last occurrence wins before `[DONE]`).
- **Done:** When line is `data: [DONE]` (allow whitespace), return nil error after flushing.
- **Errors:** Scanner err → return wrapped error.

**Note:** Do not buffer entire stream in memory.

---

## Per-Endpoint Notes

### `HandleEmbeddings` / `HandleImageGenerations`

Same as JSON non-streaming; `endpointPath` is `/v1/embeddings` or `/v1/images/generations`. Parse usage from JSON response on 2xx.

### `HandleAudioSpeech`

- JSON body; extract `model`.
- Forward request; response is **binary** (e.g. `audio/mpeg`).
- **Do not** JSON-parse response body.
- Copy `Content-Type` from backend if present.
- Metrics: `PromptTokens`, `CompletionTokens`, etc. remain `nil` unless you add optional parsing (not required).
- `TotalTimeMs` still recorded. `ReportSuccess`/`Failure` same as HTTP rules.

### `HandleAudioTranscriptions`

- `r.ParseMultipartForm`; extract `model` field via `extractModelFromMultipart`.
- Rebuild multipart request to backend: copy all **file** parts and **fields** with same field names and file names. Use `multipart.Writer` to construct new body; set `Content-Type` with boundary.
- Forward response as-is (often JSON). Parse usage if JSON 2xx.

### `HandleListModels`

- No `Route` call.
- `models, err := store.ListAllModels(ctx)`; `aliases, err := store.ListAliases(ctx)`.
- Build unique set of string IDs:
  - For each `ProviderModel`, include `ModelID`.
  - For each `ModelAlias` where `IsEnabled`, include `Alias` (the alias string).
- Sort for stable output (alphabetical by `id`).
- Response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 0,
      "owned_by": "llmate"
    }
  ]
}
```

### `HandleGetModel`

- Path param `model` = requested id (URL-decoded).
- Build the same list as `HandleListModels` (reuse internal helper `listModelObjects()` returning slice of structs).
- Find entry where `id` matches; if found `respondJSON(200, obj)` with single object shape:

```json
{
  "id": "gpt-4o",
  "object": "model",
  "created": 0,
  "owned_by": "llmate"
}
```

- If not found: `404` with `respondError`.

---

## Security and Robustness

- **Body size:** Consider `http.MaxBytesReader` on incoming requests (e.g. 32 MiB) for JSON routes to avoid OOM.
- **Context:** Propagate `r.Context()` to backend requests for cancellation.
- **Hop-by-hop headers:** Strip `Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailers`, `Transfer-Encoding`, `Upgrade` when forwarding.

---

## Testing (`handler_test.go`)

Use `httptest.NewServer` for fake backends, `httptest.NewRecorder` for gateway responses. Implement mock `Router`, `MetricsCollector`, and `db.Store` (minimal methods only).

### Required test cases

1. **Non-streaming chat completion:** Mock router returns fixed `TargetURL` pointing at `httptest` server returning 200 JSON with `usage`. Assert response body matches, status 200, `Record` called with expected token fields and provider IDs, `ReportSuccess` once.

2. **Streaming chat completion:** Backend streams SSE lines including final chunk with `usage` and `data: [DONE]`. Assert client receives lines in order, `TTFTMs` non-nil and positive, usage recorded, `IsStreamed` true.

3. **`extractModelFromJSON`:** Valid body, missing model (error), empty model (error).

4. **`extractModelFromMultipart`:** Form with `model` and file field; verify extraction.

5. **Backend error failover:** First backend returns 503, second returns 200. Mock router returns different URLs or same mock with call count. Assert success after retry and `ReportFailure` then `ReportSuccess` (or two failures + success depending on mock).

6. **`HandleListModels`:** Store returns predefined models and aliases; assert JSON `data` length and ids include both model ids and alias names.

7. **`injectStreamOptions`:** Input JSON without `stream_options` produces `include_usage: true`; input with existing `stream_options` merges without dropping keys.

8. **Binary passthrough (`HandleAudioSpeech`):** Backend returns `audio/mpeg` bytes; client response matches bytes, `Content-Type` preserved, metrics `Record` called without JSON parse panic.

### Mocks

- **Router:** `Route` returns configurable results; counters for `ReportSuccess` / `ReportFailure`.
- **MetricsCollector:** Append `[]*models.RequestLog` in slice for assertions (mutex-protected).

Run: `go test ./internal/proxy/...`

---

## Done Criteria

- [ ] All **8** handler functions implemented (`HandleChatCompletions`, `HandleCompletions`, `HandleEmbeddings`, `HandleImageGenerations`, `HandleAudioSpeech`, `HandleAudioTranscriptions`, `HandleListModels`, `HandleGetModel`).
- [ ] SSE streaming with **TTFT** measurement works and `stream_options` injection requests `include_usage` for final chunk parsing.
- [ ] **Failover** on backend **5xx** / network error up to **3** attempts for **non-streaming** (and for streaming **before** response bytes sent to client, if implemented).
- [ ] **Metrics** (`Record`) called for every request with timing; token fields when available.
- [ ] Tests pass for the key flows listed above.
- [ ] `go build ./internal/proxy/...` succeeds.

---

## File Layout Summary

| File | Contents |
|------|----------|
| `handler.go` | Interfaces `Router`, `MetricsCollector`; `RouteResult`; `Handler`, `NewHandler`; eight `Handle*` methods; `respondJSON`, `respondError`; `extractModelFromJSON`, `extractModelFromMultipart`, `injectStreamOptions` (or re-export from streaming if split); non-streaming proxy loop with failover; multipart rebuild; models list helpers |
| `streaming.go` | `UsageInfo`, `proxyStreaming`, buffer sizing, SSE line forwarding |
| `handler_test.go` | Mocks + tests |

This completes Spec 02 for Phase 1B.

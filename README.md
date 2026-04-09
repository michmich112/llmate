# LLMate

**LLMate** (*LLM available to everyone*) is a self-hosted gateway that sits between your applications and local or private LLM backends (vLLM, Ollama, llama.cpp, and similar). It exposes an **OpenAI-compatible HTTP API**, so clients and SDKs built for OpenAI can point at LLMate with minimal or no code changes.

---

## Features

- **OpenAI-compatible API** — Proxy routes such as chat completions, embeddings, images, and audio match what OpenAI-style clients expect (`/v1/*`).
- **Smart routing** — Model aliases map friendly names to real provider models. Multiple backends can share an alias with **weights** and **priority**; the gateway picks a healthy target and fails over when needed.
- **Circuit breaker per provider** — Transient failures open the circuit, reduce load on bad nodes, and allow half-open retries instead of a single global “up/down” flag.
- **Background health checks** — Providers are probed on a configurable interval so routing decisions use recent health state.
- **Admin dashboard** — A Svelte 5 web UI (embedded in the binary) for onboarding providers, editing aliases, and reviewing configuration.
- **Request analytics** — Each proxied request is logged in SQLite with routing, timing, token usage, status, and errors. **Request and response body text** can be stored **truncated** (defaults are modest, e.g. tens of KB per side) with limits and retention controlled in **Settings**; streaming responses can optionally record reconstructed text and per-chunk SSE data. This is for operations/debugging—tune or zero out limits if you want metadata-heavy logs only.
- **Single binary** — The Go server embeds the built frontend; one process, no separate static host required. **Docker** images are supported for production-style deployments.

---

## Quick start

### Prerequisites

- **Go** 1.25+ (for running or developing the gateway)
- **Node.js** 20+ (for frontend development; the Dockerfile uses Node for production builds)

### Run from source

From the repository root, the **Makefile** builds the Svelte app, copies it into `cmd/gateway/frontend_dist/` for `embed`, and compiles a **single binary** at `bin/gateway`.

```bash
git clone https://github.com/michmich112/llmate.git
cd llmate

ACCESS_KEY=change-me make run
```

`ACCESS_KEY` is required for the dashboard and `/admin/*`. For a quick local run without a prior `make build`, `make dev-backend` runs `go run` with a dev key (see `Makefile`).

Open `http://localhost:8080` and sign in with your `ACCESS_KEY` to use the dashboard. Point OpenAI-compatible clients at `http://localhost:8080` for the proxy API.

### Run with Docker

Build and run locally:

```bash
docker build -t llmate:local .
docker run --rm -p 8080:8080 \
  -e ACCESS_KEY="change-me" \
  -v llmate-data:/app/data \
  -e DB_PATH=/app/data/llmate.db \
  llmate:local
```

Published images (if your fork or upstream publishes them) are typically pulled by version tag from a container registry; see your repository’s **Releases** and registry README for exact image names.

---

## Configuration (environment)

All configuration is via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ACCESS_KEY` | **Yes** | — | Secret used for the admin UI and `/admin/*` API (`Authorization: Bearer …`). |
| `PORT` | No | `8080` | HTTP listen port. |
| `DB_PATH` | No | `./llmate.db` | SQLite database file path. Use a mounted volume in Docker (e.g. under `/app/data`). |
| `HEALTH_INTERVAL` | No | `30s` | How often to run health checks against providers. |
| `LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, or `error`. |
| `MAX_BODY_SIZE` | No | `10485760` | Maximum request body size in bytes (default 10 MiB). |

Create a `.env` file for local development if your tooling loads it; do **not** commit secrets. The proxy routes (`/v1/*`) are intended for a **trusted network**; only admin routes enforce `ACCESS_KEY`.

---

## Development

```bash
make ci          # npm ci + frontend check, go test ./..., then make build
make test        # Go tests only
make test-frontend
make dev-backend # terminal 1: gateway with ACCESS_KEY=dev-key
make dev-frontend # terminal 2: Vite dev server (see frontend tooling for API proxy)
```

See `Context.md` in the repository for architecture, data model, and request flow in more detail.

---

## License

See the repository’s `LICENSE` file (if present) for terms.

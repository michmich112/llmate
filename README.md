# LLMate

**LLMate** (*LLM available to everyone*) is a self-hosted gateway that sits between your applications and local or private LLM backends (vLLM, Ollama, llama.cpp, and similar). It exposes an **OpenAI-compatible HTTP API**, so clients and SDKs built for OpenAI can point at LLMate with minimal or no code changes.

---

## Features

- **OpenAI-compatible API** — Proxy routes such as chat completions, embeddings, images, and audio match what OpenAI-style clients expect (`/v1/*`).
- **Smart routing** — Model aliases map friendly names to real provider models. Multiple backends can share an alias with **weights** and **priority**; the gateway picks a healthy target and fails over when needed.
- **Circuit breaker per provider** — Transient failures open the circuit, reduce load on bad nodes, and allow half-open retries instead of a single global “up/down” flag.
- **Background health checks** — Providers are probed on a configurable interval so routing decisions use recent health state.
- **Admin dashboard** — A Svelte 5 web UI (embedded in the binary) for onboarding providers, editing aliases, and reviewing configuration.
- **Request analytics** — Proxied calls are logged as **metadata only** (no request/response bodies): timing, tokens, routing, and errors, stored in SQLite for inspection via the admin API.
- **Single binary** — The Go server embeds the built frontend; one process, no separate static host required. **Docker** images are supported for production-style deployments.

---

## Quick start

### Prerequisites

- **Go** 1.25+ (for running or developing the gateway)
- **Node.js** 20+ (for frontend development; the Dockerfile uses Node for production builds)

### Run from source

```bash
git clone https://github.com/<your-org>/llmate.git
cd llmate

# Build the dashboard (output goes to frontend/build, then copied for embed)
cd frontend && npm ci && npm run build && cd ..
mkdir -p cmd/gateway/frontend_dist
cp -r frontend/build/* cmd/gateway/frontend_dist/

# Run the gateway (ACCESS_KEY is required)
export ACCESS_KEY="change-me"
export PORT=8080          # optional, default 8080
export DB_PATH=./llmate.db # optional

go run ./cmd/gateway
```

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
# Backend
go test ./...
go build -o bin/gateway ./cmd/gateway

# Frontend (separate dev server; API calls proxied per project setup)
cd frontend && npm ci && npm run dev
```

```bash
cd frontend && npm run check   # Svelte/TS checks
```

See `Context.md` in the repository for architecture, data model, and request flow in more detail.

---

## Releases and CI

Pushes to `main` run tests, bump a **semantic version** from the latest `v*` tag and the **latest commit message** (see below), build a Docker image, push `:<version>`, `:v<version>`, and `:latest` to Docker Hub, attach a compressed `docker save` archive to a GitHub Release, and publish the release.

**Version bumps (conventional-style, first line of the commit message):**

| Pattern | Bump |
|--------|------|
| `BREAKING CHANGE:` in the message body, or `type!:` / `scope!:` on the first line | **major** |
| `feat:` or `feat(scope):` | **minor** |
| Anything else (including `fix:`, `chore:`, or merge commits like `Merge pull request …`) | **patch** |

Squash merges that use a conventional first line (for example `feat: add provider discovery`) behave as expected. Plain merge commits usually produce a **patch** bump unless you edit the merge message.

**Repository secrets for Docker Hub:**

- `DOCKERHUB_USERNAME` — Docker Hub user or organization name (image is `<username>/llmate`).
- `DOCKERHUB_TOKEN` — [access token](https://docs.docker.com/security/for-developers/access-tokens/) (recommended; do not use your account password in CI).

To use a different image name than `llmate`, change the `DOCKER_IMAGE_NAME` env var in `.github/workflows/release.yml`.

---

## License

See the repository’s `LICENSE` file (if present) for terms.

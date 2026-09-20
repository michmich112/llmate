#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
DB_PATH="${DB_PATH:-/tmp/llmate-e2e.db}"
PORT="${PORT:-8099}"
ACCESS_KEY="${ACCESS_KEY:-e2e-key}"
rm -f "$DB_PATH" "$DB_PATH-wal" "$DB_PATH-shm" 2>/dev/null || true
if [ ! -f bin/gateway ] || [ ! -f cmd/gateway/frontend_dist/index.html ]; then
  (cd frontend && npm run build >/dev/null 2>&1)
  rm -rf cmd/gateway/frontend_dist
  mkdir -p cmd/gateway/frontend_dist
  cp -r frontend/build/* cmd/gateway/frontend_dist/
  (cd cmd/gateway && go build -o ../../bin/gateway .)
fi
pkill -f "bin/gateway" 2>/dev/null || true
sleep 0.2
exec env ACCESS_KEY="$ACCESS_KEY" DB_DRIVER=sqlite DB_PATH="$DB_PATH" PORT="$PORT" ./bin/gateway

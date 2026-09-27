#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

# Start the gateway (SPA + admin API) in the background.
bash frontend/e2e/start-gateway.sh &
GW_PID=$!
trap 'kill "$GW_PID" 2>/dev/null || true; pkill -f "bin/gateway" 2>/dev/null || true' EXIT

# Wait for gateway readiness on 8099.
code=""
for _ in $(seq 1 120); do
  code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:8099/" || true)"
  if [ "$code" = "200" ]; then break; fi
  sleep 1
done
if [ "$code" != "200" ]; then
  echo "gateway failed to start on 8099" >&2
  exit 1
fi

cd frontend
npx cucumber-js \
  --import "e2e/cucumber/**/*.mjs" \
  --format "html:e2e/cucumber-report.html" \
  --format progress \
  --exit \
  "e2e/features/**/*.feature"

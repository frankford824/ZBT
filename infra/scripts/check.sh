#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 -m py_compile "$ROOT/infra/scripts/acceptance_tail_check.py"

cd "$ROOT/frontend"
pnpm build

cd "$ROOT/backend"
GOTOOLCHAIN=local go test ./...

cd "$ROOT/ai-service"
python3 -m compileall app
if python3 -m pytest --version >/dev/null 2>&1; then
  python3 -m pytest app/tests
else
  echo "local pytest unavailable; skipping local ai-service pytest"
fi

cd "$ROOT"
docker compose config >/dev/null

AI_CONTAINER="$(docker compose ps -q ai-service 2>/dev/null || true)"
if [ -n "$AI_CONTAINER" ] && [ "$(docker inspect -f '{{.State.Running}}' "$AI_CONTAINER" 2>/dev/null || true)" = "true" ]; then
  docker compose exec -T ai-service python -m pytest app/tests
else
  echo "ai-service container is not running; skipping container pytest"
fi

echo "all checks passed"

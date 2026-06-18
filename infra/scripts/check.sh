#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 -m py_compile \
  "$ROOT/infra/scripts/acceptance_core_check.py" \
  "$ROOT/infra/scripts/acceptance_tail_check.py"

cd "$ROOT/frontend"
pnpm build
pnpm lint

cd "$ROOT/backend"
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go vet ./...

cd "$ROOT/ai-service"
AI_PYTHON="python3"
if [ -x "$ROOT/ai-service/.venv/bin/python" ]; then
  AI_PYTHON="$ROOT/ai-service/.venv/bin/python"
fi
"$AI_PYTHON" -m compileall app
if "$AI_PYTHON" -m ruff --version >/dev/null 2>&1; then
  "$AI_PYTHON" -m ruff check app
else
  echo "local ruff unavailable; skipping ai-service ruff"
fi
if "$AI_PYTHON" -m pytest --version >/dev/null 2>&1; then
  "$AI_PYTHON" -m pytest app/tests -q -s
else
  echo "local pytest unavailable; skipping local ai-service pytest"
fi

cd "$ROOT"
if command -v docker >/dev/null 2>&1 && docker version >/dev/null 2>&1; then
  docker compose config >/dev/null
  AI_CONTAINER="$(docker compose ps -q ai-service 2>/dev/null || true)"
  if [ -n "$AI_CONTAINER" ] && [ "$(docker inspect -f '{{.State.Running}}' "$AI_CONTAINER" 2>/dev/null || true)" = "true" ]; then
    docker compose exec -T ai-service python -m pytest app/tests
  else
    echo "ai-service container is not running; skipping container pytest"
  fi
else
  echo "docker unavailable or not connected to this WSL distro; skipping docker compose config and container pytest"
fi

echo "all checks passed"

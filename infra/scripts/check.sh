#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

cd "$ROOT/frontend"
pnpm build

cd "$ROOT/backend"
GOTOOLCHAIN=local go test ./...

cd "$ROOT/ai-service"
python3 -m compileall app

cd "$ROOT"
docker compose config >/dev/null

echo "all checks passed"

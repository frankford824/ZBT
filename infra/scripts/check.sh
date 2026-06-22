#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 -m py_compile \
  "$ROOT/infra/scripts/acceptance_core_check.py" \
  "$ROOT/infra/scripts/acceptance_project1_check.py" \
  "$ROOT/infra/scripts/first_usable_release_check.py" \
  "$ROOT/infra/scripts/first_usable_release_report.py" \
  "$ROOT/infra/scripts/acceptance_tail_check.py"
python3 "$ROOT/infra/scripts/acceptance_tail_check.py" --static-docs
python3 "$ROOT/infra/scripts/first_usable_release_check.py"

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
"$AI_PYTHON" -m ruff --version >/dev/null 2>&1 || {
  echo "ai-service ruff unavailable; install ai-service dev dependencies" >&2
  exit 1
}
"$AI_PYTHON" -m ruff check app
"$AI_PYTHON" -m pytest --version >/dev/null 2>&1 || {
  echo "ai-service pytest unavailable; install ai-service dev dependencies" >&2
  exit 1
}
"$AI_PYTHON" -m pytest app/tests -q -s
"$AI_PYTHON" -m app.evaluation.provider_canary_eval --allow-skip
"$AI_PYTHON" -m app.evaluation.ocr_provider_eval \
  --provider "${OCR_PROVIDER:-http_ocr}" \
  --sample "$ROOT/docs/ex/工程1/采购文件桥梁检查.pdf" \
  --repo-root "$ROOT" \
  --min-text-chars 20 \
  --min-table-blocks 1 \
  --min-layout-bbox-count 1 \
  --min-table-bbox-count 1 \
  --min-cell-bbox-count 1 \
  --allow-skip
"$AI_PYTHON" -m app.evaluation.tender_parse_eval \
  --golden "$ROOT/docs/sample_docs/golden/工程1.parse.json"
"$AI_PYTHON" -m app.evaluation.generation_coverage_eval \
  --input "$ROOT/docs/sample_docs/golden/工程1.generation_coverage.json"
"$AI_PYTHON" -m app.evaluation.export_format_eval \
  --input "$ROOT/docs/sample_docs/golden/工程1.export.json"

cd "$ROOT"
if command -v docker >/dev/null 2>&1 && docker version >/dev/null 2>&1; then
  docker compose config >/dev/null
  AI_CONTAINER="$(docker compose ps -q ai-service 2>/dev/null || true)"
  if [ -n "$AI_CONTAINER" ] && [ "$(docker inspect -f '{{.State.Running}}' "$AI_CONTAINER" 2>/dev/null || true)" = "true" ]; then
    docker compose exec -T ai-service python -m pytest app/tests -q -s
  else
    echo "ai-service container is not running; skipping container pytest"
  fi
else
  echo "docker unavailable or not connected to this WSL distro; skipping docker compose config and container pytest"
fi

echo "all checks passed"

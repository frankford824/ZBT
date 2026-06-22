#!/usr/bin/env python3
"""First usable release readiness gate.

This script gathers the release evidence that decides whether loop work can stop.
The default local profile verifies that the repository contains the required
gates and that local checks may skip unavailable external providers explicitly.
The production profile requires non-skipped Provider/OCR canaries.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
AI_SERVICE = ROOT / "ai-service"
FALSE_ENV_VALUES = {"0", "false", "no"}
PRODUCTION_PLACEHOLDER_MARKERS = ("<replace-", "replace-with", "changeme", "todo", "placeholder")
PRODUCTION_MODE_ENV_KEYS = ("APP_ENV", "ZBT_ENV", "ENVIRONMENT", "GIN_MODE")
PRODUCTION_MODE_VALUES = {"prod", "production", "release"}
PRODUCTION_REQUIRED_RUNTIME_ENVS = (
    "DATABASE_URL",
    "MIGRATION_DATABASE_URL",
    "REDIS_URL",
    "AI_SERVICE_URL",
    "AI_CALLBACK_URL",
    "MINIO_ENDPOINT",
    "MINIO_PUBLIC_ENDPOINT",
    "MINIO_BUCKET",
)
PRODUCTION_SECRET_DEFAULTS = {
    "JWT_SECRET": "dev-only-zbt-jwt-secret",
    "AI_SERVICE_HMAC_SECRET": "dev-only-zbt-ai-callback-secret",
    "MINIO_ACCESS_KEY": "zbt_minio",
    "MINIO_SECRET_KEY": "zbt_minio_secret",
}
PRODUCTION_ROUTE_DEFAULTS = {
    "chapter_generate": ("LLM", "openai_compatible_primary", "gpt-4o-mini"),
    "knowledge_embedding": ("EMBEDDING", "openai_compatible_primary", "text-embedding-3-large"),
    "knowledge_rerank": ("RERANK", "openai_compatible_primary", "gpt-4o-mini"),
}
PRODUCTION_PROVIDER_ENV_GROUPS = {
    "openai_compatible_primary": (("OPENAI_API_KEY",),),
    "deepseek": (("DEEPSEEK_API_KEY",), ("DEEPSEEK_BASE_URL",)),
    "dashscope": (("DASHSCOPE_API_KEY",), ("DASHSCOPE_BASE_URL",)),
    "cloudflare_ai_gateway": (
        ("CLOUDFLARE_API_TOKEN", "CLOUDFLARE_AI_GATEWAY_TOKEN"),
        ("CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL"),
    ),
}
PRODUCTION_OCR_PROVIDER_ENV_GROUPS = {
    "http_ocr": (("OCR_HTTP_ENDPOINT",),),
    "mineru": (("MINERU_HTTP_ENDPOINT", "OCR_HTTP_ENDPOINT"),),
    "paddleocr": (("PADDLEOCR_HTTP_ENDPOINT", "OCR_HTTP_ENDPOINT"),),
}


class ReadinessError(RuntimeError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ReadinessError(message)


def ok(label: str, evidence: object) -> None:
    print(f"[ok] {label}: {evidence}")


def env_value(key: str) -> str:
    return os.getenv(key, "").strip()


def env_is_false(key: str) -> bool:
    return env_value(key).lower() in FALSE_ENV_VALUES


def env_is_placeholder(value: str) -> bool:
    normalized = value.strip().lower()
    return any(marker in normalized for marker in PRODUCTION_PLACEHOLDER_MARKERS)


def load_env_file(path: Path | None) -> dict[str, str]:
    if path is None:
        return {}
    require(path.is_file(), f"env file does not exist: {path}")
    loaded: dict[str, str] = {}
    for line_no, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export ") :].strip()
        require("=" in line, f"env file {path}:{line_no} is not KEY=VALUE")
        key, value = line.split("=", 1)
        key = key.strip()
        require(
            re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key) is not None,
            f"env file {path}:{line_no} has invalid key: {key}",
        )
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
            value = value[1:-1]
        loaded[key] = value
        os.environ[key] = value
    ok("env file loaded", {"path": str(path), "keys": len(loaded)})
    return loaded


def ai_python() -> str:
    venv_python = AI_SERVICE / ".venv/bin/python"
    if venv_python.is_file():
        return str(venv_python)
    return "python3"


def latest_loop_section(text: str) -> tuple[str, str]:
    matches = list(re.finditer(r"^## Loop-\d+ / .+$", text, re.MULTILINE))
    require(matches, "DEV_LOOP_LOG.md has no Loop section")
    start = matches[-1].start()
    return matches[-1].group(0), text[start:]


def check_static_release_evidence() -> None:
    required_files = [
        "infra/scripts/acceptance_project1_check.py",
        "infra/scripts/acceptance_tail_check.py",
        "infra/scripts/check.sh",
        "infra/scripts/first_usable_release_report.py",
        "infra/scripts/test_first_usable_release_report.py",
        "ai-service/app/evaluation/provider_canary_eval.py",
        "ai-service/app/evaluation/ocr_provider_eval.py",
        "ai-service/app/evaluation/tender_parse_eval.py",
        "ai-service/app/evaluation/generation_coverage_eval.py",
        "ai-service/app/evaluation/export_format_eval.py",
        "docs/sample_docs/golden/工程1.parse.json",
        "docs/sample_docs/golden/工程1.generation_coverage.json",
        "docs/sample_docs/golden/工程1.export.json",
        ".env.production.example",
        "docs/ex/工程1/采购文件桥梁检查.pdf",
        "docs/ex/工程1/响应文件格式.docx",
        "docs/ex/工程1/清单（固化）(1).xlsx",
    ]
    missing = [path for path in required_files if not (ROOT / path).is_file()]
    require(not missing, f"first usable required files are missing: {missing}")

    check_script = (ROOT / "infra/scripts/check.sh").read_text(encoding="utf-8")
    for needle in (
        "acceptance_project1_check.py",
        "provider_canary_eval --allow-skip",
        "app.evaluation.ocr_provider_eval",
        "app.evaluation.tender_parse_eval",
        "app.evaluation.generation_coverage_eval",
        "app.evaluation.export_format_eval",
    ):
        require(needle in check_script, f"check.sh missing first usable gate: {needle}")

    routing = (ROOT / "ai-service/app/config/model_routing.yaml").read_text(encoding="utf-8")
    require("provider: mock\n      model:" not in routing, "model routing still has a mock primary route")
    require("provider: openai_compatible_primary" in routing, "model routing missing openai-compatible primary")
    require("provider: local" in routing and "document_ocr" in routing, "model routing missing local OCR pipeline")

    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    for needle in (
        "acceptance_project1_check.py",
        "provider_canary_eval --strict --call-provider --require-cost",
        "app.evaluation.ocr_provider_eval",
        "BID_EXPORT_TEMPLATE_PATH",
        "AI_MODEL_PRICING_JSON",
        "first_usable_release_report.py --profile production",
        "--include-repo-check --include-project1-runtime",
    ):
        require(needle in readme, f"README missing first usable guidance: {needle}")
    gitignore = (ROOT / ".gitignore").read_text(encoding="utf-8")
    require(".env.production" in gitignore, ".gitignore must ignore real production env files")
    production_template = (ROOT / ".env.production.example").read_text(encoding="utf-8")
    for needle in (
        "APP_ENV=production",
        "DATABASE_URL=",
        "MIGRATION_DATABASE_URL=",
        "REDIS_URL=",
        "AI_SERVICE_URL=",
        "AI_CALLBACK_URL=",
        "MINIO_ENDPOINT=",
        "MINIO_PUBLIC_ENDPOINT=",
        "MINIO_BUCKET=",
        "USE_MOCK_PROVIDERS=false",
        "ALLOW_MOCK_FALLBACK=false",
        "AI_MODEL_PRICING_JSON=",
        "OPENAI_API_KEY=<replace-with-openai-api-key>",
        "OCR_HTTP_ENDPOINT=<replace-with-ocr-http-endpoint>",
        "AI_SERVICE_HMAC_SECRET=<replace-with-strong-ai-callback-secret-at-least-16-chars>",
    ):
        require(needle in production_template, f".env.production.example missing production guidance: {needle}")

    log = (ROOT / "docs/blueprint/DEV_LOOP_LOG.md").read_text(encoding="utf-8")
    latest_heading, latest_section = latest_loop_section(log)
    loop_match = re.search(r"Loop-(\d+)", latest_heading)
    loop_number = int(loop_match.group(1)) if loop_match else 0
    require(loop_number >= 169, "latest loop is older than runtime acceptance loop")
    for needle in (
        "acceptance_project1_check.py",
        "python3 infra/scripts/acceptance_project1_check.py",
        "./infra/scripts/check.sh",
        "Provider canary",
        "OCR canary",
    ):
        require(needle in log, f"DEV_LOOP_LOG missing first usable evidence: {needle}")
    ok("static first usable evidence", {"latest_loop": latest_heading, "files": len(required_files)})


def env_group_issue(label: str, alternatives: tuple[str, ...]) -> tuple[str | None, str | None]:
    configured = [key for key in alternatives if env_value(key)]
    placeholder = [key for key in configured if env_is_placeholder(env_value(key))]
    matched = [key for key in configured if key not in placeholder]
    if matched:
        return matched[0], None
    if placeholder:
        return None, f"production env {placeholder[0]} still uses a placeholder value"
    return None, f"production env missing {label}: set one of {', '.join(alternatives)}"


def required_env_issue(key: str) -> str | None:
    value = env_value(key)
    if not value:
        return f"production env missing {key}"
    if env_is_placeholder(value):
        return f"production env {key} still uses a placeholder value"
    return None


def production_mode_issue() -> str | None:
    configured = [(key, env_value(key)) for key in PRODUCTION_MODE_ENV_KEYS if env_value(key)]
    placeholder = [key for key, value in configured if env_is_placeholder(value)]
    if placeholder:
        return f"production env {placeholder[0]} still uses a placeholder value"
    valid = [key for key, value in configured if value.lower() in PRODUCTION_MODE_VALUES]
    if valid:
        return None
    return "production env must set APP_ENV, ZBT_ENV, ENVIRONMENT or GIN_MODE to prod/production/release"


def production_secret_issue(key: str, default_value: str) -> str | None:
    value = env_value(key)
    if not value:
        return f"production env missing {key}"
    if env_is_placeholder(value):
        return f"production env {key} still uses a placeholder value"
    if value == default_value or len(value) < 16:
        return f"production env {key} must not use the development default or a short value"
    return None


def production_route_target(route: str, route_kind: str, default_provider: str, default_model: str) -> dict[str, str]:
    route_prefix = route.upper().replace("-", "_")
    provider = env_value(f"{route_prefix}_PROVIDER") or env_value(f"AI_{route_kind}_PROVIDER") or default_provider
    model = env_value(f"{route_prefix}_MODEL") or env_value(f"AI_{route_kind}_MODEL") or default_model
    return {"route": route, "kind": route_kind.lower(), "provider": provider, "model": model}


def pricing_config() -> tuple[dict[str, object], list[str]]:
    issues: list[str] = []
    raw = env_value("AI_MODEL_PRICING_JSON")
    if not raw:
        return {}, ["production env missing AI_MODEL_PRICING_JSON"]
    if env_is_placeholder(raw):
        return {}, ["production env AI_MODEL_PRICING_JSON still uses a placeholder value"]
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        return {}, [f"production env AI_MODEL_PRICING_JSON is invalid JSON: {exc}"]
    if not isinstance(parsed, dict) or not parsed:
        return {}, ["production env AI_MODEL_PRICING_JSON must be a non-empty JSON object"]
    usable_rates = [
        key
        for key, value in parsed.items()
        if isinstance(key, str)
        and isinstance(value, dict)
        and (positive_rate(value.get("input_per_1m")) > 0 or positive_rate(value.get("input_per_1k")) > 0)
        and (positive_rate(value.get("output_per_1m")) > 0 or positive_rate(value.get("output_per_1k")) > 0)
    ]
    if not usable_rates:
        issues.append("production env AI_MODEL_PRICING_JSON must include positive input/output rates")
    return parsed, issues


def positive_rate(value: object) -> float:
    if isinstance(value, bool):
        return 0.0
    try:
        number = float(value)
    except (TypeError, ValueError):
        return 0.0
    if number <= 0:
        return 0.0
    return number


def pricing_has_match(pricing: dict[str, object], provider: str, model: str) -> bool:
    keys = [
        f"{provider}/{model}",
        model,
        f"{provider}/*",
        "*",
        f"{provider.lower()}/{model.lower()}",
        model.lower(),
        f"{provider.lower()}/*",
    ]
    return any(key in pricing for key in keys)


def check_production_env() -> None:
    issues: list[str] = []
    issue = production_mode_issue()
    if issue:
        issues.append(issue)
    for key in PRODUCTION_REQUIRED_RUNTIME_ENVS:
        issue = required_env_issue(key)
        if issue:
            issues.append(issue)
    if not env_is_false("USE_MOCK_PROVIDERS"):
        issues.append("production env USE_MOCK_PROVIDERS must be false")
    if not env_is_false("ALLOW_MOCK_FALLBACK"):
        issues.append("production env ALLOW_MOCK_FALLBACK must be false")

    for key, default_value in PRODUCTION_SECRET_DEFAULTS.items():
        issue = production_secret_issue(key, default_value)
        if issue:
            issues.append(issue)

    pricing, pricing_issues = pricing_config()
    issues.extend(pricing_issues)
    targets = [
        production_route_target(route, route_kind, default_provider, default_model)
        for route, (route_kind, default_provider, default_model) in PRODUCTION_ROUTE_DEFAULTS.items()
    ]
    providers = sorted({target["provider"] for target in targets})
    for provider in providers:
        if provider in {"mock", "local"}:
            issues.append(f"production routes must use real external providers, got {provider}")
            continue
        if provider not in PRODUCTION_PROVIDER_ENV_GROUPS:
            issues.append(f"production route uses unsupported provider {provider}")
            continue
        for group in PRODUCTION_PROVIDER_ENV_GROUPS[provider]:
            _, issue = env_group_issue(f"{provider} credentials", group)
            if issue:
                issues.append(issue)
    for target in targets:
        if not target["model"]:
            issues.append(f"production route {target['route']} is missing model")
            continue
        if pricing and target["provider"] in PRODUCTION_PROVIDER_ENV_GROUPS and not pricing_has_match(pricing, target["provider"], target["model"]):
            issues.append(
                "production env AI_MODEL_PRICING_JSON missing price for "
                f"{target['provider']}/{target['model']} or {target['provider']}/*"
            )

    ocr_provider = env_value("OCR_PROVIDER") or "http_ocr"
    if ocr_provider not in PRODUCTION_OCR_PROVIDER_ENV_GROUPS:
        issues.append(f"production OCR_PROVIDER is unsupported: {ocr_provider}")
    else:
        for group in PRODUCTION_OCR_PROVIDER_ENV_GROUPS[ocr_provider]:
            _, issue = env_group_issue(f"{ocr_provider} endpoint", group)
            if issue:
                issues.append(issue)

    evidence = {
        "providers": providers,
        "routes": targets,
        "ocr_provider": ocr_provider,
        "pricing_entries": len(pricing),
    }
    if issues:
        joined = "\n- ".join(dict.fromkeys(issues))
        raise ReadinessError(f"production env audit failed:\n- {joined}")
    ok("production env audit", evidence)


def run_json_command(command: list[str], *, cwd: Path, env: dict[str, str] | None = None) -> tuple[int, dict[str, Any], str]:
    completed = subprocess.run(
        command,
        cwd=str(cwd),
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        check=False,
    )
    output = completed.stdout.strip()
    try:
        parsed = json.loads(output)
    except json.JSONDecodeError:
        parsed = {}
    return completed.returncode, parsed, output


def check_provider_canary(profile: str) -> None:
    command = [
        ai_python(),
        "-m",
        "app.evaluation.provider_canary_eval",
        "--route",
        "chapter_generate",
        "--route",
        "knowledge_embedding",
        "--route",
        "knowledge_rerank",
        "--json",
    ]
    if profile == "production":
        command.extend(["--strict", "--call-provider", "--require-cost"])
    else:
        command.append("--allow-skip")
    code, result, output = run_json_command(command, cwd=AI_SERVICE)
    status = str(result.get("status") or "")
    if profile == "production":
        require(code == 0 and status == "passed", f"production Provider canary failed: {output[:1200]}")
    else:
        require(code == 0 and status in {"passed", "skipped"}, f"local Provider canary failed: {output[:1200]}")
    ok("provider canary", {"profile": profile, "status": status, "passed": result.get("passed_checks"), "total": result.get("total_checks")})


def check_ocr_canary(profile: str) -> None:
    ocr_provider = env_value("OCR_PROVIDER") or "http_ocr"
    command = [
        ai_python(),
        "-m",
        "app.evaluation.ocr_provider_eval",
        "--provider",
        ocr_provider,
        "--sample",
        str(ROOT / "docs/ex/工程1/采购文件桥梁检查.pdf"),
        "--repo-root",
        str(ROOT),
        "--min-text-chars",
        "20",
        "--min-table-blocks",
        "1",
        "--min-layout-bbox-count",
        "1",
        "--min-table-bbox-count",
        "1",
        "--min-cell-bbox-count",
        "1",
        "--json",
    ]
    if profile != "production":
        command.append("--allow-skip")
    code, result, output = run_json_command(command, cwd=AI_SERVICE)
    status = str(result.get("status") or "")
    if profile == "production":
        require(code == 0 and status == "passed", f"production OCR canary failed: {output[:1200]}")
    else:
        require(code == 0 and status in {"passed", "skipped"}, f"local OCR canary failed: {output[:1200]}")
    ok("ocr canary", {"profile": profile, "status": status, "passed": result.get("passed_checks"), "total": result.get("total_checks")})


def main() -> int:
    parser = argparse.ArgumentParser(description="Check first usable release readiness evidence.")
    parser.add_argument("--profile", choices=("local", "production"), default="local")
    parser.add_argument("--env-file", type=Path, help="Load KEY=VALUE settings before running readiness checks.")
    parser.add_argument("--audit-production-env", action="store_true", help="Only audit production env settings and skip live Provider/OCR canaries.")
    parser.add_argument("--run-canaries", action="store_true", help="Run Provider/OCR canaries in addition to static checks.")
    args = parser.parse_args()

    load_env_file(args.env_file)
    check_static_release_evidence()
    if args.audit_production_env or args.profile == "production":
        check_production_env()
    if args.audit_production_env:
        print("[ok] production env audit-only")
        return 0
    if args.run_canaries or args.profile == "production":
        check_provider_canary(args.profile)
        check_ocr_canary(args.profile)
    print(f"[ok] first usable release readiness profile={args.profile}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ReadinessError as exc:
        print(f"[error] {exc}")
        raise SystemExit(1)

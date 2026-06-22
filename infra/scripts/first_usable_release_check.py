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


class ReadinessError(RuntimeError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ReadinessError(message)


def ok(label: str, evidence: object) -> None:
    print(f"[ok] {label}: {evidence}")


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
        "ai-service/app/evaluation/provider_canary_eval.py",
        "ai-service/app/evaluation/ocr_provider_eval.py",
        "ai-service/app/evaluation/tender_parse_eval.py",
        "ai-service/app/evaluation/generation_coverage_eval.py",
        "ai-service/app/evaluation/export_format_eval.py",
        "docs/sample_docs/golden/工程1.parse.json",
        "docs/sample_docs/golden/工程1.generation_coverage.json",
        "docs/sample_docs/golden/工程1.export.json",
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
    ):
        require(needle in readme, f"README missing first usable guidance: {needle}")

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
    command = [
        ai_python(),
        "-m",
        "app.evaluation.ocr_provider_eval",
        "--provider",
        os.getenv("OCR_PROVIDER", "http_ocr"),
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
    parser.add_argument("--run-canaries", action="store_true", help="Run Provider/OCR canaries in addition to static checks.")
    args = parser.parse_args()

    check_static_release_evidence()
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

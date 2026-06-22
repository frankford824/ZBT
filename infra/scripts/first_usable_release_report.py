#!/usr/bin/env python3
"""Generate a redacted first-usable release evidence report."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import shlex
import subprocess
import time
from datetime import UTC, datetime
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
AI_SERVICE = ROOT / "ai-service"
SENSITIVE_ENV_RE = re.compile(r"(SECRET|TOKEN|PASSWORD|API_KEY|ACCESS_KEY|HMAC|JWT)", re.IGNORECASE)
URL_WITH_PASSWORD_RE = re.compile(r"([a-z][a-z0-9+.-]*://[^:/@\s]+:)([^@\s]+)(@)", re.IGNORECASE)
BEARER_RE = re.compile(r"(Bearer\s+)[A-Za-z0-9._~+/=-]+", re.IGNORECASE)
GIT_SHA_RE = re.compile(r"^[0-9a-f]{40}$")
SENSITIVE_URL_ENV_KEYS = {"DATABASE_URL", "MIGRATION_DATABASE_URL", "REDIS_URL"}
PRODUCTION_ENV_AUDIT_JSON = ROOT / "tmp/production_env_audit.json"
PROVIDER_CANARY_JSON = ROOT / "tmp/provider_canary.json"
OCR_CANARY_JSON = ROOT / "tmp/ocr_provider_canary.json"
EXPORT_FORMAT_JSON = ROOT / "tmp/export_format_eval.json"
PROJECT1_RUNTIME_JSON = ROOT / "tmp/project1_runtime_acceptance.json"
EXPECTED_ORIGIN_REMOTE = "git@github.com:frankford824/ZBT.git"
EXPECTED_GITHUB_HTTPS_REMOTE = "https://github.com/frankford824/ZBT.git"
EXPECTED_RELEASE_BRANCH = "main"
PRODUCTION_REQUIRED_ARTIFACTS = (
    ("export_format_eval", "export format artifact must be present and passed"),
    ("production_env_audit_json", "production artifact production_env_audit_json must be present and passed"),
    ("provider_canary", "production artifact provider_canary must be present and passed"),
    ("ocr_provider_canary", "production artifact ocr_provider_canary must be present and passed"),
)
PROJECT1_REQUIRED_ARTIFACTS = (
    ("project1_runtime_acceptance", "project1 runtime artifact must be present and passed"),
)
REQUIRED_PROVIDER_ROUTES = ("chapter_generate", "knowledge_embedding", "knowledge_rerank")
PROVIDER_ROUTE_KINDS = {
    "chapter_generate": "llm",
    "knowledge_embedding": "embedding",
    "knowledge_rerank": "rerank",
}
ZERO_COST_PROVIDERS = {"mock", "local"}
SUPPORTED_OCR_PROVIDERS = {"http_ocr", "mineru", "paddleocr"}
REQUIRED_OCR_CHECKS = (
    "provider.endpoint_configured",
    "sample.exists",
    "ocr.status",
    "ocr.provider",
    "ocr.text_chars",
    "ocr.table_blocks",
    "ocr.layout_bbox_count",
    "ocr.table_bbox_count",
    "ocr.cell_bbox_count",
)
REQUIRED_EXPORT_CHECKS = (
    "export.docx.openable",
    "export.docx.non_empty",
    "export.docx.cover",
    "export.docx.watermark",
    "export.docx.toc_field",
    "export.docx.update_fields",
    "export.docx.page_fields",
    "export.docx.header_footer",
    "export.docx.header_footer_text",
    "export.zip.manifest",
    "export.zip.manifest_part_count",
    "export.zip.manifest_integrity",
    "export.zip.safe_paths",
    "export.zip.docx_entries_openable",
    "export.pdf.generated",
    "export.pdf.openable",
    "export.pdf.text_layer",
    "export.pdf.first_page_nonblank",
)
PROJECT1_REQUIRED_STEPS = (
    "parse_response_matrix",
    "companion_knowledge",
    "generation_coverage_compliance",
    "docx_export",
)
PROJECT1_REQUIRED_SAMPLE_LABELS = ("tender_pdf", "response_docx", "boq_xlsx")


class ReportError(RuntimeError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ReportError(message)


def load_env_file(path: Path | None) -> tuple[dict[str, str], list[str]]:
    if path is None:
        return {}, []
    require(path.is_file(), f"env file does not exist: {path}")
    loaded: dict[str, str] = {}
    sensitive_values: list[str] = []
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
        if is_sensitive_env_key(key) and value:
            sensitive_values.append(value)
    return loaded, sensitive_values


def is_sensitive_env_key(key: str) -> bool:
    return bool(SENSITIVE_ENV_RE.search(key) or key in SENSITIVE_URL_ENV_KEYS)


def sensitive_values_from_env(env: dict[str, str]) -> list[str]:
    return [value for key, value in env.items() if value and is_sensitive_env_key(key)]


def redactor(values: list[str]):
    ordered_values = sorted({value for value in values if len(value) >= 4}, key=len, reverse=True)

    def redact(text: str) -> str:
        result = text
        for value in ordered_values:
            result = result.replace(value, "<redacted>")
        result = URL_WITH_PASSWORD_RE.sub(r"\1<redacted>\3", result)
        result = BEARER_RE.sub(r"\1<redacted>", result)
        return result

    return redact


def run_command(name: str, command: list[str], *, env: dict[str, str], timeout_s: int, redact) -> dict[str, Any]:
    return run_command_in_cwd(name, command, cwd=ROOT, env=env, timeout_s=timeout_s, redact=redact)


def run_command_in_cwd(
    name: str,
    command: list[str],
    *,
    cwd: Path,
    env: dict[str, str],
    timeout_s: int,
    redact,
) -> dict[str, Any]:
    started = time.monotonic()
    try:
        completed = subprocess.run(
            command,
            cwd=str(cwd),
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            timeout=timeout_s,
            check=False,
        )
        output = completed.stdout.strip()
        return {
            "name": name,
            "status": "passed" if completed.returncode == 0 else "failed",
            "returncode": completed.returncode,
            "duration_s": round(time.monotonic() - started, 3),
            "command": redact(shlex.join(command)),
            "output": redact(output)[-8000:],
        }
    except subprocess.TimeoutExpired as exc:
        output = (exc.stdout or "").strip() if isinstance(exc.stdout, str) else ""
        return {
            "name": name,
            "status": "failed",
            "returncode": None,
            "duration_s": round(time.monotonic() - started, 3),
            "command": redact(shlex.join(command)),
            "output": redact(output)[-8000:],
            "error": f"timed out after {timeout_s}s",
        }


def run_json_artifact_command(
    name: str,
    command: list[str],
    artifact: Path,
    *,
    env: dict[str, str],
    timeout_s: int,
    redact,
    cwd: Path = ROOT,
) -> dict[str, Any]:
    started = time.monotonic()
    clear_artifact(artifact)
    try:
        completed = subprocess.run(
            command,
            cwd=str(cwd),
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            timeout=timeout_s,
            check=False,
        )
        output = completed.stdout.strip()
        redacted_output = redact(output)
        step = {
            "name": name,
            "status": "passed" if completed.returncode == 0 else "failed",
            "returncode": completed.returncode,
            "duration_s": round(time.monotonic() - started, 3),
            "command": redact(shlex.join(command)),
            "output": redacted_output[-8000:],
        }
        if redacted_output:
            artifact.parent.mkdir(parents=True, exist_ok=True)
            artifact.write_text(redacted_output + "\n", encoding="utf-8")
            step["artifact"] = artifact_summary(name, artifact)
        return step
    except subprocess.TimeoutExpired as exc:
        output = (exc.stdout or "").strip() if isinstance(exc.stdout, str) else ""
        redacted_output = redact(output)
        return {
            "name": name,
            "status": "failed",
            "returncode": None,
            "duration_s": round(time.monotonic() - started, 3),
            "command": redact(shlex.join(command)),
            "output": redacted_output[-8000:],
            "error": f"timed out after {timeout_s}s",
        }


def clear_artifact(path: Path) -> None:
    if path.is_file():
        path.unlink()


def artifact_summary(name: str, path: Path) -> dict[str, Any]:
    if not path.is_file():
        return {"name": name, "path": str(path.relative_to(ROOT)), "status": "missing"}
    content = path.read_bytes()
    parsed_status = "not_json"
    parsed_name = ""
    semantic_issues: list[str] = ["artifact JSON must be an object"]
    try:
        parsed = json.loads(content.decode("utf-8"))
    except json.JSONDecodeError:
        parsed = None
    if isinstance(parsed, dict):
        parsed_status = str(parsed.get("status") or "unknown")
        parsed_name = str(parsed.get("name") or "")
        semantic_issues = artifact_semantic_issues(name, parsed)
    return {
        "name": name,
        "path": str(path.relative_to(ROOT)),
        "status": "present",
        "bytes": len(content),
        "sha256": hashlib.sha256(content).hexdigest(),
        "json_name": parsed_name,
        "json_status": parsed_status,
        "semantic_status": "passed" if not semantic_issues else "failed",
        "semantic_issues": semantic_issues,
    }


def artifact_semantic_issues(name: str, payload: dict[str, Any]) -> list[str]:
    if name == "production_env_audit_json":
        return production_env_artifact_issues(payload)
    if name == "provider_canary":
        return provider_canary_artifact_issues(payload)
    if name == "ocr_provider_canary":
        return ocr_canary_artifact_issues(payload)
    if name == "export_format_eval":
        return export_format_artifact_issues(payload)
    if name == "project1_runtime_acceptance":
        return project1_runtime_artifact_issues(payload)
    return []


def common_artifact_issues(payload: dict[str, Any], expected_name: str) -> list[str]:
    issues: list[str] = []
    if payload.get("name") != expected_name:
        issues.append(f"artifact JSON name must be {expected_name}")
    if payload.get("status") != "passed":
        issues.append("artifact JSON status must be passed")
    return issues


def production_env_artifact_issues(payload: dict[str, Any]) -> list[str]:
    issues = common_artifact_issues(payload, "production_env_audit")
    evidence = payload.get("evidence")
    if not isinstance(evidence, dict):
        return issues + ["production env audit evidence must be present"]

    route_index = _index_dicts(evidence.get("routes"), "route")
    for route, expected_kind in PROVIDER_ROUTE_KINDS.items():
        target = route_index.get(route)
        if not target:
            issues.append(f"production env audit route {route} must be present")
            continue
        if target.get("kind") != expected_kind:
            issues.append(f"production env audit route {route} kind must be {expected_kind}")
        if str(target.get("provider") or "") in ZERO_COST_PROVIDERS:
            issues.append(f"production env audit route {route} must use a real provider")
        if not str(target.get("model") or "").strip():
            issues.append(f"production env audit route {route} model must be present")

    pricing_index = _index_dicts(evidence.get("pricing_matches"), "route")
    for route in REQUIRED_PROVIDER_ROUTES:
        pricing = pricing_index.get(route)
        if not pricing or pricing.get("matched") is not True:
            issues.append(f"production env audit route {route} pricing must match")

    provider_requirements = _list_of_dicts(evidence.get("provider_requirements"))
    if not provider_requirements:
        issues.append("production env audit provider requirements must be present")
    for requirement in provider_requirements:
        provider = str(requirement.get("provider") or "")
        if provider in ZERO_COST_PROVIDERS:
            issues.append(f"production env audit provider {provider} must be external")
        if requirement.get("issues"):
            issues.append(f"production env audit provider {provider} must have no credential issues")
        configured_envs = requirement.get("configured_envs")
        if not isinstance(configured_envs, list) or not configured_envs:
            issues.append(f"production env audit provider {provider} must have configured credentials")

    ocr_provider = str(evidence.get("ocr_provider") or "")
    if ocr_provider not in SUPPORTED_OCR_PROVIDERS:
        issues.append("production env audit OCR provider must be supported")
    ocr_requirement = evidence.get("ocr_requirement")
    if not isinstance(ocr_requirement, dict):
        issues.append("production env audit OCR requirement must be present")
    else:
        if ocr_requirement.get("issues"):
            issues.append("production env audit OCR requirement must have no endpoint issues")
        configured_envs = ocr_requirement.get("configured_envs")
        if not isinstance(configured_envs, list) or not configured_envs:
            issues.append("production env audit OCR endpoint must be configured")
    return issues


def provider_canary_artifact_issues(payload: dict[str, Any]) -> list[str]:
    issues = common_artifact_issues(payload, "provider_canary")
    if payload.get("strict") is not True:
        issues.append("provider canary strict must be true")
    if payload.get("call_provider") is not True:
        issues.append("provider canary call_provider must be true")
    if payload.get("require_cost") is not True:
        issues.append("provider canary require_cost must be true")
    issues.extend(check_count_issues(payload, "provider canary"))

    route_index = _index_dicts(payload.get("routes"), "route")
    for route, expected_kind in PROVIDER_ROUTE_KINDS.items():
        result = route_index.get(route)
        if not result:
            issues.append(f"provider canary route {route} must be present")
            continue
        provider = str(result.get("provider") or "")
        if result.get("kind") != expected_kind:
            issues.append(f"provider canary route {route} kind must be {expected_kind}")
        if result.get("resolved") is not True:
            issues.append(f"provider canary route {route} must resolve")
        if provider in ZERO_COST_PROVIDERS:
            issues.append(f"provider canary route {route} must use a real provider")
        if not str(result.get("model") or "").strip():
            issues.append(f"provider canary route {route} model must be present")
        call = result.get("call")
        if not isinstance(call, dict) or call.get("passed") is not True:
            issues.append(f"provider canary route {route} must include a passed live call")
        accounting = result.get("accounting")
        estimated_cost = float_value(accounting.get("estimated_cost") if isinstance(accounting, dict) else None)
        if estimated_cost <= 0:
            issues.append(f"provider canary route {route} estimated_cost must be positive")
    return issues


def ocr_canary_artifact_issues(payload: dict[str, Any]) -> list[str]:
    issues = common_artifact_issues(payload, "ocr_provider_eval")
    provider = str(payload.get("provider") or "")
    if provider not in SUPPORTED_OCR_PROVIDERS:
        issues.append("OCR canary provider must be supported")
    issues.extend(check_count_issues(payload, "OCR canary", include_check_failures=False))

    check_index = _index_dicts(payload.get("checks"), "name")
    for check_name in REQUIRED_OCR_CHECKS:
        check = check_index.get(check_name)
        if not check:
            issues.append(f"OCR canary check {check_name} must be present")
        elif check.get("passed") is not True:
            issues.append(f"OCR canary check {check_name} must pass")

    metadata = payload.get("metadata")
    if not isinstance(metadata, dict):
        issues.append("OCR canary metadata must be present")
    else:
        if int_value(metadata.get("table_block_count")) < 1:
            issues.append("OCR canary metadata table_block_count must be positive")
        ocr_metadata = metadata.get("ocr")
        if not isinstance(ocr_metadata, dict):
            issues.append("OCR canary OCR metadata must be present")
        elif ocr_metadata.get("provider") != provider:
            issues.append("OCR canary OCR metadata provider must match")
    return issues


def export_format_artifact_issues(payload: dict[str, Any]) -> list[str]:
    issues = common_artifact_issues(payload, "工程1.export")
    issues.extend(check_count_issues(payload, "export format"))

    check_index = _index_dicts(payload.get("checks"), "name")
    for check_name in REQUIRED_EXPORT_CHECKS:
        check = check_index.get(check_name)
        if not check:
            issues.append(f"export format check {check_name} must be present")
        elif check.get("passed") is not True:
            issues.append(f"export format check {check_name} must pass")

    docx = payload.get("docx")
    if not isinstance(docx, dict):
        issues.append("export format DOCX result must be present")
    else:
        if int_value(docx.get("size_bytes")) <= 1024:
            issues.append("export format DOCX size must be nontrivial")
        if int_value(docx.get("table_count")) < 1:
            issues.append("export format DOCX table_count must be positive")

    zip_result = payload.get("zip")
    if not isinstance(zip_result, dict):
        issues.append("export format ZIP result must be present")
    else:
        if int_value(zip_result.get("docx_entry_count")) < 2:
            issues.append("export format ZIP must include tech and business DOCX entries")
        if zip_result.get("manifest_issues"):
            issues.append("export format ZIP manifest must have no integrity issues")

    pdf = payload.get("pdf")
    if not isinstance(pdf, dict):
        issues.append("export format PDF result must be present")
    else:
        if pdf.get("status") != "generated":
            issues.append("export format PDF must be generated, not skipped")
        if int_value(pdf.get("page_count")) < 1:
            issues.append("export format PDF page_count must be positive")
        if int_value(pdf.get("text_chars")) <= 0:
            issues.append("export format PDF text layer must be non-empty")
        if pdf.get("first_page_nonblank") is not True:
            issues.append("export format PDF first page must render nonblank")
    return issues


def project1_runtime_artifact_issues(payload: dict[str, Any]) -> list[str]:
    issues = common_artifact_issues(payload, "project1_runtime_acceptance")
    labels = {
        str(item.get("label") or "")
        for item in _list_of_dicts(payload.get("sample_files"))
    }
    for label in PROJECT1_REQUIRED_SAMPLE_LABELS:
        if label not in labels:
            issues.append(f"project1 runtime sample {label} must be present")

    steps = payload.get("steps")
    if not isinstance(steps, dict):
        return issues + ["project1 runtime steps must be present"]
    for step in PROJECT1_REQUIRED_STEPS:
        if not isinstance(steps.get(step), dict):
            issues.append(f"project1 runtime step {step} must be present")

    parse = steps.get("parse_response_matrix") if isinstance(steps.get("parse_response_matrix"), dict) else {}
    if int_value(parse.get("requirements")) < 35:
        issues.append("project1 runtime requirements must be at least 35")
    if int_value(parse.get("expected_response")) < 35:
        issues.append("project1 runtime expected_response must be at least 35")
    if int_value(parse.get("mandatory")) < 20:
        issues.append("project1 runtime mandatory requirements must be at least 20")
    if int_value(parse.get("high_priority")) < 30:
        issues.append("project1 runtime high priority requirements must be at least 30")
    if int_value(parse.get("requirements_xlsx_bytes")) <= 1024:
        issues.append("project1 runtime requirements XLSX export must be nontrivial")

    knowledge = steps.get("companion_knowledge") if isinstance(steps.get("companion_knowledge"), dict) else {}
    if knowledge.get("response_doc_status") != "processed":
        issues.append("project1 runtime response doc must be processed")
    if knowledge.get("boq_doc_status") != "processed":
        issues.append("project1 runtime BOQ doc must be processed")
    if int_value(knowledge.get("search_items")) <= 0:
        issues.append("project1 runtime knowledge search must return items")
    if knowledge.get("selected_ref_has_reference_id") is not True:
        issues.append("project1 runtime selected ref must have reference id")
    if knowledge.get("selected_ref_has_location") is not True:
        issues.append("project1 runtime selected ref must have location")

    generation = steps.get("generation_coverage_compliance") if isinstance(steps.get("generation_coverage_compliance"), dict) else {}
    if int_value(generation.get("source_refs")) <= 0:
        issues.append("project1 runtime generated chapter must have source refs")
    if generation.get("compliance_result_status") != "pass":
        issues.append("project1 runtime compliance must pass")
    if int_value(generation.get("requirements")) <= 0:
        issues.append("project1 runtime generation coverage requirements must be present")
    if int_value(generation.get("coverage_rows")) <= 0:
        issues.append("project1 runtime generation coverage rows must be present")

    export = steps.get("docx_export") if isinstance(steps.get("docx_export"), dict) else {}
    if export.get("download_ready") is not True:
        issues.append("project1 runtime DOCX export download must be ready")
    if not str(export.get("filename") or "").lower().endswith(".docx"):
        issues.append("project1 runtime export filename must be docx")
    return issues


def check_count_issues(payload: dict[str, Any], label: str, *, include_check_failures: bool = True) -> list[str]:
    issues: list[str] = []
    total = int_value(payload.get("total_checks"))
    passed = int_value(payload.get("passed_checks"))
    failed = int_value(payload.get("failed_checks"))
    if total <= 0:
        issues.append(f"{label} total_checks must be positive")
    if failed != 0:
        issues.append(f"{label} failed_checks must be 0")
    if total > 0 and passed != total:
        issues.append(f"{label} passed_checks must equal total_checks")
    if include_check_failures:
        for check in _list_of_dicts(payload.get("checks")):
            if check.get("passed") is not True:
                issues.append(f"{label} check {check.get('name') or '<unknown>'} must pass")
    return issues


def _index_dicts(value: Any, key: str) -> dict[str, dict[str, Any]]:
    return {str(item.get(key) or ""): item for item in _list_of_dicts(value) if str(item.get(key) or "")}


def _list_of_dicts(value: Any) -> list[dict[str, Any]]:
    return [item for item in value if isinstance(item, dict)] if isinstance(value, list) else []


def int_value(value: Any) -> int:
    if value is None or isinstance(value, bool):
        return 0
    try:
        return int(value)
    except (TypeError, ValueError):
        return 0


def float_value(value: Any) -> float:
    if value is None or isinstance(value, bool):
        return 0.0
    try:
        return float(value)
    except (TypeError, ValueError):
        return 0.0


def git_value(command: list[str], *, timeout_s: int | None = None) -> str:
    try:
        completed = subprocess.run(
            command,
            cwd=str(ROOT),
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            timeout=timeout_s,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return ""
    return completed.stdout.strip()


def git_value_with_error(command: list[str], *, timeout_s: int, env: dict[str, str] | None = None) -> tuple[str, str]:
    try:
        completed = subprocess.run(
            command,
            cwd=str(ROOT),
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            timeout=timeout_s,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return "", f"timed out after {timeout_s}s"
    return completed.stdout.strip(), "" if completed.returncode == 0 else completed.stdout.strip()


def git_remote_check_env() -> dict[str, str]:
    env = os.environ.copy()
    env["GIT_TERMINAL_PROMPT"] = "0"
    env.setdefault(
        "GIT_SSH_COMMAND",
        "ssh -o BatchMode=yes -o ConnectTimeout=20 -o ServerAliveInterval=10 -o ServerAliveCountMax=2",
    )
    return env


def git_remote_head_candidates(remote: str, remote_ref: str) -> list[tuple[str, list[str], dict[str, str]]]:
    env = git_remote_check_env()
    candidates = [("origin", ["git", "ls-remote", "origin", remote_ref], env)]
    if remote == EXPECTED_ORIGIN_REMOTE and shutil.which("gh"):
        candidates.append(
            (
                "github_https_gh",
                [
                    "git",
                    "-c",
                    "credential.helper=",
                    "-c",
                    "credential.helper=!gh auth git-credential",
                    "ls-remote",
                    EXPECTED_GITHUB_HTTPS_REMOTE,
                    remote_ref,
                ],
                env,
            )
        )
    return candidates


def ai_python() -> str:
    venv_python = AI_SERVICE / ".venv/bin/python"
    if venv_python.is_file():
        return str(venv_python)
    return "python3"


def remote_head_from_ls_remote(output: str, remote_ref: str) -> str:
    for line in output.splitlines():
        parts = line.strip().split()
        if len(parts) >= 2 and parts[1] == remote_ref and GIT_SHA_RE.fullmatch(parts[0]):
            return parts[0]
    return ""


def collect_git_release_state(*, include_remote: bool, timeout_s: int) -> dict[str, Any]:
    commit = git_value(["git", "rev-parse", "HEAD"], timeout_s=10)
    branch = git_value(["git", "branch", "--show-current"], timeout_s=10)
    remote = git_value(["git", "remote", "get-url", "origin"], timeout_s=10)
    status_porcelain = git_value(["git", "status", "--porcelain"], timeout_s=10)
    state: dict[str, Any] = {
        "commit": commit,
        "branch": branch,
        "remote": remote,
        "expected_remote": EXPECTED_ORIGIN_REMOTE,
        "expected_branch": EXPECTED_RELEASE_BRANCH,
        "worktree_clean": status_porcelain == "",
        "dirty_entries": status_porcelain.splitlines()[:50],
        "remote_ref": f"refs/heads/{EXPECTED_RELEASE_BRANCH}",
        "remote_head": "",
        "remote_error": "",
        "head_matches_remote": False,
        "remote_checked": include_remote,
    }
    if include_remote:
        remote_ref = f"refs/heads/{EXPECTED_RELEASE_BRANCH}"
        state["remote_check_method"] = ""
        state["remote_check_errors"] = []
        per_attempt_timeout = min(max(timeout_s, 1), 30)
        for method, command, env in git_remote_head_candidates(remote, remote_ref):
            remote_output, remote_error = git_value_with_error(
                command,
                timeout_s=per_attempt_timeout,
                env=env,
            )
            remote_head = remote_head_from_ls_remote(remote_output, remote_ref)
            if remote_head:
                state["remote_head"] = remote_head
                state["remote_check_method"] = method
                state["remote_error"] = ""
                break
            if remote_output and not remote_error:
                remote_error = f"remote output did not include a valid {remote_ref} SHA: {remote_output[-1000:]}"
            if remote_error:
                state["remote_check_errors"].append({"method": method, "error": remote_error[-1000:]})
                state["remote_error"] = remote_error
        state["head_matches_remote"] = bool(commit and state["remote_head"] == commit)
    return state


def git_release_state_blocking_items(args: argparse.Namespace, state: dict[str, Any] | None) -> list[str]:
    if args.profile != "production":
        return []
    if not state:
        return ["git release state must be available"]
    blocking: list[str] = []
    if state.get("branch") != EXPECTED_RELEASE_BRANCH:
        blocking.append(f"git branch must be {EXPECTED_RELEASE_BRANCH}")
    if state.get("remote") != EXPECTED_ORIGIN_REMOTE:
        blocking.append(f"git origin remote must be {EXPECTED_ORIGIN_REMOTE}")
    if state.get("worktree_clean") is not True:
        blocking.append("git worktree must be clean")
    if state.get("remote_checked") is not True or not state.get("remote_head"):
        blocking.append(f"git remote {EXPECTED_RELEASE_BRANCH} HEAD must be readable")
    elif state.get("head_matches_remote") is not True:
        blocking.append(f"git HEAD must match origin/{EXPECTED_RELEASE_BRANCH}")
    return blocking


def build_report(args: argparse.Namespace) -> dict[str, Any]:
    apply_first_usable_mode(args)
    loaded_env, file_sensitive_values = load_env_file(args.env_file)
    env = os.environ.copy()
    env.update(loaded_env)
    redact = redactor([*file_sensitive_values, *sensitive_values_from_env(env)])

    python = "python3"
    steps: list[dict[str, Any]] = []

    static_command = [python, "infra/scripts/first_usable_release_check.py"]
    steps.append(run_command("static_readiness", static_command, env=env, timeout_s=args.timeout_s, redact=redact))

    export_command = [
        ai_python(),
        "-m",
        "app.evaluation.export_format_eval",
        "--input",
        "../docs/sample_docs/golden/工程1.export.json",
        "--json",
    ]
    if args.profile == "production":
        export_command.append("--require-pdf")
    steps.append(
        run_json_artifact_command(
            "export_format_eval",
            export_command,
            EXPORT_FORMAT_JSON,
            env=env,
            timeout_s=args.timeout_s,
            redact=redact,
            cwd=AI_SERVICE,
        )
    )

    if args.env_file or args.profile == "production":
        audit_command = [python, "infra/scripts/first_usable_release_check.py", "--audit-production-env"]
        if args.env_file:
            audit_command.extend(["--env-file", str(args.env_file)])
        steps.append(run_command("production_env_audit", audit_command, env=env, timeout_s=args.timeout_s, redact=redact))
        audit_json_command = [python, "infra/scripts/first_usable_release_check.py", "--audit-production-env-json"]
        if args.env_file:
            audit_json_command.extend(["--env-file", str(args.env_file)])
        steps.append(
            run_json_artifact_command(
                "production_env_audit_json",
                audit_json_command,
                PRODUCTION_ENV_AUDIT_JSON,
                env=env,
                timeout_s=args.timeout_s,
                redact=redact,
            )
        )

    if args.profile == "production":
        clear_artifact(PROVIDER_CANARY_JSON)
        clear_artifact(OCR_CANARY_JSON)
        production_command = [python, "infra/scripts/first_usable_release_check.py", "--profile", "production"]
        if args.env_file:
            production_command.extend(["--env-file", str(args.env_file)])
        production_command.extend(
            [
                "--provider-canary-json-output",
                str(PROVIDER_CANARY_JSON.relative_to(ROOT)),
                "--ocr-canary-json-output",
                str(OCR_CANARY_JSON.relative_to(ROOT)),
            ]
        )
        steps.append(run_command("production_readiness", production_command, env=env, timeout_s=args.timeout_s, redact=redact))
    else:
        steps.append(
            run_command(
                "local_readiness_canaries",
                [python, "infra/scripts/first_usable_release_check.py", "--run-canaries"],
                env=env,
                timeout_s=args.timeout_s,
                redact=redact,
            )
        )

    if args.include_repo_check:
        repo_check_env = os.environ.copy()
        steps.append(
            run_command(
                "repo_wide_check",
                ["./infra/scripts/check.sh"],
                env=repo_check_env,
                timeout_s=args.timeout_s,
                redact=redact,
            )
        )
    if args.include_project1_runtime:
        clear_artifact(PROJECT1_RUNTIME_JSON)
        steps.append(
            run_command(
                "project1_runtime_acceptance",
                [
                    python,
                    "infra/scripts/acceptance_project1_check.py",
                    "--json-output",
                    "tmp/project1_runtime_acceptance.json",
                ],
                env=env,
                timeout_s=args.timeout_s,
                redact=redact,
            )
        )

    artifacts = [
        artifact
        for artifact in (
            artifact_summary("production_env_audit_json", PRODUCTION_ENV_AUDIT_JSON)
            if args.env_file or args.profile == "production"
            else None,
            artifact_summary("export_format_eval", EXPORT_FORMAT_JSON),
            artifact_summary("provider_canary", PROVIDER_CANARY_JSON)
            if args.profile == "production"
            else None,
            artifact_summary("ocr_provider_canary", OCR_CANARY_JSON)
            if args.profile == "production"
            else None,
            artifact_summary("project1_runtime_acceptance", PROJECT1_RUNTIME_JSON)
            if args.include_project1_runtime
            else None,
        )
        if artifact is not None
    ]
    git_release_state = collect_git_release_state(
        include_remote=args.profile == "production",
        timeout_s=min(max(args.timeout_s, 1), 60),
    )
    step_status = {step["name"]: step["status"] for step in steps}
    blocking_requirements = blocking_items(args, step_status, artifacts, git_release_state)
    return {
        "name": "first_usable_release_report",
        "generated_at": datetime.now(UTC).isoformat(timespec="seconds"),
        "profile": args.profile,
        "commit": git_release_state["commit"],
        "branch": git_release_state["branch"],
        "remote": git_release_state["remote"],
        "git_release_state": git_release_state,
        "env_file": str(args.env_file) if args.env_file else None,
        "first_usable_mode": bool(getattr(args, "first_usable", False)),
        "include_repo_check": args.include_repo_check,
        "include_project1_runtime": args.include_project1_runtime,
        "steps": steps,
        "artifacts": artifacts,
        "blocking_requirements": blocking_requirements,
        "loop_can_end": not blocking_requirements,
    }


def apply_first_usable_mode(args: argparse.Namespace) -> None:
    if not bool(getattr(args, "first_usable", False)):
        return
    args.profile = "production"
    args.include_repo_check = True
    args.include_project1_runtime = True


def artifact_has_passed(summary: dict[str, Any] | None) -> bool:
    return bool(
        summary
        and summary.get("status") == "present"
        and summary.get("json_status") == "passed"
        and summary.get("semantic_status") == "passed"
    )


def required_artifact_statuses(
    artifacts: list[dict[str, Any]] | None,
    required: tuple[tuple[str, str], ...],
) -> list[str]:
    indexed = {str(artifact.get("name")): artifact for artifact in artifacts or []}
    return [
        message
        for name, message in required
        if not artifact_has_passed(indexed.get(name))
    ]


def blocking_items(
    args: argparse.Namespace,
    step_status: dict[str, str],
    artifacts: list[dict[str, Any]] | None = None,
    git_release_state: dict[str, Any] | None = None,
) -> list[str]:
    blocking: list[str] = []
    if step_status.get("static_readiness") != "passed":
        blocking.append("static readiness gate must pass")
    if step_status.get("export_format_eval") != "passed":
        blocking.append("export format evaluation must pass")
    if args.profile != "production":
        blocking.append("report profile must be production")
    if args.profile == "production":
        if step_status.get("production_env_audit") != "passed":
            blocking.append("production env audit must pass")
        if step_status.get("production_readiness") != "passed":
            blocking.append("production Provider/OCR readiness must pass")
        blocking.extend(required_artifact_statuses(artifacts, PRODUCTION_REQUIRED_ARTIFACTS))
        blocking.extend(git_release_state_blocking_items(args, git_release_state))
    if not args.include_repo_check:
        blocking.append("repo-wide check must be included")
    elif step_status.get("repo_wide_check") != "passed":
        blocking.append("repo-wide check must pass")
    if not args.include_project1_runtime:
        blocking.append("project1 runtime acceptance must be included")
    elif step_status.get("project1_runtime_acceptance") != "passed":
        blocking.append("project1 runtime acceptance must pass")
    else:
        blocking.extend(required_artifact_statuses(artifacts, PROJECT1_REQUIRED_ARTIFACTS))
    return blocking


def write_report(report: dict[str, Any], output: Path | None) -> None:
    data = json.dumps(report, ensure_ascii=False, indent=2) + "\n"
    if output is None:
        print(data, end="")
        return
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(data, encoding="utf-8")
    print(f"[ok] first usable release report written: {output}")
    print(f"[ok] loop_can_end: {report['loop_can_end']}")
    if report["blocking_requirements"]:
        print("[info] blocking requirements:")
        for item in report["blocking_requirements"]:
            print(f"- {item}")


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate a redacted first usable release evidence report.")
    parser.add_argument("--profile", choices=("local", "production"), default="local")
    parser.add_argument("--env-file", type=Path)
    parser.add_argument("--output", type=Path, help="Write JSON report to this path. Prints JSON when omitted.")
    parser.add_argument(
        "--first-usable",
        action="store_true",
        help="Run the full first-usable production evidence bundle; implies production, repo check, and project1 runtime.",
    )
    parser.add_argument("--include-repo-check", action="store_true", help="Run ./infra/scripts/check.sh and include it in loop_can_end.")
    parser.add_argument("--include-project1-runtime", action="store_true", help="Run docs/ex/工程1 runtime HTTP acceptance and include it in loop_can_end.")
    parser.add_argument("--timeout-s", type=int, default=1800)
    args = parser.parse_args()

    report = build_report(args)
    write_report(report, args.output)
    return 0 if not report["blocking_requirements"] else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ReportError as exc:
        print(f"[error] {exc}")
        raise SystemExit(1)

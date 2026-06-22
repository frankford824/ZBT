#!/usr/bin/env python3
"""Generate a redacted first-usable release evidence report."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shlex
import subprocess
import time
from datetime import UTC, datetime
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SENSITIVE_ENV_RE = re.compile(r"(SECRET|TOKEN|PASSWORD|API_KEY|ACCESS_KEY|HMAC|JWT)", re.IGNORECASE)
URL_WITH_PASSWORD_RE = re.compile(r"([a-z][a-z0-9+.-]*://[^:/@\s]+:)([^@\s]+)(@)", re.IGNORECASE)
BEARER_RE = re.compile(r"(Bearer\s+)[A-Za-z0-9._~+/=-]+", re.IGNORECASE)
PRODUCTION_ENV_AUDIT_JSON = ROOT / "tmp/production_env_audit.json"
PROVIDER_CANARY_JSON = ROOT / "tmp/provider_canary.json"
OCR_CANARY_JSON = ROOT / "tmp/ocr_provider_canary.json"
PROJECT1_RUNTIME_JSON = ROOT / "tmp/project1_runtime_acceptance.json"


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
        if value and (SENSITIVE_ENV_RE.search(key) or key in {"DATABASE_URL", "MIGRATION_DATABASE_URL", "REDIS_URL"}):
            sensitive_values.append(value)
    return loaded, sensitive_values


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
    started = time.monotonic()
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
) -> dict[str, Any]:
    started = time.monotonic()
    clear_artifact(artifact)
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
    try:
        parsed = json.loads(content.decode("utf-8"))
    except json.JSONDecodeError:
        parsed = None
    if isinstance(parsed, dict):
        parsed_status = str(parsed.get("status") or "unknown")
        parsed_name = str(parsed.get("name") or "")
    return {
        "name": name,
        "path": str(path.relative_to(ROOT)),
        "status": "present",
        "bytes": len(content),
        "sha256": hashlib.sha256(content).hexdigest(),
        "json_name": parsed_name,
        "json_status": parsed_status,
    }


def git_value(command: list[str]) -> str:
    completed = subprocess.run(
        command,
        cwd=str(ROOT),
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    return completed.stdout.strip()


def build_report(args: argparse.Namespace) -> dict[str, Any]:
    loaded_env, sensitive_values = load_env_file(args.env_file)
    env = os.environ.copy()
    env.update(loaded_env)
    redact = redactor(sensitive_values)

    python = "python3"
    steps: list[dict[str, Any]] = []

    static_command = [python, "infra/scripts/first_usable_release_check.py"]
    steps.append(run_command("static_readiness", static_command, env=env, timeout_s=args.timeout_s, redact=redact))

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
        steps.append(run_command("repo_wide_check", ["./infra/scripts/check.sh"], env=env, timeout_s=args.timeout_s, redact=redact))
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

    step_status = {step["name"]: step["status"] for step in steps}
    blocking_requirements = blocking_items(args, step_status)
    artifacts = [
        artifact
        for artifact in (
            artifact_summary("production_env_audit_json", PRODUCTION_ENV_AUDIT_JSON)
            if args.env_file or args.profile == "production"
            else None,
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
    return {
        "name": "first_usable_release_report",
        "generated_at": datetime.now(UTC).isoformat(timespec="seconds"),
        "profile": args.profile,
        "commit": git_value(["git", "rev-parse", "HEAD"]),
        "branch": git_value(["git", "branch", "--show-current"]),
        "remote": git_value(["git", "remote", "get-url", "origin"]),
        "env_file": str(args.env_file) if args.env_file else None,
        "include_repo_check": args.include_repo_check,
        "include_project1_runtime": args.include_project1_runtime,
        "steps": steps,
        "artifacts": artifacts,
        "blocking_requirements": blocking_requirements,
        "loop_can_end": not blocking_requirements,
    }


def blocking_items(args: argparse.Namespace, step_status: dict[str, str]) -> list[str]:
    blocking: list[str] = []
    if step_status.get("static_readiness") != "passed":
        blocking.append("static readiness gate must pass")
    if args.profile != "production":
        blocking.append("report profile must be production")
    if args.profile == "production":
        if step_status.get("production_env_audit") != "passed":
            blocking.append("production env audit must pass")
        if step_status.get("production_readiness") != "passed":
            blocking.append("production Provider/OCR readiness must pass")
    if not args.include_repo_check:
        blocking.append("repo-wide check must be included")
    elif step_status.get("repo_wide_check") != "passed":
        blocking.append("repo-wide check must pass")
    if not args.include_project1_runtime:
        blocking.append("project1 runtime acceptance must be included")
    elif step_status.get("project1_runtime_acceptance") != "passed":
        blocking.append("project1 runtime acceptance must pass")
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

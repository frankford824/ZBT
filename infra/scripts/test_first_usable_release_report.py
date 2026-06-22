#!/usr/bin/env python3
"""Regression tests for first_usable_release_report.py."""

from __future__ import annotations

import argparse
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import first_usable_release_report as report


def valid_production_env_payload() -> dict[str, object]:
    return {
        "name": "production_env_audit",
        "status": "passed",
        "issues": [],
        "evidence": {
            "providers": ["openai_compatible_primary"],
            "routes": [
                {
                    "route": "chapter_generate",
                    "kind": "llm",
                    "provider": "openai_compatible_primary",
                    "model": "gpt-4o-mini",
                },
                {
                    "route": "knowledge_embedding",
                    "kind": "embedding",
                    "provider": "openai_compatible_primary",
                    "model": "text-embedding-3-large",
                },
                {
                    "route": "knowledge_rerank",
                    "kind": "rerank",
                    "provider": "openai_compatible_primary",
                    "model": "gpt-4o-mini",
                },
            ],
            "ocr_provider": "http_ocr",
            "provider_requirements": [
                {
                    "provider": "openai_compatible_primary",
                    "configured_envs": ["OPENAI_API_KEY"],
                    "issues": [],
                }
            ],
            "ocr_requirement": {"provider": "http_ocr", "configured_envs": ["OCR_HTTP_ENDPOINT"], "issues": []},
            "pricing_matches": [
                {"route": "chapter_generate", "matched": True},
                {"route": "knowledge_embedding", "matched": True},
                {"route": "knowledge_rerank", "matched": True},
            ],
        },
    }


def failed_production_env_payload() -> dict[str, object]:
    payload = valid_production_env_payload()
    payload["status"] = "failed"
    payload["issues"] = [
        "production env DATABASE_URL still uses a placeholder value",
        "production env missing openai_compatible_primary credentials: set one of OPENAI_API_KEY",
        "production env OCR_HTTP_ENDPOINT still uses a placeholder value",
    ]
    evidence = payload["evidence"]
    assert isinstance(evidence, dict)
    provider_requirements = evidence["provider_requirements"]
    assert isinstance(provider_requirements, list)
    provider_requirements[0]["required_env_groups"] = [["OPENAI_API_KEY"]]
    provider_requirements[0]["configured_envs"] = []
    provider_requirements[0]["issues"] = ["production env missing openai_compatible_primary credentials: set one of OPENAI_API_KEY"]
    evidence["ocr_requirement"] = {
        "provider": "http_ocr",
        "required_env_groups": [["OCR_HTTP_ENDPOINT"]],
        "configured_envs": [],
        "issues": ["production env OCR_HTTP_ENDPOINT still uses a placeholder value"],
    }
    return payload


def valid_provider_canary_payload() -> dict[str, object]:
    checks = [{"name": "router.load", "passed": True}]
    routes = []
    for route, kind in report.PROVIDER_ROUTE_KINDS.items():
        routes.append(
            {
                "route": route,
                "kind": kind,
                "resolved": True,
                "provider": "openai_compatible_primary",
                "model": "gpt-4o-mini",
                "call": {"passed": True},
                "accounting": {"estimated_cost": 0.001},
            }
        )
        checks.extend(
            [
                {"name": f"route.{route}.non_mock_provider", "passed": True},
                {"name": f"route.{route}.call_provider", "passed": True},
                {"name": f"route.{route}.estimated_cost", "passed": True},
            ]
        )
    return {
        "name": "provider_canary",
        "status": "passed",
        "strict": True,
        "call_provider": True,
        "require_cost": True,
        "passed_checks": len(checks),
        "failed_checks": 0,
        "total_checks": len(checks),
        "routes": routes,
        "checks": checks,
    }


def valid_ocr_canary_payload() -> dict[str, object]:
    checks = [
        {"name": check_name, "passed": True}
        for check_name in report.REQUIRED_OCR_CHECKS
    ]
    return {
        "name": "ocr_provider_eval",
        "status": "passed",
        "provider": "http_ocr",
        "passed_checks": len(checks),
        "failed_checks": 0,
        "total_checks": len(checks),
        "checks": checks,
        "metadata": {
            "table_block_count": 1,
            "ocr": {"provider": "http_ocr"},
        },
    }


def valid_export_format_payload() -> dict[str, object]:
    checks = [{"name": check_name, "passed": True} for check_name in report.REQUIRED_EXPORT_CHECKS]
    return {
        "name": "工程1.export",
        "status": "passed",
        "passed_checks": len(checks),
        "failed_checks": 0,
        "total_checks": len(checks),
        "docx": {"size_bytes": 4096, "table_count": 1},
        "zip": {"entry_count": 6, "docx_entry_count": 2, "manifest_issues": []},
        "pdf": {
            "status": "generated",
            "page_count": 2,
            "text_chars": 128,
            "first_page_nonblank": True,
        },
        "checks": checks,
    }


def valid_project1_runtime_payload() -> dict[str, object]:
    return {
        "name": "project1_runtime_acceptance",
        "status": "passed",
        "sample_files": [
            {"label": "tender_pdf"},
            {"label": "response_docx"},
            {"label": "boq_xlsx"},
        ],
        "steps": {
            "parse_response_matrix": {
                "requirements": 35,
                "expected_response": 35,
                "mandatory": 20,
                "high_priority": 30,
                "requirements_xlsx_bytes": 2048,
            },
            "companion_knowledge": {
                "response_doc_status": "processed",
                "boq_doc_status": "processed",
                "search_items": 1,
                "selected_ref_has_reference_id": True,
                "selected_ref_has_location": True,
            },
            "generation_coverage_compliance": {
                "source_refs": 1,
                "compliance_result_status": "pass",
                "requirements": 1,
                "coverage_rows": 1,
            },
            "docx_export": {"download_ready": True, "filename": "工程1.docx"},
        },
    }


def valid_git_release_state() -> dict[str, object]:
    commit = "abc123"
    return {
        "commit": commit,
        "branch": report.EXPECTED_RELEASE_BRANCH,
        "remote": report.EXPECTED_ORIGIN_REMOTE,
        "expected_remote": report.EXPECTED_ORIGIN_REMOTE,
        "expected_branch": report.EXPECTED_RELEASE_BRANCH,
        "worktree_clean": True,
        "dirty_entries": [],
        "remote_ref": f"refs/heads/{report.EXPECTED_RELEASE_BRANCH}",
        "remote_head": commit,
        "remote_error": "",
        "remote_check_method": "origin",
        "remote_check_errors": [],
        "head_matches_remote": True,
        "remote_checked": True,
    }


class FirstUsableReleaseReportTest(unittest.TestCase):
    def test_load_env_file_collects_sensitive_values(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            env_file = Path(directory) / ".env.production"
            env_file.write_text(
                "\n".join(
                    [
                        "# comment",
                        'export OPENAI_API_KEY="sk-production-secret"',
                        "DATABASE_URL=postgres://zbt:db-password@example.com/zbt",
                        "SAFE_FLAG=visible",
                    ]
                ),
                encoding="utf-8",
            )

            loaded, sensitive_values = report.load_env_file(env_file)

        self.assertEqual(loaded["OPENAI_API_KEY"], "sk-production-secret")
        self.assertEqual(loaded["DATABASE_URL"], "postgres://zbt:db-password@example.com/zbt")
        self.assertEqual(loaded["SAFE_FLAG"], "visible")
        self.assertIn("sk-production-secret", sensitive_values)
        self.assertIn("postgres://zbt:db-password@example.com/zbt", sensitive_values)
        self.assertNotIn("visible", sensitive_values)

    def test_redactor_masks_secrets_urls_and_bearer_tokens(self) -> None:
        redact = report.redactor(["sk-production-secret"])

        output = redact(
            "OPENAI_API_KEY=sk-production-secret "
            "DATABASE_URL=postgres://zbt:db-password@example.com/zbt "
            "Authorization: Bearer token-value-123"
        )

        self.assertNotIn("sk-production-secret", output)
        self.assertNotIn("db-password", output)
        self.assertNotIn("token-value-123", output)
        self.assertIn("OPENAI_API_KEY=<redacted>", output)
        self.assertIn("postgres://zbt:<redacted>@example.com/zbt", output)
        self.assertIn("Bearer <redacted>", output)

    def test_build_report_redacts_sensitive_process_environment_values(self) -> None:
        args = argparse.Namespace(
            profile="local",
            env_file=None,
            include_repo_check=False,
            include_project1_runtime=False,
            timeout_s=30,
        )

        def fake_run_command(name: str, _: list[str], **kwargs: object) -> dict[str, object]:
            redact = kwargs["redact"]
            assert callable(redact)
            output = redact(
                "OPENAI_API_KEY=sk-env-secret "
                "DATABASE_URL=postgres://zbt:db-password@example.com/zbt"
            )
            return {"name": name, "status": "passed", "returncode": 0, "output": output}

        with (
            patch.dict(
                os.environ,
                {
                    "OPENAI_API_KEY": "sk-env-secret",
                    "DATABASE_URL": "postgres://zbt:db-password@example.com/zbt",
                },
            ),
            patch.object(report, "run_command", side_effect=fake_run_command),
            patch.object(
                report,
                "run_json_artifact_command",
                return_value={"name": "export_format_eval", "status": "passed", "returncode": 0},
            ),
            patch.object(
                report,
                "artifact_summary",
                return_value={
                    "name": "export_format_eval",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
            ),
            patch.object(report, "collect_git_release_state", return_value=valid_git_release_state()),
        ):
            generated = report.build_report(args)

        combined_output = "\n".join(str(step.get("output") or "") for step in generated["steps"])
        self.assertNotIn("sk-env-secret", combined_output)
        self.assertNotIn("db-password", combined_output)
        self.assertIn("OPENAI_API_KEY=<redacted>", combined_output)
        self.assertIn("DATABASE_URL=<redacted>", combined_output)

    def test_blocking_items_require_production_repo_check_and_project1_runtime(self) -> None:
        args = argparse.Namespace(profile="local", include_repo_check=False, include_project1_runtime=False)
        blocking = report.blocking_items(
            args,
            {"static_readiness": "passed", "export_format_eval": "passed", "local_readiness_canaries": "passed"},
        )

        self.assertEqual(
            blocking,
            [
                "report profile must be production",
                "repo-wide check must be included",
                "project1 runtime acceptance must be included",
            ],
        )

    def test_first_usable_mode_implies_full_production_evidence_bundle(self) -> None:
        args = argparse.Namespace(
            profile="local",
            first_usable=True,
            env_file=None,
            include_repo_check=False,
            include_project1_runtime=False,
            timeout_s=30,
        )
        commands: list[tuple[str, list[str]]] = []

        def fake_run_command(name: str, command: list[str], **_: object) -> dict[str, object]:
            commands.append((name, command))
            return {"name": name, "status": "passed", "returncode": 0}

        def fake_json_artifact_command(name: str, _: list[str], __: Path, **___: object) -> dict[str, object]:
            return {"name": name, "status": "passed", "returncode": 0}

        def fake_artifact_summary(name: str, path: Path) -> dict[str, object]:
            return {
                "name": name,
                "path": str(path.relative_to(report.ROOT)),
                "status": "present",
                "json_status": "passed",
                "semantic_status": "passed",
            }

        with tempfile.TemporaryDirectory(dir=report.ROOT / "tmp") as directory:
            artifact_dir = Path(directory)
            with (
                patch.object(report, "PROVIDER_CANARY_JSON", artifact_dir / "provider_canary.json"),
                patch.object(report, "OCR_CANARY_JSON", artifact_dir / "ocr_provider_canary.json"),
                patch.object(report, "PROJECT1_RUNTIME_JSON", artifact_dir / "project1_runtime_acceptance.json"),
                patch.object(report, "load_env_file", return_value=({}, [])),
                patch.object(report, "run_command", side_effect=fake_run_command),
                patch.object(report, "run_json_artifact_command", side_effect=fake_json_artifact_command),
                patch.object(report, "artifact_summary", side_effect=fake_artifact_summary),
                patch.object(report, "collect_git_release_state", return_value=valid_git_release_state()),
            ):
                generated = report.build_report(args)

        command_names = [name for name, _ in commands]
        self.assertEqual(generated["profile"], "production")
        self.assertTrue(generated["first_usable_mode"])
        self.assertTrue(generated["include_repo_check"])
        self.assertTrue(generated["include_project1_runtime"])
        self.assertIn("production_readiness", command_names)
        self.assertIn("repo_wide_check", command_names)
        self.assertIn("project1_runtime_acceptance", command_names)
        self.assertEqual(generated["blocking_requirements"], [])

    def test_repo_wide_check_does_not_inherit_loaded_production_env(self) -> None:
        args = argparse.Namespace(
            profile="local",
            first_usable=True,
            env_file=Path(".env.production"),
            include_repo_check=False,
            include_project1_runtime=False,
            timeout_s=30,
        )
        command_envs: dict[str, dict[str, str]] = {}

        def fake_run_command(name: str, _: list[str], **kwargs: object) -> dict[str, object]:
            env = kwargs["env"]
            self.assertIsInstance(env, dict)
            command_envs[name] = dict(env)
            return {"name": name, "status": "passed", "returncode": 0}

        def fake_json_artifact_command(name: str, _: list[str], __: Path, **___: object) -> dict[str, object]:
            return {"name": name, "status": "passed", "returncode": 0}

        def fake_artifact_summary(name: str, path: Path) -> dict[str, object]:
            return {
                "name": name,
                "path": str(path.relative_to(report.ROOT)),
                "status": "present",
                "json_status": "passed",
                "semantic_status": "passed",
            }

        loaded_env = {
            "OPENAI_API_KEY": "sk-production-secret",
            "ZBT_PRODUCTION_ONLY_SENTINEL": "production-env-file-only",
        }
        with tempfile.TemporaryDirectory(dir=report.ROOT / "tmp") as directory:
            artifact_dir = Path(directory)
            with (
                patch.dict(os.environ, {}, clear=True),
                patch.object(report, "PROVIDER_CANARY_JSON", artifact_dir / "provider_canary.json"),
                patch.object(report, "OCR_CANARY_JSON", artifact_dir / "ocr_provider_canary.json"),
                patch.object(report, "PROJECT1_RUNTIME_JSON", artifact_dir / "project1_runtime_acceptance.json"),
                patch.object(report, "load_env_file", return_value=(loaded_env, ["sk-production-secret"])),
                patch.object(report, "run_command", side_effect=fake_run_command),
                patch.object(report, "run_json_artifact_command", side_effect=fake_json_artifact_command),
                patch.object(report, "artifact_summary", side_effect=fake_artifact_summary),
                patch.object(report, "collect_git_release_state", return_value=valid_git_release_state()),
            ):
                generated = report.build_report(args)

        self.assertTrue(generated["first_usable_mode"])
        self.assertEqual(command_envs["production_readiness"]["ZBT_PRODUCTION_ONLY_SENTINEL"], "production-env-file-only")
        self.assertNotIn("ZBT_PRODUCTION_ONLY_SENTINEL", command_envs["repo_wide_check"])

    def test_blocking_items_allow_complete_production_evidence(self) -> None:
        args = argparse.Namespace(profile="production", include_repo_check=True, include_project1_runtime=True)
        blocking = report.blocking_items(
            args,
            {
                "static_readiness": "passed",
                "export_format_eval": "passed",
                "production_env_audit": "passed",
                "production_readiness": "passed",
                "repo_wide_check": "passed",
                "project1_runtime_acceptance": "passed",
            },
            [
                {
                    "name": "production_env_audit_json",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {"name": "export_format_eval", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                {"name": "provider_canary", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                {
                    "name": "ocr_provider_canary",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {
                    "name": "project1_runtime_acceptance",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
            ],
            valid_git_release_state(),
        )

        self.assertEqual(blocking, [])

    def test_blocking_items_require_clean_synced_git_release_state(self) -> None:
        args = argparse.Namespace(profile="production", include_repo_check=True, include_project1_runtime=True)
        dirty_state = valid_git_release_state()
        dirty_state.update(
            {
                "branch": "codex/test",
                "remote": "https://github.com/frankford824/ZBT.git",
                "worktree_clean": False,
                "dirty_entries": [" M README.md"],
                "remote_head": "def456",
                "head_matches_remote": False,
            }
        )
        blocking = report.blocking_items(
            args,
            {
                "static_readiness": "passed",
                "export_format_eval": "passed",
                "production_env_audit": "passed",
                "production_readiness": "passed",
                "repo_wide_check": "passed",
                "project1_runtime_acceptance": "passed",
            },
            [
                {
                    "name": "production_env_audit_json",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {"name": "export_format_eval", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                {"name": "provider_canary", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                {
                    "name": "ocr_provider_canary",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {
                    "name": "project1_runtime_acceptance",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
            ],
            dirty_state,
        )

        self.assertEqual(
            blocking,
            [
                "git branch must be main",
                "git origin remote must be git@github.com:frankford824/ZBT.git",
                "git worktree must be clean",
                "git HEAD must match origin/main",
            ],
        )

    def test_blocking_items_require_readable_remote_head(self) -> None:
        args = argparse.Namespace(profile="production", include_repo_check=True, include_project1_runtime=True)
        git_state = valid_git_release_state()
        git_state.update({"remote_head": "", "remote_error": "timeout", "head_matches_remote": False})

        blocking = report.git_release_state_blocking_items(args, git_state)

        self.assertEqual(blocking, ["git remote main HEAD must be readable"])

    def test_remote_head_from_ls_remote_ignores_ssh_noise(self) -> None:
        commit = "a" * 40

        parsed = report.remote_head_from_ls_remote(
            f"Connection to github.com closed by remote host.\n{commit}\trefs/heads/main\n",
            "refs/heads/main",
        )

        self.assertEqual(parsed, commit)
        self.assertEqual(report.remote_head_from_ls_remote("Connection closed\n", "refs/heads/main"), "")

    def test_collect_git_release_state_uses_gh_https_fallback_for_remote_head(self) -> None:
        commit = "b" * 40

        def fake_git_value(command: list[str], **_: object) -> str:
            if command == ["git", "rev-parse", "HEAD"]:
                return commit
            if command == ["git", "branch", "--show-current"]:
                return report.EXPECTED_RELEASE_BRANCH
            if command == ["git", "remote", "get-url", "origin"]:
                return report.EXPECTED_ORIGIN_REMOTE
            if command == ["git", "status", "--porcelain"]:
                return ""
            raise AssertionError(f"unexpected git value command: {command}")

        with (
            patch.object(report, "git_value", side_effect=fake_git_value),
            patch.object(report.shutil, "which", return_value="/home/wsfwk/.local/bin/gh"),
            patch.object(
                report,
                "git_value_with_error",
                side_effect=[
                    ("", "timed out after 30s"),
                    (f"{commit}\trefs/heads/{report.EXPECTED_RELEASE_BRANCH}", ""),
                ],
            ) as remote_call,
        ):
            state = report.collect_git_release_state(include_remote=True, timeout_s=60)

        self.assertEqual(state["remote_head"], commit)
        self.assertEqual(state["remote_check_method"], "github_https_gh")
        self.assertEqual(state["remote_check_errors"], [{"method": "origin", "error": "timed out after 30s"}])
        self.assertTrue(state["head_matches_remote"])
        self.assertEqual(remote_call.call_count, 2)

    def test_blocking_items_require_semantically_valid_artifacts(self) -> None:
        args = argparse.Namespace(profile="production", include_repo_check=True, include_project1_runtime=True)
        blocking = report.blocking_items(
            args,
            {
                "static_readiness": "passed",
                "export_format_eval": "passed",
                "production_env_audit": "passed",
                "production_readiness": "passed",
                "repo_wide_check": "passed",
                "project1_runtime_acceptance": "passed",
            },
            [
                {
                    "name": "production_env_audit_json",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {"name": "export_format_eval", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                {"name": "provider_canary", "status": "present", "json_status": "passed", "semantic_status": "failed"},
                {
                    "name": "ocr_provider_canary",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {
                    "name": "project1_runtime_acceptance",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
            ],
            valid_git_release_state(),
        )

        self.assertEqual(blocking, ["production artifact provider_canary must be present and passed"])

    def test_blocking_items_require_production_artifacts_to_pass(self) -> None:
        args = argparse.Namespace(profile="production", include_repo_check=True, include_project1_runtime=True)
        blocking = report.blocking_items(
            args,
            {
                "static_readiness": "passed",
                "export_format_eval": "passed",
                "production_env_audit": "passed",
                "production_readiness": "passed",
                "repo_wide_check": "passed",
                "project1_runtime_acceptance": "passed",
            },
            [
                {"name": "production_env_audit_json", "status": "present", "json_status": "failed"},
                {"name": "export_format_eval", "status": "missing"},
                {"name": "provider_canary", "status": "missing"},
                {"name": "ocr_provider_canary", "status": "present", "json_status": "not_json"},
                {
                    "name": "project1_runtime_acceptance",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
            ],
            valid_git_release_state(),
        )

        self.assertEqual(
            blocking,
            [
                "export format artifact must be present and passed",
                "production artifact production_env_audit_json must be present and passed",
                "production artifact provider_canary must be present and passed",
                "production artifact ocr_provider_canary must be present and passed",
            ],
        )

    def test_next_actions_explain_remaining_production_inputs(self) -> None:
        args = argparse.Namespace(
            profile="production",
            env_file=Path(".env.production.example"),
            include_repo_check=True,
            include_project1_runtime=True,
        )
        tmp_root = report.ROOT / "tmp"
        tmp_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=tmp_root) as directory:
            audit_artifact = Path(directory) / "production_env_audit.json"
            audit_artifact.write_text(json.dumps(failed_production_env_payload()), encoding="utf-8")
            audit_summary = report.artifact_summary("production_env_audit_json", audit_artifact)

            actions = report.build_next_actions(
                args,
                {
                    "static_readiness": "passed",
                    "export_format_eval": "passed",
                    "production_env_audit": "failed",
                    "production_readiness": "failed",
                    "repo_wide_check": "passed",
                    "project1_runtime_acceptance": "passed",
                },
                [
                    audit_summary,
                    {"name": "export_format_eval", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                    {"name": "provider_canary", "status": "present", "json_status": "failed", "semantic_status": "failed"},
                    {"name": "ocr_provider_canary", "status": "present", "json_status": "failed", "semantic_status": "failed"},
                    {
                        "name": "project1_runtime_acceptance",
                        "status": "present",
                        "json_status": "passed",
                        "semantic_status": "passed",
                    },
                ],
                [
                    "production env audit must pass",
                    "production Provider/OCR readiness must pass",
                    "production artifact provider_canary must be present and passed",
                    "production artifact ocr_provider_canary must be present and passed",
                ],
            )

        actions_by_id = {str(action["id"]): action for action in actions}
        self.assertIn("production_env_inputs", actions_by_id)
        self.assertIn("provider_canary_live_calls", actions_by_id)
        self.assertIn("ocr_provider_live_check", actions_by_id)
        self.assertIn("final_first_usable_report", actions_by_id)
        env_action = actions_by_id["production_env_inputs"]
        self.assertEqual(env_action["env_file"], ".env.production")
        self.assertIn("DATABASE_URL", env_action["missing_or_placeholder_env_keys"])
        self.assertIn("OPENAI_API_KEY", env_action["missing_or_placeholder_env_keys"])
        self.assertIn("OCR_HTTP_ENDPOINT", env_action["missing_or_placeholder_env_keys"])
        self.assertEqual(env_action["provider_requirements"][0]["provider"], "openai_compatible_primary")
        self.assertEqual(env_action["ocr_requirement"]["provider"], "http_ocr")

    def test_blocking_items_require_project1_runtime_artifact_to_pass(self) -> None:
        args = argparse.Namespace(profile="production", include_repo_check=True, include_project1_runtime=True)
        blocking = report.blocking_items(
            args,
            {
                "static_readiness": "passed",
                "export_format_eval": "passed",
                "production_env_audit": "passed",
                "production_readiness": "passed",
                "repo_wide_check": "passed",
                "project1_runtime_acceptance": "passed",
            },
            [
                {
                    "name": "production_env_audit_json",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {"name": "export_format_eval", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                {"name": "provider_canary", "status": "present", "json_status": "passed", "semantic_status": "passed"},
                {
                    "name": "ocr_provider_canary",
                    "status": "present",
                    "json_status": "passed",
                    "semantic_status": "passed",
                },
                {
                    "name": "project1_runtime_acceptance",
                    "status": "present",
                    "json_status": "failed",
                    "semantic_status": "failed",
                },
            ],
            valid_git_release_state(),
        )

        self.assertEqual(blocking, ["project1 runtime artifact must be present and passed"])

    def test_artifact_summary_records_json_status_and_hash(self) -> None:
        tmp_root = report.ROOT / "tmp"
        tmp_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=tmp_root) as directory:
            artifact = Path(directory) / "production_env_audit.json"
            artifact.write_text(
                json.dumps(valid_production_env_payload()),
                encoding="utf-8",
            )

            summary = report.artifact_summary("production_env_audit_json", artifact)

        self.assertEqual(summary["name"], "production_env_audit_json")
        self.assertEqual(summary["status"], "present")
        self.assertEqual(summary["json_name"], "production_env_audit")
        self.assertEqual(summary["json_status"], "passed")
        self.assertEqual(summary["semantic_status"], "passed")
        self.assertEqual(summary["semantic_issues"], [])
        self.assertGreater(summary["bytes"], 0)
        self.assertRegex(str(summary["sha256"]), r"^[0-9a-f]{64}$")

    def test_artifact_summary_rejects_fake_passed_provider_canary(self) -> None:
        tmp_root = report.ROOT / "tmp"
        tmp_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=tmp_root) as directory:
            artifact = Path(directory) / "provider_canary.json"
            artifact.write_text(
                json.dumps({"name": "provider_canary", "status": "passed", "routes": []}),
                encoding="utf-8",
            )

            summary = report.artifact_summary("provider_canary", artifact)

        self.assertEqual(summary["json_status"], "passed")
        self.assertEqual(summary["semantic_status"], "failed")
        self.assertIn("provider canary call_provider must be true", summary["semantic_issues"])
        self.assertIn("provider canary route chapter_generate must be present", summary["semantic_issues"])

    def test_artifact_summary_rejects_fake_passed_export_format(self) -> None:
        tmp_root = report.ROOT / "tmp"
        tmp_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=tmp_root) as directory:
            artifact = Path(directory) / "export_format_eval.json"
            artifact.write_text(
                json.dumps(
                    {
                        "name": "工程1.export",
                        "status": "passed",
                        "passed_checks": 1,
                        "failed_checks": 0,
                        "total_checks": 1,
                        "docx": {"size_bytes": 2048, "table_count": 1},
                        "zip": {"docx_entry_count": 2, "manifest_issues": []},
                        "pdf": {"status": "skipped"},
                        "checks": [{"name": "export.docx.openable", "passed": True}],
                    },
                    ensure_ascii=False,
                ),
                encoding="utf-8",
            )

            summary = report.artifact_summary("export_format_eval", artifact)

        self.assertEqual(summary["json_status"], "passed")
        self.assertEqual(summary["semantic_status"], "failed")
        self.assertIn("export format PDF must be generated, not skipped", summary["semantic_issues"])
        self.assertIn("export format check export.zip.manifest_integrity must be present", summary["semantic_issues"])

    def test_artifact_summary_accepts_complete_provider_ocr_and_project1_artifacts(self) -> None:
        tmp_root = report.ROOT / "tmp"
        tmp_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=tmp_root) as directory:
            artifact_specs = [
                ("export_format_eval", "export_format_eval.json", valid_export_format_payload()),
                ("provider_canary", "provider_canary.json", valid_provider_canary_payload()),
                ("ocr_provider_canary", "ocr_provider_canary.json", valid_ocr_canary_payload()),
                ("project1_runtime_acceptance", "project1_runtime_acceptance.json", valid_project1_runtime_payload()),
            ]
            summaries = []
            for name, filename, payload in artifact_specs:
                artifact = Path(directory) / filename
                artifact.write_text(json.dumps(payload), encoding="utf-8")
                summaries.append(report.artifact_summary(name, artifact))

        self.assertEqual([summary["semantic_status"] for summary in summaries], ["passed", "passed", "passed", "passed"])
        self.assertTrue(all(summary["semantic_issues"] == [] for summary in summaries))

    def test_clear_artifact_removes_stale_json_before_new_evidence(self) -> None:
        tmp_root = report.ROOT / "tmp"
        tmp_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=tmp_root) as directory:
            artifact = Path(directory) / "provider_canary.json"
            artifact.write_text(json.dumps({"name": "provider_canary", "status": "passed"}), encoding="utf-8")

            report.clear_artifact(artifact)

            summary = report.artifact_summary("provider_canary", artifact)

        self.assertEqual(summary["status"], "missing")

    def test_json_artifact_command_preserves_full_output(self) -> None:
        tmp_root = report.ROOT / "tmp"
        tmp_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=tmp_root) as directory:
            artifact = Path(directory) / "production_env_audit.large.json"
            payload = "x" * 9000

            step = report.run_json_artifact_command(
                "production_env_audit_json",
                [
                    sys.executable,
                    "-c",
                    (
                        "import json; "
                        f"print(json.dumps({{'name':'production_env_audit','status':'passed','payload':'{payload}'}}))"
                    ),
                ],
                artifact,
                env=os.environ.copy(),
                timeout_s=30,
                redact=lambda value: value,
            )

            saved = json.loads(artifact.read_text(encoding="utf-8"))

        self.assertEqual(step["status"], "passed")
        self.assertLessEqual(len(str(step["output"])), 8000)
        self.assertEqual(saved["name"], "production_env_audit")
        self.assertEqual(saved["status"], "passed")
        self.assertEqual(saved["payload"], payload)

    def test_production_report_indexes_provider_and_ocr_canary_artifacts(self) -> None:
        args = argparse.Namespace(
            profile="production",
            env_file=None,
            include_repo_check=False,
            include_project1_runtime=False,
            timeout_s=30,
        )
        commands: list[tuple[str, list[str]]] = []
        artifact_commands: list[tuple[str, list[str]]] = []

        def fake_run_command(name: str, command: list[str], **_: object) -> dict[str, object]:
            commands.append((name, command))
            return {"name": name, "status": "passed", "returncode": 0}

        def fake_json_artifact_command(name: str, command: list[str], artifact: Path, **__: object) -> dict[str, object]:
            artifact_commands.append((name, command))
            return {
                "name": name,
                "status": "failed",
                "returncode": 1,
                "artifact": {"name": name, "path": str(artifact.relative_to(report.ROOT)), "status": "present"},
            }

        def fake_artifact_summary(name: str, path: Path) -> dict[str, object]:
            return {"name": name, "path": str(path.relative_to(report.ROOT)), "status": "missing"}

        with tempfile.TemporaryDirectory(dir=report.ROOT / "tmp") as directory:
            artifact_dir = Path(directory)
            with (
                patch.object(report, "PROVIDER_CANARY_JSON", artifact_dir / "provider_canary.json"),
                patch.object(report, "OCR_CANARY_JSON", artifact_dir / "ocr_provider_canary.json"),
                patch.object(report, "PROJECT1_RUNTIME_JSON", artifact_dir / "project1_runtime_acceptance.json"),
                patch.object(report, "load_env_file", return_value=({}, [])),
                patch.object(report, "run_command", side_effect=fake_run_command),
                patch.object(report, "run_json_artifact_command", side_effect=fake_json_artifact_command),
                patch.object(report, "artifact_summary", side_effect=fake_artifact_summary),
                patch.object(report, "collect_git_release_state", return_value=valid_git_release_state()),
            ):
                generated = report.build_report(args)

        production_command = next(command for name, command in commands if name == "production_readiness")
        self.assertIn("--provider-canary-json-output", production_command)
        self.assertIn(str((artifact_dir / "provider_canary.json").relative_to(report.ROOT)), production_command)
        self.assertIn("--ocr-canary-json-output", production_command)
        self.assertIn(str((artifact_dir / "ocr_provider_canary.json").relative_to(report.ROOT)), production_command)
        export_command = next(command for name, command in artifact_commands if name == "export_format_eval")
        self.assertIn("app.evaluation.export_format_eval", export_command)
        self.assertIn("--require-pdf", export_command)
        self.assertEqual(
            [artifact["name"] for artifact in generated["artifacts"]],
            ["production_env_audit_json", "export_format_eval", "provider_canary", "ocr_provider_canary"],
        )
        self.assertEqual(generated["commit"], valid_git_release_state()["commit"])
        self.assertEqual(generated["branch"], report.EXPECTED_RELEASE_BRANCH)
        self.assertEqual(generated["remote"], report.EXPECTED_ORIGIN_REMOTE)
        self.assertEqual(generated["git_release_state"]["head_matches_remote"], True)


if __name__ == "__main__":
    unittest.main()

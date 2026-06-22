#!/usr/bin/env python3
"""Regression tests for first_usable_release_report.py."""

from __future__ import annotations

import argparse
import tempfile
import unittest
from pathlib import Path

import first_usable_release_report as report


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

    def test_blocking_items_require_production_repo_check_and_project1_runtime(self) -> None:
        args = argparse.Namespace(profile="local", include_repo_check=False, include_project1_runtime=False)
        blocking = report.blocking_items(
            args,
            {"static_readiness": "passed", "local_readiness_canaries": "passed"},
        )

        self.assertEqual(
            blocking,
            [
                "report profile must be production",
                "repo-wide check must be included",
                "project1 runtime acceptance must be included",
            ],
        )

    def test_blocking_items_allow_complete_production_evidence(self) -> None:
        args = argparse.Namespace(profile="production", include_repo_check=True, include_project1_runtime=True)
        blocking = report.blocking_items(
            args,
            {
                "static_readiness": "passed",
                "production_env_audit": "passed",
                "production_readiness": "passed",
                "repo_wide_check": "passed",
                "project1_runtime_acceptance": "passed",
            },
        )

        self.assertEqual(blocking, [])


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Regression tests for first_usable_release_check.py."""

from __future__ import annotations

import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import first_usable_release_check as check


def valid_production_env() -> dict[str, str]:
    return {
        "APP_ENV": "production",
        "DATABASE_URL": "postgres://zbt_app:prod-db-secret@example.com/zbt?sslmode=require",
        "MIGRATION_DATABASE_URL": "postgres://zbt:prod-migration-secret@example.com/zbt?sslmode=require",
        "REDIS_URL": "redis://redis.example.com:6379/0",
        "AI_SERVICE_URL": "http://ai-service:8000",
        "AI_CALLBACK_URL": "http://backend:8080/api/v1/ai/callbacks/tasks",
        "MINIO_ENDPOINT": "https://r2.example.com",
        "MINIO_PUBLIC_ENDPOINT": "https://r2.example.com",
        "MINIO_BUCKET": "zbt-private-prod",
        "MINIO_ACCESS_KEY": "prod-r2-access-key-123456",
        "MINIO_SECRET_KEY": "prod-r2-secret-key-123456",
        "JWT_SECRET": "prod-jwt-secret-1234567890",
        "AI_SERVICE_HMAC_SECRET": "prod-ai-hmac-secret-1234567890",
        "USE_MOCK_PROVIDERS": "false",
        "ALLOW_MOCK_FALLBACK": "false",
        "OPENAI_API_KEY": "unit-test-openai-key",
        "OCR_PROVIDER": "http_ocr",
        "OCR_HTTP_ENDPOINT": "https://ocr.example.com/parse",
        "AI_MODEL_PRICING_JSON": json.dumps(
            {
                "openai_compatible_primary/*": {"input_per_1m": 2, "output_per_1m": 8},
            }
        ),
    }


class FirstUsableReleaseCheckTest(unittest.TestCase):
    def test_production_env_audit_reports_missing_inputs_as_matrix(self) -> None:
        with patch.dict(os.environ, {}, clear=True):
            audit = check.production_env_audit()

        self.assertEqual(audit["status"], "failed")
        self.assertIn("production env missing DATABASE_URL", audit["issues"])
        self.assertIn("production env USE_MOCK_PROVIDERS must be false", audit["issues"])
        self.assertIn("production env missing AI_MODEL_PRICING_JSON", audit["issues"])
        self.assertEqual(
            [route["route"] for route in audit["evidence"]["routes"]],
            ["chapter_generate", "knowledge_embedding", "knowledge_rerank"],
        )
        self.assertEqual(audit["evidence"]["provider_requirements"][0]["provider"], "openai_compatible_primary")

    def test_production_env_audit_accepts_cloudflare_gateway_matrix(self) -> None:
        env = valid_production_env()
        env.update(
            {
                "AI_LLM_PROVIDER": "cloudflare_ai_gateway",
                "AI_LLM_MODEL": "openai/gpt-4.1",
                "AI_EMBEDDING_PROVIDER": "cloudflare_ai_gateway",
                "AI_EMBEDDING_MODEL": "@cf/baai/bge-large-en-v1.5",
                "AI_RERANK_PROVIDER": "cloudflare_ai_gateway",
                "AI_RERANK_MODEL": "openai/gpt-4.1",
                "CLOUDFLARE_API_TOKEN": "unit-test-cloudflare-token",
                "CLOUDFLARE_ACCOUNT_ID": "0123456789abcdef0123456789abcdef",
                "AI_MODEL_PRICING_JSON": json.dumps(
                    {
                        "cloudflare_ai_gateway/*": {"input_per_1m": 2, "output_per_1m": 8},
                    }
                ),
            }
        )

        with patch.dict(os.environ, env, clear=True):
            audit = check.production_env_audit()

        self.assertEqual(audit["status"], "passed")
        self.assertEqual(audit["issues"], [])
        self.assertEqual(audit["evidence"]["providers"], ["cloudflare_ai_gateway"])
        self.assertEqual(
            audit["evidence"]["provider_requirements"][0]["configured_envs"],
            ["CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ACCOUNT_ID"],
        )
        self.assertTrue(all(item["matched"] for item in audit["evidence"]["pricing_matches"]))
        self.assertEqual(audit["evidence"]["ocr_requirement"]["configured_envs"], ["OCR_HTTP_ENDPOINT"])

    def test_production_env_audit_fails_when_pricing_misses_selected_provider(self) -> None:
        env = valid_production_env()
        env["AI_MODEL_PRICING_JSON"] = json.dumps(
            {
                "deepseek/*": {"input_per_1m": 1, "output_per_1m": 2},
            }
        )

        with patch.dict(os.environ, env, clear=True):
            audit = check.production_env_audit()

        self.assertEqual(audit["status"], "failed")
        self.assertIn(
            "production env AI_MODEL_PRICING_JSON missing price for "
            "openai_compatible_primary/gpt-4o-mini or openai_compatible_primary/*",
            audit["issues"],
        )
        self.assertTrue(any(not item["matched"] for item in audit["evidence"]["pricing_matches"]))

    def test_canary_json_outputs_are_written_before_status_gate(self) -> None:
        provider_result = {
            "name": "provider_canary",
            "status": "passed",
            "passed_checks": 1,
            "total_checks": 1,
            "routes": [],
        }
        ocr_result = {
            "name": "ocr_provider_eval",
            "status": "passed",
            "passed_checks": 1,
            "total_checks": 1,
            "checks": [],
        }
        with tempfile.TemporaryDirectory() as directory:
            provider_output = Path(directory) / "provider_canary.json"
            ocr_output = Path(directory) / "ocr_provider_canary.json"
            with patch.object(check, "run_json_command", return_value=(0, provider_result, json.dumps(provider_result))):
                check.check_provider_canary("production", json_output=provider_output)
            with patch.object(check, "run_json_command", return_value=(0, ocr_result, json.dumps(ocr_result))):
                check.check_ocr_canary("production", json_output=ocr_output)

            saved_provider = json.loads(provider_output.read_text(encoding="utf-8"))
            saved_ocr = json.loads(ocr_output.read_text(encoding="utf-8"))

        self.assertEqual(saved_provider["name"], "provider_canary")
        self.assertEqual(saved_provider["status"], "passed")
        self.assertEqual(saved_ocr["name"], "ocr_provider_eval")
        self.assertEqual(saved_ocr["status"], "passed")


if __name__ == "__main__":
    unittest.main()

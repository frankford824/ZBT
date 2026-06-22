#!/usr/bin/env python3
"""Regression tests for acceptance_project1_check.py helpers."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import acceptance_project1_check as project1


class AcceptanceProject1CheckTest(unittest.TestCase):
    def test_write_json_output_records_runtime_evidence(self) -> None:
        evidence = {
            "name": "project1_runtime_acceptance",
            "status": "passed",
            "sample_files": [{"label": "tender_pdf", "sha256": "abc123"}],
            "steps": {
                "parse_response_matrix": {"requirements": 40},
                "generation_coverage_compliance": {"coverage_rows": 64},
                "docx_export": {"download_ready": True},
            },
        }
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "project1_runtime_acceptance.json"

            project1.write_json_output(evidence, output)

            saved = json.loads(output.read_text(encoding="utf-8"))

        self.assertEqual(saved["name"], "project1_runtime_acceptance")
        self.assertEqual(saved["status"], "passed")
        self.assertEqual(saved["steps"]["parse_response_matrix"]["requirements"], 40)
        self.assertTrue(saved["steps"]["docx_export"]["download_ready"])


if __name__ == "__main__":
    unittest.main()

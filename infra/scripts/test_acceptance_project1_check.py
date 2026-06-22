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

    def test_select_traceable_knowledge_source_ref_prefers_ref_with_id_and_location(self) -> None:
        selected = project1.select_traceable_knowledge_source_ref(
            [
                {"source_ref": {"chunk_id": "chunk-1", "document_id": "doc-1"}},
                {
                    "source_ref": {
                        "reference_id": "knowledge:doc-2:chunk-2",
                        "chunk_id": "chunk-2",
                        "document_id": "doc-2",
                    }
                },
            ],
            [{"chunk_id": "chunk-0", "document_id": "doc-0"}],
        )

        self.assertEqual(selected["reference_id"], "knowledge:doc-2:chunk-2")
        self.assertEqual(selected["chunk_id"], "chunk-2")

    def test_select_traceable_knowledge_source_ref_rejects_refs_without_reference_id(self) -> None:
        with self.assertRaises(project1.AcceptanceError):
            project1.select_traceable_knowledge_source_ref(
                [{"source_ref": {"chunk_id": "chunk-1", "document_id": "doc-1"}}],
                [{"chunk_id": "chunk-0", "document_id": "doc-0"}],
            )


if __name__ == "__main__":
    unittest.main()

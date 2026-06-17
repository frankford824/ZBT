import json

from app.evaluation.generation_coverage_eval import evaluate_generation_coverage


def test_generation_coverage_eval_passes_for_covered_requirements_and_resolved_refs(tmp_path) -> None:
    spec = tmp_path / "coverage.json"
    spec.write_text(
        json.dumps(
            {
                "name": "coverage-pass",
                "requirements": [
                    {
                        "id": "req-qualification-1",
                        "module": "qualification",
                        "mandatory": True,
                        "requirement": "提供营业执照",
                    },
                    {
                        "id": "req-evaluation-1",
                        "module": "evaluation",
                        "priority": "high",
                        "requirement": "技术方案完整性",
                    },
                ],
                "knowledge_chunks": [
                    {"chunk_id": "chunk-1", "document_id": "doc-1"},
                    {"chunk_id": "chunk-2", "document_id": "doc-2"},
                ],
                "chapters": [
                    {
                        "id": "chapter-1",
                        "source_refs": [{"chunk_id": "chunk-1", "document_id": "doc-1"}],
                        "model_metadata": {
                            "self_check": {
                                "requirement_coverage": [
                                    {
                                        "requirement_id": "req-qualification-1",
                                        "satisfied": True,
                                        "evidence": "已提供营业执照章节说明。",
                                        "source_refs": [{"chunk_id": "chunk-1", "document_id": "doc-1"}],
                                    },
                                    {
                                        "requirement_id": "req-evaluation-1",
                                        "status": "covered",
                                        "evidence": "技术方案章节覆盖评分点。",
                                        "source_refs": [{"chunk_id": "chunk-2", "document_id": "doc-2"}],
                                    },
                                ]
                            }
                        },
                    }
                ],
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_generation_coverage(spec)

    assert result["status"] == "passed"
    assert result["mandatory_coverage_ratio"] == 1
    assert result["source_ref_resolution_ratio"] == 1
    assert result["failed_checks"] == 0


def test_generation_coverage_eval_fails_for_missing_coverage_and_unresolved_refs(tmp_path) -> None:
    spec = tmp_path / "coverage.json"
    spec.write_text(
        json.dumps(
            {
                "requirements": [
                    {"id": "req-1", "mandatory": True, "requirement": "提供营业执照"},
                    {"id": "req-2", "mandatory": True, "requirement": "提供安全资质"},
                ],
                "thresholds": {
                    "min_mandatory_coverage_ratio": 1,
                    "min_source_ref_resolution_ratio": 1,
                },
                "knowledge_chunks": [{"chunk_id": "chunk-1", "document_id": "doc-1"}],
                "chapters": [
                    {
                        "id": "chapter-1",
                        "source_refs": [{"chunk_id": "missing-chunk", "document_id": "doc-x"}],
                        "requirement_coverage": [
                            {
                                "requirement_id": "req-1",
                                "satisfied": True,
                                "source_refs": [{"chunk_id": "chunk-1", "document_id": "doc-1"}],
                            },
                            {
                                "requirement_id": "req-2",
                                "satisfied": False,
                                "needs_review": True,
                                "source_refs": [],
                            },
                        ],
                    }
                ],
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_generation_coverage(spec)

    assert result["status"] == "failed"
    assert result["mandatory_coverage_ratio"] == 0.5
    assert result["source_ref_resolution_ratio"] < 1
    failed_names = {check["name"] for check in result["checks"] if not check["passed"]}
    assert "generation.requirements.mandatory_coverage_ratio" in failed_names
    assert "generation.requirements.req-2.covered" in failed_names
    assert "generation.source_refs.resolution_ratio" in failed_names

import json

from app.evaluation.tender_parse_eval import evaluate_golden


def test_evaluate_golden_passes_for_complete_text_sample(tmp_path) -> None:
    sample = tmp_path / "sample.txt"
    sample.write_text(
        "\n".join(
            [
                "项目名称：智慧交通平台建设",
                "投标截止时间：2026年7月15日",
                "资格要求：具备类似项目业绩和安全资质证书",
                "无效投标：未按要求签章或投标有效期不足",
                "评分标准：技术方案完整性和项目团队能力",
                "响应文件格式：按附件格式签章提交",
                "工程量清单：按固化清单报价",
            ]
        ),
        encoding="utf-8",
    )
    golden = tmp_path / "golden.json"
    golden.write_text(
        json.dumps(
            {
                "name": "unit-sample",
                "documents": [
                    {
                        "id": "tender",
                        "path": "sample.txt",
                        "content_type": "text/plain",
                        "parser": "plain-text",
                        "min_chunks": 1,
                        "text_contains": ["智慧交通平台建设"],
                    }
                ],
                "tender_parse": {
                    "document_id": "tender",
                    "filename": "sample.txt",
                    "content_type": "text/plain",
                    "fields": [
                        {"path": "project_name", "contains": "智慧交通平台建设"},
                        {"path": "deadline", "equals": "2026-07-15"},
                    ],
                    "requirements": {
                        "min_count": 5,
                        "required_modules": [
                            "qualification",
                            "evaluation",
                            "submission",
                            "invalid_risk",
                            "annex",
                        ],
                        "must_contain": [
                            {"module": "qualification", "text": "安全资质证书"},
                            {"module": "invalid_risk", "text": "投标有效期不足"},
                        ],
                    },
                    "evidence": {"min_count": 5, "max_missing_source_count": 0},
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_golden(golden, repo_root=tmp_path)

    assert result["status"] == "passed"
    assert result["failed_checks"] == 0


def test_evaluate_golden_fails_when_required_field_is_missing(tmp_path) -> None:
    sample = tmp_path / "sample.txt"
    sample.write_text("项目名称：智慧交通平台建设", encoding="utf-8")
    golden = tmp_path / "golden.json"
    golden.write_text(
        json.dumps(
            {
                "documents": [{"id": "tender", "path": "sample.txt", "content_type": "text/plain"}],
                "tender_parse": {
                    "document_id": "tender",
                    "filename": "sample.txt",
                    "content_type": "text/plain",
                    "fields": [{"path": "deadline", "equals": "2026-07-15"}],
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_golden(golden, repo_root=tmp_path)

    assert result["status"] == "failed"
    assert any(check["name"] == "tender_parse.field.deadline.equals" for check in result["checks"])

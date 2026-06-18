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
                        "require_traceable_source_refs": True,
                        "require_reference_ids": True,
                        "require_source_locations": True,
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
                    "evidence": {
                        "min_count": 5,
                        "max_missing_source_count": 0,
                        "require_traceable": True,
                        "require_reference_ids": True,
                        "require_source_locations": True,
                    },
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_golden(golden, repo_root=tmp_path)

    assert result["status"] == "passed"
    assert result["failed_checks"] == 0


def test_evaluate_golden_fails_when_source_refs_are_not_traceable(tmp_path) -> None:
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
                    "requirements": {
                        "min_count": 1,
                        "require_traceable_source_refs": True,
                        "require_reference_ids": True,
                        "require_source_locations": True,
                    },
                    "evidence": {
                        "min_count": 1,
                        "require_traceable": True,
                        "require_reference_ids": True,
                        "require_source_locations": True,
                    },
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_golden(golden, repo_root=tmp_path)

    assert result["status"] == "failed"
    failed_names = {check["name"] for check in result["checks"] if not check["passed"]}
    assert "tender_parse.requirements.traceable_source_refs" in failed_names
    assert "tender_parse.requirements.source_reference_ids" in failed_names
    assert "tender_parse.requirements.source_locations" in failed_names


def test_evaluate_golden_checks_table_block_structure(tmp_path) -> None:
    from openpyxl import Workbook

    sample = tmp_path / "sample.xlsx"
    workbook = Workbook()
    sheet = workbook.active
    sheet.title = "报价"
    sheet.append(["项目号", "项目名称", "最高单价", "综合单价"])
    sheet.append(["1", "中级养护技术员", "17800", "17700"])
    workbook.save(sample)
    workbook.close()
    golden = tmp_path / "golden.json"
    golden.write_text(
        json.dumps(
            {
                "documents": [
                    {
                        "id": "boq",
                        "path": "sample.xlsx",
                        "content_type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
                        "parser": "openpyxl",
                        "min_table_blocks": 1,
                        "table_blocks": {
                            "required_sources": ["xlsx"],
                            "min_total_rows": 2,
                            "min_blocks_with_rows": 1,
                            "min_blocks_with_bbox": 0,
                            "min_cells_with_bbox": 0,
                            "require_md_table": True,
                            "must_contain": ["最高单价", "中级养护技术员"],
                            "md_table_must_contain": ["| 项目号 | 项目名称 | 最高单价 | 综合单价 |"],
                        },
                    }
                ],
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_golden(golden, repo_root=tmp_path)

    assert result["status"] == "passed"
    assert result["failed_checks"] == 0


def test_evaluate_golden_fails_when_table_block_contract_is_missing(tmp_path) -> None:
    sample = tmp_path / "sample.txt"
    sample.write_text("项目名称：智慧交通平台建设", encoding="utf-8")
    golden = tmp_path / "golden.json"
    golden.write_text(
        json.dumps(
            {
                "documents": [
                    {
                        "id": "tender",
                        "path": "sample.txt",
                        "content_type": "text/plain",
                        "table_blocks": {
                            "required_sources": ["pdf"],
                            "min_total_rows": 1,
                            "min_blocks_with_bbox": 1,
                            "min_cells_with_bbox": 1,
                            "require_md_table": True,
                            "must_contain": ["最高单价"],
                            "md_table_must_contain": ["| 最高单价 |"],
                        },
                    }
                ],
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_golden(golden, repo_root=tmp_path)

    assert result["status"] == "failed"
    failed_names = {check["name"] for check in result["checks"] if not check["passed"]}
    assert "document.tender.table_blocks.source.pdf" in failed_names
    assert "document.tender.table_blocks.total_rows" in failed_names
    assert "document.tender.table_blocks.with_bbox" in failed_names
    assert "document.tender.table_blocks.cells_with_bbox" in failed_names
    assert "document.tender.table_blocks.md_table_present" in failed_names
    assert "document.tender.table_blocks.must_contain[1]" in failed_names
    assert "document.tender.table_blocks.md_table_must_contain[1]" in failed_names


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

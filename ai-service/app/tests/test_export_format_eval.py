from __future__ import annotations

import json

from app.evaluation.export_format_eval import evaluate_export_format


def test_export_format_eval_passes_docx_zip_and_skipped_pdf(tmp_path, monkeypatch) -> None:
    spec = tmp_path / "export.json"
    spec.write_text(
        json.dumps(
            {
                "name": "export-pass",
                "bid_title": "工程1桥梁检查采购响应文件",
                "part_title": "技术标",
                "layout": {
                    "watermark_text": "内部评审",
                    "generated_at": "2026-06-18",
                },
                "chapters": [
                    {
                        "title": "项目实施方案",
                        "plain_text": (
                            "# 总体安排\n"
                            "本章响应桥梁检查项目服务要求。\n\n"
                            "| 项目 | 内容 |\n"
                            "| --- | --- |\n"
                            "| 服务周期 | 90日历天 |"
                        ),
                    }
                ],
                "docx": {
                    "required_text": ["工程1桥梁检查采购响应文件", "项目实施方案", "服务周期"],
                    "min_tables": 1,
                },
                "zip": {
                    "parts": [
                        {
                            "code": "tech",
                            "title": "技术标",
                            "chapters": [{"title": "实施方案", "plain_text": "技术响应。"}],
                        },
                        {
                            "code": "business",
                            "title": "商务标",
                            "chapters": [{"title": "报价说明", "plain_text": "商务响应。"}],
                        },
                    ],
                    "expected_entries": [
                        "01_投标文件/01-技术标.docx",
                        "01_投标文件/02-商务标.docx",
                        "manifest.json",
                    ],
                    "min_docx_entries": 2,
                },
                "pdf": {
                    "enabled": True,
                    "allow_skip": True,
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )
    monkeypatch.setenv("LIBREOFFICE_PATH", "/not-installed/soffice")

    result = evaluate_export_format(spec)

    assert result["status"] == "passed"
    assert result["failed_checks"] == 0
    assert result["docx"]["table_count"] == 1
    assert result["zip"]["docx_entry_count"] == 2
    assert result["zip"]["manifest_issues"] == []
    assert result["pdf"]["status"] == "skipped"
    passed_names = {check["name"] for check in result["checks"] if check["passed"]}
    assert "export.docx.cover" in passed_names
    assert "export.docx.watermark" in passed_names
    assert "export.docx.header_footer_text" in passed_names
    assert "export.zip.manifest_integrity" in passed_names
    assert "export.zip.safe_paths" in passed_names


def test_export_format_eval_fails_when_docx_required_text_is_missing(tmp_path) -> None:
    spec = tmp_path / "export.json"
    spec.write_text(
        json.dumps(
            {
                "bid_title": "工程1桥梁检查采购响应文件",
                "part_title": "技术标",
                "chapters": [{"title": "项目实施方案", "plain_text": "正文。"}],
                "docx": {
                    "required_text": ["不存在的承诺条款"],
                    "require_toc": False,
                    "require_update_fields": False,
                    "require_page_fields": False,
                    "require_header_footer": False,
                },
                "zip": {"enabled": False},
                "pdf": {"enabled": False},
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    result = evaluate_export_format(spec)

    assert result["status"] == "failed"
    failed_names = {check["name"] for check in result["checks"] if not check["passed"]}
    assert "export.docx.text.不存在的承诺条款" in failed_names


def test_export_format_eval_requires_pdf_when_configured(tmp_path, monkeypatch) -> None:
    spec = tmp_path / "export.json"
    spec.write_text(
        json.dumps(
            {
                "bid_title": "工程1桥梁检查采购响应文件",
                "part_title": "技术标",
                "layout": {"generated_at": "2026-06-18"},
                "chapters": [{"title": "项目实施方案", "plain_text": "正文。"}],
                "docx": {
                    "required_text": ["工程1桥梁检查采购响应文件"],
                    "require_watermark": False,
                },
                "zip": {"enabled": False},
                "pdf": {"enabled": True, "allow_skip": True},
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )
    monkeypatch.setenv("LIBREOFFICE_PATH", "/not-installed/soffice")

    result = evaluate_export_format(spec, require_pdf=True)

    assert result["status"] == "failed"
    assert result["pdf"]["status"] == "failed"
    failed = {check["name"]: check for check in result["checks"] if not check["passed"]}
    assert failed["export.pdf.generated"]["actual"] == "LibreOffice PDF conversion failed"

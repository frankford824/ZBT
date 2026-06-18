from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.schemas.export import (
    MAX_EXPORT_ATTACHMENT_COUNT,
    MAX_EXPORT_CALLBACK_URL_LENGTH,
    MAX_EXPORT_CHAPTER_PLAIN_TEXT_LENGTH,
    MAX_EXPORT_FILENAME_LENGTH,
    MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH,
    MAX_EXPORT_LAYOUT_CONTEXT_ITEMS,
    MAX_EXPORT_LAYOUT_CONTEXT_KEY_LENGTH,
    MAX_EXPORT_LAYOUT_CONTEXT_STRING_LENGTH,
    MAX_EXPORT_OBJECT_KEY_LENGTH,
    MAX_EXPORT_TITLE_LENGTH,
    MAX_EXPORT_TOTAL_CHAPTERS,
    MAX_EXPORT_TOTAL_CHAPTER_TEXT_LENGTH,
    DocumentExportRequest,
    ExportAttachment,
    ExportChapter,
    ExportLayoutOptions,
    ExportPart,
    inline_content_size,
)


def _export_payload(**extra: object) -> dict[str, object]:
    payload: dict[str, object] = {
        "tenant_id": "tenant-demo",
        "export_id": "export-demo",
        "bid_id": "bid-demo",
        "bid_title": "测试项目",
        "export_type": "docx",
        "part_code": "tech",
        "part_title": "技术标",
        "filename": "测试项目.docx",
        "object_key": "tenant-demo/bid_export/export.docx",
        "callback_url": "http://backend:8080/api/v1/ai/callbacks/tasks",
        "chapters": [{"title": "实施计划", "plain_text": "实施正文"}],
    }
    payload.update(extra)
    return payload


def test_export_attachment_rejects_mixed_content_sources() -> None:
    with pytest.raises(ValidationError, match="exactly one attachment content source"):
        ExportAttachment(
            filename="bad.txt",
            object_key="tenant-demo/assets/bad.txt",
            content_base64="YQ==",
        )


def test_export_attachment_rejects_invalid_inline_base64() -> None:
    with pytest.raises(ValidationError, match="content_base64 is invalid"):
        ExportAttachment(filename="bad.txt", content_base64="not-base64")


def test_document_export_request_rejects_too_many_attachments() -> None:
    attachments = [
        ExportAttachment(filename=f"file-{index}.txt", object_key=f"tenant-demo/assets/{index}.txt")
        for index in range(MAX_EXPORT_ATTACHMENT_COUNT)
    ]

    with pytest.raises(ValidationError, match="too many export attachments"):
        DocumentExportRequest(
            tenant_id="tenant-demo",
            export_id="export-demo",
            bid_id="bid-demo",
            bid_title="测试项目",
            part_code="tech",
            part_title="技术标",
            filename="测试项目.zip",
            object_key="tenant-demo/bid_export/export.zip",
            attachments=attachments,
            boq_files=[
                ExportAttachment(filename="boq.xlsx", object_key="tenant-demo/assets/boq.xlsx")
            ],
        )


def test_inline_content_size_rejects_oversized_encoded_content() -> None:
    encoded = "A" * (MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH + 1)
    with pytest.raises(ValueError, match="attachment content is too large"):
        inline_content_size(encoded)


def test_export_chapter_rejects_oversized_fields() -> None:
    with pytest.raises(ValidationError):
        ExportChapter(title="x" * (MAX_EXPORT_TITLE_LENGTH + 1), plain_text="正文")

    with pytest.raises(ValidationError):
        ExportChapter(title="实施计划", plain_text="x" * (MAX_EXPORT_CHAPTER_PLAIN_TEXT_LENGTH + 1))


def test_document_export_request_rejects_oversized_document_fields() -> None:
    oversized_cases = [
        {"bid_title": "x" * (MAX_EXPORT_TITLE_LENGTH + 1)},
        {"filename": "x" * (MAX_EXPORT_FILENAME_LENGTH + 1)},
        {"object_key": "x" * (MAX_EXPORT_OBJECT_KEY_LENGTH + 1)},
        {"callback_url": "http://backend/" + "x" * MAX_EXPORT_CALLBACK_URL_LENGTH},
        {"export_type": "xlsx"},
    ]

    for extra in oversized_cases:
        with pytest.raises(ValidationError):
            DocumentExportRequest(**_export_payload(**extra))


def test_document_export_request_rejects_blank_required_fields() -> None:
    for field in (
        "tenant_id",
        "export_id",
        "bid_id",
        "bid_title",
        "part_code",
        "part_title",
        "filename",
        "object_key",
    ):
        with pytest.raises(ValidationError):
            DocumentExportRequest(**_export_payload(**{field: "   "}))


def test_document_export_request_rejects_oversized_chapter_budget() -> None:
    chapters = [ExportChapter(title=f"章节{index}", plain_text="正文") for index in range(200)]
    too_many_parts = [
        ExportPart(code=f"p{index}", title=f"分册{index}", chapters=chapters)
        for index in range((MAX_EXPORT_TOTAL_CHAPTERS // len(chapters)) + 1)
    ]
    with pytest.raises(ValidationError, match="too many export chapters"):
        DocumentExportRequest(**_export_payload(parts=too_many_parts))

    chapter_count = (MAX_EXPORT_TOTAL_CHAPTER_TEXT_LENGTH // MAX_EXPORT_CHAPTER_PLAIN_TEXT_LENGTH) + 1
    with pytest.raises(ValidationError, match="export chapters are too large"):
        DocumentExportRequest(
            **_export_payload(
                chapters=[
                    {"title": f"章节{index}", "plain_text": "x" * MAX_EXPORT_CHAPTER_PLAIN_TEXT_LENGTH}
                    for index in range(chapter_count)
                ]
            )
        )


def test_export_layout_context_rejects_oversized_or_invalid_values() -> None:
    invalid_contexts = [
        {f"k{index}": "v" for index in range(MAX_EXPORT_LAYOUT_CONTEXT_ITEMS + 1)},
        {"x" * (MAX_EXPORT_LAYOUT_CONTEXT_KEY_LENGTH + 1): "v"},
        {"note": "x" * (MAX_EXPORT_LAYOUT_CONTEXT_STRING_LENGTH + 1)},
        {"level1": {"level2": {"level3": {"level4": {"level5": "too-deep"}}}}},
    ]

    for context in invalid_contexts:
        with pytest.raises(ValidationError):
            ExportLayoutOptions(context=context)


def test_document_export_request_strips_bounded_text_fields() -> None:
    request = DocumentExportRequest(
        task_id=" task-demo ",
        tenant_id=" tenant-demo ",
        export_id=" export-demo ",
        bid_id=" bid-demo ",
        bid_title=" 测试项目 ",
        export_type="docx",
        part_code=" tech ",
        part_title=" 技术标 ",
        filename=" 测试项目.docx ",
        object_key=" tenant-demo/bid_export/export.docx ",
        callback_url=" http://backend:8080/api/v1/ai/callbacks/tasks ",
        chapters=[{"title": " 实施计划 ", "plain_text": " 实施正文 "}],
        layout={
            "template_name": " zbt-standard ",
            "header_text": " 页眉 ",
            "context": {" review_round ": " 第1轮 ", "sealed": True},
        },
    )

    assert request.task_id == "task-demo"
    assert request.tenant_id == "tenant-demo"
    assert request.export_id == "export-demo"
    assert request.bid_id == "bid-demo"
    assert request.bid_title == "测试项目"
    assert request.part_code == "tech"
    assert request.part_title == "技术标"
    assert request.filename == "测试项目.docx"
    assert request.object_key == "tenant-demo/bid_export/export.docx"
    assert request.callback_url == "http://backend:8080/api/v1/ai/callbacks/tasks"
    assert request.chapters[0].title == "实施计划"
    assert request.chapters[0].plain_text == "实施正文"
    assert request.layout.template_name == "zbt-standard"
    assert request.layout.header_text == "页眉"
    assert request.layout.context == {"review_round": "第1轮", "sealed": True}

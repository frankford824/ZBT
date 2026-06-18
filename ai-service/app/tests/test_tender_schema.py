from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.schemas.tender import (
    MAX_TENDER_CALLBACK_URL_LENGTH,
    MAX_TENDER_FILENAME_LENGTH,
    MAX_TENDER_OBJECT_KEY_LENGTH,
    MAX_TENDER_RESPONSE_BBOX_POINTS,
    MAX_TENDER_RESPONSE_EVIDENCE,
    MAX_TENDER_RESPONSE_JSON_BYTES,
    MAX_TENDER_RESPONSE_REQUIREMENT_ITEMS,
    MAX_TENDER_RESPONSE_SOURCE_TEXT_LENGTH,
    MAX_TENDER_RESPONSE_TEXT_LENGTH,
    MAX_TENDER_TITLE_LENGTH,
    TenderParseFieldEvidence,
    TenderParseModuleResult,
    TenderParseRequest,
    TenderParseStructuredResult,
    TenderRequirementItem,
)


def _tender_payload(**extra: object) -> dict[str, object]:
    payload: dict[str, object] = {
        "tenant_id": "tenant-demo",
        "bid_id": "bid-demo",
        "bid_title": "智慧交通项目",
        "file_id": "file-demo",
        "object_key": "tenant-demo/bid_tender/file-demo",
        "filename": "招标文件.pdf",
        "content_type": "application/pdf",
        "callback_url": "http://backend:8080/api/v1/ai/callbacks/tasks",
    }
    payload.update(extra)
    return payload


def test_tender_parse_request_rejects_oversized_document_fields() -> None:
    oversized_cases = [
        {"bid_title": "x" * (MAX_TENDER_TITLE_LENGTH + 1)},
        {"object_key": "x" * (MAX_TENDER_OBJECT_KEY_LENGTH + 1)},
        {"filename": "x" * (MAX_TENDER_FILENAME_LENGTH + 1)},
        {"callback_url": "http://backend/" + "x" * MAX_TENDER_CALLBACK_URL_LENGTH},
    ]

    for extra in oversized_cases:
        with pytest.raises(ValidationError):
            TenderParseRequest(**_tender_payload(**extra))


def test_tender_parse_request_rejects_blank_required_fields() -> None:
    for field in ("tenant_id", "file_id", "object_key", "filename", "content_type"):
        with pytest.raises(ValidationError):
            TenderParseRequest(**_tender_payload(**{field: "   "}))


def test_tender_parse_request_strips_bounded_document_fields() -> None:
    request = TenderParseRequest(
        task_id=" task-demo ",
        tenant_id=" tenant-demo ",
        bid_id=" bid-demo ",
        bid_title=" 智慧交通项目 ",
        file_id=" file-demo ",
        object_key=" tenant-demo/bid_tender/file-demo ",
        filename=" 招标文件.pdf ",
        content_type=" application/pdf ",
        callback_url=" http://backend:8080/api/v1/ai/callbacks/tasks ",
    )

    assert request.task_id == "task-demo"
    assert request.tenant_id == "tenant-demo"
    assert request.bid_id == "bid-demo"
    assert request.bid_title == "智慧交通项目"
    assert request.file_id == "file-demo"
    assert request.object_key == "tenant-demo/bid_tender/file-demo"
    assert request.filename == "招标文件.pdf"
    assert request.content_type == "application/pdf"
    assert request.callback_url == "http://backend:8080/api/v1/ai/callbacks/tasks"


def test_tender_parse_response_rejects_oversized_evidence_fields() -> None:
    with pytest.raises(ValidationError):
        TenderParseFieldEvidence(
            field="qualification",
            source_text="原文" * (MAX_TENDER_RESPONSE_SOURCE_TEXT_LENGTH + 1),
        )

    with pytest.raises(ValidationError):
        TenderParseFieldEvidence(
            field="qualification",
            source_text="原文",
            page_start=0,
        )

    with pytest.raises(ValidationError):
        TenderParseFieldEvidence(
            field="qualification",
            source_text="原文",
            bbox=[0.0] * (MAX_TENDER_RESPONSE_BBOX_POINTS + 1),
        )


def test_tender_parse_module_response_rejects_oversized_collections() -> None:
    evidence = [
        TenderParseFieldEvidence(field=f"field-{index}", source_text="原文")
        for index in range(MAX_TENDER_RESPONSE_EVIDENCE + 1)
    ]
    with pytest.raises(ValidationError):
        TenderParseModuleResult(module="qualification", title="资格要求", evidence=evidence)

    with pytest.raises(ValidationError):
        TenderParseModuleResult(
            module="qualification",
            title="资格要求",
            fields={"oversized": "x" * MAX_TENDER_RESPONSE_JSON_BYTES},
        )


def test_tender_parse_structured_response_rejects_oversized_requirements_and_metadata() -> None:
    requirements = [
        TenderRequirementItem(
            id=f"qualification-{index}",
            module="qualification",
            type="qualification",
            requirement="提供资质证明",
        )
        for index in range(MAX_TENDER_RESPONSE_REQUIREMENT_ITEMS + 1)
    ]
    with pytest.raises(ValidationError):
        TenderParseStructuredResult(
            project_name="智慧交通项目",
            bid_type="combined",
            source_file={"filename": "招标文件.pdf"},
            requirement_items=requirements,
        )

    with pytest.raises(ValidationError):
        TenderParseStructuredResult(
            project_name="智慧交通项目",
            bid_type="combined",
            source_file={"filename": "招标文件.pdf"},
            parse_metadata={"oversized": "x" * MAX_TENDER_RESPONSE_JSON_BYTES},
        )

    with pytest.raises(ValidationError):
        TenderRequirementItem(
            id="qualification-001",
            module="qualification",
            type="qualification",
            requirement="资质" * (MAX_TENDER_RESPONSE_TEXT_LENGTH + 1),
        )

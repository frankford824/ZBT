from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.schemas.tender import (
    MAX_TENDER_CALLBACK_URL_LENGTH,
    MAX_TENDER_FILENAME_LENGTH,
    MAX_TENDER_OBJECT_KEY_LENGTH,
    MAX_TENDER_TITLE_LENGTH,
    TenderParseRequest,
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

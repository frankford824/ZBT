from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.schemas.export import (
    MAX_EXPORT_ATTACHMENT_COUNT,
    MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH,
    DocumentExportRequest,
    ExportAttachment,
    inline_content_size,
)


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

from __future__ import annotations

import base64
import binascii
from typing import Any, Self

from pydantic import BaseModel, Field, model_validator

MAX_EXPORT_ATTACHMENT_COUNT = 50
MAX_EXPORT_INLINE_ATTACHMENT_BYTES = 20 * 1024 * 1024
MAX_EXPORT_INLINE_ATTACHMENT_TOTAL_BYTES = 50 * 1024 * 1024
MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH = (
    (MAX_EXPORT_INLINE_ATTACHMENT_BYTES + 2) // 3 * 4
)


class ExportChapter(BaseModel):
    title: str
    plain_text: str


class ExportPart(BaseModel):
    code: str = Field(min_length=1, max_length=64)
    title: str = Field(min_length=1, max_length=255)
    chapters: list[ExportChapter] = Field(default_factory=list, max_length=200)
    attachments: list["ExportAttachment"] = Field(default_factory=list, max_length=50)


class ExportAttachment(BaseModel):
    filename: str = Field(min_length=1, max_length=255)
    category: str = Field(default="attachment", max_length=64)
    content_type: str | None = Field(default=None, max_length=255)
    content_base64: str | None = Field(
        default=None,
        max_length=MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH,
    )
    local_path: str | None = None
    object_key: str | None = Field(default=None, max_length=1024)
    zip_path: str | None = Field(default=None, max_length=512)

    @model_validator(mode="after")
    def validate_content_source(self) -> Self:
        if self.local_path and self.local_path.strip():
            raise ValueError("local_path is not allowed")
        has_object_key = bool(self.object_key and self.object_key.strip())
        has_inline_content = bool(self.content_base64 and self.content_base64.strip())
        if has_object_key == has_inline_content:
            raise ValueError("exactly one attachment content source is required")
        if has_inline_content:
            inline_content_size(self.content_base64 or "")
        return self


class ExportLayoutOptions(BaseModel):
    template_name: str = "zbt-standard"
    include_cover: bool = True
    include_toc: bool = True
    include_page_numbers: bool = True
    include_manifest: bool = True
    render_body: bool = True
    validate_pdf: bool = True
    e_bidding_structure: str = "standard"
    header_text: str | None = None
    footer_text: str | None = None
    watermark_text: str | None = None
    generated_at: str | None = None
    context: dict[str, Any] = Field(default_factory=dict)


class DocumentExportRequest(BaseModel):
    task_id: str | None = None
    tenant_id: str
    export_id: str
    bid_id: str
    bid_title: str
    export_type: str = "docx"
    part_code: str
    part_title: str
    filename: str
    object_key: str
    chapters: list[ExportChapter] = Field(default_factory=list, max_length=300)
    parts: list[ExportPart] = Field(default_factory=list, max_length=20)
    attachments: list[ExportAttachment] = Field(default_factory=list, max_length=50)
    boq_files: list[ExportAttachment] = Field(default_factory=list, max_length=50)
    layout: ExportLayoutOptions = Field(default_factory=ExportLayoutOptions)
    callback_url: str | None = None

    @model_validator(mode="after")
    def validate_attachment_budget(self) -> Self:
        attachments = [*self.attachments, *self.boq_files]
        for part in self.parts:
            attachments.extend(part.attachments)
        if len(attachments) > MAX_EXPORT_ATTACHMENT_COUNT:
            raise ValueError("too many export attachments")
        inline_total = sum(
            inline_content_size(attachment.content_base64)
            for attachment in attachments
            if attachment.content_base64
        )
        if inline_total > MAX_EXPORT_INLINE_ATTACHMENT_TOTAL_BYTES:
            raise ValueError("export attachments are too large")
        return self


def inline_content_size(content_base64: str | None) -> int:
    if not content_base64:
        return 0
    if len(content_base64) > MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH:
        raise ValueError("attachment content is too large")
    try:
        content = base64.b64decode(content_base64, validate=True)
    except binascii.Error as exc:
        raise ValueError("attachment content_base64 is invalid") from exc
    if not content or len(content) > MAX_EXPORT_INLINE_ATTACHMENT_BYTES:
        raise ValueError("attachment content is too large")
    return len(content)

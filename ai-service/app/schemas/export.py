from __future__ import annotations

import base64
import binascii
import json
import math
from typing import Annotated, Any, Literal, Self

from pydantic import BaseModel, Field, StringConstraints, field_validator, model_validator

MAX_EXPORT_ATTACHMENT_COUNT = 50
MAX_EXPORT_INLINE_ATTACHMENT_BYTES = 20 * 1024 * 1024
MAX_EXPORT_INLINE_ATTACHMENT_TOTAL_BYTES = 50 * 1024 * 1024
MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH = (
    (MAX_EXPORT_INLINE_ATTACHMENT_BYTES + 2) // 3 * 4
)
MAX_EXPORT_ID_LENGTH = 128
MAX_EXPORT_CODE_LENGTH = 64
MAX_EXPORT_TITLE_LENGTH = 255
MAX_EXPORT_FILENAME_LENGTH = 255
MAX_EXPORT_CONTENT_TYPE_LENGTH = 255
MAX_EXPORT_OBJECT_KEY_LENGTH = 1024
MAX_EXPORT_ZIP_PATH_LENGTH = 512
MAX_EXPORT_CALLBACK_URL_LENGTH = 2048
MAX_EXPORT_LAYOUT_CONTEXT_ITEMS = 50
MAX_EXPORT_LAYOUT_CONTEXT_KEY_LENGTH = 64
MAX_EXPORT_LAYOUT_CONTEXT_STRING_LENGTH = 1000
MAX_EXPORT_LAYOUT_CONTEXT_BYTES = 32 * 1024
MAX_EXPORT_LAYOUT_CONTEXT_DEPTH = 4
MAX_EXPORT_LAYOUT_TEXT_LENGTH = 255
MAX_EXPORT_GENERATED_AT_LENGTH = 64
MAX_EXPORT_CHAPTERS = 300
MAX_EXPORT_PARTS = 20
MAX_EXPORT_PART_CHAPTERS = 200
MAX_EXPORT_TOTAL_CHAPTERS = 500
MAX_EXPORT_CHAPTER_PLAIN_TEXT_LENGTH = 120_000
MAX_EXPORT_TOTAL_CHAPTER_TEXT_LENGTH = 3_000_000

ExportID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_ID_LENGTH),
]
ExportOptionalID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_EXPORT_ID_LENGTH),
]
ExportCode = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_CODE_LENGTH),
]
ExportTitle = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_TITLE_LENGTH),
]
ExportOptionalShortText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_EXPORT_LAYOUT_TEXT_LENGTH),
]
ExportGeneratedAt = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_EXPORT_GENERATED_AT_LENGTH),
]
ExportFilename = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_FILENAME_LENGTH),
]
ExportContentType = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_CONTENT_TYPE_LENGTH),
]
ExportObjectKey = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_OBJECT_KEY_LENGTH),
]
ExportZipPath = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_ZIP_PATH_LENGTH),
]
ExportChapterText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_EXPORT_CHAPTER_PLAIN_TEXT_LENGTH),
]
ExportCallbackURL = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_EXPORT_CALLBACK_URL_LENGTH),
]
ExportType = Literal["docx", "pdf", "zip"]
ExportEBiddingStructure = Literal["standard", "flat"]


class ExportChapter(BaseModel):
    title: ExportTitle
    plain_text: ExportChapterText


class ExportPart(BaseModel):
    code: ExportCode
    title: ExportTitle
    chapters: list[ExportChapter] = Field(default_factory=list, max_length=MAX_EXPORT_PART_CHAPTERS)
    attachments: list["ExportAttachment"] = Field(default_factory=list, max_length=MAX_EXPORT_ATTACHMENT_COUNT)


class ExportAttachment(BaseModel):
    filename: ExportFilename
    category: ExportOptionalShortText = "attachment"
    content_type: ExportContentType | None = None
    content_base64: str | None = Field(
        default=None,
        max_length=MAX_EXPORT_INLINE_ATTACHMENT_BASE64_LENGTH,
    )
    local_path: ExportZipPath | None = None
    object_key: ExportObjectKey | None = None
    zip_path: ExportZipPath | None = None

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
    template_name: ExportOptionalShortText = "zbt-standard"
    include_cover: bool = True
    include_toc: bool = True
    include_page_numbers: bool = True
    include_manifest: bool = True
    render_body: bool = True
    validate_pdf: bool = True
    e_bidding_structure: ExportEBiddingStructure = "standard"
    header_text: ExportOptionalShortText | None = None
    footer_text: ExportOptionalShortText | None = None
    watermark_text: ExportOptionalShortText | None = None
    generated_at: ExportGeneratedAt | None = None
    context: dict[str, Any] = Field(default_factory=dict)

    @field_validator("context")
    @classmethod
    def context_must_be_bounded(cls, value: dict[str, Any]) -> dict[str, Any]:
        return bounded_layout_context(value)


class DocumentExportRequest(BaseModel):
    task_id: ExportOptionalID | None = None
    tenant_id: ExportID
    export_id: ExportID
    bid_id: ExportID
    bid_title: ExportTitle
    export_type: ExportType = "docx"
    part_code: ExportCode
    part_title: ExportTitle
    filename: ExportFilename
    object_key: ExportObjectKey
    chapters: list[ExportChapter] = Field(default_factory=list, max_length=MAX_EXPORT_CHAPTERS)
    parts: list[ExportPart] = Field(default_factory=list, max_length=MAX_EXPORT_PARTS)
    attachments: list[ExportAttachment] = Field(default_factory=list, max_length=MAX_EXPORT_ATTACHMENT_COUNT)
    boq_files: list[ExportAttachment] = Field(default_factory=list, max_length=MAX_EXPORT_ATTACHMENT_COUNT)
    layout: ExportLayoutOptions = Field(default_factory=ExportLayoutOptions)
    callback_url: ExportCallbackURL | None = None

    @model_validator(mode="after")
    def validate_export_budget(self) -> Self:
        attachments = [*self.attachments, *self.boq_files]
        chapters = [*self.chapters]
        for part in self.parts:
            attachments.extend(part.attachments)
            chapters.extend(part.chapters)
        if len(attachments) > MAX_EXPORT_ATTACHMENT_COUNT:
            raise ValueError("too many export attachments")
        if len(chapters) > MAX_EXPORT_TOTAL_CHAPTERS:
            raise ValueError("too many export chapters")
        total_chapter_text = sum(len(chapter.title) + len(chapter.plain_text) for chapter in chapters)
        if total_chapter_text > MAX_EXPORT_TOTAL_CHAPTER_TEXT_LENGTH:
            raise ValueError("export chapters are too large")
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


def bounded_layout_context(value: dict[str, Any]) -> dict[str, Any]:
    if len(value) > MAX_EXPORT_LAYOUT_CONTEXT_ITEMS:
        raise ValueError("layout context has too many items")
    normalized = _bounded_context_dict(value, depth=0)
    try:
        encoded = json.dumps(normalized, ensure_ascii=False, separators=(",", ":"), allow_nan=False)
    except ValueError as exc:
        raise ValueError("layout context contains invalid numeric values") from exc
    if len(encoded.encode("utf-8")) > MAX_EXPORT_LAYOUT_CONTEXT_BYTES:
        raise ValueError("layout context is too large")
    return normalized


def _bounded_context_dict(value: dict[str, Any], *, depth: int) -> dict[str, Any]:
    if depth >= MAX_EXPORT_LAYOUT_CONTEXT_DEPTH:
        raise ValueError("layout context is too deep")
    if len(value) > MAX_EXPORT_LAYOUT_CONTEXT_ITEMS:
        raise ValueError("layout context has too many items")
    normalized: dict[str, Any] = {}
    for raw_key, raw_value in value.items():
        key = _bounded_context_key(raw_key)
        if key in normalized:
            raise ValueError("layout context contains duplicate keys after trimming")
        normalized[key] = _bounded_context_value(raw_value, depth=depth + 1)
    return normalized


def _bounded_context_key(value: object) -> str:
    if not isinstance(value, str):
        raise ValueError("layout context keys must be strings")
    key = value.strip()
    if not key or len(key) > MAX_EXPORT_LAYOUT_CONTEXT_KEY_LENGTH:
        raise ValueError("layout context key is invalid")
    return key


def _bounded_context_value(value: Any, *, depth: int) -> Any:
    if value is None or isinstance(value, bool | int):
        return value
    if isinstance(value, float):
        if not math.isfinite(value):
            raise ValueError("layout context contains invalid numeric values")
        return value
    if isinstance(value, str):
        text = value.strip()
        if len(text) > MAX_EXPORT_LAYOUT_CONTEXT_STRING_LENGTH:
            raise ValueError("layout context string is too long")
        return text
    if isinstance(value, list):
        if len(value) > MAX_EXPORT_LAYOUT_CONTEXT_ITEMS:
            raise ValueError("layout context list has too many items")
        if depth >= MAX_EXPORT_LAYOUT_CONTEXT_DEPTH:
            raise ValueError("layout context is too deep")
        return [_bounded_context_value(item, depth=depth + 1) for item in value]
    if isinstance(value, dict):
        return _bounded_context_dict(value, depth=depth)
    raise ValueError("layout context value type is not supported")

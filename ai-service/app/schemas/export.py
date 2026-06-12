from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class ExportChapter(BaseModel):
    title: str
    plain_text: str


class ExportPart(BaseModel):
    code: str
    title: str
    chapters: list[ExportChapter] = Field(default_factory=list)
    attachments: list["ExportAttachment"] = Field(default_factory=list)


class ExportAttachment(BaseModel):
    filename: str
    category: str = "attachment"
    content_type: str | None = None
    content_base64: str | None = None
    local_path: str | None = None
    object_key: str | None = None
    zip_path: str | None = None


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
    tenant_id: str
    export_id: str
    bid_id: str
    bid_title: str
    export_type: str = "docx"
    part_code: str
    part_title: str
    filename: str
    object_key: str
    chapters: list[ExportChapter] = Field(default_factory=list)
    parts: list[ExportPart] = Field(default_factory=list)
    attachments: list[ExportAttachment] = Field(default_factory=list)
    boq_files: list[ExportAttachment] = Field(default_factory=list)
    layout: ExportLayoutOptions = Field(default_factory=ExportLayoutOptions)
    callback_url: str | None = None

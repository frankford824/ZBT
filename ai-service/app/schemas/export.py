from __future__ import annotations

from pydantic import BaseModel, Field


class ExportChapter(BaseModel):
    title: str
    plain_text: str


class ExportPart(BaseModel):
    code: str
    title: str
    chapters: list[ExportChapter] = Field(default_factory=list)


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
    callback_url: str | None = None

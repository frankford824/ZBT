from __future__ import annotations

from pydantic import BaseModel, Field


class KnowledgeProcessRequest(BaseModel):
    tenant_id: str
    document_id: str
    file_id: str
    object_key: str
    filename: str
    content_type: str
    callback_url: str | None = None


class KnowledgeChunk(BaseModel):
    title: str
    content: str
    section_path: str
    page_start: int | None = None
    page_end: int | None = None
    metadata: dict[str, object] = Field(default_factory=dict)


class KnowledgeProcessResult(BaseModel):
    processed_title: str
    summary: str
    chunks: list[KnowledgeChunk]
    metadata: dict[str, object]

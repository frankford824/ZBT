from __future__ import annotations

from pydantic import BaseModel, Field

from app.schemas.common import SourceRef


class ChapterGenerateRequest(BaseModel):
    task_id: str | None = None
    tenant_id: str
    bid_document_id: str
    bid_part_id: str
    chapter_id: str
    chapter_title: str
    tender_requirements: list[str] = Field(default_factory=list)
    selected_knowledge_refs: list[str] = Field(default_factory=list)
    callback_url: str | None = None
    model_hint: str | None = None


class ChapterGenerateResponse(BaseModel):
    trace_id: str
    tiptap_json: dict[str, object]
    source_refs: list[SourceRef]
    self_check: dict[str, object]
    needs_human_input: list[str]
    model_metadata: dict[str, object]
    token_usage: dict[str, int]

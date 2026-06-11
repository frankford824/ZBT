from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field


class HealthResponse(BaseModel):
    service: str
    status: str
    time: datetime


class TaskAccepted(BaseModel):
    task_id: str
    status: str = "queued"
    route: dict[str, object] = Field(default_factory=dict)


class SourceRef(BaseModel):
    chunk_id: str
    document_id: str
    title: str
    page_start: int | None = None
    page_end: int | None = None

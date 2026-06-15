from __future__ import annotations

from pydantic import BaseModel


class TenderParseRequest(BaseModel):
    task_id: str | None = None
    tenant_id: str
    bid_id: str | None = None
    bid_title: str | None = None
    file_id: str
    object_key: str
    filename: str
    content_type: str
    callback_url: str | None = None

from __future__ import annotations

from pydantic import BaseModel


class KnowledgeProcessRequest(BaseModel):
    tenant_id: str
    document_id: str
    file_id: str
    object_key: str
    filename: str
    content_type: str
    callback_url: str | None = None

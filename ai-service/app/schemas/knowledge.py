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
    embedding: list[float] | None = None


class KnowledgeProcessResult(BaseModel):
    processed_title: str
    summary: str
    chunks: list[KnowledgeChunk]
    metadata: dict[str, object]


class KnowledgeEmbeddingRequest(BaseModel):
    tenant_id: str
    texts: list[str] = Field(min_length=1, max_length=32)


class KnowledgeEmbeddingResponse(BaseModel):
    provider: str
    model: str
    dimensions: int
    embeddings: list[list[float]]
    route: dict[str, object]


class KnowledgeRerankDocument(BaseModel):
    id: str
    title: str
    content: str
    section_path: str = ""
    score: float = 0.0


class KnowledgeRerankRequest(BaseModel):
    tenant_id: str
    query: str
    documents: list[KnowledgeRerankDocument] = Field(min_length=1, max_length=60)
    top_k: int = Field(default=8, ge=1, le=20)


class KnowledgeRerankResult(BaseModel):
    id: str
    index: int
    score: float


class KnowledgeRerankResponse(BaseModel):
    provider: str
    model: str
    results: list[KnowledgeRerankResult]
    route: dict[str, object]

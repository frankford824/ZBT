from __future__ import annotations

from typing import Annotated

from pydantic import BaseModel, Field, StringConstraints

MAX_KNOWLEDGE_TENANT_ID_LENGTH = 128
MAX_KNOWLEDGE_EMBEDDING_TEXT_LENGTH = 12000
MAX_KNOWLEDGE_RERANK_QUERY_LENGTH = 2000
MAX_KNOWLEDGE_RERANK_DOCUMENT_ID_LENGTH = 128
MAX_KNOWLEDGE_RERANK_TITLE_LENGTH = 255
MAX_KNOWLEDGE_RERANK_SECTION_PATH_LENGTH = 512
MAX_KNOWLEDGE_RERANK_CONTENT_LENGTH = 2400

KnowledgeTenantID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_KNOWLEDGE_TENANT_ID_LENGTH),
]
KnowledgeEmbeddingText = Annotated[
    str,
    StringConstraints(
        strip_whitespace=True,
        min_length=1,
        max_length=MAX_KNOWLEDGE_EMBEDDING_TEXT_LENGTH,
    ),
]
KnowledgeRerankQuery = Annotated[
    str,
    StringConstraints(
        strip_whitespace=True,
        min_length=1,
        max_length=MAX_KNOWLEDGE_RERANK_QUERY_LENGTH,
    ),
]
KnowledgeRerankDocumentID = Annotated[
    str,
    StringConstraints(
        strip_whitespace=True,
        min_length=1,
        max_length=MAX_KNOWLEDGE_RERANK_DOCUMENT_ID_LENGTH,
    ),
]
KnowledgeRerankTitle = Annotated[
    str,
    StringConstraints(
        strip_whitespace=True,
        min_length=1,
        max_length=MAX_KNOWLEDGE_RERANK_TITLE_LENGTH,
    ),
]
KnowledgeRerankSectionPath = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_KNOWLEDGE_RERANK_SECTION_PATH_LENGTH),
]
KnowledgeRerankContent = Annotated[
    str,
    StringConstraints(
        strip_whitespace=True,
        min_length=1,
        max_length=MAX_KNOWLEDGE_RERANK_CONTENT_LENGTH,
    ),
]


class KnowledgeProcessRequest(BaseModel):
    task_id: str | None = None
    tenant_id: KnowledgeTenantID
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
    tenant_id: KnowledgeTenantID
    texts: list[KnowledgeEmbeddingText] = Field(min_length=1, max_length=32)


class KnowledgeEmbeddingResponse(BaseModel):
    provider: str
    model: str
    dimensions: int
    embeddings: list[list[float]]
    route: dict[str, object]
    token_usage: dict[str, int] = Field(default_factory=dict)
    estimated_cost: float = 0
    quota_usage: dict[str, object] = Field(default_factory=dict)


class KnowledgeRerankDocument(BaseModel):
    id: KnowledgeRerankDocumentID
    title: KnowledgeRerankTitle
    content: KnowledgeRerankContent
    section_path: KnowledgeRerankSectionPath = ""
    score: float = 0.0


class KnowledgeRerankRequest(BaseModel):
    tenant_id: KnowledgeTenantID
    query: KnowledgeRerankQuery
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
    token_usage: dict[str, int] = Field(default_factory=dict)
    estimated_cost: float = 0
    quota_usage: dict[str, object] = Field(default_factory=dict)

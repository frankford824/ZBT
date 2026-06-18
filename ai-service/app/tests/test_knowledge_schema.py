from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.schemas.knowledge import (
    MAX_KNOWLEDGE_EMBEDDING_TEXT_LENGTH,
    MAX_KNOWLEDGE_RERANK_CONTENT_LENGTH,
    MAX_KNOWLEDGE_RERANK_DOCUMENT_ID_LENGTH,
    MAX_KNOWLEDGE_RERANK_QUERY_LENGTH,
    MAX_KNOWLEDGE_RERANK_SECTION_PATH_LENGTH,
    MAX_KNOWLEDGE_RERANK_TITLE_LENGTH,
    KnowledgeEmbeddingRequest,
    KnowledgeRerankDocument,
    KnowledgeRerankRequest,
)


def test_knowledge_embedding_request_rejects_oversized_or_empty_text() -> None:
    with pytest.raises(ValidationError):
        KnowledgeEmbeddingRequest(
            tenant_id="tenant-demo",
            texts=["x" * (MAX_KNOWLEDGE_EMBEDDING_TEXT_LENGTH + 1)],
        )

    with pytest.raises(ValidationError):
        KnowledgeEmbeddingRequest(tenant_id="tenant-demo", texts=[""])

    with pytest.raises(ValidationError):
        KnowledgeEmbeddingRequest(tenant_id="tenant-demo", texts=["   "])


def test_knowledge_rerank_request_rejects_oversized_query() -> None:
    document = KnowledgeRerankDocument(id="chunk-1", title="标题", content="正文")

    with pytest.raises(ValidationError):
        KnowledgeRerankRequest(
            tenant_id="tenant-demo",
            query="x" * (MAX_KNOWLEDGE_RERANK_QUERY_LENGTH + 1),
            documents=[document],
        )


def test_knowledge_rerank_document_rejects_oversized_fields() -> None:
    oversized_cases = [
        {"id": "x" * (MAX_KNOWLEDGE_RERANK_DOCUMENT_ID_LENGTH + 1), "title": "标题", "content": "正文"},
        {"id": "chunk-1", "title": "x" * (MAX_KNOWLEDGE_RERANK_TITLE_LENGTH + 1), "content": "正文"},
        {"id": "chunk-1", "title": "标题", "content": "x" * (MAX_KNOWLEDGE_RERANK_CONTENT_LENGTH + 1)},
        {
            "id": "chunk-1",
            "title": "标题",
            "content": "正文",
            "section_path": "x" * (MAX_KNOWLEDGE_RERANK_SECTION_PATH_LENGTH + 1),
        },
    ]

    for payload in oversized_cases:
        with pytest.raises(ValidationError):
            KnowledgeRerankDocument(**payload)


def test_knowledge_rerank_document_rejects_empty_required_fields() -> None:
    for payload in (
        {"id": "", "title": "标题", "content": "正文"},
        {"id": "   ", "title": "标题", "content": "正文"},
        {"id": "chunk-1", "title": "", "content": "正文"},
        {"id": "chunk-1", "title": "   ", "content": "正文"},
        {"id": "chunk-1", "title": "标题", "content": ""},
        {"id": "chunk-1", "title": "标题", "content": "   "},
    ):
        with pytest.raises(ValidationError):
            KnowledgeRerankDocument(**payload)


def test_knowledge_rerank_request_strips_bounded_text_fields() -> None:
    request = KnowledgeRerankRequest(
        tenant_id=" tenant-demo ",
        query=" 智慧交通 ",
        documents=[
            KnowledgeRerankDocument(
                id=" chunk-1 ",
                title=" 标题 ",
                content=" 正文 ",
                section_path=" 第一章 ",
            )
        ],
    )

    assert request.tenant_id == "tenant-demo"
    assert request.query == "智慧交通"
    assert request.documents[0].id == "chunk-1"
    assert request.documents[0].title == "标题"
    assert request.documents[0].content == "正文"
    assert request.documents[0].section_path == "第一章"

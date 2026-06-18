from __future__ import annotations

import math

import pytest
from pydantic import ValidationError

from app.schemas.generation import (
    MAX_CHAPTER_ACTION_PLAIN_TEXT_LENGTH,
    MAX_CHAPTER_ACTION_TIPTAP_JSON_BYTES,
    MAX_REQUIREMENT_REFS,
    MAX_RETRIEVED_KNOWLEDGE_CONTENT_LENGTH,
    MAX_RETRIEVED_KNOWLEDGE_REFS,
    MAX_SELECTED_KNOWLEDGE_REFS,
    MAX_TENDER_REQUIREMENT_LENGTH,
    MAX_TENDER_REQUIREMENTS,
    ChapterActionRequest,
    ChapterGenerateRequest,
    RetrievedKnowledgeRef,
    TenderRequirementRef,
)


def _chapter_payload(**extra: object) -> dict[str, object]:
    payload: dict[str, object] = {
        "tenant_id": "tenant-demo",
        "bid_document_id": "bid-demo",
        "bid_part_id": "part-tech",
        "chapter_id": "chapter-demo",
        "chapter_title": "技术方案",
    }
    payload.update(extra)
    return payload


def _requirement_ref(index: int = 0) -> TenderRequirementRef:
    return TenderRequirementRef(
        id=f"requirement-{index}",
        module="evaluation",
        type="scoring",
        requirement=f"技术方案完整性评分 {index} 分",
        priority="high",
        score=20,
    )


def _knowledge_ref(index: int = 0) -> RetrievedKnowledgeRef:
    return RetrievedKnowledgeRef(
        chunk_id=f"chunk-{index}",
        document_id=f"document-{index}",
        title=f"知识素材-{index}",
        content="用于章节生成的素材",
        score=0.8,
    )


def test_chapter_generate_request_rejects_oversized_lists() -> None:
    with pytest.raises(ValidationError):
        ChapterGenerateRequest(
            **_chapter_payload(
                tender_requirements=["响应要求"] * (MAX_TENDER_REQUIREMENTS + 1),
            )
        )

    with pytest.raises(ValidationError):
        ChapterGenerateRequest(
            **_chapter_payload(
                requirement_refs=[_requirement_ref(index) for index in range(MAX_REQUIREMENT_REFS + 1)],
            )
        )

    with pytest.raises(ValidationError):
        ChapterGenerateRequest(
            **_chapter_payload(
                selected_knowledge_refs=[f"chunk-{index}" for index in range(MAX_SELECTED_KNOWLEDGE_REFS + 1)],
            )
        )

    with pytest.raises(ValidationError):
        ChapterGenerateRequest(
            **_chapter_payload(
                retrieved_knowledge_refs=[_knowledge_ref(index) for index in range(MAX_RETRIEVED_KNOWLEDGE_REFS + 1)],
            )
        )


def test_chapter_generate_request_rejects_oversized_text_fields() -> None:
    with pytest.raises(ValidationError):
        ChapterGenerateRequest(
            **_chapter_payload(tender_requirements=["x" * (MAX_TENDER_REQUIREMENT_LENGTH + 1)])
        )

    with pytest.raises(ValidationError):
        RetrievedKnowledgeRef(
            chunk_id="chunk-demo",
            document_id="document-demo",
            title="知识素材",
            content="x" * (MAX_RETRIEVED_KNOWLEDGE_CONTENT_LENGTH + 1),
        )


def test_chapter_generate_request_rejects_invalid_scores_and_required_text() -> None:
    with pytest.raises(ValidationError):
        RetrievedKnowledgeRef(
            chunk_id="chunk-demo",
            document_id="document-demo",
            title="知识素材",
            score=math.nan,
        )

    with pytest.raises(ValidationError):
        TenderRequirementRef(id="requirement-demo", requirement="   ")

    with pytest.raises(ValidationError):
        TenderRequirementRef(id="requirement-demo", requirement="响应要求", priority="urgent")


def test_chapter_action_request_rejects_invalid_action_and_oversized_body() -> None:
    with pytest.raises(ValidationError):
        ChapterActionRequest(**_chapter_payload(action="rewrite"))

    with pytest.raises(ValidationError):
        ChapterActionRequest(
            **_chapter_payload(current_plain_text="x" * (MAX_CHAPTER_ACTION_PLAIN_TEXT_LENGTH + 1))
        )


def test_chapter_action_request_rejects_oversized_tiptap_json() -> None:
    oversized_text = "x" * MAX_CHAPTER_ACTION_TIPTAP_JSON_BYTES
    with pytest.raises(ValidationError):
        ChapterActionRequest(
            **_chapter_payload(
                current_tiptap_json={
                    "type": "doc",
                    "content": [{"type": "paragraph", "text": oversized_text}],
                }
            )
        )


def test_chapter_requests_strip_bounded_text_fields() -> None:
    request = ChapterActionRequest(
        **_chapter_payload(
            task_id=" task-demo ",
            tenant_id=" tenant-demo ",
            chapter_title=" 技术方案 ",
            tender_requirements=[" 响应招标文件 "],
            requirement_refs=[
                TenderRequirementRef(
                    id=" requirement-demo ",
                    module=" evaluation ",
                    type=" scoring ",
                    requirement=" 技术方案完整性 ",
                    expected_response=" 保留来源 ",
                    source_text=" 招标原文 ",
                )
            ],
            selected_knowledge_refs=[" chunk-demo "],
            retrieved_knowledge_refs=[
                RetrievedKnowledgeRef(
                    chunk_id=" chunk-demo ",
                    document_id=" document-demo ",
                    title=" 知识素材 ",
                    section_path=" 第一章 ",
                    content=" 素材正文 ",
                )
            ],
            callback_url=" http://backend:8080/api/v1/ai/callbacks/tasks ",
            model_hint=" model-a ",
            instruction=" 优化表达 ",
            current_plain_text=" 原文 ",
        )
    )

    assert request.task_id == "task-demo"
    assert request.tenant_id == "tenant-demo"
    assert request.chapter_title == "技术方案"
    assert request.tender_requirements == ["响应招标文件"]
    assert request.requirement_refs[0].id == "requirement-demo"
    assert request.requirement_refs[0].module == "evaluation"
    assert request.requirement_refs[0].type == "scoring"
    assert request.requirement_refs[0].requirement == "技术方案完整性"
    assert request.requirement_refs[0].expected_response == "保留来源"
    assert request.requirement_refs[0].source_text == "招标原文"
    assert request.selected_knowledge_refs == ["chunk-demo"]
    assert request.retrieved_knowledge_refs[0].chunk_id == "chunk-demo"
    assert request.retrieved_knowledge_refs[0].document_id == "document-demo"
    assert request.retrieved_knowledge_refs[0].title == "知识素材"
    assert request.retrieved_knowledge_refs[0].section_path == "第一章"
    assert request.retrieved_knowledge_refs[0].content == "素材正文"
    assert request.callback_url == "http://backend:8080/api/v1/ai/callbacks/tasks"
    assert request.model_hint == "model-a"
    assert request.instruction == "优化表达"
    assert request.current_plain_text == "原文"

from pathlib import Path

from app.gateway.model_router import ModelRouter
from app.schemas.generation import ChapterGenerateRequest, RetrievedKnowledgeRef


def _dot(left: list[float], right: list[float]) -> float:
    return sum(left_value * right_value for left_value, right_value in zip(left, right, strict=True))


def test_model_router_resolves_mock_provider() -> None:
    router = ModelRouter.from_yaml(Path("app/config/model_routing.yaml"))
    target = router.resolve("chapter_generate", tenant_id="tenant-demo")

    assert target.provider == "mock"
    assert target.require_source_refs is True


def test_mock_chapter_generation_has_source_refs() -> None:
    router = ModelRouter.from_yaml(Path("app/config/model_routing.yaml"))
    provider = router.get_llm("chapter_generate", tenant_id="tenant-demo")
    response = provider.generate_chapter(
        ChapterGenerateRequest(
            tenant_id="tenant-demo",
            bid_document_id="bid-demo",
            bid_part_id="part-tech",
            chapter_id="chapter-demo",
            chapter_title="技术方案",
        )
    )

    assert response.source_refs
    assert response.needs_human_input


def test_mock_chapter_generation_prefers_retrieved_refs() -> None:
    router = ModelRouter.from_yaml(Path("app/config/model_routing.yaml"))
    provider = router.get_llm("chapter_generate", tenant_id="tenant-demo")
    response = provider.generate_chapter(
        ChapterGenerateRequest(
            tenant_id="tenant-demo",
            bid_document_id="bid-demo",
            bid_part_id="part-tech",
            chapter_id="chapter-demo",
            chapter_title="技术方案",
            retrieved_knowledge_refs=[
                RetrievedKnowledgeRef(
                    chunk_id="00000000-0000-4000-8000-00000000c001",
                    document_id="00000000-0000-4000-8000-00000000d001",
                    title="真实知识库素材",
                    content="用于章节生成的真实 chunk",
                )
            ],
        )
    )

    assert response.source_refs[0].chunk_id == "00000000-0000-4000-8000-00000000c001"
    assert response.self_check["retrieved_ref_count"] == 1


def test_mock_embedding_is_deterministic_and_semantic_enough_for_pgvector_smoke() -> None:
    router = ModelRouter.from_yaml(Path("app/config/model_routing.yaml"))
    provider = router.get_embedding("knowledge_embedding", tenant_id="tenant-demo")

    query = provider.embed_text("智慧交通 项目理解")
    related = provider.embed_text("智慧交通平台项目理解与实施方案")
    unrelated = provider.embed_text("财务审计报表与资产折旧说明")

    assert len(query) == 1024
    assert query == provider.embed_text("智慧交通 项目理解")
    assert _dot(query, related) > _dot(query, unrelated)

from pathlib import Path

import pytest

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


def test_mock_rerank_prefers_query_overlap() -> None:
    router = ModelRouter.from_yaml(Path("app/config/model_routing.yaml"))
    provider = router.get_rerank("knowledge_rerank", tenant_id="tenant-demo")

    order = provider.rerank(
        "智慧交通 项目实施",
        [
            "财务审计报表与资产折旧说明",
            "智慧交通平台项目实施方案与里程碑计划",
            "公司人员资质证书清单",
        ],
    )

    assert order[0] == 1


def test_openai_compatible_provider_is_registered_but_unhealthy_without_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    router = ModelRouter.from_yaml(Path("app/config/model_routing.yaml"))

    health = router.health_check()

    assert "openai_compatible_primary" in health
    assert health["openai_compatible_primary"] is False


def test_router_uses_explicit_fallback_when_primary_provider_unavailable(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    router = ModelRouter(
        {
            "providers": {
                "mock": {"type": "mock"},
                "openai_compatible_primary": {
                    "type": "openai_compatible",
                    "base_url_env": "OPENAI_BASE_URL",
                    "api_key_env": "OPENAI_API_KEY",
                },
            },
            "routes": {
                "chapter_generate": {
                    "primary": {"provider": "openai_compatible_primary", "model": "real-model"},
                    "fallback": [{"provider": "mock", "model": "mock-model"}],
                }
            },
        }
    )

    target = router.resolve("chapter_generate", tenant_id="tenant-demo")

    assert target.provider == "mock"
    assert target.fallback_from == "openai_compatible_primary"


def test_shipped_routing_config_declares_only_buildable_providers() -> None:
    router = ModelRouter.from_yaml(Path("app/config/model_routing.yaml"))
    declared = set(router.config.get("providers", {}).keys())
    assert declared == set(router.providers.keys())


def test_router_rejects_unsupported_provider_type() -> None:
    with pytest.raises(ValueError, match="unsupported type"):
        ModelRouter(
            {
                "providers": {"acme": {"type": "anthropic"}},
                "routes": {},
            }
        )


def test_router_does_not_silently_fallback_to_mock(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    router = ModelRouter(
        {
            "providers": {
                "mock": {"type": "mock"},
                "openai_compatible_primary": {
                    "type": "openai_compatible",
                    "base_url_env": "OPENAI_BASE_URL",
                    "api_key_env": "OPENAI_API_KEY",
                },
            },
            "routes": {
                "chapter_generate": {
                    "primary": {"provider": "openai_compatible_primary", "model": "real-model"},
                }
            },
        }
    )

    with pytest.raises(RuntimeError, match="no configured provider"):
        router.get_llm("chapter_generate", tenant_id="tenant-demo")

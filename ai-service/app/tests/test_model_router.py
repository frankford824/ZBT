from pathlib import Path

from app.gateway.model_router import ModelRouter
from app.schemas.generation import ChapterGenerateRequest


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

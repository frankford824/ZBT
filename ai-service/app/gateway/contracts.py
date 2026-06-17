from __future__ import annotations

from typing import Protocol

from app.schemas.generation import ChapterActionRequest, ChapterGenerateRequest, ChapterGenerateResponse


class LLMProvider(Protocol):
    def complete(self, prompt: str) -> str: ...

    def generate_json(self, prompt: str, schema_name: str) -> dict[str, object]: ...

    def stream(self, prompt: str) -> list[str]: ...

    def count_tokens(self, text: str) -> int: ...

    def health_check(self) -> bool: ...

    def generate_chapter(self, payload: ChapterGenerateRequest) -> ChapterGenerateResponse: ...

    def chapter_action(self, payload: ChapterActionRequest) -> ChapterGenerateResponse: ...


class EmbeddingProvider(Protocol):
    def embed_text(self, text: str) -> list[float]: ...

    def embed_batch(self, texts: list[str]) -> list[list[float]]: ...

    def get_dimensions(self) -> int: ...

    def health_check(self) -> bool: ...


class RerankProvider(Protocol):
    def rerank(self, query: str, documents: list[str]) -> list[int]: ...

    def health_check(self) -> bool: ...


class OCRProvider(Protocol):
    def recognize_document(
        self,
        *,
        filename: str,
        content_type: str,
        content: bytes,
    ) -> dict[str, object]: ...

    def recognize_page(
        self,
        *,
        filename: str,
        content_type: str,
        content: bytes,
        page_index: int | None = None,
    ) -> dict[str, object]: ...

    def extract_layout(self, result: dict[str, object]) -> list[dict[str, object]]: ...

    def extract_tables(self, result: dict[str, object]) -> list[dict[str, object]]: ...

    def health_check(self) -> bool: ...

from __future__ import annotations

from app.schemas.common import SourceRef
from app.schemas.generation import ChapterGenerateRequest, ChapterGenerateResponse


class MockProvider:
    name = "mock"

    def complete(self, prompt: str) -> str:
        return f"mock completion for: {prompt[:80]}"

    def generate_json(self, prompt: str, schema_name: str) -> dict[str, object]:
        return {"schema": schema_name, "summary": self.complete(prompt)}

    def stream(self, prompt: str) -> list[str]:
        return [self.complete(prompt)]

    def count_tokens(self, text: str) -> int:
        return max(1, len(text) // 4)

    def health_check(self) -> bool:
        return True

    def embed_text(self, text: str) -> list[float]:
        seed = float(min(len(text), 100)) / 100
        return [seed] * 1024

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        return [self.embed_text(text) for text in texts]

    def get_dimensions(self) -> int:
        return 1024

    def rerank(self, query: str, documents: list[str]) -> list[int]:
        return list(range(len(documents)))

    def parse_pdf(self, object_key: str) -> dict[str, object]:
        return {"object_key": object_key, "pages": [], "ocr_required": False}

    def parse_image(self, object_key: str) -> dict[str, object]:
        return {"object_key": object_key, "text": "", "confidence": 1.0}

    def extract_layout(self, object_key: str) -> dict[str, object]:
        return {"object_key": object_key, "blocks": []}

    def extract_tables(self, object_key: str) -> list[dict[str, object]]:
        return []

    def generate_chapter(self, payload: ChapterGenerateRequest) -> ChapterGenerateResponse:
        return ChapterGenerateResponse(
            trace_id="trace-mock-chapter",
            tiptap_json={
                "type": "doc",
                "content": [
                    {
                        "type": "paragraph",
                        "content": [
                            {
                                "type": "text",
                                "text": f"{payload.chapter_title} 根据招标要求和知识库素材生成。",
                            }
                        ],
                    }
                ],
            },
            source_refs=[
                SourceRef(
                    chunk_id="chunk-demo",
                    document_id="doc-demo",
                    title="智慧交通实施案例",
                    page_start=12,
                    page_end=15,
                )
            ],
            self_check={"status": "pass", "notes": ["mock provider validates schema only"]},
            needs_human_input=["企业资质证书编号", "项目经理证书有效期"],
            model_metadata={"provider": self.name, "model": payload.model_hint or "mock-model"},
            token_usage={"input_tokens": 128, "output_tokens": 256},
        )

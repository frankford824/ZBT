from __future__ import annotations

import hashlib
import math
import re

from app.schemas.common import SourceRef
from app.schemas.generation import ChapterActionRequest, ChapterGenerateRequest, ChapterGenerateResponse


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
        dimensions = self.get_dimensions()
        vector = [0.0] * dimensions
        for token in _embedding_tokens(text):
            digest = hashlib.sha256(token.encode("utf-8")).digest()
            index = int.from_bytes(digest[:4], "big") % dimensions
            weight = 1.0 + min(len(token), 8) / 8.0
            vector[index] += weight
        norm = math.sqrt(sum(value * value for value in vector))
        if norm == 0:
            return self.embed_text("__empty__")
        return [value / norm for value in vector]

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
        refs = [
            SourceRef(
                chunk_id=ref.chunk_id,
                document_id=ref.document_id,
                title=ref.title,
                page_start=ref.page_start,
                page_end=ref.page_end,
            )
            for ref in payload.retrieved_knowledge_refs[:5]
        ]
        if not refs:
            refs = [
                SourceRef(
                    chunk_id="chunk-demo",
                    document_id="doc-demo",
                    title="智慧交通实施案例",
                    page_start=12,
                    page_end=15,
                )
            ]
        context_titles = "、".join(ref.title for ref in refs[:3])
        context_text = f"已引用知识库素材：{context_titles}。" if context_titles else "未检索到可引用知识库素材。"
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
                                "text": f"{payload.chapter_title} 根据招标要求和知识库素材生成。{context_text}",
                            }
                        ],
                    }
                ],
            },
            source_refs=refs,
            self_check={
                "status": "pass",
                "notes": ["mock provider validates schema only"],
                "retrieved_ref_count": len(payload.retrieved_knowledge_refs),
            },
            needs_human_input=["企业资质证书编号", "项目经理证书有效期"],
            model_metadata={"provider": self.name, "model": payload.model_hint or "mock-model"},
            token_usage={
                "input_tokens": 128 + sum(self.count_tokens(ref.content) for ref in payload.retrieved_knowledge_refs[:5]),
                "output_tokens": 256,
            },
        )

    def chapter_action(self, payload: ChapterActionRequest) -> ChapterGenerateResponse:
        refs = [
            SourceRef(
                chunk_id=ref.chunk_id,
                document_id=ref.document_id,
                title=ref.title,
                page_start=ref.page_start,
                page_end=ref.page_end,
            )
            for ref in payload.retrieved_knowledge_refs[:5]
        ]
        if not refs:
            refs = [
                SourceRef(
                    chunk_id="chunk-demo",
                    document_id="doc-demo",
                    title="章节改写参考资料",
                    page_start=1,
                    page_end=1,
                )
            ]
        action_labels = {
            "optimize": "优化表达",
            "expand": "扩写内容",
            "shorten": "压缩篇幅",
            "add_detail": "补充细节",
            "self_check": "章节自检",
        }
        action_label = action_labels.get(payload.action, "优化表达")
        base_text = payload.current_plain_text.strip() or f"{payload.chapter_title} 请补充章节内容。"
        if payload.action == "shorten":
            rewritten = f"{action_label}：{base_text[:120]}。"
        elif payload.action == "self_check":
            rewritten = base_text
        else:
            rewritten = f"{action_label}：{base_text} 已结合招标要求和知识库引用进行处理。"
        return ChapterGenerateResponse(
            trace_id=f"trace-mock-chapter-action-{payload.action}",
            tiptap_json={
                "type": "doc",
                "content": [
                    {
                        "type": "paragraph",
                        "content": [{"type": "text", "text": rewritten}],
                    }
                ],
            },
            source_refs=refs,
            self_check={
                "status": "pass" if payload.action != "self_check" else "needs_review",
                "action": payload.action,
                "notes": [
                    "mock rewrite assistant validates schema only",
                    "事实性企业资质、证书、人员和金额仍需人工核对",
                ],
                "retrieved_ref_count": len(payload.retrieved_knowledge_refs),
            },
            needs_human_input=["需人工复核事实性资质、人员、证书、金额和日期"],
            model_metadata={"provider": self.name, "model": payload.model_hint or "mock-rewrite-model", "action": payload.action},
            token_usage={
                "input_tokens": self.count_tokens(base_text) + sum(self.count_tokens(ref.content) for ref in payload.retrieved_knowledge_refs[:5]),
                "output_tokens": self.count_tokens(rewritten),
            },
        )


def _embedding_tokens(text: str) -> list[str]:
    normalized = text.lower()
    tokens: list[str] = []
    tokens.extend(re.findall(r"[a-z0-9]+", normalized))
    for segment in re.findall(r"[\u4e00-\u9fff]+", normalized):
        tokens.extend(segment)
        if len(segment) > 1:
            tokens.extend(segment[index : index + 2] for index in range(len(segment) - 1))
        if len(segment) > 2:
            tokens.extend(segment[index : index + 3] for index in range(len(segment) - 2))
    if not tokens:
        tokens.append("__empty__")
    return tokens

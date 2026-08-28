from __future__ import annotations

import hashlib
import math
import re

from app.schemas.common import SourceRef
from app.schemas.cost import CostAdviceRequest, CostAdviceResponse
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

    def list_models(self) -> list[str]:
        return []

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
        query_tokens = set(_embedding_tokens(query))
        if not query_tokens:
            return list(range(len(documents)))
        normalized_query = query.lower().strip()
        scored: list[tuple[float, int]] = []
        for index, document in enumerate(documents):
            document_tokens = set(_embedding_tokens(document))
            overlap = len(query_tokens & document_tokens)
            coverage = overlap / max(len(query_tokens), 1)
            density = overlap / max(len(document_tokens), 1)
            phrase_bonus = 1.0 if normalized_query and normalized_query in document.lower() else 0.0
            scored.append((phrase_bonus + coverage + density, index))
        return [index for _, index in sorted(scored, key=lambda item: (-item[0], item[1]))]

    def recognize_document(
        self,
        *,
        filename: str,
        content_type: str,
        content: bytes,
    ) -> dict[str, object]:
        return {
            "filename": filename,
            "content_type": content_type,
            "size_bytes": len(content),
            "text": "",
            "pages": [],
            "blocks": [],
            "tables": [],
            "confidence": 1.0,
        }

    def recognize_page(
        self,
        *,
        filename: str,
        content_type: str,
        content: bytes,
        page_index: int | None = None,
    ) -> dict[str, object]:
        result = self.recognize_document(filename=filename, content_type=content_type, content=content)
        result["page"] = page_index
        return result

    def extract_layout(self, result: dict[str, object]) -> list[dict[str, object]]:
        blocks = result.get("blocks")
        return blocks if isinstance(blocks, list) else []

    def extract_tables(self, result: dict[str, object]) -> list[dict[str, object]]:
        tables = result.get("tables")
        return tables if isinstance(tables, list) else []

    def parse_pdf(self, object_key: str) -> dict[str, object]:
        return {"object_key": object_key, "pages": [], "ocr_required": False}

    def parse_image(self, object_key: str) -> dict[str, object]:
        return {"object_key": object_key, "text": "", "confidence": 1.0}

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
        requirement_coverage = _requirement_coverage(payload, refs)
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
                "notes": ["mock provider validates schema and requirement coverage only"],
                "retrieved_ref_count": len(payload.retrieved_knowledge_refs),
                "requirement_ref_count": len(payload.requirement_refs),
                "requirement_coverage": requirement_coverage,
            },
            needs_human_input=["企业资质证书编号", "项目经理证书有效期"],
            model_metadata={
                "provider": self.name,
                "model": payload.model_hint or "mock-model",
                "requirement_ref_count": len(payload.requirement_refs),
            },
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
        requirement_coverage = _requirement_coverage(payload, refs)
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
                    "mock rewrite assistant validates schema and requirement coverage only",
                    "事实性企业资质、证书、人员和金额仍需人工核对",
                ],
                "retrieved_ref_count": len(payload.retrieved_knowledge_refs),
                "requirement_ref_count": len(payload.requirement_refs),
                "requirement_coverage": requirement_coverage,
            },
            needs_human_input=["需人工复核事实性资质、人员、证书、金额和日期"],
            model_metadata={
                "provider": self.name,
                "model": payload.model_hint or "mock-rewrite-model",
                "action": payload.action,
                "requirement_ref_count": len(payload.requirement_refs),
            },
            token_usage={
                "input_tokens": self.count_tokens(base_text) + sum(self.count_tokens(ref.content) for ref in payload.retrieved_knowledge_refs[:5]),
                "output_tokens": self.count_tokens(rewritten),
            },
        )

    def cost_advice(self, payload: CostAdviceRequest) -> CostAdviceResponse:
        budget_gap = payload.total_budget - payload.total_actual
        risk_flags: list[str] = []
        focus_items: list[str] = []
        if payload.margin_rate < 20:
            risk_flags.append("利润率低于 20%，需要复核人力、外采和运维成本边界。")
        if payload.total_actual > payload.total_budget:
            risk_flags.append("实际成本已超过预算，需要冻结非必要支出并重新审批。")
        for item in payload.overrun_items[:5]:
            focus_items.append(f"{item.category} / {item.name} 超预算 {item.actual_amount - item.budget_amount:.2f} 元")
        recommendations = list(payload.recommendations)
        if not recommendations:
            recommendations.append("当前成本结构未发现明显异常，建议保持周度实际成本滚动更新。")
        if payload.overrun_items:
            recommendations.append("优先处理超预算成本项，按供应商、合同和交付范围拆分责任。")
        if payload.margin_rate < 25:
            recommendations.append("将低毛利分类拆成可谈判采购项和内部交付效率项，分别制定压降目标。")
        summary = (
            f"{payload.cost_project_name or payload.project_name} 当前预算 {payload.total_budget:.2f}，"
            f"实际 {payload.total_actual:.2f}，预算差额 {budget_gap:.2f}，利润率 {payload.margin_rate:.2f}%。"
        )
        input_text = " ".join(
            [
                payload.project_name,
                payload.cost_project_name,
                *payload.recommendations,
                *(item.name for item in payload.overrun_items),
            ]
        )
        output_text = " ".join([summary, *recommendations, *risk_flags, *focus_items])
        return CostAdviceResponse(
            trace_id=f"trace-mock-cost-advice-{payload.cost_project_id[:8]}",
            summary=summary,
            recommendations=recommendations[:6],
            risk_flags=risk_flags,
            focus_items=focus_items,
            model_metadata={"provider": self.name, "model": payload.model_hint or "mock-cost-advice-model"},
            token_usage={
                "input_tokens": self.count_tokens(input_text),
                "output_tokens": self.count_tokens(output_text),
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


def _requirement_coverage(
    payload: ChapterGenerateRequest | ChapterActionRequest,
    refs: list[SourceRef],
) -> list[dict[str, object]]:
    source_refs = [ref.model_dump() for ref in refs[:3]]
    return [
        {
            "requirement_id": requirement.id,
            "module": requirement.module,
            "requirement": requirement.requirement,
            "satisfied": not requirement.needs_review,
            "evidence": "已纳入章节生成上下文，待人工复核事实性材料。",
            "source_refs": source_refs,
            "needs_review": requirement.needs_review,
        }
        for requirement in payload.requirement_refs[:20]
    ]

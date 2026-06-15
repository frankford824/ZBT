from __future__ import annotations

import json
import os
import re
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Any

from app.schemas.common import SourceRef
from app.schemas.cost import CostAdviceRequest, CostAdviceResponse
from app.schemas.generation import ChapterActionRequest, ChapterGenerateRequest, ChapterGenerateResponse


@dataclass(frozen=True)
class OpenAICompatibleTarget:
    model: str
    dimensions: int | None = None
    temperature: float | None = None
    timeout_s: int | None = None


class OpenAICompatibleProvider:
    def __init__(
        self,
        name: str,
        *,
        base_url_env: str,
        api_key_env: str,
        default_base_url: str = "",
        target: OpenAICompatibleTarget | None = None,
    ) -> None:
        self.name = name
        self.base_url_env = base_url_env
        self.api_key_env = api_key_env
        self.default_base_url = default_base_url
        self.target = target

    def bind(self, target: Any) -> "OpenAICompatibleProvider":
        return OpenAICompatibleProvider(
            self.name,
            base_url_env=self.base_url_env,
            api_key_env=self.api_key_env,
            default_base_url=self.default_base_url,
            target=OpenAICompatibleTarget(
                model=target.model,
                dimensions=target.dimensions,
                temperature=target.temperature,
                timeout_s=target.timeout_s,
            ),
        )

    def health_check(self) -> bool:
        return bool(self._api_key()) and bool(os.getenv(self.base_url_env, self.default_base_url).strip())

    def complete(self, prompt: str) -> str:
        data = self._post_json(
            "/chat/completions",
            {
                "model": self._model(),
                "messages": [{"role": "user", "content": prompt}],
                "temperature": self._temperature(),
            },
        )
        return _choice_text(data)

    def generate_json(self, prompt: str, schema_name: str) -> dict[str, object]:
        data = self._post_json(
            "/chat/completions",
            {
                "model": self._model(),
                "messages": [
                    {
                        "role": "system",
                        "content": f"Return only valid JSON that matches the {schema_name} contract.",
                    },
                    {"role": "user", "content": prompt},
                ],
                "temperature": self._temperature(),
                "response_format": {"type": "json_object"},
            },
        )
        return _json_from_text(_choice_text(data))

    def stream(self, prompt: str) -> list[str]:
        return [self.complete(prompt)]

    def count_tokens(self, text: str) -> int:
        value = text.strip()
        if not value:
            return 0
        return max(1, len(value) // 4)

    def embed_text(self, text: str) -> list[float]:
        return self.embed_batch([text])[0]

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        payload: dict[str, object] = {"model": self._model(), "input": texts}
        if self.target and self.target.dimensions:
            payload["dimensions"] = self.target.dimensions
        data = self._post_json("/embeddings", payload)
        embeddings = [item["embedding"] for item in sorted(data.get("data", []), key=lambda item: item.get("index", 0))]
        if len(embeddings) != len(texts):
            raise RuntimeError(f"{self.name} embedding count mismatch: got {len(embeddings)} want {len(texts)}")
        return embeddings

    def get_dimensions(self) -> int:
        if self.target and self.target.dimensions:
            return self.target.dimensions
        configured = os.getenv(f"{self.name.upper()}_EMBEDDING_DIMENSIONS", "").strip()
        if configured.isdigit():
            return int(configured)
        return 1024

    def rerank(self, query: str, documents: list[str]) -> list[int]:
        prompt = {
            "query": query,
            "documents": [{"index": index, "content": content[:2400]} for index, content in enumerate(documents)],
            "instruction": "Rank the documents by relevance to query. Return JSON: {\"indexes\":[...]} only.",
        }
        result = self.generate_json(json.dumps(prompt, ensure_ascii=False), "KnowledgeRerank")
        indexes = result.get("indexes", [])
        if not isinstance(indexes, list):
            return list(range(len(documents)))
        parsed_indexes = [_parse_rank_index(index, len(documents)) for index in indexes]
        return [index for index in parsed_indexes if index is not None]

    def generate_chapter(self, payload: ChapterGenerateRequest) -> ChapterGenerateResponse:
        prompt = _chapter_prompt(payload)
        result = self.generate_json(prompt, "ChapterGenerateResponse")
        return _chapter_response_from_json(result, payload, self.name, self._model())

    def chapter_action(self, payload: ChapterActionRequest) -> ChapterGenerateResponse:
        prompt = _chapter_action_prompt(payload)
        result = self.generate_json(prompt, "ChapterGenerateResponse")
        return _chapter_response_from_json(result, payload, self.name, self._model())

    def cost_advice(self, payload: CostAdviceRequest) -> CostAdviceResponse:
        prompt = {
            "project_name": payload.project_name,
            "cost_project_name": payload.cost_project_name,
            "total_budget": payload.total_budget,
            "total_actual": payload.total_actual,
            "margin_rate": payload.margin_rate,
            "category_totals": [item.model_dump() for item in payload.category_totals],
            "overrun_items": [item.model_dump() for item in payload.overrun_items[:10]],
            "recommendations": payload.recommendations,
            "instruction": "Return JSON with summary, recommendations, risk_flags, focus_items.",
        }
        result = self.generate_json(json.dumps(prompt, ensure_ascii=False), "CostAdviceResponse")
        output_text = json.dumps(result, ensure_ascii=False)
        return CostAdviceResponse(
            trace_id=f"trace-{self.name}-cost-{int(time.time() * 1000)}",
            summary=str(result.get("summary") or ""),
            recommendations=_string_list(result.get("recommendations")),
            risk_flags=_string_list(result.get("risk_flags")),
            focus_items=_string_list(result.get("focus_items")),
            model_metadata={"provider": self.name, "model": self._model()},
            token_usage={
                "input_tokens": self.count_tokens(json.dumps(prompt, ensure_ascii=False)),
                "output_tokens": self.count_tokens(output_text),
            },
        )

    def _model(self) -> str:
        if not self.target or not self.target.model:
            raise RuntimeError(f"{self.name} route is missing model")
        return self.target.model

    def _temperature(self) -> float:
        if self.target and self.target.temperature is not None:
            return self.target.temperature
        return 0.2

    def _timeout(self) -> int:
        if self.target and self.target.timeout_s:
            return self.target.timeout_s
        return int(os.getenv("OPENAI_COMPATIBLE_TIMEOUT_S", "120"))

    def _api_key(self) -> str:
        return os.getenv(self.api_key_env, "").strip()

    def _base_url(self) -> str:
        base_url = os.getenv(self.base_url_env, self.default_base_url).strip()
        if not base_url:
            raise RuntimeError(f"{self.name} is not configured: missing {self.base_url_env}")
        return base_url.rstrip("/")

    def _post_json(self, path: str, payload: dict[str, object]) -> dict[str, Any]:
        api_key = self._api_key()
        if not api_key:
            raise RuntimeError(f"{self.name} is not configured: missing {self.api_key_env}")
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            self._base_url() + path,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {api_key}",
                "Content-Type": "application/json",
            },
        )
        try:
            with urllib.request.urlopen(req, timeout=self._timeout()) as response:
                return json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            error_body = exc.read().decode("utf-8", "replace")
            raise RuntimeError(f"{self.name} {path} returned {exc.code}: {error_body}") from exc


def _choice_text(data: dict[str, Any]) -> str:
    choices = data.get("choices", [])
    if not choices:
        raise RuntimeError("chat completion returned no choices")
    message = choices[0].get("message", {})
    content = message.get("content", "")
    if not isinstance(content, str) or not content.strip():
        raise RuntimeError("chat completion returned empty content")
    return content


def _json_from_text(text: str) -> dict[str, object]:
    cleaned = text.strip()
    if cleaned.startswith("```"):
        cleaned = re.sub(r"^```(?:json)?", "", cleaned).strip()
        cleaned = re.sub(r"```$", "", cleaned).strip()
    parsed = json.loads(cleaned)
    if not isinstance(parsed, dict):
        raise RuntimeError("model returned non-object JSON")
    return parsed


def _parse_rank_index(value: object, document_count: int) -> int | None:
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        index = value
    elif isinstance(value, float) and value.is_integer():
        index = int(value)
    elif isinstance(value, str) and value.strip().isdigit():
        index = int(value.strip())
    else:
        return None
    if 0 <= index < document_count:
        return index
    return None


def _chapter_prompt(payload: ChapterGenerateRequest) -> str:
    refs = [ref.model_dump() for ref in payload.retrieved_knowledge_refs[:8]]
    return json.dumps(
        {
            "chapter_title": payload.chapter_title,
            "tender_requirements": payload.tender_requirements,
            "selected_knowledge_refs": payload.selected_knowledge_refs,
            "retrieved_knowledge_refs": refs,
            "instruction": (
                "Generate bid chapter JSON with fields: tiptap_json, source_refs, "
                "self_check, needs_human_input. Keep unsupported facts in needs_human_input."
            ),
        },
        ensure_ascii=False,
    )


def _chapter_action_prompt(payload: ChapterActionRequest) -> str:
    return json.dumps(
        {
            "action": payload.action,
            "instruction": payload.instruction,
            "chapter_title": payload.chapter_title,
            "current_plain_text": payload.current_plain_text,
            "current_tiptap_json": payload.current_tiptap_json,
            "retrieved_knowledge_refs": [ref.model_dump() for ref in payload.retrieved_knowledge_refs[:8]],
            "output_contract": "Return JSON with tiptap_json, source_refs, self_check, needs_human_input.",
        },
        ensure_ascii=False,
    )


def _chapter_response_from_json(
    result: dict[str, object],
    payload: ChapterGenerateRequest,
    provider: str,
    model: str,
) -> ChapterGenerateResponse:
    tiptap_json = result.get("tiptap_json")
    if not isinstance(tiptap_json, dict):
        text = str(result.get("plain_text") or result.get("content") or "")
        tiptap_json = {
            "type": "doc",
            "content": [{"type": "paragraph", "content": [{"type": "text", "text": text}]}],
        }
    source_refs = []
    for ref in result.get("source_refs", []):
        if isinstance(ref, dict):
            source_refs.append(SourceRef(**ref))
    if not source_refs:
        source_refs = [
            SourceRef(
                chunk_id=ref.chunk_id,
                document_id=ref.document_id,
                title=ref.title,
                page_start=ref.page_start,
                page_end=ref.page_end,
            )
            for ref in payload.retrieved_knowledge_refs[:5]
        ]
    output_text = json.dumps(result, ensure_ascii=False)
    input_text = _chapter_prompt(payload)
    return ChapterGenerateResponse(
        trace_id=f"trace-{provider}-{int(time.time() * 1000)}",
        tiptap_json=tiptap_json,
        source_refs=source_refs,
        self_check=result.get("self_check") if isinstance(result.get("self_check"), dict) else {"status": "needs_review"},
        needs_human_input=_string_list(result.get("needs_human_input")),
        model_metadata={"provider": provider, "model": model},
        token_usage={
            "input_tokens": max(1, len(input_text) // 4),
            "output_tokens": max(1, len(output_text) // 4),
        },
    )


def _string_list(value: object) -> list[str]:
    if isinstance(value, list):
        return [str(item) for item in value if str(item).strip()]
    if isinstance(value, str) and value.strip():
        return [value]
    return []

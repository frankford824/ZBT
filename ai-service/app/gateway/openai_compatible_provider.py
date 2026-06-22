from __future__ import annotations

import json
import math
import os
import re
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any

from app.schemas.common import SourceRef
from app.schemas.cost import CostAdviceRequest, CostAdviceResponse
from app.schemas.generation import ChapterActionRequest, ChapterGenerateRequest, ChapterGenerateResponse


_HEADER_NAME_RE = re.compile(r"^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$")
_CLOUDFLARE_ACCOUNT_ID_RE = re.compile(r"^[0-9a-fA-F]{32}$")
DEFAULT_OPENAI_COMPATIBLE_RESPONSE_MAX_BYTES = 8 * 1024 * 1024


class OpenAICompatibleResponseTooLargeError(RuntimeError):
    pass


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
        api_key_required: bool = True,
        auth_header_name: str = "",
        auth_header_env: str = "",
        extra_headers_env: str = "",
        target: OpenAICompatibleTarget | None = None,
    ) -> None:
        self.name = name
        self.base_url_env = base_url_env
        self.api_key_env = api_key_env
        self.default_base_url = default_base_url
        self.api_key_required = api_key_required
        self.auth_header_name = auth_header_name
        self.auth_header_env = auth_header_env
        self.extra_headers_env = extra_headers_env
        self.target = target

    def bind(self, target: Any) -> "OpenAICompatibleProvider":
        return OpenAICompatibleProvider(
            self.name,
            base_url_env=self.base_url_env,
            api_key_env=self.api_key_env,
            default_base_url=self.default_base_url,
            api_key_required=self.api_key_required,
            auth_header_name=self.auth_header_name,
            auth_header_env=self.auth_header_env,
            extra_headers_env=self.extra_headers_env,
            target=OpenAICompatibleTarget(
                model=target.model,
                dimensions=target.dimensions,
                temperature=target.temperature,
                timeout_s=target.timeout_s,
            ),
        )

    def health_check(self) -> bool:
        try:
            self._base_url()
            self._headers()
        except RuntimeError:
            return False
        return True

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
        dimensions = self._target_dimensions()
        if dimensions:
            payload["dimensions"] = dimensions
        data = self._post_json("/embeddings", payload)
        embeddings = _embedding_vectors_from_response(data, len(texts), self.name)
        if len(embeddings) != len(texts):
            raise RuntimeError(f"{self.name} embedding count mismatch: got {len(embeddings)} want {len(texts)}")
        return embeddings

    def get_dimensions(self) -> int:
        dimensions = self._target_dimensions()
        if dimensions:
            return dimensions
        return _env_positive_int(f"{self.name.upper()}_EMBEDDING_DIMENSIONS", 1024)

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
        if self.target and self.target.timeout_s and self.target.timeout_s > 0:
            return self.target.timeout_s
        return _env_positive_int("OPENAI_COMPATIBLE_TIMEOUT_S", 120)

    def _target_dimensions(self) -> int | None:
        if self.target and self.target.dimensions and self.target.dimensions > 0:
            return self.target.dimensions
        return None

    def _api_key(self) -> str:
        return os.getenv(self.api_key_env, "").strip()

    def _base_url(self) -> str:
        base_url = os.getenv(self.base_url_env, self.default_base_url).strip()
        if not base_url:
            raise RuntimeError(f"{self.name} is not configured: missing {self.base_url_env}")
        return _safe_base_url(base_url, self.name, self.base_url_env)

    def _headers(self) -> dict[str, str]:
        headers = {"Content-Type": "application/json"}
        api_key = self._api_key()
        if api_key:
            headers["Authorization"] = _bearer_header(api_key)
        elif self.api_key_required:
            raise RuntimeError(f"{self.name} is not configured: missing {self.api_key_env}")

        if self.auth_header_name:
            auth_header_name = _safe_header_name(self.auth_header_name, self.name, "auth header")
            reserved_names = {key.lower(): key for key in headers}
            if auth_header_name.lower() in reserved_names:
                reserved_name = reserved_names[auth_header_name.lower()]
                raise RuntimeError(f"{self.name} auth header must not override {reserved_name}")
            if not self.auth_header_env:
                raise RuntimeError(f"{self.name} auth header is missing an environment variable")
            token = os.getenv(self.auth_header_env, "").strip()
            if not token:
                raise RuntimeError(f"{self.name} is not configured: missing {self.auth_header_env}")
            headers[auth_header_name] = _bearer_header(token)

        extra_headers = _extra_headers_from_env(self.extra_headers_env, self.name)
        reserved_names = {key.lower() for key in headers}
        for key in extra_headers:
            if key.lower() in reserved_names:
                raise RuntimeError(f"{self.name} extra headers must not override {key}")
        headers.update(extra_headers)
        return headers

    def _post_json(self, path: str, payload: dict[str, object]) -> dict[str, Any]:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            self._base_url() + path,
            data=body,
            method="POST",
            headers=self._headers(),
        )
        try:
            with urllib.request.urlopen(req, timeout=self._timeout()) as response:
                parsed = json.loads(_read_limited_response(response, self._max_response_bytes()).decode("utf-8"))
                if not isinstance(parsed, dict):
                    raise RuntimeError(f"{self.name} {path} returned non-object JSON")
                return parsed
        except OpenAICompatibleResponseTooLargeError as exc:
            raise RuntimeError(f"{self.name} {path} response is too large") from exc
        except urllib.error.HTTPError as exc:
            raise RuntimeError(f"{self.name} {path} returned HTTP {exc.code}") from exc

    def _max_response_bytes(self) -> int:
        return _env_positive_int(
            "OPENAI_COMPATIBLE_MAX_RESPONSE_BYTES",
            DEFAULT_OPENAI_COMPATIBLE_RESPONSE_MAX_BYTES,
        )


class CloudflareAIGatewayProvider(OpenAICompatibleProvider):
    def __init__(self, name: str = "cloudflare_ai_gateway", target: OpenAICompatibleTarget | None = None) -> None:
        super().__init__(
            name,
            base_url_env="CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL",
            api_key_env="CLOUDFLARE_API_TOKEN",
            api_key_required=True,
            extra_headers_env="CLOUDFLARE_AI_GATEWAY_HEADERS",
            target=target,
        )

    def bind(self, target: Any) -> "CloudflareAIGatewayProvider":
        return CloudflareAIGatewayProvider(
            self.name,
            target=OpenAICompatibleTarget(
                model=target.model,
                dimensions=target.dimensions,
                temperature=target.temperature,
                timeout_s=target.timeout_s,
            ),
        )

    def _api_key(self) -> str:
        return (
            os.getenv("CLOUDFLARE_API_TOKEN", "").strip()
            or os.getenv("CLOUDFLARE_AI_GATEWAY_TOKEN", "").strip()
        )

    def _base_url(self) -> str:
        configured_base_url = os.getenv(self.base_url_env, "").strip()
        if configured_base_url:
            return _safe_base_url(configured_base_url, self.name, self.base_url_env)
        account_id = os.getenv("CLOUDFLARE_ACCOUNT_ID", "").strip()
        if not _CLOUDFLARE_ACCOUNT_ID_RE.fullmatch(account_id):
            raise RuntimeError(
                f"{self.name} requires CLOUDFLARE_ACCOUNT_ID or {self.base_url_env}"
            )
        return f"https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/v1"

    def _headers(self) -> dict[str, str]:
        headers = super()._headers()
        gateway_id = os.getenv("CLOUDFLARE_AI_GATEWAY_ID", "").strip()
        if gateway_id:
            normalized_names = {key.lower() for key in headers}
            if "cf-aig-gateway-id" in normalized_names:
                raise RuntimeError(f"{self.name} gateway id header is configured more than once")
            headers["cf-aig-gateway-id"] = _safe_header_value(gateway_id)
        return headers

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        model = self._model()
        if not _is_cloudflare_workers_ai_model(model):
            return super().embed_batch(texts)
        data = self._post_ai_run(model, {"text": texts})
        return _cloudflare_embedding_vectors(data, len(texts), self.name)

    def rerank(self, query: str, documents: list[str]) -> list[int]:
        model = self._model()
        if not _is_cloudflare_workers_ai_model(model):
            return super().rerank(query, documents)
        data = self._post_ai_run(
            model,
            {
                "query": query,
                "contexts": [{"text": document[:2400]} for document in documents],
                "top_k": len(documents),
            },
        )
        return _cloudflare_rerank_indexes(data, len(documents), self.name)

    def _post_ai_run(self, model: str, payload: dict[str, object]) -> dict[str, Any]:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            self._ai_run_url(model),
            data=body,
            method="POST",
            headers=self._headers(),
        )
        try:
            with urllib.request.urlopen(req, timeout=self._timeout()) as response:
                parsed = json.loads(_read_limited_response(response, self._max_response_bytes()).decode("utf-8"))
                if not isinstance(parsed, dict):
                    raise RuntimeError(f"{self.name} /ai/run returned non-object JSON")
                return _cloudflare_result(parsed, self.name)
        except OpenAICompatibleResponseTooLargeError as exc:
            raise RuntimeError(f"{self.name} /ai/run response is too large") from exc
        except urllib.error.HTTPError as exc:
            raise RuntimeError(f"{self.name} /ai/run returned HTTP {exc.code}") from exc

    def _ai_run_url(self, model: str) -> str:
        safe_model = _safe_cloudflare_model(model, self.name)
        account_id = os.getenv("CLOUDFLARE_ACCOUNT_ID", "").strip()
        if not _CLOUDFLARE_ACCOUNT_ID_RE.fullmatch(account_id):
            raise RuntimeError(f"{self.name} /ai/run requires CLOUDFLARE_ACCOUNT_ID")
        return f"https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{safe_model}"


def _read_limited_response(response: object, max_bytes: int) -> bytes:
    content = response.read(max_bytes + 1)
    if len(content) > max_bytes:
        raise OpenAICompatibleResponseTooLargeError
    return content


def _is_cloudflare_workers_ai_model(model: str) -> bool:
    return model.strip().startswith("@cf/")


def _safe_cloudflare_model(model: str, provider_name: str) -> str:
    value = model.strip()
    if not _is_cloudflare_workers_ai_model(value):
        raise RuntimeError(f"{provider_name} /ai/run model must start with @cf/")
    if _contains_url_unsafe_character(value) or any(char in value for char in "?#"):
        raise RuntimeError(f"{provider_name} /ai/run model is invalid")
    return value


def _cloudflare_result(data: dict[str, Any], provider_name: str) -> dict[str, Any]:
    if data.get("success") is False:
        raise RuntimeError(f"{provider_name} /ai/run returned success=false")
    result = data.get("result")
    if isinstance(result, dict):
        return result
    return data


def _cloudflare_embedding_vectors(data: dict[str, Any], expected_count: int, provider_name: str) -> list[list[float]]:
    items = data.get("data")
    if isinstance(items, list) and len(items) == expected_count and all(isinstance(item, dict) for item in items):
        return _embedding_vectors_from_response(data, expected_count, provider_name)
    if isinstance(items, list) and expected_count == 1 and items and all(isinstance(item, (int, float)) for item in items):
        return [_embedding_vector(items, provider_name)]
    if not isinstance(items, list) or len(items) != expected_count:
        raise RuntimeError(f"{provider_name} Workers AI embedding response data count mismatch")
    return [_embedding_vector(item, provider_name) for item in items]


def _cloudflare_rerank_indexes(data: dict[str, Any], document_count: int, provider_name: str) -> list[int]:
    items = data.get("response")
    if not isinstance(items, list):
        items = data.get("data")
    if not isinstance(items, list):
        raise RuntimeError(f"{provider_name} Workers AI rerank response must include response or data")
    if all(isinstance(item, (int, float)) and not isinstance(item, bool) for item in items):
        scored = [(index, float(score), index) for index, score in enumerate(items[:document_count])]
        return [index for index, _score, _order in sorted(scored, key=lambda item: (-item[1], item[2]))]

    ranked: list[tuple[int, float, int]] = []
    for order, item in enumerate(items):
        if not isinstance(item, dict):
            continue
        index = _parse_rank_index(
            item.get("index", item.get("id", item.get("document_index"))),
            document_count,
        )
        if index is None:
            index = order if order < document_count else None
        if index is None:
            continue
        score = _finite_score(
            item.get("score", item.get("relevance_score", item.get("similarity"))),
            default=float(document_count - order),
        )
        ranked.append((index, score, order))
    seen: set[int] = set()
    ordered: list[int] = []
    for index, _score, _order in sorted(ranked, key=lambda item: (-item[1], item[2])):
        if index in seen:
            continue
        seen.add(index)
        ordered.append(index)
    ordered.extend(index for index in range(document_count) if index not in seen)
    return ordered


def _finite_score(value: object, *, default: float) -> float:
    if isinstance(value, bool):
        return default
    try:
        score = float(value)
    except (TypeError, ValueError):
        return default
    return score if math.isfinite(score) else default


def _safe_base_url(value: str, provider_name: str, env_name: str) -> str:
    if _contains_url_unsafe_character(value):
        raise RuntimeError(f"{provider_name} base URL env {env_name} is invalid")
    parsed = urllib.parse.urlparse(value)
    if (
        parsed.scheme not in {"http", "https"}
        or not parsed.netloc
        or not parsed.hostname
        or parsed.username is not None
        or parsed.password is not None
        or parsed.params
        or parsed.query
        or parsed.fragment
    ):
        raise RuntimeError(f"{provider_name} base URL env {env_name} must be an absolute HTTP(S) URL")
    if _contains_url_unsafe_character(parsed.netloc) or _contains_url_unsafe_character(parsed.path):
        raise RuntimeError(f"{provider_name} base URL env {env_name} is invalid")
    return urllib.parse.urlunparse((parsed.scheme, parsed.netloc, parsed.path.rstrip("/"), "", "", ""))


def _contains_url_unsafe_character(value: str) -> bool:
    return "\\" in value or any(ord(char) <= 0x20 or ord(char) == 0x7F for char in value)


def _choice_text(data: dict[str, Any]) -> str:
    choices = data.get("choices", [])
    if not choices:
        raise RuntimeError("chat completion returned no choices")
    message = choices[0].get("message", {})
    content = message.get("content", "")
    if not isinstance(content, str) or not content.strip():
        raise RuntimeError("chat completion returned empty content")
    return content


def _embedding_vectors_from_response(
    data: dict[str, Any],
    expected_count: int,
    provider_name: str,
) -> list[list[float]]:
    items = data.get("data")
    if not isinstance(items, list):
        raise RuntimeError(f"{provider_name} embedding response data must be a list")
    if len(items) != expected_count:
        raise RuntimeError(f"{provider_name} embedding count mismatch: got {len(items)} want {expected_count}")
    has_indexes = any(isinstance(item, dict) and "index" in item for item in items)
    embeddings: list[list[float] | None] = [None] * expected_count
    seen_indexes: set[int] = set()
    for response_index, item in enumerate(items):
        if not isinstance(item, dict):
            raise RuntimeError(f"{provider_name} embedding response item must be an object")
        embedding = _embedding_vector(item.get("embedding"), provider_name)
        if not has_indexes:
            embeddings[response_index] = embedding
            continue
        raw_index = item.get("index")
        if isinstance(raw_index, bool) or not isinstance(raw_index, int):
            raise RuntimeError(f"{provider_name} embedding response index is invalid")
        if raw_index < 0 or raw_index >= expected_count:
            raise RuntimeError(f"{provider_name} embedding response index is out of range")
        if raw_index in seen_indexes:
            raise RuntimeError(f"{provider_name} embedding response index is duplicated")
        seen_indexes.add(raw_index)
        embeddings[raw_index] = embedding
    if any(embedding is None for embedding in embeddings):
        raise RuntimeError(f"{provider_name} embedding response index is incomplete")
    return [embedding for embedding in embeddings if embedding is not None]


def _embedding_vector(value: object, provider_name: str) -> list[float]:
    if not isinstance(value, list) or not value:
        raise RuntimeError(f"{provider_name} embedding vector must be a non-empty list")
    vector: list[float] = []
    for coordinate in value:
        if isinstance(coordinate, bool) or not isinstance(coordinate, (int, float)):
            raise RuntimeError(f"{provider_name} embedding vector contains a non-numeric value")
        number = float(coordinate)
        if not math.isfinite(number):
            raise RuntimeError(f"{provider_name} embedding vector contains a non-finite value")
        vector.append(number)
    return vector


def _env_positive_int(name: str, default: int) -> int:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        value = int(raw)
    except ValueError:
        return default
    return value if value > 0 else default


def _bearer_header(token: str) -> str:
    value = _safe_header_value(token)
    if value.lower().startswith("bearer "):
        return value
    return f"Bearer {value}"


def _extra_headers_from_env(env_name: str, provider_name: str) -> dict[str, str]:
    if not env_name:
        return {}
    raw = os.getenv(env_name, "").strip()
    if not raw:
        return {}
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{provider_name} extra headers env {env_name} must be a JSON object") from exc
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{provider_name} extra headers env {env_name} must be a JSON object")

    headers: dict[str, str] = {}
    seen_names: set[str] = set()
    for key, value in parsed.items():
        header_name = _safe_header_name(
            str(key),
            provider_name,
            f"extra headers env {env_name}",
        )
        normalized_name = header_name.lower()
        if normalized_name in seen_names:
            raise RuntimeError(f"{provider_name} extra headers env {env_name} contains duplicate header {header_name}")
        seen_names.add(normalized_name)
        if not isinstance(value, str):
            raise RuntimeError(f"{provider_name} extra headers env {env_name} values must be strings")
        headers[header_name] = _safe_header_value(value)
    return headers


def _safe_header_name(value: str, provider_name: str, label: str) -> str:
    name = value.strip()
    if not _HEADER_NAME_RE.fullmatch(name):
        raise RuntimeError(f"{provider_name} {label} contains an invalid header name")
    return name


def _safe_header_value(value: str) -> str:
    cleaned = value.strip()
    if not cleaned or any(char in cleaned for char in "\r\n"):
        raise RuntimeError("configured HTTP header value is invalid")
    return cleaned


def _json_from_text(text: str) -> dict[str, object]:
    cleaned = text.strip()
    decoder = json.JSONDecoder()
    candidates = [cleaned]
    fenced = re.fullmatch(r"```(?:json)?\s*(.*?)\s*```", cleaned, flags=re.DOTALL | re.IGNORECASE)
    if fenced:
        candidates.append(fenced.group(1).strip())
    candidates.extend(cleaned[match.start() :] for match in re.finditer(r"\{", cleaned))

    first_error: Exception | None = None
    for candidate in candidates:
        if not candidate:
            continue
        try:
            parsed, _ = decoder.raw_decode(candidate)
        except json.JSONDecodeError as exc:
            if first_error is None:
                first_error = exc
            continue
        if not isinstance(parsed, dict):
            raise RuntimeError("model returned non-object JSON")
        return parsed
    raise RuntimeError("model returned invalid JSON") from first_error


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
    requirement_refs = [ref.model_dump() for ref in payload.requirement_refs[:20]]
    return json.dumps(
        {
            "chapter_title": payload.chapter_title,
            "tender_requirements": payload.tender_requirements,
            "requirement_refs": requirement_refs,
            "selected_knowledge_refs": payload.selected_knowledge_refs,
            "retrieved_knowledge_refs": refs,
            "instruction": (
                "Generate bid chapter JSON with fields: tiptap_json, source_refs, "
                "self_check, needs_human_input. self_check must include requirement_coverage "
                "items with requirement_id, satisfied, evidence, source_refs. Keep unsupported "
                "facts in needs_human_input."
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
            "tender_requirements": payload.tender_requirements,
            "requirement_refs": [ref.model_dump() for ref in payload.requirement_refs[:20]],
            "retrieved_knowledge_refs": [ref.model_dump() for ref in payload.retrieved_knowledge_refs[:8]],
            "output_contract": (
                "Return JSON with tiptap_json, source_refs, self_check, needs_human_input. "
                "self_check.requirement_coverage must review every requirement_ref."
            ),
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
    self_check = result.get("self_check") if isinstance(result.get("self_check"), dict) else {"status": "needs_review"}
    if payload.requirement_refs and not isinstance(self_check.get("requirement_coverage"), list):
        self_check = dict(self_check)
        self_check["requirement_coverage"] = _fallback_requirement_coverage(payload, source_refs)
        self_check.setdefault("status", "needs_review")
    return ChapterGenerateResponse(
        trace_id=f"trace-{provider}-{int(time.time() * 1000)}",
        tiptap_json=tiptap_json,
        source_refs=source_refs,
        self_check=self_check,
        needs_human_input=_string_list(result.get("needs_human_input")),
        model_metadata={
            "provider": provider,
            "model": model,
            "requirement_ref_count": len(payload.requirement_refs),
        },
        token_usage={
            "input_tokens": max(1, len(input_text) // 4),
            "output_tokens": max(1, len(output_text) // 4),
        },
    )


def _fallback_requirement_coverage(
    payload: ChapterGenerateRequest,
    refs: list[SourceRef],
) -> list[dict[str, object]]:
    source_refs = [ref.model_dump() for ref in refs[:3]]
    return [
        {
            "requirement_id": requirement.id,
            "module": requirement.module,
            "requirement": requirement.requirement,
            "satisfied": False,
            "evidence": "",
            "source_refs": source_refs,
            "needs_review": True,
        }
        for requirement in payload.requirement_refs[:20]
    ]


def _string_list(value: object) -> list[str]:
    if isinstance(value, list):
        return [str(item) for item in value if str(item).strip()]
    if isinstance(value, str) and value.strip():
        return [value]
    return []

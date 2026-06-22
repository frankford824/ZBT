from __future__ import annotations

import json
from io import BytesIO
from pathlib import Path

import pytest

from app.evaluation.provider_canary_eval import evaluate_provider_canary


class _FakeHTTPResponse:
    def __init__(self, body: bytes, status: int = 200) -> None:
        self._body = BytesIO(body)
        self.status = status
        self.code = status

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return None

    def read(self, size: int = -1) -> bytes:
        return self._body.read(size)

    def getcode(self) -> int:
        return self.status


@pytest.fixture(autouse=True)
def _provider_canary_env(monkeypatch: pytest.MonkeyPatch) -> None:
    for key in (
        "USE_MOCK_PROVIDERS",
        "ALLOW_MOCK_FALLBACK",
        "OPENAI_API_KEY",
        "OPENAI_BASE_URL",
        "AI_LLM_PROVIDER",
        "AI_LLM_MODEL",
        "AI_EMBEDDING_PROVIDER",
        "AI_EMBEDDING_MODEL",
        "AI_RERANK_PROVIDER",
        "AI_RERANK_MODEL",
        "AI_MODEL_PRICING_JSON",
        "CLOUDFLARE_ACCOUNT_ID",
        "CLOUDFLARE_API_TOKEN",
        "CLOUDFLARE_AI_GATEWAY_ID",
        "CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL",
    ):
        monkeypatch.delenv(key, raising=False)


def test_provider_canary_skips_without_real_provider_config() -> None:
    result = evaluate_provider_canary(Path("app/config/model_routing.yaml"), routes=["chapter_generate"])

    assert result["status"] == "skipped"
    failed = {check["name"] for check in result["checks"] if not check["passed"]}
    assert "route.chapter_generate.non_mock_provider" in failed


def test_provider_canary_passes_production_routes_without_mock_fallback(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")
    monkeypatch.setenv("OPENAI_API_KEY", "test-key")
    monkeypatch.setenv(
        "AI_MODEL_PRICING_JSON",
        '{"openai_compatible_primary/*":{"input_per_1m":2,"output_per_1m":8}}',
    )

    result = evaluate_provider_canary(
        Path("app/config/model_routing.yaml"),
        routes=["chapter_generate", "knowledge_embedding", "knowledge_rerank"],
        require_cost=True,
        strict=True,
    )

    assert result["status"] == "passed"
    assert {route["provider"] for route in result["routes"]} == {"openai_compatible_primary"}
    assert all(route["accounting"]["estimated_cost"] > 0 for route in result["routes"])


def test_provider_canary_can_call_openai_compatible_llm(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured: dict[str, object] = {}

    def fake_urlopen(req, timeout):
        captured["url"] = req.full_url
        captured["body"] = json.loads(req.data.decode("utf-8"))
        captured["authorization"] = req.get_header("Authorization")
        return _FakeHTTPResponse(b'{"choices":[{"message":{"content":"ZBT_OK"}}]}')

    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")
    monkeypatch.setenv("OPENAI_API_KEY", "test-key")
    monkeypatch.setenv("OPENAI_BASE_URL", "https://provider.example.test/v1")
    monkeypatch.setattr("app.gateway.openai_compatible_provider.urllib.request.urlopen", fake_urlopen)

    result = evaluate_provider_canary(
        Path("app/config/model_routing.yaml"),
        routes=["chapter_generate"],
        call_provider=True,
        strict=True,
    )

    assert result["status"] == "passed"
    assert captured["url"] == "https://provider.example.test/v1/chat/completions"
    assert captured["authorization"] == "Bearer test-key"
    assert captured["body"]["model"] == "gpt-4o-mini"
    call_check = next(check for check in result["checks"] if check["name"] == "route.chapter_generate.call_provider")
    assert call_check["passed"] is True


def test_provider_canary_require_cost_fails_without_pricing(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")
    monkeypatch.setenv("OPENAI_API_KEY", "test-key")

    result = evaluate_provider_canary(
        Path("app/config/model_routing.yaml"),
        routes=["chapter_generate"],
        require_cost=True,
        strict=True,
    )

    assert result["status"] == "failed"
    cost_check = next(check for check in result["checks"] if check["name"] == "route.chapter_generate.estimated_cost")
    assert cost_check["passed"] is False
    assert cost_check["actual"] == 0


def test_provider_canary_can_call_cloudflare_workers_ai_embedding_and_rerank(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured_urls: list[str] = []

    def fake_urlopen(req, timeout):
        _ = timeout
        captured_urls.append(req.full_url)
        body = json.loads(req.data.decode("utf-8"))
        if req.full_url.endswith("/chat/completions"):
            assert body["model"] == "openai/gpt-4.1"
            return _FakeHTTPResponse(b'{"choices":[{"message":{"content":"ZBT_OK"}}]}')
        if req.full_url.endswith("/ai/run/@cf/baai/bge-large-en-v1.5"):
            assert body["text"] == ["ZBT provider canary embedding sample"]
            return _FakeHTTPResponse(b'{"success":true,"result":{"data":[[0.1,0.2]],"shape":[1,2]}}')
        if req.full_url.endswith("/ai/run/@cf/baai/bge-reranker-base"):
            assert body["query"] == "bid document requirement"
            return _FakeHTTPResponse(
                b'{"success":true,"result":{"response":[{"id":0,"score":0.25},{"id":1,"score":0.92}]}}',
            )
        raise AssertionError(f"unexpected URL {req.full_url}")

    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")
    monkeypatch.setenv("AI_LLM_PROVIDER", "cloudflare_ai_gateway")
    monkeypatch.setenv("AI_LLM_MODEL", "openai/gpt-4.1")
    monkeypatch.setenv("AI_EMBEDDING_PROVIDER", "cloudflare_ai_gateway")
    monkeypatch.setenv("AI_EMBEDDING_MODEL", "@cf/baai/bge-large-en-v1.5")
    monkeypatch.setenv("AI_RERANK_PROVIDER", "cloudflare_ai_gateway")
    monkeypatch.setenv("AI_RERANK_MODEL", "@cf/baai/bge-reranker-base")
    monkeypatch.setenv("CLOUDFLARE_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
    monkeypatch.setenv("CLOUDFLARE_API_TOKEN", "cf-token")
    monkeypatch.setenv(
        "AI_MODEL_PRICING_JSON",
        '{"cloudflare_ai_gateway/*":{"input_per_1m":2,"output_per_1m":8}}',
    )
    monkeypatch.setattr("app.gateway.openai_compatible_provider.urllib.request.urlopen", fake_urlopen)

    result = evaluate_provider_canary(
        Path("app/config/model_routing.yaml"),
        routes=["chapter_generate", "knowledge_embedding", "knowledge_rerank"],
        call_provider=True,
        require_cost=True,
        strict=True,
    )

    assert result["status"] == "passed"
    assert {route["provider"] for route in result["routes"]} == {"cloudflare_ai_gateway"}
    assert all(route["accounting"]["estimated_cost"] > 0 for route in result["routes"])
    assert any(url.endswith("/ai/run/@cf/baai/bge-large-en-v1.5") for url in captured_urls)
    assert any(url.endswith("/ai/run/@cf/baai/bge-reranker-base") for url in captured_urls)

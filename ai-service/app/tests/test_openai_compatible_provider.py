from __future__ import annotations

import io
import json
import urllib.error

import pytest

from app.gateway.openai_compatible_provider import (
    CloudflareAIGatewayProvider,
    OpenAICompatibleProvider,
    OpenAICompatibleTarget,
    _chapter_prompt,
    _chapter_response_from_json,
    _json_from_text,
)
from app.schemas.generation import ChapterGenerateRequest, TenderRequirementRef


def test_openai_rerank_accepts_numeric_string_indexes(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
    )
    monkeypatch.setattr(
        provider,
        "generate_json",
        lambda _prompt, _schema: {"indexes": ["2", 0, True, "bad", 1.5, 1.0, 99]},
    )

    order = provider.rerank("query", ["a", "b", "c"])

    assert order == [2, 0, 1]


def test_openai_provider_timeout_uses_default_for_invalid_env(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
    )
    monkeypatch.setenv("OPENAI_COMPATIBLE_TIMEOUT_S", "bad")

    assert provider._timeout() == 120

    monkeypatch.setenv("OPENAI_COMPATIBLE_TIMEOUT_S", "0")
    assert provider._timeout() == 120

    monkeypatch.setenv("OPENAI_COMPATIBLE_TIMEOUT_S", "45")
    assert provider._timeout() == 45


def test_openai_provider_normalizes_valid_base_url(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
    )
    monkeypatch.setenv("FAKE_OPENAI_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai/")

    assert provider._base_url() == "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai"


def test_cloudflare_ai_gateway_provider_builds_current_rest_base_url_and_headers(monkeypatch) -> None:
    provider = CloudflareAIGatewayProvider()
    monkeypatch.delenv("CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL", raising=False)
    monkeypatch.setenv("CLOUDFLARE_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
    monkeypatch.setenv("CLOUDFLARE_API_TOKEN", "cf-token")
    monkeypatch.setenv("CLOUDFLARE_AI_GATEWAY_ID", "production-gateway")

    assert (
        provider._base_url()
        == "https://api.cloudflare.com/client/v4/accounts/0123456789abcdef0123456789abcdef/ai/v1"
    )
    headers = provider._headers()

    assert provider.health_check() is True
    assert headers["Authorization"] == "Bearer cf-token"
    assert headers["cf-aig-gateway-id"] == "production-gateway"


def test_cloudflare_ai_gateway_provider_accepts_explicit_base_url_override(monkeypatch) -> None:
    provider = CloudflareAIGatewayProvider()
    monkeypatch.setenv("CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/compat/")
    monkeypatch.delenv("CLOUDFLARE_API_TOKEN", raising=False)
    monkeypatch.setenv("CLOUDFLARE_AI_GATEWAY_TOKEN", "legacy-token")

    assert provider._base_url() == "https://gateway.ai.cloudflare.com/v1/acct/gateway/compat"
    assert provider._headers()["Authorization"] == "Bearer legacy-token"


def test_cloudflare_ai_gateway_provider_rejects_invalid_account_id(monkeypatch) -> None:
    provider = CloudflareAIGatewayProvider()
    monkeypatch.delenv("CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL", raising=False)
    monkeypatch.setenv("CLOUDFLARE_ACCOUNT_ID", "bad/account")
    monkeypatch.setenv("CLOUDFLARE_API_TOKEN", "cf-token")

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="requires CLOUDFLARE_ACCOUNT_ID"):
        provider._base_url()


def test_cloudflare_ai_gateway_provider_rejects_duplicate_gateway_id_header(monkeypatch) -> None:
    provider = CloudflareAIGatewayProvider()
    monkeypatch.setenv("CLOUDFLARE_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
    monkeypatch.setenv("CLOUDFLARE_API_TOKEN", "cf-token")
    monkeypatch.setenv("CLOUDFLARE_AI_GATEWAY_ID", "production-gateway")
    monkeypatch.setenv("CLOUDFLARE_AI_GATEWAY_HEADERS", '{"cf-aig-gateway-id":"other"}')

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="gateway id header is configured more than once"):
        provider._headers()


@pytest.mark.parametrize(
    "base_url",
    [
        "file:///etc/passwd",
        "https://token@example.test/v1",
        "https://example.test/v1?debug=1",
        "https://example.test/v1#fragment",
        "https://example.test/v1\\chat",
        "https://example.test/v1\nX-Injected: yes",
        "example.test/v1",
    ],
)
def test_openai_provider_rejects_invalid_base_url(monkeypatch, base_url) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        api_key_required=False,
    )
    monkeypatch.setenv("FAKE_OPENAI_BASE_URL", base_url)

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="base URL env FAKE_OPENAI_BASE_URL"):
        provider._base_url()


def test_openai_embedding_dimensions_require_positive_int(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
    )
    monkeypatch.setenv("FAKE_EMBEDDING_DIMENSIONS", "0")
    assert provider.get_dimensions() == 1024

    monkeypatch.setenv("FAKE_EMBEDDING_DIMENSIONS", "bad")
    assert provider.get_dimensions() == 1024

    monkeypatch.setenv("FAKE_EMBEDDING_DIMENSIONS", "1536")
    assert provider.get_dimensions() == 1536


def test_openai_provider_ignores_invalid_direct_target_positive_ints(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        target=OpenAICompatibleTarget(model="embedding-model", dimensions=-3, timeout_s=-5),
    )
    captured_payloads = []
    monkeypatch.setenv("FAKE_EMBEDDING_DIMENSIONS", "1536")
    monkeypatch.setenv("OPENAI_COMPATIBLE_TIMEOUT_S", "45")
    monkeypatch.setattr(
        provider,
        "_post_json",
        lambda _path, payload: captured_payloads.append(payload) or {"data": [{"embedding": [1]}]},
    )

    assert provider.get_dimensions() == 1536
    assert provider._timeout() == 45
    assert provider.embed_batch(["text"]) == [[1.0]]
    assert "dimensions" not in captured_payloads[0]


def test_openai_embed_batch_reorders_indexed_embeddings_and_normalizes_numbers(monkeypatch) -> None:
    provider = _embedding_provider()
    monkeypatch.setattr(
        provider,
        "_post_json",
        lambda _path, _payload: {
            "data": [
                {"index": 1, "embedding": [3, 4.5]},
                {"index": 0, "embedding": [1, 2]},
            ]
        },
    )

    assert provider.embed_batch(["first", "second"]) == [[1.0, 2.0], [3.0, 4.5]]


@pytest.mark.parametrize(
    ("response", "message"),
    [
        ({"data": {"index": 0, "embedding": [1]}}, "data must be a list"),
        ({"data": [{"index": 0, "embedding": [1]}, {"index": 0, "embedding": [2]}]}, "duplicated"),
        ({"data": [{"index": 2, "embedding": [1]}]}, "out of range"),
        ({"data": [{"index": 0, "embedding": []}]}, "non-empty list"),
        ({"data": [{"index": 0, "embedding": [True]}]}, "non-numeric"),
        ({"data": [{"index": 0, "embedding": [float("nan")]}]}, "non-finite"),
    ],
)
def test_openai_embed_batch_rejects_invalid_embedding_response(monkeypatch, response, message) -> None:
    provider = _embedding_provider()
    monkeypatch.setattr(provider, "_post_json", lambda _path, _payload: response)
    response_data = response.get("data")
    texts = ["text"] * (len(response_data) if isinstance(response_data, list) else 1)

    with pytest.raises(RuntimeError, match=message):
        provider.embed_batch(texts)


def test_openai_http_error_does_not_expose_response_body(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
    )
    monkeypatch.setenv("FAKE_OPENAI_BASE_URL", "https://example.test/v1")
    monkeypatch.setenv("FAKE_OPENAI_API_KEY", "test-key")

    def raise_http_error(*_args, **_kwargs):
        raise urllib.error.HTTPError(
            "https://example.test/v1/chat/completions",
            429,
            "Too Many Requests",
            {},
            io.BytesIO(b'{"error":"tenant secret prompt fragment"}'),
        )

    monkeypatch.setattr("urllib.request.urlopen", raise_http_error)

    with pytest.raises(RuntimeError) as exc_info:
        provider._post_json("/chat/completions", {"model": "fake", "messages": []})

    message = str(exc_info.value)
    assert message == "fake /chat/completions returned HTTP 429"
    assert "tenant secret" not in message


def test_openai_post_json_rejects_non_object_success_response(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
    )
    monkeypatch.setenv("FAKE_OPENAI_BASE_URL", "https://example.test/v1")
    monkeypatch.setenv("FAKE_OPENAI_API_KEY", "test-key")

    class FakeResponse:
        def __enter__(self) -> "FakeResponse":
            return self

        def __exit__(self, *_args) -> None:
            return None

        def read(self, size: int = -1) -> bytes:
            _ = size
            return b'["tenant secret response fragment"]'

    monkeypatch.setattr("urllib.request.urlopen", lambda *_args, **_kwargs: FakeResponse())

    with pytest.raises(RuntimeError) as exc_info:
        provider._post_json("/chat/completions", {"model": "fake", "messages": []})

    message = str(exc_info.value)
    assert message == "fake /chat/completions returned non-object JSON"
    assert "tenant secret" not in message


def test_openai_post_json_rejects_oversized_success_response(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
    )
    monkeypatch.setenv("FAKE_OPENAI_BASE_URL", "https://example.test/v1")
    monkeypatch.setenv("FAKE_OPENAI_API_KEY", "test-key")
    monkeypatch.setenv("OPENAI_COMPATIBLE_MAX_RESPONSE_BYTES", "4")

    class FakeResponse:
        def __init__(self) -> None:
            self._body = io.BytesIO(b'{"tenant_secret":"response fragment"}')

        def __enter__(self) -> "FakeResponse":
            return self

        def __exit__(self, *_args) -> None:
            return None

        def read(self, size: int = -1) -> bytes:
            return self._body.read(size)

    monkeypatch.setattr("urllib.request.urlopen", lambda *_args, **_kwargs: FakeResponse())

    with pytest.raises(RuntimeError) as exc_info:
        provider._post_json("/chat/completions", {"model": "fake", "messages": []})

    message = str(exc_info.value)
    assert message == "fake /chat/completions response is too large"
    assert "tenant_secret" not in message


def test_openai_provider_supports_authenticated_gateway_without_provider_key(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "cloudflare",
        base_url_env="FAKE_CF_GATEWAY_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        api_key_required=False,
        auth_header_name="cf-aig-authorization",
        auth_header_env="FAKE_CF_GATEWAY_TOKEN",
        extra_headers_env="FAKE_CF_GATEWAY_HEADERS",
    )
    monkeypatch.setenv("FAKE_CF_GATEWAY_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai")
    monkeypatch.delenv("FAKE_OPENAI_API_KEY", raising=False)
    monkeypatch.setenv("FAKE_CF_GATEWAY_TOKEN", "gateway-token")
    monkeypatch.setenv("FAKE_CF_GATEWAY_HEADERS", '{"cf-aig-metadata":"{\\"tenant\\":\\"demo\\"}"}')

    headers = provider._headers()

    assert provider.health_check() is True
    assert "Authorization" not in headers
    assert headers["cf-aig-authorization"] == "Bearer gateway-token"
    assert headers["cf-aig-metadata"] == '{"tenant":"demo"}'


def test_openai_provider_rejects_invalid_extra_gateway_headers(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "cloudflare",
        base_url_env="FAKE_CF_GATEWAY_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        api_key_required=False,
        extra_headers_env="FAKE_CF_GATEWAY_HEADERS",
    )
    monkeypatch.setenv("FAKE_CF_GATEWAY_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai")
    monkeypatch.setenv("FAKE_CF_GATEWAY_HEADERS", '{"Bad\\nHeader":"value"}')

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="invalid header name"):
        provider._headers()


def test_openai_provider_rejects_extra_headers_that_override_auth(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "cloudflare",
        base_url_env="FAKE_CF_GATEWAY_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        auth_header_name="cf-aig-authorization",
        auth_header_env="FAKE_CF_GATEWAY_TOKEN",
        extra_headers_env="FAKE_CF_GATEWAY_HEADERS",
    )
    monkeypatch.setenv("FAKE_CF_GATEWAY_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai")
    monkeypatch.setenv("FAKE_OPENAI_API_KEY", "provider-key")
    monkeypatch.setenv("FAKE_CF_GATEWAY_TOKEN", "gateway-token")
    monkeypatch.setenv("FAKE_CF_GATEWAY_HEADERS", '{"Authorization":"Bearer override"}')

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="must not override Authorization"):
        provider._headers()


def test_openai_provider_rejects_gateway_auth_header_that_overrides_provider_auth(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "cloudflare",
        base_url_env="FAKE_CF_GATEWAY_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        auth_header_name="authorization",
        auth_header_env="FAKE_CF_GATEWAY_TOKEN",
    )
    monkeypatch.setenv("FAKE_CF_GATEWAY_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai")
    monkeypatch.setenv("FAKE_OPENAI_API_KEY", "provider-key")
    monkeypatch.setenv("FAKE_CF_GATEWAY_TOKEN", "gateway-token")

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="auth header must not override Authorization"):
        provider._headers()


def test_openai_provider_rejects_invalid_gateway_auth_header_name(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "cloudflare",
        base_url_env="FAKE_CF_GATEWAY_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        api_key_required=False,
        auth_header_name="bad header",
        auth_header_env="FAKE_CF_GATEWAY_TOKEN",
    )
    monkeypatch.setenv("FAKE_CF_GATEWAY_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai")
    monkeypatch.setenv("FAKE_CF_GATEWAY_TOKEN", "gateway-token")

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="auth header contains an invalid header name"):
        provider._headers()


def test_openai_provider_health_check_rejects_auth_header_without_env(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "cloudflare",
        base_url_env="FAKE_CF_GATEWAY_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        auth_header_name="cf-aig-authorization",
    )
    monkeypatch.setenv("FAKE_CF_GATEWAY_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai")
    monkeypatch.setenv("FAKE_OPENAI_API_KEY", "provider-key")

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="auth header is missing an environment variable"):
        provider._headers()


def test_openai_provider_rejects_duplicate_extra_headers_case_insensitively(monkeypatch) -> None:
    provider = OpenAICompatibleProvider(
        "cloudflare",
        base_url_env="FAKE_CF_GATEWAY_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        api_key_required=False,
        extra_headers_env="FAKE_CF_GATEWAY_HEADERS",
    )
    monkeypatch.setenv("FAKE_CF_GATEWAY_BASE_URL", "https://gateway.ai.cloudflare.com/v1/acct/gateway/openai")
    monkeypatch.setenv("FAKE_CF_GATEWAY_HEADERS", '{"cf-aig-metadata":"a","CF-AIG-METADATA":"b"}')

    assert provider.health_check() is False
    with pytest.raises(RuntimeError, match="duplicate header"):
        provider._headers()


def test_chapter_prompt_includes_requirement_refs() -> None:
    payload = ChapterGenerateRequest(
        tenant_id="tenant-demo",
        bid_document_id="bid-demo",
        bid_part_id="part-tech",
        chapter_id="chapter-demo",
        chapter_title="总体技术方案",
        tender_requirements=["评审要点：技术方案完整性"],
        requirement_refs=[
            TenderRequirementRef(
                id="evaluation-001",
                module="evaluation",
                type="scoring",
                requirement="技术方案完整性评分 20 分",
                priority="high",
                score=20,
            )
        ],
    )

    prompt = json.loads(_chapter_prompt(payload))

    assert prompt["requirement_refs"][0]["id"] == "evaluation-001"
    assert prompt["requirement_refs"][0]["score"] == 20
    assert "requirement_coverage" in prompt["instruction"]


def test_chapter_response_adds_requirement_coverage_when_model_omits_it() -> None:
    payload = ChapterGenerateRequest(
        tenant_id="tenant-demo",
        bid_document_id="bid-demo",
        bid_part_id="part-tech",
        chapter_id="chapter-demo",
        chapter_title="总体技术方案",
        requirement_refs=[
            TenderRequirementRef(
                id="evaluation-001",
                module="evaluation",
                type="scoring",
                requirement="技术方案完整性评分 20 分",
                priority="high",
            )
        ],
    )

    response = _chapter_response_from_json(
        {"plain_text": "章节正文", "self_check": {"status": "pass"}},
        payload,
        "fake",
        "model-demo",
    )

    coverage = response.self_check["requirement_coverage"]
    assert coverage[0]["requirement_id"] == "evaluation-001"
    assert coverage[0]["satisfied"] is False
    assert coverage[0]["needs_review"] is True
    assert response.model_metadata["requirement_ref_count"] == 1


def test_json_from_text_accepts_fenced_or_explained_json() -> None:
    assert _json_from_text('{"indexes":[1,0]}') == {"indexes": [1, 0]}
    assert _json_from_text('```json\n{"indexes":[2,0]}\n```') == {"indexes": [2, 0]}
    assert _json_from_text('结果如下：\n{"summary":"ok","recommendations":["a"]}\n请确认。') == {
        "summary": "ok",
        "recommendations": ["a"],
    }


def test_json_from_text_rejects_non_object_json() -> None:
    with pytest.raises(RuntimeError, match="non-object JSON"):
        _json_from_text('[{"summary":"ok"}]')


def _embedding_provider() -> OpenAICompatibleProvider:
    return OpenAICompatibleProvider(
        "fake",
        base_url_env="FAKE_OPENAI_BASE_URL",
        api_key_env="FAKE_OPENAI_API_KEY",
        target=OpenAICompatibleTarget(model="embedding-model"),
    )

from __future__ import annotations

import io
import urllib.error

import pytest

from app.gateway.openai_compatible_provider import OpenAICompatibleProvider


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

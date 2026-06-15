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

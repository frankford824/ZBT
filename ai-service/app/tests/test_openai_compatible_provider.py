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

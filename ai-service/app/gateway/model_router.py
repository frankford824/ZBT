from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel

from app.gateway.mock_provider import MockProvider
from app.gateway.openai_compatible_provider import OpenAICompatibleProvider


class RouteTarget(BaseModel):
    provider: str
    model: str
    temperature: float | None = None
    output: str | None = None
    schema_name: str | None = None
    stream: bool = False
    require_source_refs: bool = False
    timeout_s: int | None = None
    dimensions: int | None = None
    fallback_from: str | None = None


class ModelRouter:
    def __init__(self, config: dict[str, Any]) -> None:
        self.config = config
        self.providers = self._build_providers(config.get("providers", {}))

    @classmethod
    def from_yaml(cls, path: Path) -> "ModelRouter":
        with path.open("r", encoding="utf-8") as handle:
            return cls(yaml.safe_load(handle))

    def resolve(self, task_type: str, tenant_id: str) -> RouteTarget:
        _ = tenant_id
        candidates = [self.config["routes"][task_type]["primary"], *self.config["routes"][task_type].get("fallback", [])]
        primary_provider = str(candidates[0]["provider"])
        for route in candidates:
            target = self._route_target(route)
            provider = self.providers.get(target.provider)
            if provider is not None and provider.health_check():
                if target.provider != primary_provider:
                    target.fallback_from = primary_provider
                return target
        provider_names = ", ".join(str(route["provider"]) for route in candidates)
        raise RuntimeError(f"no configured provider is available for {task_type}: {provider_names}")

    def fallback(self, task_type: str) -> list[RouteTarget]:
        return [RouteTarget(**item) for item in self.config["routes"][task_type].get("fallback", [])]

    def get_llm(self, task_type: str, tenant_id: str) -> object:
        target = self.resolve(task_type, tenant_id)
        return self._provider_for_target(target)

    def get_embedding(self, task_type: str, tenant_id: str) -> object:
        target = self.resolve(task_type, tenant_id)
        return self._provider_for_target(target)

    def get_rerank(self, task_type: str, tenant_id: str) -> object:
        target = self.resolve(task_type, tenant_id)
        return self._provider_for_target(target)

    def log_call(self, **kwargs: object) -> dict[str, object]:
        return {"logged": True, **kwargs}

    def enforce_quota(self, tenant_id: str) -> bool:
        _ = tenant_id
        return True

    def health_check(self) -> dict[str, bool]:
        return {name: provider.health_check() for name, provider in self.providers.items()}

    def _route_target(self, route: dict[str, Any]) -> RouteTarget:
        return RouteTarget(
            provider=route["provider"],
            model=route["model"],
            temperature=route.get("temperature"),
            output=route.get("output"),
            schema_name=route.get("schema"),
            stream=bool(route.get("stream", False)),
            require_source_refs=bool(route.get("require_source_refs", False)),
            timeout_s=route.get("timeout_s"),
            dimensions=route.get("dimensions"),
        )

    def _provider_for_target(self, target: RouteTarget) -> object:
        provider = self.providers.get(target.provider)
        if provider is None:
            raise RuntimeError(f"provider {target.provider} is not registered")
        if hasattr(provider, "bind"):
            return provider.bind(target)
        return provider

    def _build_providers(self, provider_config: dict[str, Any]) -> dict[str, object]:
        providers: dict[str, object] = {}
        for name, config in provider_config.items():
            provider_type = str(config.get("type", "")).strip()
            if provider_type == "mock":
                providers[name] = MockProvider()
            elif provider_type == "openai_compatible":
                providers[name] = OpenAICompatibleProvider(
                    name,
                    base_url_env=str(config.get("base_url_env") or "OPENAI_BASE_URL"),
                    api_key_env=str(config.get("api_key_env") or "OPENAI_API_KEY"),
                    default_base_url=str(config.get("default_base_url") or ""),
                )
        if "mock" not in providers:
            providers["mock"] = MockProvider()
        return providers

from __future__ import annotations

import os
from copy import deepcopy
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
        self.config = self._apply_provider_mode(config)
        self.providers = self._build_providers(self.config.get("providers", {}))

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

    SUPPORTED_PROVIDER_TYPES = ("mock", "openai_compatible")

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
            else:
                supported = ", ".join(self.SUPPORTED_PROVIDER_TYPES)
                raise ValueError(
                    f"provider '{name}' has unsupported type '{provider_type}'; "
                    f"supported types: {supported}"
                )
        if "mock" not in providers:
            providers["mock"] = MockProvider()
        return providers

    LLM_ROUTES = {
        "tender_parse",
        "outline_generate",
        "chapter_generate",
        "chapter_self_check",
        "compliance_check",
        "rewrite_assistant",
        "cost_advice",
    }
    EMBEDDING_ROUTES = {"knowledge_embedding"}
    RERANK_ROUTES = {"knowledge_rerank"}

    def _apply_provider_mode(self, config: dict[str, Any]) -> dict[str, Any]:
        effective = deepcopy(config)
        if os.getenv("USE_MOCK_PROVIDERS", "true").strip().lower() not in {"0", "false", "no"}:
            return effective

        for task_type, route in effective.get("routes", {}).items():
            primary = route.get("primary", {})
            if primary.get("provider") != "mock":
                continue
            route_kind = self._route_kind(task_type)
            if route_kind == "":
                continue
            provider = self._route_env(task_type, "PROVIDER") or os.getenv(f"AI_{route_kind}_PROVIDER", "")
            model = self._route_env(task_type, "MODEL") or os.getenv(f"AI_{route_kind}_MODEL", "")
            provider = provider.strip()
            model = model.strip()
            if not provider or not model:
                raise ValueError(
                    "USE_MOCK_PROVIDERS=false requires "
                    f"{task_type.upper()}_PROVIDER/{task_type.upper()}_MODEL or "
                    f"AI_{route_kind}_PROVIDER/AI_{route_kind}_MODEL"
                )
            mock_primary = deepcopy(primary)
            primary["provider"] = provider
            primary["model"] = model
            fallback = route.setdefault("fallback", [])
            if not any(item.get("provider") == "mock" for item in fallback):
                fallback.append(mock_primary)
        return effective

    def _route_kind(self, task_type: str) -> str:
        if task_type in self.LLM_ROUTES:
            return "LLM"
        if task_type in self.EMBEDDING_ROUTES:
            return "EMBEDDING"
        if task_type in self.RERANK_ROUTES:
            return "RERANK"
        return ""

    def _route_env(self, task_type: str, suffix: str) -> str:
        key = task_type.upper().replace("-", "_") + "_" + suffix
        return os.getenv(key, "")

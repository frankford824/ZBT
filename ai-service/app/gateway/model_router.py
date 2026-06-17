from __future__ import annotations

import os
from copy import deepcopy
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel

from app.gateway.mock_provider import MockProvider
from app.gateway.openai_compatible_provider import OpenAICompatibleProvider


class LocalPipelineProvider:
    name = "local"

    def health_check(self) -> bool:
        return True

    def bind(self, target: Any) -> "LocalPipelineProvider":
        _ = target
        return self


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
        return self.resolve_candidates(task_type, tenant_id)[0]

    def resolve_candidates(self, task_type: str, tenant_id: str) -> list[RouteTarget]:
        _ = tenant_id
        candidates = [self.config["routes"][task_type]["primary"], *self.config["routes"][task_type].get("fallback", [])]
        candidate_targets = [self._route_target(task_type, route) for route in candidates]
        missing_providers = [target.provider for target in candidate_targets if target.provider not in self.providers]
        if missing_providers:
            missing = ", ".join(dict.fromkeys(missing_providers))
            raise RuntimeError(f"provider is not registered for {task_type}: {missing}")
        primary_provider = candidate_targets[0].provider
        targets: list[RouteTarget] = []
        for target in candidate_targets:
            provider = self.providers[target.provider]
            if provider.health_check():
                if target.provider != primary_provider:
                    target.fallback_from = primary_provider
                targets.append(target)
        if targets:
            return targets
        provider_names = ", ".join(str(route["provider"]) for route in candidates)
        raise RuntimeError(f"no configured provider is available for {task_type}: {provider_names}")

    def fallback(self, task_type: str) -> list[RouteTarget]:
        return [self._route_target(task_type, item) for item in self.config["routes"][task_type].get("fallback", [])]

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

    def provider_backed_mock_routes(self) -> list[str]:
        provider_backed_routes = self.LLM_ROUTES | self.EMBEDDING_ROUTES | self.RERANK_ROUTES | self.OCR_ROUTES
        mock_routes: list[str] = []
        for task_type, route in self.config.get("routes", {}).items():
            if task_type not in provider_backed_routes:
                continue
            primary = route.get("primary", {})
            if self._route_target(task_type, primary).provider == "mock":
                mock_routes.append(f"{task_type}.primary")
            for index, fallback in enumerate(route.get("fallback", []), start=1):
                if self._route_target(task_type, fallback).provider == "mock":
                    mock_routes.append(f"{task_type}.fallback[{index}]")
        return mock_routes

    def _route_target(self, task_type: str, route: dict[str, Any]) -> RouteTarget:
        provider = str(route["provider"]).strip()
        model = str(route.get("model") or "").strip()
        route_kind = self._route_kind(task_type)
        if route_kind and provider not in {"mock", "local"}:
            provider = (
                self._route_env(task_type, "PROVIDER")
                or os.getenv(f"AI_{route_kind}_PROVIDER", "")
                or provider
            ).strip()
            model = (
                self._route_env(task_type, "MODEL")
                or os.getenv(f"AI_{route_kind}_MODEL", "")
                or model
            ).strip()
        return RouteTarget(
            provider=provider,
            model=model,
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

    def provider_for_target(self, target: RouteTarget) -> object:
        return self._provider_for_target(target)

    SUPPORTED_PROVIDER_TYPES = ("mock", "openai_compatible", "local")

    def _build_providers(self, provider_config: dict[str, Any]) -> dict[str, object]:
        providers: dict[str, object] = {}
        for name, config in provider_config.items():
            provider_type = str(config.get("type", "")).strip()
            if provider_type == "mock":
                providers[name] = MockProvider()
            elif provider_type == "local":
                providers[name] = LocalPipelineProvider()
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
    OCR_ROUTES = {"document_ocr"}

    def _apply_provider_mode(self, config: dict[str, Any]) -> dict[str, Any]:
        effective = deepcopy(config)
        allow_mock_fallback = self._allow_mock_fallback()
        if not allow_mock_fallback:
            for route in effective.get("routes", {}).values():
                fallback = route.get("fallback", [])
                if isinstance(fallback, list):
                    route["fallback"] = [item for item in fallback if item.get("provider") != "mock"]

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
            if allow_mock_fallback and not any(item.get("provider") == "mock" for item in fallback):
                fallback.append(mock_primary)
        return effective

    def _route_kind(self, task_type: str) -> str:
        if task_type in self.LLM_ROUTES:
            return "LLM"
        if task_type in self.EMBEDDING_ROUTES:
            return "EMBEDDING"
        if task_type in self.RERANK_ROUTES:
            return "RERANK"
        if task_type in self.OCR_ROUTES:
            return "OCR"
        return ""

    def _route_env(self, task_type: str, suffix: str) -> str:
        key = task_type.upper().replace("-", "_") + "_" + suffix
        return os.getenv(key, "")

    def _allow_mock_fallback(self) -> bool:
        return os.getenv("ALLOW_MOCK_FALLBACK", "true").strip().lower() not in {
            "0",
            "false",
            "no",
        }

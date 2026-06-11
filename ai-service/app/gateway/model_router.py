from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel

from app.gateway.mock_provider import MockProvider


class RouteTarget(BaseModel):
    provider: str
    model: str
    temperature: float | None = None
    output: str | None = None
    schema_name: str | None = None
    stream: bool = False
    require_source_refs: bool = False
    timeout_s: int | None = None


class ModelRouter:
    def __init__(self, config: dict[str, Any]) -> None:
        self.config = config
        self.providers = {"mock": MockProvider()}

    @classmethod
    def from_yaml(cls, path: Path) -> "ModelRouter":
        with path.open("r", encoding="utf-8") as handle:
            return cls(yaml.safe_load(handle))

    def resolve(self, task_type: str, tenant_id: str) -> RouteTarget:
        _ = tenant_id
        route = self.config["routes"][task_type]["primary"]
        return RouteTarget(
            provider=route["provider"],
            model=route["model"],
            temperature=route.get("temperature"),
            output=route.get("output"),
            schema_name=route.get("schema"),
            stream=bool(route.get("stream", False)),
            require_source_refs=bool(route.get("require_source_refs", False)),
            timeout_s=route.get("timeout_s"),
        )

    def fallback(self, task_type: str) -> list[RouteTarget]:
        return [RouteTarget(**item) for item in self.config["routes"][task_type].get("fallback", [])]

    def get_llm(self, task_type: str, tenant_id: str) -> MockProvider:
        target = self.resolve(task_type, tenant_id)
        provider = self.providers.get(target.provider)
        if provider is None:
            provider = self.providers["mock"]
        return provider

    def get_embedding(self, task_type: str, tenant_id: str) -> MockProvider:
        target = self.resolve(task_type, tenant_id)
        provider = self.providers.get(target.provider)
        if provider is None:
            provider = self.providers["mock"]
        return provider

    def log_call(self, **kwargs: object) -> dict[str, object]:
        return {"logged": True, **kwargs}

    def enforce_quota(self, tenant_id: str) -> bool:
        _ = tenant_id
        return True

    def health_check(self) -> dict[str, bool]:
        return {name: provider.health_check() for name, provider in self.providers.items()}

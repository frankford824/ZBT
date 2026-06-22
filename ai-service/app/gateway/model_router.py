from __future__ import annotations

import json
import math
import os
from copy import deepcopy
from pathlib import Path
from threading import RLock
from typing import Any

import yaml
from pydantic import BaseModel, field_validator

from app.gateway.mock_provider import MockProvider
from app.gateway.openai_compatible_provider import CloudflareAIGatewayProvider, OpenAICompatibleProvider

MAX_AI_ESTIMATED_COST = 100000.0


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

    @field_validator("provider", "model", mode="before")
    @classmethod
    def _non_empty_route_string(cls, value: object, info: Any) -> str:
        text = "" if value is None else str(value).strip()
        if not text:
            raise ValueError(f"route {info.field_name} must be non-empty")
        return text

    @field_validator("temperature", mode="before")
    @classmethod
    def _valid_temperature(cls, value: object) -> float | None:
        if value is None or value == "":
            return None
        if isinstance(value, bool):
            raise ValueError("route temperature must be a number between 0 and 2")
        try:
            number = float(value)
        except (TypeError, ValueError) as exc:
            raise ValueError("route temperature must be a number between 0 and 2") from exc
        if not math.isfinite(number) or number < 0 or number > 2:
            raise ValueError("route temperature must be a number between 0 and 2")
        return number

    @field_validator("timeout_s", "dimensions", mode="before")
    @classmethod
    def _positive_int(cls, value: object, info: Any) -> int | None:
        if value is None or value == "":
            return None
        if isinstance(value, bool):
            raise ValueError(f"route {info.field_name} must be a positive integer")
        if isinstance(value, float) and not value.is_integer():
            raise ValueError(f"route {info.field_name} must be a positive integer")
        try:
            number = int(value)
        except (TypeError, ValueError) as exc:
            raise ValueError(f"route {info.field_name} must be a positive integer") from exc
        if number <= 0:
            raise ValueError(f"route {info.field_name} must be a positive integer")
        return number


class ModelRouter:
    def __init__(self, config: dict[str, Any]) -> None:
        self.config = self._apply_provider_mode(config)
        self.providers = self._build_providers(self.config.get("providers", {}))
        self._tenant_usage: dict[str, float] = {}
        self._call_log: list[dict[str, object]] = []
        self._lock = RLock()

    @classmethod
    def from_yaml(cls, path: Path) -> "ModelRouter":
        with path.open("r", encoding="utf-8") as handle:
            return cls(yaml.safe_load(handle))

    def resolve(self, task_type: str, tenant_id: str) -> RouteTarget:
        return self.resolve_candidates(task_type, tenant_id)[0]

    def resolve_candidates(self, task_type: str, tenant_id: str) -> list[RouteTarget]:
        route_kind = self._route_kind(task_type)
        tenant_over_budget = bool(route_kind and not self.enforce_quota(tenant_id))
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
        if tenant_over_budget:
            downgraded = []
            if self._quota_exceed_policy() == "downgrade_then_block":
                downgraded = [target for target in targets if self._is_zero_cost_provider(target.provider)]
            if downgraded:
                return downgraded
            raise RuntimeError(f"tenant AI quota exceeded for {self._tenant_key(tenant_id)}")
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
        tenant_id = self._tenant_key(kwargs.get("tenant_id", ""))
        estimated_cost = self._cost_from_call(kwargs)
        with self._lock:
            self._tenant_usage[tenant_id] = self._round_money(
                self._tenant_usage.get(tenant_id, 0.0) + estimated_cost
            )
            usage = self.quota_status(tenant_id)
            event = self._call_event(tenant_id, estimated_cost, usage, kwargs)
            self._call_log.append(event)
        return {"logged": True, **event, "usage": usage}

    def enforce_quota(self, tenant_id: str) -> bool:
        return not bool(self.quota_status(tenant_id)["exceeded"])

    def quota_status(self, tenant_id: str) -> dict[str, object]:
        tenant_key = self._tenant_key(tenant_id)
        with self._lock:
            used = self._round_money(self._tenant_usage.get(tenant_key, 0.0))
        budget = self._tenant_budget(tenant_key)
        remaining = None if budget is None else self._round_money(max(0.0, budget - used))
        return {
            "tenant_id": tenant_key,
            "currency": str(self._quota_config().get("currency") or "CNY"),
            "budget": budget,
            "used": used,
            "remaining": remaining,
            "exceeded": budget is not None and used >= budget,
            "policy": self._quota_exceed_policy(),
        }

    def call_log_snapshot(self) -> list[dict[str, object]]:
        with self._lock:
            return deepcopy(self._call_log)

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

    def production_route_readiness_issues(self, tenant_id: str = "production-config") -> list[str]:
        issues: list[str] = []
        route_names = sorted(self.LLM_ROUTES | self.EMBEDDING_ROUTES | self.RERANK_ROUTES)
        configured_routes = self.config.get("routes", {})
        for task_type in route_names:
            if task_type not in configured_routes:
                continue
            try:
                target = self.resolve(task_type, tenant_id=tenant_id)
            except Exception as exc:  # noqa: BLE001 - production startup must expose route readiness failures.
                issues.append(f"{task_type}: {exc}")
                continue
            if self._is_zero_cost_provider(target.provider):
                issues.append(f"{task_type}: zero-cost provider {target.provider}")
                continue
            if self.estimate_cost(target.provider, target.model, 1000, 1000) <= 0:
                issues.append(f"{task_type}: missing pricing for {target.provider}/{target.model}")
        return issues

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

    SUPPORTED_PROVIDER_TYPES = ("mock", "openai_compatible", "cloudflare_ai_gateway", "local")

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
                    api_key_required=bool(config.get("api_key_required", True)),
                    auth_header_name=str(config.get("auth_header_name") or ""),
                    auth_header_env=str(config.get("auth_header_env") or ""),
                    extra_headers_env=str(config.get("extra_headers_env") or ""),
                )
            elif provider_type == "cloudflare_ai_gateway":
                providers[name] = CloudflareAIGatewayProvider(name)
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

    def _quota_config(self) -> dict[str, Any]:
        config = self.config.get("quotas", {})
        return config if isinstance(config, dict) else {}

    def _quota_exceed_policy(self) -> str:
        policy = str(self._quota_config().get("on_exceed") or "block").strip().lower()
        return policy or "block"

    def _tenant_key(self, tenant_id: object) -> str:
        text = "" if tenant_id is None else str(tenant_id).strip()
        return text or "__unknown__"

    def _tenant_budget(self, tenant_id: str) -> float | None:
        quotas = self._quota_config()
        per_tenant = quotas.get("per_tenant_monthly_budget")
        if isinstance(per_tenant, dict) and tenant_id in per_tenant:
            return self._positive_money_or_none(per_tenant[tenant_id])
        return self._positive_money_or_none(quotas.get("default_tenant_monthly_budget"))

    def _positive_money_or_none(self, value: object) -> float | None:
        amount = self._finite_float(value)
        if amount is None or amount <= 0:
            return None
        return self._round_money(amount)

    def _cost_from_call(self, payload: dict[str, object]) -> float:
        for source in (payload, payload.get("model_metadata"), payload.get("result")):
            if isinstance(source, dict) and "estimated_cost" in source:
                amount = self._estimated_cost_or_zero(source["estimated_cost"])
                if amount > 0:
                    return amount
        provider = str(payload.get("provider") or "").strip()
        model = str(payload.get("model") or "").strip()
        input_tokens = self._positive_int(payload.get("input_tokens"))
        output_tokens = self._positive_int(payload.get("output_tokens"))
        return self.estimate_cost(provider, model, input_tokens, output_tokens)

    def estimate_cost(self, provider: str, model: str, input_tokens: int, output_tokens: int) -> float:
        if input_tokens <= 0 and output_tokens <= 0:
            return 0.0
        rate = self._pricing_for(provider, model)
        if not rate:
            return 0.0
        input_per_1k = self._positive_rate(rate.get("input_per_1k"))
        output_per_1k = self._positive_rate(rate.get("output_per_1k"))
        if input_per_1k == 0 and self._positive_rate(rate.get("input_per_1m")) > 0:
            input_per_1k = self._positive_rate(rate.get("input_per_1m")) / 1000
        if output_per_1k == 0 and self._positive_rate(rate.get("output_per_1m")) > 0:
            output_per_1k = self._positive_rate(rate.get("output_per_1m")) / 1000
        return self._estimated_cost_or_zero(
            (input_tokens * input_per_1k + output_tokens * output_per_1k) / 1000
        )

    def _pricing_for(self, provider: str, model: str) -> dict[str, object] | None:
        pricing = self._pricing_config()
        for key in self._pricing_lookup_keys(provider, model):
            rate = pricing.get(key)
            if isinstance(rate, dict):
                return rate
        return None

    def _pricing_config(self) -> dict[str, object]:
        configured = self.config.get("pricing")
        if isinstance(configured, dict):
            return configured
        raw = os.getenv("AI_MODEL_PRICING_JSON", "").strip()
        if not raw:
            return {}
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            return {}
        return parsed if isinstance(parsed, dict) else {}

    def _pricing_lookup_keys(self, provider: str, model: str) -> list[str]:
        provider = provider.strip()
        model = model.strip()
        candidates = [
            f"{provider}/{model}",
            model,
            f"{provider}/*",
            "*",
            f"{provider.lower()}/{model.lower()}",
            model.lower(),
            f"{provider.lower()}/*",
        ]
        keys: list[str] = []
        seen: set[str] = set()
        for key in candidates:
            if key not in seen:
                keys.append(key)
                seen.add(key)
        return keys

    def _positive_rate(self, value: object) -> float:
        amount = self._finite_float(value)
        if amount is None or amount <= 0:
            return 0.0
        return amount

    def _estimated_cost_or_zero(self, value: object) -> float:
        amount = self._finite_float(value)
        if amount is None or amount <= 0 or amount > MAX_AI_ESTIMATED_COST:
            return 0.0
        return self._round_money(amount)

    def _positive_int(self, value: object) -> int:
        if value is None or isinstance(value, bool):
            return 0
        try:
            number = int(value)
        except (TypeError, ValueError):
            return 0
        return number if number > 0 else 0

    def _finite_float(self, value: object) -> float | None:
        if value is None or isinstance(value, bool):
            return None
        try:
            amount = float(value)
        except (TypeError, ValueError):
            return None
        if not math.isfinite(amount):
            return None
        return amount

    def _round_money(self, value: float) -> float:
        return round(value, 4)

    def _call_event(
        self,
        tenant_id: str,
        estimated_cost: float,
        usage: dict[str, object],
        payload: dict[str, object],
    ) -> dict[str, object]:
        event: dict[str, object] = {
            "sequence": len(self._call_log) + 1,
            "tenant_id": tenant_id,
            "estimated_cost": estimated_cost,
            "usage_after_call": usage,
        }
        safe_fields = (
            "task_type",
            "provider",
            "model",
            "status",
            "input_tokens",
            "output_tokens",
            "latency_ms",
            "trace_id",
            "fallback_from",
        )
        for field in safe_fields:
            value = payload.get(field)
            if isinstance(value, str | int | float | bool) or value is None:
                event[field] = value
        return event

    def _is_zero_cost_provider(self, provider_name: str) -> bool:
        provider_config = self.config.get("providers", {}).get(provider_name, {})
        provider_type = ""
        if isinstance(provider_config, dict):
            provider_type = str(provider_config.get("type") or "").strip()
        return provider_name in {"mock", "local"} or provider_type in {"mock", "local"}

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
from typing import Any

from app.gateway.model_router import ModelRouter, RouteTarget


DEFAULT_ROUTES = ("chapter_generate", "knowledge_embedding", "knowledge_rerank")
PROVIDER_BACKED_ROUTE_KINDS = {
    "tender_parse": "llm",
    "outline_generate": "llm",
    "chapter_generate": "llm",
    "chapter_self_check": "llm",
    "compliance_check": "llm",
    "rewrite_assistant": "llm",
    "cost_advice": "llm",
    "knowledge_embedding": "embedding",
    "knowledge_rerank": "rerank",
}
ZERO_COST_PROVIDERS = {"mock", "local"}


def evaluate_provider_canary(
    routing_file: Path | None = None,
    *,
    routes: list[str] | tuple[str, ...] | None = None,
    tenant_id: str = "provider-canary",
    call_provider: bool = False,
    require_cost: bool = False,
    strict: bool = False,
) -> dict[str, Any]:
    selected_routes = tuple(route.strip() for route in (routes or DEFAULT_ROUTES) if route.strip())
    checks: list[dict[str, Any]] = []
    route_results: list[dict[str, Any]] = []
    routing_path = routing_file or _routing_file_from_env()

    _add_check(checks, "routing_file.exists", routing_path.is_file(), True, str(routing_path))
    if not routing_path.is_file():
        return _result(
            "failed" if strict else "skipped",
            checks,
            route_results,
            routing_path=routing_path,
            tenant_id=tenant_id,
            call_provider=call_provider,
            require_cost=require_cost,
            strict=strict,
        )

    try:
        router = ModelRouter.from_yaml(routing_path)
    except Exception as exc:  # noqa: BLE001 - canary must report config errors as checks.
        _add_check(checks, "router.load", False, "valid ModelRouter config", _safe_error(exc))
        return _result(
            "failed",
            checks,
            route_results,
            routing_path=routing_path,
            tenant_id=tenant_id,
            call_provider=call_provider,
            require_cost=require_cost,
            strict=strict,
        )
    _add_check(checks, "router.load", True, "valid ModelRouter config", "loaded")

    mock_routes = router.provider_backed_mock_routes()
    _add_check(checks, "router.provider_backed_mock_routes", not mock_routes, [], mock_routes)

    real_route_count = 0
    for route_name in selected_routes:
        route_result = _evaluate_route(
            router,
            route_name,
            tenant_id=tenant_id,
            call_provider=call_provider,
            require_cost=require_cost,
            checks=checks,
        )
        route_results.append(route_result)
        if route_result.get("provider") not in ZERO_COST_PROVIDERS and route_result.get("resolved"):
            real_route_count += 1

    if real_route_count == 0 and not strict:
        status = "skipped"
    elif any(not check["passed"] for check in checks):
        status = "failed"
    else:
        status = "passed"
    return _result(
        status,
        checks,
        route_results,
        routing_path=routing_path,
        tenant_id=tenant_id,
        call_provider=call_provider,
        require_cost=require_cost,
        strict=strict,
    )


def _evaluate_route(
    router: ModelRouter,
    route_name: str,
    *,
    tenant_id: str,
    call_provider: bool,
    require_cost: bool,
    checks: list[dict[str, Any]],
) -> dict[str, Any]:
    prefix = f"route.{route_name}"
    route_kind = PROVIDER_BACKED_ROUTE_KINDS.get(route_name)
    _add_check(checks, f"{prefix}.provider_backed", route_kind is not None, "known provider-backed route", route_kind)
    if route_kind is None:
        return {"route": route_name, "resolved": False, "error": "not provider-backed"}

    try:
        target = router.resolve(route_name, tenant_id=tenant_id)
    except Exception as exc:  # noqa: BLE001 - canary must show route resolution failures.
        _add_check(checks, f"{prefix}.resolved", False, "route resolves", _safe_error(exc))
        return {"route": route_name, "resolved": False, "error": _safe_error(exc)}

    result: dict[str, Any] = {
        "route": route_name,
        "kind": route_kind,
        "resolved": True,
        "provider": target.provider,
        "model": target.model,
        "fallback_from": target.fallback_from,
    }
    _add_check(checks, f"{prefix}.resolved", True, "route resolves", _target_summary(target))
    _add_check(checks, f"{prefix}.non_mock_provider", target.provider != "mock", "not mock", target.provider)
    _add_check(checks, f"{prefix}.model", bool(target.model.strip()), "non-empty model", target.model)

    try:
        provider = router.provider_for_target(target)
    except Exception as exc:  # noqa: BLE001 - canary must show provider binding failures.
        _add_check(checks, f"{prefix}.provider_bound", False, "provider bound", _safe_error(exc))
        result["error"] = _safe_error(exc)
        return result
    _add_check(checks, f"{prefix}.provider_bound", True, "provider bound", type(provider).__name__)

    health = _provider_health(provider)
    _add_check(checks, f"{prefix}.health", health, True, health)

    sample = _route_sample(route_kind)
    call_tokens = {"input_tokens": sample["input_tokens"], "output_tokens": sample["output_tokens"]}
    if call_provider and target.provider not in ZERO_COST_PROVIDERS:
        call_result = _call_provider(route_kind, provider, sample)
        result["call"] = call_result
        _add_check(
            checks,
            f"{prefix}.call_provider",
            bool(call_result.get("passed")),
            "provider returns usable output",
            call_result.get("actual"),
        )
        if isinstance(call_result.get("input_tokens"), int):
            call_tokens["input_tokens"] = int(call_result["input_tokens"])
        if isinstance(call_result.get("output_tokens"), int):
            call_tokens["output_tokens"] = int(call_result["output_tokens"])

    accounting = router.log_call(
        tenant_id=tenant_id,
        task_type=route_name,
        provider=target.provider,
        model=target.model,
        input_tokens=call_tokens["input_tokens"],
        output_tokens=call_tokens["output_tokens"],
        status="done",
        trace_id=f"canary-{route_name}",
        fallback_from=target.fallback_from,
    )
    result["accounting"] = _safe_accounting(accounting)
    estimated_cost = _float_value(accounting.get("estimated_cost"))
    _add_check(checks, f"{prefix}.accounting_logged", accounting.get("logged") is True, True, accounting.get("logged"))
    _add_check(checks, f"{prefix}.quota_usage", isinstance(accounting.get("usage"), dict), "quota usage dict", accounting.get("usage"))
    if require_cost and target.provider not in ZERO_COST_PROVIDERS:
        _add_check(checks, f"{prefix}.estimated_cost", estimated_cost > 0, ">0", estimated_cost)
    else:
        _add_check(checks, f"{prefix}.estimated_cost_nonnegative", estimated_cost >= 0, ">=0", estimated_cost)
    return result


def _call_provider(route_kind: str, provider: object, sample: dict[str, Any]) -> dict[str, Any]:
    try:
        if route_kind == "embedding" and hasattr(provider, "embed_text"):
            vector = provider.embed_text(sample["text"])
            return {
                "passed": isinstance(vector, list) and bool(vector),
                "actual": {"dimensions": len(vector) if isinstance(vector, list) else 0},
                "input_tokens": _provider_count_tokens(provider, sample["text"]),
                "output_tokens": 0,
            }
        if route_kind == "rerank" and hasattr(provider, "rerank"):
            indexes = provider.rerank(sample["query"], sample["documents"])
            return {
                "passed": isinstance(indexes, list) and bool(indexes),
                "actual": {"indexes": indexes},
                "input_tokens": _provider_count_tokens(provider, sample["query"] + "\n".join(sample["documents"])),
                "output_tokens": len(indexes) if isinstance(indexes, list) else 0,
            }
        if hasattr(provider, "complete"):
            text = str(provider.complete(sample["prompt"]))
            return {
                "passed": bool(text.strip()),
                "actual": _short_text(text),
                "input_tokens": _provider_count_tokens(provider, sample["prompt"]),
                "output_tokens": _provider_count_tokens(provider, text),
            }
        return {"passed": False, "actual": f"provider does not support route kind {route_kind}"}
    except Exception as exc:  # noqa: BLE001 - canary must report provider failures.
        return {"passed": False, "actual": _safe_error(exc)}


def _provider_health(provider: object) -> bool:
    if not hasattr(provider, "health_check"):
        return False
    try:
        return bool(provider.health_check())
    except Exception:  # noqa: BLE001 - health checks should never crash canary output.
        return False


def _route_sample(route_kind: str) -> dict[str, Any]:
    if route_kind == "embedding":
        return {"text": "ZBT provider canary embedding sample", "input_tokens": 1000, "output_tokens": 0}
    if route_kind == "rerank":
        return {
            "query": "bid document requirement",
            "documents": ["technical proposal requirement", "unrelated office notice"],
            "input_tokens": 1000,
            "output_tokens": 200,
        }
    return {"prompt": "Reply with the exact text ZBT_OK.", "input_tokens": 1000, "output_tokens": 200}


def _provider_count_tokens(provider: object, text: str) -> int:
    if hasattr(provider, "count_tokens"):
        try:
            return max(0, int(provider.count_tokens(text)))
        except Exception:  # noqa: BLE001 - token counting is only audit metadata.
            return _rough_tokens(text)
    return _rough_tokens(text)


def _rough_tokens(text: str) -> int:
    value = text.strip()
    return max(1, len(value) // 4) if value else 0


def _routing_file_from_env() -> Path:
    configured = os.getenv("MODEL_ROUTING_FILE", "").strip()
    if configured:
        return Path(configured)
    return _repo_root() / "ai-service/app/config/model_routing.yaml"


def _result(
    status: str,
    checks: list[dict[str, Any]],
    route_results: list[dict[str, Any]],
    *,
    routing_path: Path,
    tenant_id: str,
    call_provider: bool,
    require_cost: bool,
    strict: bool,
) -> dict[str, Any]:
    passed = sum(1 for check in checks if check["passed"])
    total = len(checks)
    return {
        "name": "provider_canary",
        "status": status,
        "routing_file": str(routing_path),
        "tenant_id": tenant_id,
        "call_provider": call_provider,
        "require_cost": require_cost,
        "strict": strict,
        "passed_checks": passed,
        "failed_checks": total - passed,
        "total_checks": total,
        "routes": route_results,
        "checks": checks,
    }


def _target_summary(target: RouteTarget) -> dict[str, str | None]:
    return {"provider": target.provider, "model": target.model, "fallback_from": target.fallback_from}


def _safe_accounting(accounting: dict[str, object]) -> dict[str, object]:
    return {
        "estimated_cost": accounting.get("estimated_cost"),
        "usage": accounting.get("usage") if isinstance(accounting.get("usage"), dict) else {},
        "fallback_from": accounting.get("fallback_from"),
    }


def _float_value(value: object) -> float:
    if value is None or isinstance(value, bool):
        return 0.0
    try:
        return float(value)
    except (TypeError, ValueError):
        return 0.0


def _add_check(checks: list[dict[str, Any]], name: str, passed: bool, expected: Any, actual: Any) -> None:
    checks.append({"name": name, "passed": bool(passed), "expected": expected, "actual": actual})


def _safe_error(exc: Exception) -> str:
    message = str(exc).strip()
    return _short_text(message or exc.__class__.__name__)


def _short_text(value: str) -> str:
    normalized = " ".join(value.split())
    return normalized[:240]


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def main() -> int:
    parser = argparse.ArgumentParser(description="Evaluate non-mock AI provider routing and accounting.")
    parser.add_argument("--routing-file", type=Path, default=None)
    parser.add_argument("--route", action="append", dest="routes", help="Route to evaluate. Can be repeated.")
    parser.add_argument("--tenant-id", default="provider-canary")
    parser.add_argument("--call-provider", action="store_true", help="Send a minimal live request to the selected providers.")
    parser.add_argument("--require-cost", action="store_true", help="Fail when non-mock routes do not produce positive estimated_cost.")
    parser.add_argument("--strict", action="store_true", help="Fail instead of skipping when no non-mock provider route is available.")
    parser.add_argument("--allow-skip", action="store_true", help="Exit 0 when no non-mock provider route is configured.")
    parser.add_argument("--json", action="store_true", help="Print full JSON result.")
    args = parser.parse_args()

    result = evaluate_provider_canary(
        args.routing_file,
        routes=args.routes,
        tenant_id=args.tenant_id,
        call_provider=args.call_provider,
        require_cost=args.require_cost,
        strict=args.strict,
    )
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(
            f"{result['status']} passed={result['passed_checks']}/{result['total_checks']} "
            f"routes={','.join(route['route'] for route in result['routes'])}"
        )
        for check in result["checks"]:
            if not check["passed"]:
                print(f"- {check['name']}: expected={check['expected']!r} actual={check['actual']!r}")
    if result["status"] == "passed" or (result["status"] == "skipped" and args.allow_skip):
        return 0
    if result["status"] == "skipped":
        return 2
    return 1


if __name__ == "__main__":
    raise SystemExit(main())

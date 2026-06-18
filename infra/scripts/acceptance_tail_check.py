#!/usr/bin/env python3
"""Runtime acceptance checks for x.md items 39-50.

The script assumes the local Docker stack is running and uses only the public
HTTP APIs that a developer would use while validating the SaaS prototype.
"""

from __future__ import annotations

import json
import os
import sys
import time
import uuid
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[2]
API_BASE = os.getenv("ZBT_API_BASE", "http://127.0.0.1:5173/api/v1").rstrip("/")
AI_BASE = os.getenv("ZBT_AI_BASE", "http://127.0.0.1:8000").rstrip("/")
TENANT_ID = os.getenv("ZBT_ACCEPTANCE_TENANT_ID", "00000000-0000-4000-8000-000000000001")
EMAIL = os.getenv("ZBT_ACCEPTANCE_EMAIL", "admin@zbt.local")
PASSWORD = os.getenv("ZBT_ACCEPTANCE_PASSWORD", "demo-password")


class AcceptanceError(RuntimeError):
    pass


def request_json(method: str, url: str, token: str | None = None, payload: object | None = None, expected: tuple[int, ...] = (200,)) -> object:
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = Request(url, data=data, headers=headers, method=method)
    try:
        with urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8")
            if resp.status not in expected:
                raise AcceptanceError(f"{method} {url} returned {resp.status}, expected {expected}: {body}")
            return json.loads(body) if body else {}
    except HTTPError as exc:
        body = exc.read().decode("utf-8")
        raise AcceptanceError(f"{method} {url} returned {exc.code}, expected {expected}: {body}") from exc
    except URLError as exc:
        raise AcceptanceError(f"{method} {url} failed: {exc}") from exc


def api(method: str, path: str, token: str | None = None, payload: object | None = None, expected: tuple[int, ...] = (200,)) -> object:
    return request_json(method, f"{API_BASE}{path}", token=token, payload=payload, expected=expected)


def ai(method: str, path: str, payload: object | None = None, expected: tuple[int, ...] = (200,)) -> object:
    return request_json(method, f"{AI_BASE}{path}", payload=payload, expected=expected)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AcceptanceError(message)


def ok(label: str, evidence: object) -> None:
    print(f"[ok] {label}: {evidence}")


def login() -> str:
    payload = {"tenant_id": TENANT_ID, "email": EMAIL, "password": PASSWORD}
    result = api("POST", "/auth/login", payload=payload)
    token = result.get("access_token") if isinstance(result, dict) else None
    require(isinstance(token, str) and token, "login did not return access_token")
    ok("login", EMAIL)
    return token


def create_bid(token: str, title: str) -> dict[str, object]:
    bid = api(
        "POST",
        "/bids",
        token=token,
        payload={"title": title, "project_name": title, "bid_type": "combined"},
        expected=(201,),
    )
    require(isinstance(bid, dict) and bid.get("id"), "create bid did not return id")
    return bid


def fetch_bid(token: str, bid_id: str) -> dict[str, object]:
    bid = api("GET", f"/bids/{bid_id}", token=token)
    require(isinstance(bid, dict), f"bid {bid_id} response is not an object")
    return bid


def submit_bid(token: str, bid_id: str) -> dict[str, object]:
    detail = api("POST", f"/bids/{bid_id}/submit-for-approval", token=token, expected=(201,))
    require(isinstance(detail, dict), "submit approval response is not an object")
    instance = detail.get("instance")
    require(isinstance(instance, dict) and instance.get("status") == "pending", "approval submission did not create pending instance")
    require(instance.get("bid_document_id") == bid_id, "approval instance is not linked to submitted bid")
    return detail


def approve_until_done(token: str, instance_id: str) -> dict[str, object]:
    detail: dict[str, object] = {}
    for index in range(1, 8):
        detail = api(
            "POST",
            f"/approvals/{instance_id}/approve",
            token=token,
            payload={"comment": f"tail acceptance approve step {index}"},
        )
        instance = detail.get("instance") if isinstance(detail, dict) else None
        require(isinstance(instance, dict), "approval approve response missing instance")
        if instance.get("status") == "approved":
            return detail
    raise AcceptanceError(f"approval instance {instance_id} did not become approved after repeated approvals")


def create_won_project(token: str, name: str) -> dict[str, object]:
    project = api("POST", "/projects", token=token, payload={"name": name}, expected=(201,))
    require(isinstance(project, dict) and project.get("id"), "create project did not return id")
    project_id = str(project["id"])
    closed = api("POST", f"/projects/{project_id}/transition", token=token, payload={"status": "closed", "result": "won"})
    require(isinstance(closed, dict) and closed.get("status") == "closed" and closed.get("result") == "won", "project did not transition to closed/won")
    return closed


def check_approval_flow(token: str, stamp: str) -> None:
    submitted = create_bid(token, f"xmd-tail-submit-{stamp}")
    submit_detail = submit_bid(token, str(submitted["id"]))
    submitted_bid = fetch_bid(token, str(submitted["id"]))
    require(submitted_bid.get("status") == "in_review", "submitted bid did not enter in_review")
    ok("39 submit approval", {"bid_id": submitted["id"], "approval_id": submit_detail["instance"]["id"]})

    approving = create_bid(token, f"xmd-tail-approve-{stamp}")
    approve_detail = submit_bid(token, str(approving["id"]))
    approved = approve_until_done(token, str(approve_detail["instance"]["id"]))
    approved_bid = fetch_bid(token, str(approving["id"]))
    require(approved["instance"]["status"] == "approved", "approval did not finish as approved")
    require(approved_bid.get("status") == "approved", "approved bid did not enter approved status")
    ok("40 approve approval", {"bid_id": approving["id"], "approval_id": approved["instance"]["id"]})

    rejecting = create_bid(token, f"xmd-tail-reject-{stamp}")
    reject_detail = submit_bid(token, str(rejecting["id"]))
    rejected = api(
        "POST",
        f"/approvals/{reject_detail['instance']['id']}/reject",
        token=token,
        payload={"comment": "tail acceptance reject"},
    )
    rejected_bid = fetch_bid(token, str(rejecting["id"]))
    require(rejected["instance"]["status"] == "rejected", "approval did not finish as rejected")
    require(rejected_bid.get("status") == "editing", "rejected bid did not return to editing")
    ok("40-41 reject approval returns bid to editing", {"bid_id": rejecting["id"], "approval_id": rejected["instance"]["id"]})


def check_cost_and_case_flow(token: str, stamp: str) -> dict[str, object]:
    project_name = f"xmd-tail-won-project-{stamp}"
    project = create_won_project(token, project_name)
    project_id = str(project["id"])

    cost_project = api("POST", f"/projects/{project_id}/create-cost-project", token=token, expected=(201,))
    require(isinstance(cost_project, dict) and cost_project.get("id"), "create cost project did not return id")
    cost_project_id = str(cost_project["id"])
    ok("42 won project creates cost project", {"project_id": project_id, "cost_project_id": cost_project_id})

    item = api(
        "POST",
        f"/cost-projects/{cost_project_id}/items",
        token=token,
        payload={
            "category": "labor",
            "name": f"xmd-tail-labor-{stamp}",
            "cost_type": "labor",
            "budget_amount": 10000,
            "actual_amount": 8500,
            "status": "actual",
            "vendor": "acceptance vendor",
            "note": "x.md tail acceptance",
        },
        expected=(201,),
    )
    require(isinstance(item, dict) and item.get("id"), "create cost item did not return id")
    ok("43 records cost item", {"cost_project_id": cost_project_id, "item_id": item["id"]})

    analysis = api("GET", f"/cost-projects/{cost_project_id}/analysis", token=token)
    project_analysis = analysis.get("project") if isinstance(analysis, dict) else None
    require(isinstance(project_analysis, dict), "cost analysis missing project totals")
    require(float(project_analysis.get("total_budget", 0)) >= 10000, "cost analysis total_budget did not include item")
    require(float(project_analysis.get("total_actual", 0)) >= 8500, "cost analysis total_actual did not include item")
    require("margin_rate" in project_analysis, "cost analysis missing margin_rate")
    ok("44 cost analysis budget actual margin", {
        "total_budget": project_analysis["total_budget"],
        "total_actual": project_analysis["total_actual"],
        "margin_rate": project_analysis["margin_rate"],
    })

    archived = api("POST", f"/projects/{project_id}/archive-case", token=token, expected=(201,))
    case = archived.get("case") if isinstance(archived, dict) else None
    file_asset = archived.get("file") if isinstance(archived, dict) else None
    require(isinstance(case, dict) and case.get("document_id") and case.get("chunk_id"), "archive case response missing document/chunk")
    require(isinstance(file_asset, dict) and file_asset.get("status") == "ready", "archive case file is not ready")
    ok("45 won case archives to knowledge", {
        "project_id": project_id,
        "document_id": case["document_id"],
        "chunk_id": case["chunk_id"],
        "file_id": file_asset["id"],
    })
    return {"project_name": project_name, "document_id": case["document_id"]}


def check_ai_logging_and_search(token: str, project_name: str, document_id: str) -> None:
    before = api("GET", "/ai-call-logs?limit=50", token=token)
    before_ids = {item.get("id") for item in before.get("items", [])} if isinstance(before, dict) else set()

    search = api("POST", "/knowledge/search", token=token, payload={"query": project_name, "doc_type": "won_case", "limit": 5})
    items = search.get("items") if isinstance(search, dict) else None
    require(isinstance(items, list) and items, "knowledge search did not return archived won case")
    require(any(item.get("document", {}).get("id") == document_id for item in items), "knowledge search did not include archived document")

    after = api("GET", "/ai-call-logs?limit=50", token=token)
    logs = after.get("items", []) if isinstance(after, dict) else []
    new_logs = [item for item in logs if item.get("id") not in before_ids]
    require(any(item.get("task_type") == "knowledge_embedding" and item.get("status") == "done" for item in new_logs + logs[:5]), "knowledge search did not create a done knowledge_embedding ai_call_log")
    ok("46 AI calls write ai_call_logs", {"new_log_count": len(new_logs), "searched_document_id": document_id})


def check_model_router_health() -> None:
    model_health = ai("GET", "/models/health")
    providers = model_health.get("providers") if isinstance(model_health, dict) else None
    require(isinstance(providers, dict) and providers.get("mock") is True, "AI /models/health did not report healthy mock provider")
    ok("48 MockProvider is healthy", providers)


def latest_loop_section(loop_log: str) -> tuple[str, str]:
    marker = "\n## Loop-"
    start = loop_log.rfind(marker)
    if start >= 0:
        start += 1
    elif loop_log.startswith("## Loop-"):
        start = 0
    else:
        raise AcceptanceError("DEV_LOOP_LOG.md has no Loop section")
    section = loop_log[start:]
    heading = section.splitlines()[0] if section else ""
    require(heading.startswith("## Loop-"), "DEV_LOOP_LOG.md latest section heading is invalid")
    return heading, section


def check_static_docs() -> None:
    routing = (ROOT / "ai-service/app/config/model_routing.yaml").read_text(encoding="utf-8")
    require("providers:" in routing and "mock:" in routing, "model_routing.yaml missing mock provider")
    for route in ("knowledge_embedding", "knowledge_rerank", "chapter_generate", "cost_advice", "document_export", "document_ocr"):
        require(route in routing, f"model_routing.yaml missing route {route}")
    ok("47 model routing file controls model routes", "ai-service/app/config/model_routing.yaml")

    readme = (ROOT / "README.md").read_text(encoding="utf-8")
    for needle in ("docker compose up -d --build", "./infra/scripts/check.sh", "MODEL_ROUTING_FILE", "MockProvider", "demo-password", "provider_profile.endpoint_env", "api_key_env", "poll_endpoint_env"):
        require(needle in readme, f"README.md missing startup guidance: {needle}")
    ok("49 README startup guidance", "README.md")

    page_routes = (ROOT / "docs/blueprint/PAGE_ROUTE_MAP.md").read_text(encoding="utf-8")
    for needle in ("/knowledge/documents/:documentId/preview", "external-tools", "外部标讯"):
        require(needle in page_routes, f"PAGE_ROUTE_MAP.md missing route/page tab: {needle}")

    team_page = (ROOT / "frontend/src/features/team/index.tsx").read_text(encoding="utf-8")
    require("preset.token_env" not in team_page, "Team external data source UI exposes token env names")
    require("endpoint_hint" not in team_page, "Team external data source UI exposes endpoint templates")
    require(
        "{ title: '数据源', dataIndex: 'tool_provider'" not in team_page,
        "Team external audit UI exposes provider keys directly",
    )
    for needle in ("授权状态", "授权说明", "启用能力"):
        require(needle in team_page, f"Team external data source UI missing user-facing label: {needle}")
    for needle in ("formatExternalToolProviderName", "externalToolDisplayNameMap"):
        require(needle in team_page, f"Team external audit UI missing provider display guard: {needle}")

    bid_page = (ROOT / "frontend/src/features/bid/index.tsx").read_text(encoding="utf-8")
    require(
        "JSON.stringify(value, null, 2)" not in bid_page,
        "Bid parse confirmation UI exposes raw JSON in module field editor",
    )
    require("subtitle={bid.data?.title ?? bidId}" not in bid_page, "Bid editor subtitle exposes bid UUID fallback")
    require("正在编辑标书" in bid_page, "Bid editor subtitle missing business fallback")
    for needle in ("parseModuleObjectSummary", "parseModuleFieldItemSummary"):
        require(needle in bid_page, f"Bid parse confirmation UI missing readable formatter: {needle}")
    api_client = (ROOT / "frontend/src/shared/api/client.ts").read_text(encoding="utf-8")
    require("`响应矩阵-${bidId}" not in api_client, "Requirement export fallback filename exposes bid UUID")
    for needle in ("bidRequirementFallbackFilename", "downloadSafeFilenamePart"):
        require(needle in api_client, f"Requirement export fallback filename missing business guard: {needle}")
    api_routes = (ROOT / "backend/internal/api/routes.go").read_text(encoding="utf-8")
    for needle in ("bidRequirementExportFilename(document.Title", "downloadSafeFilenamePart"):
        require(needle in api_routes, f"Requirement export response filename missing business guard: {needle}")
    for forbidden in ("label: '引用号'", "label: '定位码'", "label: '坐标'"):
        require(forbidden not in bid_page, f"Bid requirement source UI exposes technical locator field: {forbidden}")
    require("label: '原文位置'" in bid_page, "Bid requirement source UI missing business locator fallback")

    knowledge_page = (ROOT / "frontend/src/features/knowledge/index.tsx").read_text(encoding="utf-8")
    for forbidden in ("<Tag>坐标：", "`坐标: ${bboxText}`"):
        require(forbidden not in knowledge_page, f"File preview source UI exposes raw OCR coordinates: {forbidden}")
    for needle in ("原文位置：已定位", "sanitizePreviewLocatorText"):
        require(needle in knowledge_page, f"File preview source UI missing business locator guard: {needle}")

    compliance_page = (ROOT / "frontend/src/features/compliance/index.tsx").read_text(encoding="utf-8")
    for forbidden in ("规则编号", "填写标书编号", "title: '编码'"):
        require(forbidden not in compliance_page, f"Compliance UI exposes technical field: {forbidden}")
    for needle in ("createComplianceRuleCode", "可选，选择需要检查的标书"):
        require(needle in compliance_page, f"Compliance UI missing user-facing workflow guard: {needle}")

    ai_pipeline = (ROOT / "docs/blueprint/AI_PIPELINE.md").read_text(encoding="utf-8")
    require("尚未接业务入口" not in ai_pipeline, "AI_PIPELINE.md still claims external MCP has no business entry")
    for needle in ("外部标讯", "provider_profile.endpoint_env", "api_key_env", "poll_endpoint_env"):
        require(needle in ai_pipeline, f"AI_PIPELINE.md missing current capability note: {needle}")

    sample_eval = (ROOT / "docs/blueprint/SAMPLE_DOCS_EVALUATION.md").read_text(encoding="utf-8")
    for needle in ("provider_profile.endpoint_env", "api_key_env", "poll_endpoint_env"):
        require(needle in sample_eval, f"SAMPLE_DOCS_EVALUATION.md missing OCR profile audit note: {needle}")

    loop_log = (ROOT / "docs/blueprint/DEV_LOOP_LOG.md").read_text(encoding="utf-8")
    latest_heading, latest_section = latest_loop_section(loop_log)
    for needle in ("### 本轮目标", "### 代码交付", "### 检查结果", "### 偏离蓝图"):
        require(needle in latest_section, f"DEV_LOOP_LOG.md latest section missing {needle}: {latest_heading}")
    ok("50 DEV_LOOP_LOG records latest delivery loop", latest_heading)


def check_model_router_and_docs() -> None:
    check_model_router_health()
    check_static_docs()


def main(argv: list[str] | None = None) -> int:
    args = sys.argv[1:] if argv is None else argv
    if args == ["--static-docs"]:
        check_static_docs()
        print("[ok] x.md tail static acceptance docs passed")
        return 0
    if args:
        raise AcceptanceError("usage: acceptance_tail_check.py [--static-docs]")

    stamp = time.strftime("%Y%m%d%H%M%S") + "-" + uuid.uuid4().hex[:8]
    token = login()
    check_approval_flow(token, stamp)
    case_context = check_cost_and_case_flow(token, stamp)
    check_ai_logging_and_search(token, str(case_context["project_name"]), str(case_context["document_id"]))
    check_model_router_and_docs()
    print("[ok] x.md tail acceptance items 39-50 passed")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except AcceptanceError as exc:
        print(f"[fail] {exc}", file=sys.stderr)
        raise SystemExit(1)

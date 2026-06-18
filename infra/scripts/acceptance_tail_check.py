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
    require(
        "render: (_, row) => row.error_message" not in team_page,
        "Team external audit UI exposes raw external tool errors",
    )
    require(
        "{ title: '请求摘要', dataIndex: 'request_summary'" not in team_page,
        "Team external audit UI exposes structural request summaries",
    )
    require(
        "return row.response_summary || '调用成功'" not in team_page,
        "Team external audit UI exposes raw success summaries",
    )
    for needle in (
        "formatExternalToolProviderName",
        "externalToolDisplayNameMap",
        "formatExternalToolAuditResult",
        "externalToolBusinessErrorMessage",
        "formatExternalToolAuditRequest",
        "formatExternalToolSuccessSummary",
        "parseExternalToolSummary",
        "countExternalToolResultItems",
        "提交内容",
        "结果摘要",
    ):
        require(needle in team_page, f"Team external audit UI missing provider display guard: {needle}")

    external_tool_store = (ROOT / "backend/internal/platform/externaltool/store.go").read_text(encoding="utf-8")
    for needle in (
        "validExternalToolEndpoint",
        "newExternalToolHTTPClient",
        "publicExternalToolNetIP",
        "externalToolSpecialUseIPPrefixes",
        "externalToolEndpointHasSensitiveQuery",
        "structuralSummary",
        "redactExternalToolError",
        "normalizeExternalToolMetadata",
        "validExternalToolMoney",
        "maxExternalToolCostPerCall",
        "maxExternalToolArgumentsJSONBytes",
        "maxExternalToolResponseBytes",
        "maxExternalToolResourceTypeRunes",
        "maxExternalToolConfigMetadataJSONBytes",
        "maxExternalToolAuditMetadataJSONBytes",
        "normalizeExternalToolArguments",
        "marshalExternalToolMetadataJSON",
        "readExternalToolResponseBody",
        "net.DefaultResolver.LookupNetIP",
        "CheckRedirect",
    ):
        require(needle in external_tool_store, f"External tool gateway missing public endpoint guard: {needle}")
    for forbidden in (
        "metadataRaw, _ := json.Marshal(normalized.Metadata)",
        "metadataRaw, _ := json.Marshal(input.Metadata)",
    ):
        require(forbidden not in external_tool_store, f"External tool gateway still ignores metadata marshal failure: {forbidden}")
    require(
        "json.Marshal(value)" not in external_tool_store,
        "External tool audit summary still serializes raw response values",
    )
    require(
        'if endpoint != "" && !validExternalToolEndpoint(endpoint)' in external_tool_store,
        "External tool config does not validate endpoints with the public endpoint guard",
    )
    require(
        "client: newExternalToolHTTPClient()" in external_tool_store,
        "External tool gateway does not use the guarded HTTP client",
    )
    external_tool_tests = (ROOT / "backend/internal/platform/externaltool/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestNormalizeConfigRejectsUnsafeExternalEndpoints",
        "TestExternalToolHTTPClientRejectsLocalhostDial",
        "TestSummarizeValueDoesNotPersistRawExternalResponse",
        "TestSafeErrorRedactsExternalEndpointAndSecrets",
        "TestNormalizeConfigNormalizesCostMetadata",
        "TestNormalizeConfigRejectsInvalidExternalToolMoney",
        "TestMarshalExternalToolMetadataJSONRejectsInvalidAndOversizedValues",
        "TestCostPerCallIgnoresInvalidStoredMetadata",
        "TestNormalizeInvokeRequestRejectsOversizedAndNonJSONArguments",
        "TestCallStreamableHTTPRejectsNonJSONArgumentsBeforeOutboundRequest",
        "TestCallStreamableHTTPRejectsOversizedResponseWithoutLeakingBody",
    ):
        require(needle in external_tool_tests, f"External tool gateway missing SSRF regression test: {needle}")
    external_tool_presets = (ROOT / "backend/internal/platform/externaltool/presets.go").read_text(encoding="utf-8")
    for needle in ("?token=", "<token>", "?api_key=", "?access_token="):
        require(needle not in external_tool_presets, f"External tool preset leaks token-in-URL hint: {needle}")
    require("structuralArrayCount" in team_page, "Team external audit UI cannot count structural response summaries")

    bid_page = (ROOT / "frontend/src/features/bid/index.tsx").read_text(encoding="utf-8")
    api_client = (ROOT / "frontend/src/shared/api/client.ts").read_text(encoding="utf-8")
    require("getUserFacingErrorMessage" in api_client, "API client missing shared user-facing error filter")
    require(
        "JSON.stringify(value, null, 2)" not in bid_page,
        "Bid parse confirmation UI exposes raw JSON in module field editor",
    )
    for forbidden in ("parseResult.data.error_message?.trim()", "task.error_message?.trim()) return task.error_message.trim()"):
        require(forbidden not in bid_page, f"Bid task UI exposes raw task error message: {forbidden}")
    require("empty evidence or source" not in bid_page, "Bid requirement evidence modal exposes internal English validation error")
    require("new Error('请填写响应证据或来源')" in bid_page, "Bid requirement evidence modal missing business validation rejection")
    require("getUserFacingErrorMessage(parseResult.data.error_message" in bid_page, "Bid parse failure UI missing user-facing error filter")
    require("getUserFacingErrorMessage(task.error_message" in bid_page, "Bid task failure UI missing user-facing error filter")
    require("subtitle={bid.data?.title ?? bidId}" not in bid_page, "Bid editor subtitle exposes bid UUID fallback")
    require("正在编辑标书" in bid_page, "Bid editor subtitle missing business fallback")
    for needle in ("parseModuleObjectSummary", "parseModuleFieldItemSummary"):
        require(needle in bid_page, f"Bid parse confirmation UI missing readable formatter: {needle}")
    require("`响应矩阵-${bidId}" not in api_client, "Requirement export fallback filename exposes bid UUID")
    for needle in ("bidRequirementFallbackFilename", "downloadSafeFilenamePart"):
        require(needle in api_client, f"Requirement export fallback filename missing business guard: {needle}")
    for needle in ("safeDownloadFilename", "WINDOWS_RESERVED_DOWNLOAD_NAMES", "MAX_DOWNLOAD_FILENAME_LENGTH"):
        require(needle in api_client, f"Requirement export response filename missing client download guard: {needle}")
    require(
        "filename: safeDownloadFilename(filenameFromContentDisposition(response.headers['content-disposition']), fallbackFilename)"
        in api_client,
        "Requirement export filename must sanitize Content-Disposition before browser download",
    )
    api_routes = (ROOT / "backend/internal/api/routes.go").read_text(encoding="utf-8")
    for needle in ("bidRequirementExportFilename(document.Title", "downloadSafeFilenamePart"):
        require(needle in api_routes, f"Requirement export response filename missing business guard: {needle}")
    for needle in (
        "boundedQueryLimit(c, 50, 200)",
        "boundedQueryLimit(c, 50, 100)",
        "func boundedQueryLimit",
        "respondBadRequest(c)",
    ):
        require(needle in api_routes, f"API list endpoint missing bounded query limit guard: {needle}")
    api_routes_tests = (ROOT / "backend/internal/api/routes_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestBoundedQueryLimitDefaultsAndAcceptsRange",
        "TestBoundedQueryLimitRejectsInvalidValues",
    ):
        require(needle in api_routes_tests, f"API list endpoint missing bounded query limit regression test: {needle}")
    for forbidden in ("label: '引用号'", "label: '定位码'", "label: '坐标'"):
        require(forbidden not in bid_page, f"Bid requirement source UI exposes technical locator field: {forbidden}")
    require("label: '原文位置'" in bid_page, "Bid requirement source UI missing business locator fallback")

    file_service = (ROOT / "backend/internal/platform/file/service.go").read_text(encoding="utf-8")
    for needle in ("maxFilenameRunes", "utf8.RuneCountInString(base)", "return \"\""):
        require(needle in file_service, f"File upload service missing filename length guard: {needle}")
    file_service_tests = (ROOT / "backend/internal/platform/file/service_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestSanitizeFilenameRejectsOversizedNames",
        "TestSanitizeFilenameAllowsBoundedUnicodeNames",
    ):
        require(needle in file_service_tests, f"File upload service missing filename length regression test: {needle}")

    bid_store = (ROOT / "backend/internal/platform/bid/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxExportFilenameRunes",
        "maxExportFilenameLabelRunes",
        "stripFilenameControlChars",
        "utf8.RuneCountInString(suffix)",
        "base = truncateRunes(base, baseLimit)",
    ):
        require(needle in bid_store, f"Bid export filename missing filename boundary guard: {needle}")
    bid_store_tests = (ROOT / "backend/internal/platform/bid/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestExportFilenameSanitizesUnsafeTitleCharacters",
        "TestExportFilenameCapsLongTitleAndPreservesSuffix",
    ):
        require(needle in bid_store_tests, f"Bid export filename missing boundary regression test: {needle}")

    tender_store = (ROOT / "backend/internal/platform/tender/store.go").read_text(encoding="utf-8")
    require(
        "config, _ := json.Marshal(req.Config)" not in tender_store,
        "Tender source config still ignores JSON marshal errors",
    )
    for needle in (
        "maxSourceConfigEntries",
        "maxSourceConfigKeyRunes",
        "maxSourceConfigJSONBytes",
        "normalizeSourceConfig",
        "config, err := normalizeSourceConfig(req.Config)",
        "len(raw) > maxSourceConfigJSONBytes",
    ):
        require(needle in tender_store, f"Tender source config missing input boundary: {needle}")
    tender_store_tests = (ROOT / "backend/internal/platform/tender/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestCreateSourceRejectsOversizedConfigBeforeDB",
        "TestNormalizeSourceConfigTrimsKeysAndBoundsJSON",
        "TestNormalizeSourceConfigRejectsInvalidShape",
    ):
        require(needle in tender_store_tests, f"Tender source config missing regression test: {needle}")
    for needle in (
        "maxTenderTitleRunes",
        "maxTenderShortTextRunes",
        "maxTenderSummaryRunes",
        "maxTenderListItems",
        "maxTenderMetadataJSONBytes",
        "maxTenderBudgetAmount",
        "maxTenderSourceNameRunes",
        "maxTenderSourceTypeRunes",
        "maxTenderURLRunes",
        "normalizeTenderRequiredText",
        "normalizeTenderOptionalText",
        "normalizeTenderTextList",
        "normalizeTenderMetadata",
        "validateOptionalTenderAmount",
        "math.IsNaN",
        "len([]rune(value)) > maxTenderURLRunes",
    ):
        require(needle in tender_store, f"Tender write request missing business input boundary: {needle}")
    for needle in (
        "TestNormalizeTenderWriteRequestTrimsBusinessFieldsAndLists",
        "TestCreateTenderRejectsOversizedBusinessFieldsBeforeDB",
        "TestNormalizeTenderWriteRequestRejectsOversizedBusinessFields",
        "TestNormalizeTenderWriteRequestRejectsInvalidMetadataAndBudget",
        "TestNormalizeSourceWriteRequestTrimsIdentityAndDefaultsType",
        "TestCreateSourceRejectsOversizedIdentityBeforeDB",
    ):
        require(needle in tender_store_tests, f"Tender write request missing business input boundary regression test: {needle}")

    knowledge_page = (ROOT / "frontend/src/features/knowledge/index.tsx").read_text(encoding="utf-8")
    for forbidden in ("<Tag>坐标：", "`坐标: ${bboxText}`"):
        require(forbidden not in knowledge_page, f"File preview source UI exposes raw OCR coordinates: {forbidden}")
    require(
        "sanitizePreviewLocatorText(normalizePreviewParam" not in knowledge_page,
        "File preview source locator sanitizer collapses line boundaries before filtering",
    )
    for needle in ("原文位置：已定位", "sanitizePreviewLocatorText"):
        require(needle in knowledge_page, f"File preview source UI missing business locator guard: {needle}")
    require("normalizePreviewLocatorParam" in knowledge_page, "File preview source UI missing line-preserving locator normalization")
    require("document.error_message?.trim()" not in knowledge_page, "Knowledge document UI exposes raw task error message")
    require("getUserFacingErrorMessage(document.error_message" in knowledge_page, "Knowledge document UI missing user-facing error filter")

    cost_page = (ROOT / "frontend/src/features/cost/index.tsx").read_text(encoding="utf-8")
    require("if (errorMessage?.trim()) return errorMessage.trim()" not in cost_page, "Cost advice UI exposes raw task error message")
    require("getUserFacingErrorMessage(errorMessage" in cost_page, "Cost advice UI missing user-facing error filter")

    cost_store = (ROOT / "backend/internal/platform/cost/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxCostNameRunes",
        "maxCostShortTextRunes",
        "maxCostNoteRunes",
        "maxCostAmount",
        "maxCostExternalTaskIDRunes",
        "maxCostTaskResultJSONBytes",
        "maxCostTaskRouteJSONBytes",
        "validateCostTextLength",
        "boundedCostText",
        "marshalCostTaskJSON",
        "normalizeCostAdviceCallbackPayload",
        "normalizeAcceptedTask",
        "utf8.RuneCountInString",
        "value > maxCostAmount",
    ):
        require(needle in cost_store, f"Cost store missing business input boundary: {needle}")
    for forbidden in (
        "resultJSON, _ := json.Marshal(payload.Result)",
        "routeJSON, _ := json.Marshal(accepted.Route)",
    ):
        require(forbidden not in cost_store, f"Cost store still ignores AI task JSON marshal failure: {forbidden}")
    cost_store_tests = (ROOT / "backend/internal/platform/cost/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestCostProjectWriteRejectsOversizedNameBeforeDB",
        "TestBoundedCostTextTrimsGeneratedFallbackNames",
        "TestNormalizeItemRequestRejectsOversizedTextFields",
        "TestNormalizeItemRequestAcceptsBoundedUnicodeText",
        "TestCostAdviceCallbackRejectsInvalidResultBeforeDB",
        "TestCostAdviceCallbackRejectsOversizedResultBeforeDB",
        "TestNormalizeAcceptedTaskRejectsOversizedRoute",
    ):
        require(needle in cost_store_tests, f"Cost store missing business input boundary regression test: {needle}")

    project_store = (ROOT / "backend/internal/platform/project/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxProjectNameRunes",
        "maxProjectMilestoneTitleRunes",
        "maxProjectMilestoneNoteRunes",
        "maxProjectGeneratedFilenameRunes",
        "maxProjectKnowledgeMetadataBytes",
        "maxProjectLogMetadataBytes",
        "normalizeProjectName",
        "normalizeMilestoneWriteRequest",
        "marshalProjectMetadataJSON",
        "wonCaseDocumentMetadata",
        "validateProjectTextLength",
        "boundedProjectText",
        "utf8.RuneCountInString",
    ):
        require(needle in project_store, f"Project store missing business input boundary: {needle}")
    for forbidden in (
        "metadataJSON, _ := json.Marshal(metadata)",
        "chunkMetadataJSON, _ := json.Marshal(chunkMetadata)",
    ):
        require(forbidden not in project_store, f"Project store still ignores metadata marshal failure: {forbidden}")
    project_store_tests = (ROOT / "backend/internal/platform/project/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestProjectWriteRejectsOversizedNameBeforeDB",
        "TestNormalizeProjectNameAcceptsBoundedUnicodeText",
        "TestNormalizeMilestoneWriteRequestRejectsOversizedText",
        "TestNormalizeMilestoneWriteRequestAcceptsBoundedUnicodeText",
        "TestBoundedProjectTextTrimsGeneratedFallbackNames",
        "TestMarshalProjectMetadataJSONRejectsInvalidAndOversizedValues",
        "TestWonCaseDocumentMetadataCopiesAndOverridesSystemFields",
    ):
        require(needle in project_store_tests, f"Project store missing business input boundary regression test: {needle}")

    compliance_store = (ROOT / "backend/internal/platform/compliance/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxComplianceCheckNameRunes",
        "maxComplianceLevelSelections",
        "maxComplianceRuleCodeRunes",
        "maxComplianceRuleMetadataBytes",
        "maxComplianceCheckConfigBytes",
        "maxComplianceReportMetadataBytes",
        "maxComplianceIssueLocationBytes",
        "maxComplianceGateMetadataBytes",
        "normalizeCheckName",
        "normalizeRuleMetadata",
        "marshalComplianceJSON",
        "validateComplianceTextLength",
        "boundedComplianceText",
        "utf8.RuneCountInString",
    ):
        require(needle in compliance_store, f"Compliance store missing business input boundary: {needle}")
    for forbidden in (
        "configRaw, _ := json.Marshal(config)",
        "metadata, _ := json.Marshal(map[string]any{",
        "location, _ := json.Marshal(locationMap)",
        "metadataJSON, _ := json.Marshal(metadata)",
    ):
        require(forbidden not in compliance_store, f"Compliance store still ignores JSON marshal failure: {forbidden}")
    compliance_store_tests = (ROOT / "backend/internal/platform/compliance/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestNormalizeLevelsDedupesAndBoundsSelections",
        "TestCreateCheckRejectsOversizedNameAndLevelsBeforeDB",
        "TestNormalizeRuleRejectsOversizedTextAndMetadata",
        "TestNormalizeRuleTrimsMetadataKeysAndAcceptsBoundedUnicodeText",
        "TestNormalizeRuleMetadataRejectsTooManyEntries",
        "TestMarshalComplianceJSONRejectsInvalidAndOversizedValues",
        "TestBoundedComplianceTextTrimsGeneratedIssueText",
    ):
        require(needle in compliance_store_tests, f"Compliance store missing business input boundary regression test: {needle}")

    approval_store = (ROOT / "backend/internal/platform/approval/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxApprovalChainNameRunes",
        "maxApprovalSteps",
        "maxApprovalStepsJSONBytes",
        "maxApprovalDecisionCommentRunes",
        "marshalApprovalSteps",
        "normalizeDecisionComment",
        "validateChainStepActors",
        "validateApprovalTextLength",
        "boundedApprovalText",
        "utf8.RuneCountInString",
    ):
        require(needle in approval_store, f"Approval store missing business input boundary: {needle}")
    approval_store_tests = (ROOT / "backend/internal/platform/approval/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestCreateChainRejectsOversizedConfigBeforeDB",
        "TestNormalizeChainRejectsOversizedTextFields",
        "TestNormalizeChainAcceptsBoundedUnicodeText",
        "TestNormalizeDecisionCommentBoundsAndTrims",
        "TestMarshalApprovalStepsRejectsOversizedSnapshot",
        "TestBoundedApprovalTextTrimsGeneratedValues",
    ):
        require(needle in approval_store_tests, f"Approval store missing business input boundary regression test: {needle}")

    saas_store = (ROOT / "backend/internal/platform/saas/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxSaaSTenantNameRunes",
        "maxSaaSUserNameRunes",
        "maxSaaSEmailRunes",
        "maxSaaSPasswordBytes",
        "maxSaaSRoleCodesPerMember",
        "maxSaaSNotificationReadIDs",
        "normalizeEmail",
        "normalizePassword",
        "normalizeRoleCodes",
        "normalizeNotificationIDs",
        "validateSaaSTextLength",
        "utf8.RuneCountInString",
        "mail.ParseAddress",
    ):
        require(needle in saas_store, f"SaaS store missing account input boundary: {needle}")
    saas_store_tests = (ROOT / "backend/internal/platform/saas/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestRegisterRejectsOversizedIdentityBeforeDB",
        "TestNormalizeEmailLowercasesAndRejectsInvalidAddresses",
        "TestNormalizePasswordBoundsBcryptInput",
        "TestInviteMemberRejectsOversizedIdentityBeforeDB",
        "TestUpdateMemberRejectsInvalidRoleCodesBeforeDB",
        "TestNormalizeRoleCodesDedupesAndBoundsValues",
        "TestCreateAndUpdateRoleRejectOversizedFieldsBeforeDB",
        "TestMarkNotificationsReadRejectsInvalidIDsBeforeDB",
        "TestNormalizeNotificationIDsDedupesAndCanonicalizes",
    ):
        require(needle in saas_store_tests, f"SaaS store missing account input boundary regression test: {needle}")

    compliance_page = (ROOT / "frontend/src/features/compliance/index.tsx").read_text(encoding="utf-8")
    for forbidden in ("规则编号", "填写标书编号", "title: '编码'"):
        require(forbidden not in compliance_page, f"Compliance UI exposes technical field: {forbidden}")
    require("['L1', 'L2', 'L3', 'L4'].map((value) => ({ label: value, value }))" not in compliance_page, "Compliance level selector exposes raw level codes")
    require("render: (value) => <Tag>{value}</Tag>" not in compliance_page, "Compliance rule table exposes raw level codes")
    for needle in ("createComplianceRuleCode", "可选，选择需要检查的标书"):
        require(needle in compliance_page, f"Compliance UI missing user-facing workflow guard: {needle}")
    for needle in ("complianceLevelLabels", "complianceLevelLabel", "基础完整性", "响应一致性", "废标条款", "评分优化"):
        require(needle in compliance_page, f"Compliance UI missing business level label: {needle}")

    api_spec = (ROOT / "docs/blueprint/API_SPEC.md").read_text(encoding="utf-8")
    require("stateless JWT" not in api_spec, "API_SPEC.md still describes logout as stateless JWT")
    for needle in ("session_revoked_at", "未被撤销", "除 `/healthz` 和 `/models/health` 外均强制验签"):
        require(needle in api_spec, f"API_SPEC.md missing current auth/HMAC behavior: {needle}")

    ai_call_store = (ROOT / "backend/internal/platform/aicall/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxAIEstimatedCost",
        "maxAIBizRefJSONBytes",
        "maxAIBizRefExternalTaskIDRunes",
        "normalizeRecordBizRef",
        "math.Round(value*10000) / 10000",
        "value > maxAIEstimatedCost",
    ):
        require(needle in ai_call_store, f"AI call cost normalization missing DB-safe guard: {needle}")
    require(
        "bizRefJSON, _ := json.Marshal(input.BizRef)" not in ai_call_store,
        "AI call log still ignores biz_ref JSON marshal failure",
    )
    ai_call_tests = (ROOT / "backend/internal/platform/aicall/pricing_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestNormalizeRecordSanitizesOversizedExplicitCostAndFallsBackToPricing",
        "TestNormalizeRecordRoundsEstimatedCostToDatabaseScale",
        "TestEstimateCostRejectsOversizedComputedCost",
        "TestNormalizeRecordBizRefRejectsInvalidAndOversizedValues",
        "TestNormalizeRecordBizRefTrimsAndBoundsExternalTaskID",
    ):
        require(needle in ai_call_tests, f"AI call cost normalization missing regression test: {needle}")
    model_router = (ROOT / "ai-service/app/gateway/model_router.py").read_text(encoding="utf-8")
    for needle in ("MAX_AI_ESTIMATED_COST", "_estimated_cost_or_zero", "round(value, 4)"):
        require(needle in model_router, f"ModelRouter cost normalization missing quota guard: {needle}")
    model_router_tests = (ROOT / "ai-service/app/tests/test_model_router.py").read_text(encoding="utf-8")
    for needle in (
        "test_router_log_call_ignores_oversized_estimated_cost",
        "test_router_log_call_rounds_estimated_cost_to_audit_scale",
        "test_router_pricing_ignores_oversized_computed_cost",
    ):
        require(needle in model_router_tests, f"ModelRouter cost normalization missing regression test: {needle}")

    common_schema = (ROOT / "ai-service/app/schemas/common.py").read_text(encoding="utf-8")
    for needle in (
        "MAX_SOURCE_REF_ID_LENGTH",
        "MAX_SOURCE_REF_TITLE_LENGTH",
        "MAX_RESPONSE_METADATA_BYTES",
        "MAX_RESPONSE_TOKEN_USAGE_VALUE",
        "TaskAcceptedID",
        "SourceRefID",
        "bounded_json_object",
        "bounded_token_usage",
        "StringConstraints(strip_whitespace=True",
    ):
        require(needle in common_schema, f"Common AI schema missing response guard: {needle}")

    knowledge_schema = (ROOT / "ai-service/app/schemas/knowledge.py").read_text(encoding="utf-8")
    for needle in (
        "MAX_KNOWLEDGE_EMBEDDING_TEXT_LENGTH",
        "MAX_KNOWLEDGE_RERANK_QUERY_LENGTH",
        "MAX_KNOWLEDGE_RERANK_CONTENT_LENGTH",
        "MAX_KNOWLEDGE_OBJECT_KEY_LENGTH",
        "MAX_KNOWLEDGE_FILENAME_LENGTH",
        "KnowledgeObjectKey",
        "KnowledgeFilename",
        "KnowledgeCallbackURL",
        "KnowledgeEmbeddingText",
        "KnowledgeRerankQuery",
        "KnowledgeRerankContent",
        "StringConstraints(strip_whitespace=True",
    ):
        require(needle in knowledge_schema, f"Knowledge AI schema missing request size guard: {needle}")
    knowledge_schema_tests = (ROOT / "ai-service/app/tests/test_knowledge_schema.py").read_text(encoding="utf-8")
    for needle in (
        "test_knowledge_embedding_request_rejects_oversized_or_empty_text",
        "test_knowledge_rerank_request_rejects_oversized_query",
        "test_knowledge_rerank_document_rejects_oversized_fields",
        "test_knowledge_rerank_request_strips_bounded_text_fields",
        "test_knowledge_process_request_rejects_oversized_document_fields",
        "test_knowledge_process_request_rejects_blank_required_document_fields",
    ):
        require(needle in knowledge_schema_tests, f"Knowledge AI schema missing regression test: {needle}")
    knowledge_store = (ROOT / "backend/internal/platform/knowledge/store.go").read_text(encoding="utf-8")
    for needle in ("maxKnowledgeSearchQueryChars", "normalizeKnowledgeSearchQuery(req.Query)"):
        require(needle in knowledge_store, f"Knowledge search missing query size guard: {needle}")
    for needle in (
        "maxKnowledgeTemplateNameRunes",
        "maxKnowledgeTemplateContentJSONBytes",
        "normalizeDocumentTemplateRequest",
        "normalizeDocumentTemplateContent",
        "maxKnowledgeCategoryNameRunes",
        "maxKnowledgeTagNameRunes",
        "maxKnowledgeDocumentTagIDs",
        "normalizeKnowledgeCategoryInput",
        "normalizeKnowledgeTagInput",
        "normalizeKnowledgeDocumentUpdate",
        "validateKnowledgeDocumentReferences",
        "maxKnowledgeExternalTaskIDRunes",
        "maxKnowledgeTaskResultJSONBytes",
        "maxKnowledgeTaskRouteJSONBytes",
        "marshalKnowledgeTaskJSON",
        "normalizeKnowledgeCallbackPayload",
        "normalizeAcceptedKnowledgeTask",
        "validateKnowledgeTextLength",
    ):
        require(needle in knowledge_store, f"Knowledge template missing content size guard: {needle}")
    for forbidden in (
        "payloadJSON, _ := json.Marshal(payload)",
        "resultJSON, _ := json.Marshal(payload.Result)",
        "routeJSON, _ := json.Marshal(accepted.Route)",
    ):
        require(forbidden not in knowledge_store, f"Knowledge store still ignores AI task JSON marshal failure: {forbidden}")
    knowledge_store_tests = (ROOT / "backend/internal/platform/knowledge/store_test.go").read_text(encoding="utf-8")
    require(
        "TestNormalizeKnowledgeSearchQueryTrimsAndCapsRunes" in knowledge_store_tests,
        "Knowledge search query size guard missing regression test",
    )
    for needle in (
        "TestNormalizeDocumentTemplateRequestTrimsDefaultsAndBoundsContent",
        "TestNormalizeDocumentTemplateRequestRejectsOversizedFieldsAndContent",
        "TestCreateDocumentTemplateRejectsInvalidRequestBeforeDB",
        "TestNormalizeKnowledgeCategoryInputBoundsText",
        "TestNormalizeKnowledgeTagInputBoundsTextAndColor",
        "TestNormalizeKnowledgeDocumentUpdateBoundsFieldsAndIDs",
        "TestKnowledgeWriteMethodsRejectInvalidInputsBeforeDB",
        "TestKnowledgeCallbackRejectsInvalidResultBeforeDB",
        "TestKnowledgeCallbackRejectsOversizedResultBeforeDB",
        "TestNormalizeKnowledgeCallbackBoundsDocumentFields",
        "TestKnowledgeCallbackRejectsInvalidDoneChunksBeforeDB",
        "TestNormalizeAcceptedKnowledgeTaskRejectsOversizedRoute",
    ):
        require(needle in knowledge_store_tests, f"Knowledge template missing content boundary regression test: {needle}")

    generation_schema = (ROOT / "ai-service/app/schemas/generation.py").read_text(encoding="utf-8")
    for needle in (
        "MAX_TENDER_REQUIREMENTS",
        "MAX_REQUIREMENT_REFS",
        "MAX_RETRIEVED_KNOWLEDGE_REFS",
        "MAX_CHAPTER_ACTION_PLAIN_TEXT_LENGTH",
        "MAX_CHAPTER_ACTION_TIPTAP_JSON_BYTES",
        "MAX_GENERATION_RESPONSE_SOURCE_REFS",
        "MAX_GENERATION_RESPONSE_TIPTAP_JSON_BYTES",
        "MAX_GENERATION_RESPONSE_SELF_CHECK_BYTES",
        "MAX_GENERATION_RESPONSE_METADATA_BYTES",
        "GenerationResponseNote",
        "ChapterActionType",
        "StringConstraints(strip_whitespace=True",
        "current_tiptap_json_must_be_bounded",
        "tiptap_json_must_be_bounded",
        "token_usage_must_be_bounded",
    ):
        require(needle in generation_schema, f"Chapter generation schema missing request size guard: {needle}")
    generation_schema_tests = (ROOT / "ai-service/app/tests/test_generation_schema.py").read_text(encoding="utf-8")
    for needle in (
        "test_chapter_generate_request_rejects_oversized_lists",
        "test_chapter_generate_request_rejects_oversized_text_fields",
        "test_chapter_action_request_rejects_invalid_action_and_oversized_body",
        "test_chapter_action_request_rejects_oversized_tiptap_json",
        "test_chapter_requests_strip_bounded_text_fields",
        "test_chapter_generate_response_rejects_oversized_output_lists_and_text",
        "test_chapter_generate_response_rejects_oversized_json_outputs",
        "test_chapter_generate_response_rejects_invalid_refs_and_token_usage",
        "test_chapter_generate_response_strips_bounded_text_fields",
    ):
        require(needle in generation_schema_tests, f"Chapter generation schema missing regression test: {needle}")

    bid_store = (ROOT / "backend/internal/platform/bid/store.go").read_text(encoding="utf-8")
    for needle in (
        "maxBidExternalTaskIDRunes",
        "maxBidTaskPayloadJSONBytes",
        "maxBidTaskResultJSONBytes",
        "maxBidTaskRouteJSONBytes",
        "maxBidRequirementCoverageJSONBytes",
        "maxBidRequirementCoverageRefsJSONBytes",
        "maxBidMaterialSelectionJSONBytes",
        "maxBidChapterContentJSONBytes",
        "maxBidChapterSourceRefsJSONBytes",
        "maxBidChapterNeedsHumanInputJSONBytes",
        "maxBidChapterModelMetadataJSONBytes",
        "maxBidChapterTokenUsageJSONBytes",
        "maxBidKnowledgeReferenceMetadataBytes",
        "marshalBidTaskJSON",
        "marshalBidBusinessJSON",
        "marshalChapterGenerationJSON",
        "marshalChapterVersionJSON",
        "normalizeBidCallbackPayload",
        "validateBidTaskTextLength",
    ):
        require(needle in bid_store, f"Bid AI task boundary missing guard: {needle}")
    for forbidden in (
        "payloadJSON, _ := json.Marshal(requestPayload)",
        "payloadJSON, _ := json.Marshal(payload)",
        "resultJSON, _ := json.Marshal(payload.Result)",
        "routeJSON, _ := json.Marshal(accepted.Route)",
        "sourceRefsRaw, _ := json.Marshal(sourceRefs)",
        "metadataRaw, _ := json.Marshal(coverageMetadata)",
        "body, _ := json.Marshal(selectedRefs)",
        "contentJSON, _ := json.Marshal(generation.TiptapJSON)",
        "contentJSON, _ := json.Marshal(content)",
        "sourceRefsJSON, _ := json.Marshal(sourceRefs)",
        "needsHumanInputJSON, _ := json.Marshal(generation.NeedsHumanInput)",
        "contentJSON, _ := json.Marshal(chapter.Content)",
        "sourceRefsJSON, _ := json.Marshal(chapter.SourceRefs)",
        "needsHumanInputJSON, _ := json.Marshal(chapter.NeedsHumanInput)",
        "modelMetadataJSON, _ := json.Marshal(modelMetadata)",
        "tokenUsageJSON, _ := json.Marshal(tokenUsage)",
    ):
        require(forbidden not in bid_store, f"Bid store still ignores guarded JSON marshal failure: {forbidden}")
    bid_store_tests = (ROOT / "backend/internal/platform/bid/store_test.go").read_text(encoding="utf-8")
    for needle in (
        "TestNormalizeAcceptedTaskRejectsOversizedRoute",
        "TestBidCallbackRejectsInvalidResultBeforeDB",
        "TestBidCallbackRejectsOversizedResultBeforeDB",
        "TestMarshalBidBusinessJSONRejectsInvalidAndOversizedValues",
        "TestManualRequirementCoverageRejectsInvalidSourceRefsBeforeDB",
        "TestBatchRequirementCoverageRejectsOversizedMetadataBeforeDB",
        "TestMarshalChapterGenerationJSONRejectsInvalidAndOversizedState",
        "TestMarshalChapterGenerationJSONCarriesEnrichedSelfCheck",
        "TestMarshalChapterVersionJSONRejectsInvalidAndOversizedState",
        "TestNormalizeBidCallbackBoundsErrorMessage",
        "TestBindAcceptedTaskRejectsInvalidPayloadBeforeDB",
    ):
        require(needle in bid_store_tests, f"Bid AI task boundary missing regression test: {needle}")

    cost_schema = (ROOT / "ai-service/app/schemas/cost.py").read_text(encoding="utf-8")
    for needle in (
        "MAX_COST_CATEGORY_TOTALS",
        "MAX_COST_OVERRUN_ITEMS",
        "MAX_COST_RECOMMENDATIONS",
        "MAX_COST_AMOUNT",
        "MAX_COST_RESPONSE_SUMMARY_LENGTH",
        "MAX_COST_RESPONSE_TEXT_LENGTH",
        "MAX_COST_RESPONSE_ITEMS",
        "MAX_COST_RESPONSE_METADATA_BYTES",
        "CostAmount",
        "CostRecommendation",
        "CostResponseSummary",
        "CostResponseText",
        "StringConstraints(strip_whitespace=True",
        "allow_inf_nan=False",
        "token_usage_must_be_bounded",
    ):
        require(needle in cost_schema, f"Cost advice schema missing request size/cost guard: {needle}")
    cost_schema_tests = (ROOT / "ai-service/app/tests/test_cost_schema.py").read_text(encoding="utf-8")
    for needle in (
        "test_cost_advice_request_rejects_oversized_lists",
        "test_cost_advice_request_rejects_invalid_money_values",
        "test_cost_overrun_item_rejects_unbounded_text_and_invalid_enums",
        "test_cost_advice_request_strips_bounded_text_fields",
        "test_cost_advice_response_rejects_oversized_lists_and_text",
        "test_cost_advice_response_rejects_oversized_metadata_and_invalid_token_usage",
        "test_cost_advice_response_strips_bounded_text_fields",
    ):
        require(needle in cost_schema_tests, f"Cost advice schema missing regression test: {needle}")

    tender_schema = (ROOT / "ai-service/app/schemas/tender.py").read_text(encoding="utf-8")
    for needle in (
        "MAX_TENDER_OBJECT_KEY_LENGTH",
        "MAX_TENDER_FILENAME_LENGTH",
        "MAX_TENDER_CALLBACK_URL_LENGTH",
        "MAX_TENDER_RESPONSE_EVIDENCE",
        "MAX_TENDER_RESPONSE_REQUIREMENT_ITEMS",
        "MAX_TENDER_RESPONSE_JSON_BYTES",
        "MAX_TENDER_RESPONSE_BBOX_POINTS",
        "TenderObjectKey",
        "TenderFilename",
        "TenderCallbackURL",
        "TenderResponseText",
        "TenderResponseSourceText",
        "TenderResponseBBoxValue",
        "enhancement_error_must_be_bounded",
        "fields_must_be_bounded",
        "structured_dicts_must_be_bounded",
        "material_suggestions_must_be_bounded",
        "StringConstraints(strip_whitespace=True",
    ):
        require(needle in tender_schema, f"Tender parse schema missing document request guard: {needle}")
    tender_schema_tests = (ROOT / "ai-service/app/tests/test_tender_schema.py").read_text(encoding="utf-8")
    for needle in (
        "test_tender_parse_request_rejects_oversized_document_fields",
        "test_tender_parse_request_rejects_blank_required_fields",
        "test_tender_parse_request_strips_bounded_document_fields",
        "test_tender_parse_response_rejects_oversized_evidence_fields",
        "test_tender_parse_module_response_rejects_oversized_collections",
        "test_tender_parse_structured_response_rejects_oversized_requirements_and_metadata",
    ):
        require(needle in tender_schema_tests, f"Tender parse schema missing regression test: {needle}")
    ai_main = (ROOT / "ai-service/app/main.py").read_text(encoding="utf-8")
    for needle in (
        "TenderParseStructuredResult",
        "structured = tender_structured_result_for_callback(structured)",
        "def tender_structured_result_for_callback",
    ):
        require(needle in ai_main, f"Tender parse callback missing final response boundary: {needle}")
    ai_main_tests = (ROOT / "ai-service/app/tests/test_main_security.py").read_text(encoding="utf-8")
    require(
        "test_process_tender_parse_rejects_oversized_model_structured_result" in ai_main_tests,
        "Tender parse callback missing final response boundary regression test",
    )

    export_schema = (ROOT / "ai-service/app/schemas/export.py").read_text(encoding="utf-8")
    for needle in (
        "MAX_EXPORT_TOTAL_CHAPTERS",
        "MAX_EXPORT_TOTAL_CHAPTER_TEXT_LENGTH",
        "MAX_EXPORT_LAYOUT_CONTEXT_BYTES",
        "ExportChapterText",
        "ExportCallbackURL",
        "ExportType",
        "StringConstraints(strip_whitespace=True",
        "bounded_layout_context",
        "validate_export_budget",
    ):
        require(needle in export_schema, f"Document export schema missing request size guard: {needle}")
    export_schema_tests = (ROOT / "ai-service/app/tests/test_export_schema.py").read_text(encoding="utf-8")
    for needle in (
        "test_export_chapter_rejects_oversized_fields",
        "test_document_export_request_rejects_oversized_document_fields",
        "test_document_export_request_rejects_blank_required_fields",
        "test_document_export_request_rejects_oversized_chapter_budget",
        "test_export_layout_context_rejects_oversized_or_invalid_values",
        "test_document_export_request_strips_bounded_text_fields",
    ):
        require(needle in export_schema_tests, f"Document export schema missing regression test: {needle}")
    ai_main = (ROOT / "ai-service/app/main.py").read_text(encoding="utf-8")
    for needle in (
        "document_export_payload_for_endpoint",
        '"export_type" in payload.model_fields_set',
        "payload.export_type != export_type",
        "HTTPException(status_code=400",
        'detail="export_type must match export endpoint"',
        'payload.model_copy(update={"export_type": export_type})',
    ):
        require(needle in ai_main, f"Document export endpoint missing type consistency guard: {needle}")
    main_security_tests = (ROOT / "ai-service/app/tests/test_main_security.py").read_text(encoding="utf-8")
    for needle in (
        "test_document_export_endpoint_normalizes_omitted_export_type_to_path",
        "test_document_export_endpoint_rejects_explicit_export_type_mismatch",
    ):
        require(needle in main_security_tests, f"Document export endpoint missing consistency regression test: {needle}")

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

#!/usr/bin/env python3
"""Runtime acceptance checks for x.md items 1-38.

The script assumes the local Docker stack is running and validates the main SaaS
workflow through public HTTP APIs plus a small static check for the editor UI.
It creates timestamped data so repeated runs do not depend on previous state.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import time
import uuid
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[2]
API_BASE = os.getenv("ZBT_API_BASE", "http://127.0.0.1:5173/api/v1").rstrip("/")
BACKEND_BASE = os.getenv("ZBT_BACKEND_BASE", "http://127.0.0.1:8080").rstrip("/")
FRONTEND_BASE = os.getenv("ZBT_FRONTEND_BASE", "http://127.0.0.1:5173").rstrip("/")
AI_BASE = os.getenv("ZBT_AI_BASE", "http://127.0.0.1:8000").rstrip("/")
TENANT_ID = os.getenv("ZBT_ACCEPTANCE_TENANT_ID", "00000000-0000-4000-8000-000000000001")
OTHER_TENANT_ID = os.getenv("ZBT_ACCEPTANCE_OTHER_TENANT_ID", "00000000-0000-4000-8000-000000000002")
ADMIN_EMAIL = os.getenv("ZBT_ACCEPTANCE_EMAIL", "admin@zbt.local")
OTHER_EMAIL = os.getenv("ZBT_ACCEPTANCE_OTHER_EMAIL", "other@zbt.local")
PASSWORD = os.getenv("ZBT_ACCEPTANCE_PASSWORD", "demo-password")


class AcceptanceError(RuntimeError):
    pass


def request_bytes(
    method: str,
    url: str,
    token: str | None = None,
    payload: object | bytes | None = None,
    headers: dict[str, str] | None = None,
    expected: tuple[int, ...] = (200,),
    timeout: int = 30,
) -> tuple[int, bytes]:
    data: bytes | None
    request_headers = dict(headers or {})
    if payload is None:
        data = None
    elif isinstance(payload, bytes):
        data = payload
    else:
        data = json.dumps(payload).encode("utf-8")
        request_headers.setdefault("Content-Type", "application/json")
    if token:
        request_headers["Authorization"] = f"Bearer {token}"
    req = Request(url, data=data, headers=request_headers, method=method)
    try:
        with urlopen(req, timeout=timeout) as resp:
            body = resp.read()
            if resp.status not in expected:
                raise AcceptanceError(f"{method} {url} returned {resp.status}, expected {expected}: {body.decode('utf-8', 'replace')}")
            return resp.status, body
    except HTTPError as exc:
        body = exc.read()
        if exc.code in expected:
            return exc.code, body
        raise AcceptanceError(f"{method} {url} returned {exc.code}, expected {expected}: {body.decode('utf-8', 'replace')}") from exc
    except URLError as exc:
        raise AcceptanceError(f"{method} {url} failed: {exc}") from exc


def request_json(
    method: str,
    url: str,
    token: str | None = None,
    payload: object | None = None,
    headers: dict[str, str] | None = None,
    expected: tuple[int, ...] = (200,),
    timeout: int = 30,
) -> object:
    status, body = request_bytes(method, url, token=token, payload=payload, headers=headers, expected=expected, timeout=timeout)
    if status == 204 or not body:
        return {}
    try:
        return json.loads(body.decode("utf-8"))
    except json.JSONDecodeError as exc:
        raise AcceptanceError(f"{method} {url} returned non-JSON body: {body[:120].decode('utf-8', 'replace')}") from exc


def api(
    method: str,
    path: str,
    token: str | None = None,
    payload: object | None = None,
    headers: dict[str, str] | None = None,
    expected: tuple[int, ...] = (200,),
) -> object:
    return request_json(method, f"{API_BASE}{path}", token=token, payload=payload, headers=headers, expected=expected)


def ai(method: str, path: str, payload: object | None = None, expected: tuple[int, ...] = (200,)) -> object:
    return request_json(method, f"{AI_BASE}{path}", payload=payload, expected=expected)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AcceptanceError(message)


def ok(label: str, evidence: object) -> None:
    print(f"[ok] {label}: {evidence}")


def login(email: str = ADMIN_EMAIL, tenant_id: str = TENANT_ID) -> tuple[str, dict[str, Any]]:
    result = api("POST", "/auth/login", payload={"tenant_id": tenant_id, "email": email, "password": PASSWORD})
    require(isinstance(result, dict), "login response is not an object")
    token = result.get("access_token")
    session = result.get("session")
    require(isinstance(token, str) and token, f"login for {email} did not return access_token")
    require(isinstance(session, dict), f"login for {email} did not return session")
    return token, session


def poll_until(label: str, fetcher, predicate, timeout: int = 90, interval: float = 2.0) -> object:
    deadline = time.time() + timeout
    last: object = None
    while time.time() < deadline:
        last = fetcher()
        if predicate(last):
            return last
        time.sleep(interval)
    raise AcceptanceError(f"{label} did not finish within {timeout}s; last={last}")


def poll_ai_task(token: str, task_id: str, timeout: int = 90) -> dict[str, Any]:
    task = poll_until(
        f"ai task {task_id}",
        lambda: api("GET", f"/ai-tasks/{task_id}", token=token),
        lambda value: isinstance(value, dict) and value.get("status") in {"done", "failed"},
        timeout=timeout,
    )
    require(isinstance(task, dict), f"task {task_id} response is not an object")
    require(task.get("status") == "done", f"task {task_id} ended as {task.get('status')}: {task.get('error_message')}")
    return task


def poll_generation_job(token: str, job_id: str, timeout: int = 120) -> dict[str, Any]:
    detail = poll_until(
        f"generation job {job_id}",
        lambda: api("GET", f"/generation-jobs/{job_id}", token=token),
        lambda value: isinstance(value, dict) and isinstance(value.get("job"), dict) and value["job"].get("status") in {"done", "failed", "cancelled"},
        timeout=timeout,
    )
    require(isinstance(detail, dict) and isinstance(detail.get("job"), dict), f"generation job {job_id} response is invalid")
    require(detail["job"].get("status") == "done", f"generation job {job_id} ended as {detail['job'].get('status')}: {detail['job'].get('error_message')}")
    return detail


def poll_export(token: str, export_id: str, timeout: int = 120) -> dict[str, Any]:
    detail = poll_until(
        f"export {export_id}",
        lambda: api("GET", f"/bid-exports/{export_id}", token=token),
        lambda value: isinstance(value, dict) and isinstance(value.get("export"), dict) and value["export"].get("status") in {"done", "failed"},
        timeout=timeout,
    )
    require(isinstance(detail, dict) and isinstance(detail.get("export"), dict), f"export {export_id} response is invalid")
    require(detail["export"].get("status") == "done", f"export {export_id} ended as {detail['export'].get('status')}: {detail['export'].get('error_message')}")
    require(isinstance(detail.get("download"), dict) and detail["download"].get("url"), f"export {export_id} missing download URL")
    return detail


def upload_asset(token: str, filename: str, content: str, biz_type: str = "knowledge") -> dict[str, Any]:
    body = content.encode("utf-8")
    presign = api(
        "POST",
        "/files/presign-upload",
        token=token,
        payload={
            "filename": filename,
            "content_type": "text/plain; charset=utf-8",
            "size_bytes": len(body),
            "biz_type": biz_type,
        },
        expected=(201,),
    )
    require(isinstance(presign, dict) and isinstance(presign.get("file"), dict), f"presign response for {filename} is invalid")
    upload_url = presign.get("upload_url")
    method = str(presign.get("method") or "PUT")
    headers = presign.get("headers")
    require(isinstance(upload_url, str) and upload_url, f"presign response for {filename} missing upload_url")
    require(isinstance(headers, dict), f"presign response for {filename} missing headers")
    request_bytes(method, upload_url, payload=body, headers={str(k): str(v) for k, v in headers.items()}, expected=(200, 204))
    confirmed = api("POST", f"/files/{presign['file']['id']}/confirm", token=token)
    if isinstance(confirmed, dict) and isinstance(confirmed.get("file"), dict):
        return confirmed["file"]
    require(isinstance(confirmed, dict) and confirmed.get("id"), f"confirm response for {filename} is invalid")
    return confirmed


def check_runtime_stack() -> None:
    compose = subprocess.run(
        ["docker", "compose", "ps", "--services", "--filter", "status=running"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    require(compose.returncode == 0, f"docker compose ps failed: {compose.stderr.strip()}")
    running = set(compose.stdout.split())
    expected = {"frontend", "backend", "ai-service", "postgres", "redis", "minio"}
    require(expected.issubset(running), f"docker compose running services missing {sorted(expected - running)}")
    backend_health = request_json("GET", f"{BACKEND_BASE}/healthz")
    ai_health = request_json("GET", f"{AI_BASE}/healthz")
    _, frontend_body = request_bytes("GET", FRONTEND_BASE, expected=(200,))
    require(isinstance(backend_health, dict) and backend_health.get("status") == "ok", "backend health is not ok")
    require(isinstance(ai_health, dict) and ai_health.get("status") == "ok", "ai-service health is not ok")
    require(b"root" in frontend_body or b"<!doctype html" in frontend_body.lower(), "frontend did not return app html")
    ok("1 runtime stack", sorted(running))


def check_auth_tenant_rbac(stamp: str) -> tuple[str, dict[str, Any], str, dict[str, Any]]:
    token, session = login()
    require(session.get("tenant", {}).get("id") == TENANT_ID, "admin login returned wrong tenant")
    require(session.get("permissions", {}).get("team") == "full", "admin session missing team full permission")

    registered_email = f"xmd-core-admin-{stamp}@example.test"
    registered = api(
        "POST",
        "/auth/register",
        payload={
            "tenant_name": f"xmd-core-tenant-{stamp}",
            "admin_name": "XMD Core Admin",
            "email": registered_email,
            "password": PASSWORD,
        },
    )
    require(isinstance(registered, dict) and registered.get("access_token"), "register did not return access_token")
    require(registered.get("session", {}).get("tenant", {}).get("name") == f"xmd-core-tenant-{stamp}", "register did not create requested tenant")
    ok("2-3 register/login and create tenant", {"login_tenant": TENANT_ID, "registered_email": registered_email})

    role_code = f"xmd_core_{stamp.replace('-', '_')}"
    permissions = {
        "dashboard": "read",
        "tender": "read",
        "project": "read",
        "bid": "full",
        "knowledge": "read",
        "compliance": "read",
        "team": "none",
        "cost": "none",
    }
    created_role = api("POST", "/roles", token=token, payload={"code": role_code, "name": f"XMD Core {stamp}", "permissions": permissions}, expected=(201,))
    require(isinstance(created_role, dict) and created_role.get("id"), "create role did not return id")
    updated_permissions = dict(permissions)
    updated_permissions["compliance"] = "full"
    updated_role = api("PATCH", f"/roles/{created_role['id']}", token=token, payload={"name": f"XMD Core Updated {stamp}", "permissions": updated_permissions})
    require(isinstance(updated_role, dict) and updated_role.get("permissions", {}).get("compliance") == "full", "update role did not persist permissions")

    member_email = f"xmd-core-member-{stamp}@example.test"
    member = api("POST", "/tenant/members/invite", token=token, payload={"email": member_email, "name": "XMD Core Member", "role_code": role_code}, expected=(201,))
    require(isinstance(member, dict) and member.get("id") and member.get("user", {}).get("id"), "invite member did not return member and user")
    patched = api("PATCH", f"/tenant/members/{member['id']}", token=token, payload={"role_codes": [role_code], "status": "active"})
    require(isinstance(patched, dict) and any(role.get("code") == role_code for role in patched.get("roles", [])), "patch member did not attach custom role")
    member_token, member_session = login(member_email)
    require(member_session.get("permissions", {}).get("bid") == "full", "custom member missing bid full permission")
    require(member_session.get("permissions", {}).get("cost") == "none", "custom member should not have cost permission")
    ok("4-6 invite members, configure roles, role-based menus", {"member": member_email, "role": role_code})

    viewer_token, viewer_session = login("viewer@zbt.local")
    api("GET", "/cost-projects", token=viewer_token, expected=(403,))
    require(viewer_session.get("permissions", {}).get("cost") == "none", "viewer should have no cost permission")
    shell = (ROOT / "frontend/src/layouts/ShellLayout.tsx").read_text(encoding="utf-8")
    require("permissionAllows(permissions[item.module], 'read')" in shell, "ShellLayout does not filter menus by read permission")
    require("permissionAllows(permissions[child.module], 'read')" in shell, "ShellLayout does not filter child menus by read permission")
    ok("7 API permission enforcement", {"viewer": "GET /cost-projects -> 403"})

    tenant_bid = create_bid(token, f"xmd-core-tenant1-bid-{stamp}", "combined")
    other_token, _ = login(OTHER_EMAIL, OTHER_TENANT_ID)
    api("GET", f"/bids/{tenant_bid['id']}", token=other_token, expected=(404,))
    spoofed_admin_read = api("GET", f"/bids/{tenant_bid['id']}", token=token, headers={"X-Tenant-ID": OTHER_TENANT_ID})
    require(isinstance(spoofed_admin_read, dict) and spoofed_admin_read.get("id") == tenant_bid["id"], "spoofed tenant header overrode authenticated admin tenant")
    api("GET", f"/bids/{tenant_bid['id']}", token=other_token, headers={"X-Tenant-ID": TENANT_ID}, expected=(404,))
    ok("8 tenant isolation", {"tenant1_bid": tenant_bid["id"], "tenant2_get": 404, "spoofed_header": "session tenant wins"})
    return token, session, member_token, member


def check_dashboard(token: str) -> None:
    dashboard = api("GET", "/dashboard/summary", token=token)
    require(isinstance(dashboard, dict), "dashboard summary is not an object")
    require(isinstance(dashboard.get("stats"), dict), "dashboard summary missing stats")
    require(isinstance(dashboard.get("pending_approvals"), list), "dashboard summary missing pending approvals")
    require(isinstance(dashboard.get("notifications"), list), "dashboard summary missing notifications")
    ok("9 dashboard stats/todos/notifications", {
        "stats": sorted(dashboard["stats"].keys()),
        "pending_approvals": len(dashboard["pending_approvals"]),
        "notifications": len(dashboard["notifications"]),
    })


def create_bid(token: str, title: str, bid_type: str) -> dict[str, Any]:
    bid = api("POST", "/bids", token=token, payload={"title": title, "project_name": title, "bid_type": bid_type}, expected=(201,))
    require(isinstance(bid, dict) and bid.get("id") and bid.get("bid_type") == bid_type, f"create {bid_type} bid did not return expected bid")
    return bid


def check_tenders_projects(token: str, stamp: str, member: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any], dict[str, Any]]:
    source = api(
        "POST",
        "/tender-sources",
        token=token,
        payload={"name": f"xmd-core-source-{stamp}", "source_type": "website", "url": "http://ai-service:8000/healthz"},
        expected=(201,),
    )
    require(isinstance(source, dict) and source.get("id"), "create tender source did not return id")
    verified = api("POST", f"/tender-sources/{source['id']}/verify", token=token)
    require(isinstance(verified, dict) and verified.get("last_verify_status") == "ok", "tender source verify did not succeed")
    tender = api(
        "POST",
        "/tenders",
        token=token,
        payload={
            "source_id": source["id"],
            "title": f"xmd-core smart campus tender {stamp}",
            "purchaser": "Acceptance Purchaser",
            "region": "CA",
            "budget_amount": 1250000,
            "budget_text": "1.25M",
            "publish_date": "2026-06-11",
            "deadline": "2026-07-11",
            "status": "open",
            "match_score": 88,
            "summary": "Network, security, and deployment requirements.",
            "requirements": ["network design", "security compliance", "implementation plan"],
            "risk_flags": ["signature_required"],
            "source_url": "http://example.test/tenders/xmd-core",
            "metadata": {"stamp": stamp},
        },
        expected=(201,),
    )
    require(isinstance(tender, dict) and tender.get("id"), "create tender did not return id")
    searched = api("GET", f"/tenders?q={stamp}", token=token)
    require(isinstance(searched, dict) and any(item.get("id") == tender["id"] for item in searched.get("items", [])), "tender search did not find created tender")
    favorite = api("POST", f"/tenders/{tender['id']}/favorite", token=token)
    require(isinstance(favorite, dict) and favorite.get("favorite") is True, "favorite tender did not return favorite true")
    ok("10 tenders search/favorite/source config", {"tender": tender["id"], "source": source["id"]})

    project = api("POST", f"/tenders/{tender['id']}/create-project", token=token, expected=(201,))
    require(isinstance(project, dict) and project.get("id"), "create project from tender did not return id")
    ok("11 create project from tender", {"project": project["id"], "tender": tender["id"]})

    project_id = str(project["id"])
    for status in ("bidding", "compliance_review", "submitted"):
        project = api("POST", f"/projects/{project_id}/transition", token=token, payload={"status": status})
        require(isinstance(project, dict) and project.get("status") == status, f"project did not transition to {status}")
    ok("12 project board flow", {"project": project_id, "status": project["status"]})

    milestone = api(
        "POST",
        f"/projects/{project_id}/milestones",
        token=token,
        payload={"title": f"xmd-core milestone {stamp}", "status": "open", "due_date": "2026-07-01", "sort_order": 10},
        expected=(201,),
    )
    require(isinstance(milestone, dict) and milestone.get("id"), "create milestone did not return id")
    milestone = api(
        "PATCH",
        f"/projects/{project_id}/milestones/{milestone['id']}",
        token=token,
        payload={"title": f"xmd-core milestone {stamp}", "status": "done", "due_date": "2026-07-01", "sort_order": 10},
    )
    require(isinstance(milestone, dict) and milestone.get("status") == "done", "update milestone did not mark done")
    project_member = api(
        "POST",
        f"/projects/{project_id}/members",
        token=token,
        payload={"user_id": member["user"]["id"], "role": "bidder"},
        expected=(201,),
    )
    require(isinstance(project_member, dict) and project_member.get("role") == "bidder", "add project member failed")
    related_bid = create_bid(token, f"xmd-core-related-bid-{stamp}", "combined")
    patched_bid = api("PATCH", f"/bids/{related_bid['id']}", token=token, payload={"project_id": project_id})
    require(isinstance(patched_bid, dict) and patched_bid.get("project_id") == project_id, "related bid did not link to project")
    project_detail = api("GET", f"/projects/{project_id}", token=token)
    require(isinstance(project_detail, dict) and int(project_detail.get("milestone_count", 0)) >= 1, "project detail missing milestone count")
    require(int(project_detail.get("bid_count", 0)) >= 1, "project detail missing related bid count")
    ok("13 project detail milestones/members/related bids", {"project": project_id, "milestone": milestone["id"], "member": project_member["id"], "bid_count": project_detail["bid_count"]})
    return tender, project, related_bid


def check_bid_outline_flow(token: str, stamp: str) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, Any]], list[dict[str, Any]]]:
    combined = create_bid(token, f"xmd-core-combined-{stamp}", "combined")
    separated = create_bid(token, f"xmd-core-separated-{stamp}", "separated")
    separated_parts = api("GET", f"/bids/{separated['id']}/parts", token=token)
    parts = separated_parts.get("items") if isinstance(separated_parts, dict) else None
    require(isinstance(parts, list) and {"tech", "business"}.issubset({part.get("code") for part in parts}), "separated bid does not include tech and business parts")
    ok("14-16 create combined/separated bid with tech/business parts", {"combined": combined["id"], "separated": separated["id"], "parts": [part.get("code") for part in parts]})

    file_asset = upload_asset(
        token,
        f"xmd-core-tender-{stamp}.txt",
        "Tender requirements: network design, security compliance, project plan, official signatures.",
        "knowledge",
    )
    uploaded = api("POST", f"/bids/{combined['id']}/upload-tender-file", token=token, payload={"file_id": file_asset["id"]}, expected=(202,))
    require(
        isinstance(uploaded, dict)
        and uploaded.get("file", {}).get("file_asset_id") == file_asset["id"]
        and uploaded.get("file", {}).get("status") == "active"
        and uploaded.get("parse_result", {}).get("status") == "queued",
        "upload tender file did not attach active file and queued parse result",
    )
    parsed = api("POST", f"/bids/{combined['id']}/parse-tender", token=token, expected=(202,))
    require(isinstance(parsed, dict) and parsed.get("parse_result", {}).get("status") == "ready", "parse tender did not return ready parse result")
    confirmed = api("PUT", f"/bids/{combined['id']}/parse-result", token=token, payload={"structured_result": parsed["parse_result"]["structured_result"]})
    require(isinstance(confirmed, dict) and confirmed.get("status") == "confirmed", "confirm parse result did not return confirmed")
    outline = api("POST", f"/bids/{combined['id']}/outline/generate", token=token, expected=(202,))
    require(isinstance(outline, dict) and outline.get("task", {}).get("status") == "done", "outline generation did not complete")
    parts = outline.get("parts")
    chapters = outline.get("chapters")
    require(isinstance(parts, list) and parts, "outline generation returned no parts")
    require(isinstance(chapters, list) and chapters, "outline generation returned no chapters")
    first_part = parts[0]
    first_chapter = chapters[0]
    edited = api(
        "PUT",
        f"/bids/{combined['id']}/parts/{first_part['id']}/outline",
        token=token,
        payload={
            "chapters": [
                {"id": first_chapter["id"], "title": f"{first_chapter['title']} Acceptance", "plain_text": "Edited acceptance outline text.", "sort_order": first_chapter["sort_order"]},
                {"title": f"xmd-core extra chapter {stamp}", "plain_text": "Extra edited outline chapter.", "sort_order": int(first_chapter["sort_order"]) + 1},
            ]
        },
    )
    require(isinstance(edited, dict) and len(edited.get("chapters", [])) >= 2, "update part outline did not persist edited chapters")
    ok("17-21 tender upload/parse/confirm/outline/edit", {"bid": combined["id"], "file": file_asset["id"], "chapters": len(edited["chapters"])})
    separated_outline = api("POST", f"/bids/{separated['id']}/outline/generate", token=token, expected=(202,))
    require(isinstance(separated_outline, dict) and len(separated_outline.get("chapters", [])) >= 2, "separated bid outline did not create chapters")
    return combined, separated, edited["chapters"], separated_outline["chapters"]


def check_knowledge_and_generation(token: str, stamp: str, combined: dict[str, Any], chapters: list[dict[str, Any]]) -> dict[str, Any]:
    knowledge_file = upload_asset(
        token,
        f"xmd-core-knowledge-{stamp}.txt",
        "Network design uses redundant switches. Security compliance includes audit logs. Implementation plan has acceptance milestones.",
        "knowledge",
    )
    knowledge_doc = api("POST", "/knowledge/documents", token=token, payload={"file_id": knowledge_file["id"]}, expected=(201,))
    require(isinstance(knowledge_doc, dict) and knowledge_doc.get("id"), "create knowledge document did not return id")
    process = api("POST", f"/knowledge/documents/{knowledge_doc['id']}/process", token=token, expected=(202,))
    require(isinstance(process, dict) and process.get("id"), "process knowledge document did not return task id")
    poll_ai_task(token, str(process["id"]))
    processed_doc = api("GET", f"/knowledge/documents/{knowledge_doc['id']}", token=token)
    require(isinstance(processed_doc, dict) and processed_doc.get("parse_status") == "processed", "knowledge document was not processed")
    search = api("POST", "/knowledge/search", token=token, payload={"query": "network security compliance acceptance milestones", "limit": 5})
    items = search.get("items") if isinstance(search, dict) else None
    source_refs = search.get("source_refs") if isinstance(search, dict) else None
    require(isinstance(items, list) and items, "knowledge search did not return chunks")
    selected_ref = source_refs[0] if isinstance(source_refs, list) and source_refs else items[0].get("source_ref")
    require(isinstance(selected_ref, dict), "knowledge search did not return source ref")
    material = api("PUT", f"/bids/{combined['id']}/material-selection", token=token, payload={"selected_refs": [selected_ref], "notes": f"xmd-core {stamp}"})
    require(isinstance(material, dict) and len(material.get("selected_refs", [])) == 1, "material selection did not persist selected ref")
    ok("22-25 knowledge upload/process/chunk/search/select", {"document": knowledge_doc["id"], "selected_refs": len(material["selected_refs"])})

    chapter_id = str(chapters[0]["id"])
    generated = api("POST", f"/bids/{combined['id']}/generate", token=token, payload={"chapter_ids": [chapter_id]}, expected=(202,))
    require(isinstance(generated, dict) and isinstance(generated.get("job"), dict), "generate bid did not return job")
    poll_generation_job(token, str(generated["job"]["id"]))
    chapter_list = api("GET", f"/bids/{combined['id']}/chapters", token=token)
    all_chapters = chapter_list.get("items") if isinstance(chapter_list, dict) else None
    require(isinstance(all_chapters, list) and all_chapters, "chapter list after generation is empty")
    chapter = next((item for item in all_chapters if item.get("id") == chapter_id), None)
    require(isinstance(chapter, dict), "generated chapter not found")
    require(chapter.get("status") == "generated", f"chapter status after generation is {chapter.get('status')}")
    require(len(chapter.get("source_refs", [])) > 0, "generated chapter missing source_refs")
    require(len(chapter.get("needs_human_input", [])) > 0, "generated chapter missing needs_human_input")
    ok("26,29,30 generate chapters with source refs and human input flags", {"chapter": chapter_id, "source_refs": len(chapter["source_refs"]), "needs_human_input": len(chapter["needs_human_input"])})

    accepted = api("POST", f"/chapters/{chapter_id}/accept", token=token)
    require(isinstance(accepted, dict) and accepted.get("status") == "accepted", "accept chapter did not mark accepted")
    regenerated = api("POST", f"/chapters/{chapter_id}/regenerate", token=token, expected=(202,))
    require(isinstance(regenerated, dict) and regenerated.get("task", {}).get("id"), "regenerate chapter did not return task id")
    poll_ai_task(token, str(regenerated["task"]["id"]))
    manual = api(
        "PUT",
        f"/chapters/{chapter_id}/content",
        token=token,
        payload={
            "title": f"{chapter['title']} Manual",
            "content": {"type": "doc", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Manual acceptance edit with preserved references."}]}]},
            "plain_text": "Manual acceptance edit with preserved references.",
        },
    )
    require(isinstance(manual, dict) and manual.get("status") == "edited", "manual edit did not set edited status")
    versions = api("GET", f"/chapters/{chapter_id}/versions", token=token)
    require(isinstance(versions, dict) and len(versions.get("items", [])) >= 3, "chapter versions did not include generated/regenerated/manual versions")
    diff = api("GET", f"/chapters/{chapter_id}/diff", token=token)
    require(isinstance(diff, dict) and diff.get("previous") is not None, "chapter diff missing previous version")
    ok("27-28 chapter accept/regenerate/manual edit/diff/versions", {"chapter": chapter_id, "versions": len(versions["items"])})
    return manual


def check_editor_and_exports(token: str, combined: dict[str, Any], separated: dict[str, Any], generated_chapter: dict[str, Any]) -> None:
    _, editor_html = request_bytes("GET", f"{FRONTEND_BASE}/bids/{combined['id']}/editor?chapter={generated_chapter['id']}", expected=(200,))
    require(b"root" in editor_html or b"<!doctype html" in editor_html.lower(), "editor route did not return app html")
    editor_source = (ROOT / "frontend/src/features/bid/index.tsx").read_text(encoding="utf-8")
    for needle in ("章节大纲", "EditorContent", "AI 助手", "source_refs", "needs_human_input"):
        require(needle in editor_source, f"editor source missing {needle}")
    ok("31 three-column editor", {"path": f"/bids/{combined['id']}/editor?chapter={generated_chapter['id']}"})

    combined_export = api("POST", f"/bids/{combined['id']}/exports", token=token, payload={"export_type": "docx", "part_code": "combined_body"}, expected=(202,))
    require(isinstance(combined_export, dict) and combined_export.get("export", {}).get("id"), "combined docx export did not return export id")
    combined_done = poll_export(token, str(combined_export["export"]["id"]))
    ok("32 export combined docx", {"export": combined_done["export"]["id"], "filename": combined_done["export"]["filename"]})

    tech_generation = api("POST", f"/bids/{separated['id']}/generate", token=token, payload={"part_code": "tech"}, expected=(202,))
    business_generation = api("POST", f"/bids/{separated['id']}/generate", token=token, payload={"part_code": "business"}, expected=(202,))
    require(isinstance(tech_generation, dict) and isinstance(tech_generation.get("job"), dict), "tech generation did not return job")
    require(isinstance(business_generation, dict) and isinstance(business_generation.get("job"), dict), "business generation did not return job")
    poll_generation_job(token, str(tech_generation["job"]["id"]))
    poll_generation_job(token, str(business_generation["job"]["id"]))
    tech_export = api("POST", f"/bids/{separated['id']}/exports", token=token, payload={"export_type": "docx", "part_code": "tech"}, expected=(202,))
    business_export = api("POST", f"/bids/{separated['id']}/exports", token=token, payload={"export_type": "docx", "part_code": "business"}, expected=(202,))
    require(isinstance(tech_export, dict) and tech_export.get("export", {}).get("id"), "tech docx export did not return export id")
    require(isinstance(business_export, dict) and business_export.get("export", {}).get("id"), "business docx export did not return export id")
    tech_done = poll_export(token, str(tech_export["export"]["id"]))
    business_done = poll_export(token, str(business_export["export"]["id"]))
    ok("33 export tech/business docx", {"tech": tech_done["export"]["filename"], "business": business_done["export"]["filename"]})

    zip_export = api("POST", f"/bids/{separated['id']}/exports", token=token, payload={"export_type": "zip", "part_code": "all"}, expected=(202,))
    require(isinstance(zip_export, dict) and zip_export.get("export", {}).get("id"), "zip export did not return export id")
    zip_done = poll_export(token, str(zip_export["export"]["id"]))
    ok("34 package ZIP", {"zip": zip_done["export"]["filename"]})


def check_compliance(token: str, stamp: str, bid: dict[str, Any]) -> None:
    pass_rule = api(
        "POST",
        "/compliance/rules",
        token=token,
        payload={
            "code": f"xmd_core_pass_{stamp.replace('-', '_')}",
            "name": f"XMD Core Pass Rule {stamp}",
            "category": "acceptance",
            "level": "L4",
            "severity": "pass",
            "description": "Temporary acceptance rule to verify pass severity support.",
            "enabled": True,
            "metadata": {"stamp": stamp},
        },
        expected=(201,),
    )
    require(isinstance(pass_rule, dict) and pass_rule.get("severity") == "pass", "compliance rule API did not accept pass severity")
    rules = api("GET", "/compliance/rules", token=token)
    items = rules.get("items") if isinstance(rules, dict) else None
    severities = {item.get("severity") for item in items} if isinstance(items, list) else set()
    require({"pass", "warn", "fail_candidate", "fail"}.issubset(severities), f"compliance rules missing severities: {severities}")
    check = api(
        "POST",
        "/compliance/checks",
        token=token,
        payload={"name": f"xmd-core-compliance-{stamp}", "bid_document_id": bid["id"], "levels": ["L1", "L2", "L3", "L4"]},
        expected=(202,),
    )
    require(isinstance(check, dict) and isinstance(check.get("check"), dict), "create compliance check did not return snapshot")
    require(check["check"].get("status") == "done", "compliance check did not finish synchronously")
    issues = api("GET", f"/compliance/checks/{check['check']['id']}/issues", token=token)
    issue_items = issues.get("items") if isinstance(issues, dict) else None
    require(isinstance(issue_items, list) and issue_items, "compliance issues list is empty")
    issue_severities = {item.get("severity") for item in issue_items}
    require(issue_severities.issubset({"pass", "warn", "fail_candidate", "fail"}), f"unexpected issue severities: {issue_severities}")
    located = [item for item in issue_items if isinstance(item.get("location"), dict) and item["location"].get("path")]
    require(located, "compliance issues missing editor location")
    for issue in issue_items:
        require(issue.get("evidence") and issue.get("suggestion"), f"issue {issue.get('id')} missing evidence or suggestion")
    first_location = located[0]["location"]
    require(str(first_location.get("path", "")).startswith(f"/bids/{bid['id']}/editor"), "issue location path does not target bid editor")
    require(first_location.get("chapter_id"), "issue location missing chapter_id")
    ok("35-38 compliance check/status/evidence/suggestion/editor location", {"check": check["check"]["id"], "issues": len(issue_items), "location": first_location["path"]})


def main() -> int:
    stamp = time.strftime("%Y%m%d%H%M%S") + "-" + uuid.uuid4().hex[:8]
    check_runtime_stack()
    token, _session, _member_token, member = check_auth_tenant_rbac(stamp)
    check_dashboard(token)
    check_tenders_projects(token, stamp, member)
    combined, separated, combined_chapters, _separated_chapters = check_bid_outline_flow(token, stamp)
    generated_chapter = check_knowledge_and_generation(token, stamp, combined, combined_chapters)
    check_editor_and_exports(token, combined, separated, generated_chapter)
    check_compliance(token, stamp, combined)
    print("[ok] x.md items 1-38 acceptance complete")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except AcceptanceError as exc:
        print(f"[error] {exc}", file=sys.stderr)
        raise SystemExit(1)

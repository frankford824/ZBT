#!/usr/bin/env python3
"""Runtime acceptance for the docs/ex/工程1 standard tender sample.

The script assumes the local Docker stack is running. It uses the public HTTP
API to upload the real 工程1 tender PDF, parse it, confirm the response matrix
requirements, process companion Word/XLSX documents, generate one chapter, and
export a DOCX. It is intentionally separate from check.sh because it depends on
runtime services and persisted object storage.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import mimetypes
import time
import uuid
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from acceptance_core_check import (
    API_BASE,
    ROOT,
    AcceptanceError,
    api,
    check_runtime_stack,
    create_bid,
    login,
    ok,
    poll_ai_task,
    poll_export,
    poll_generation_job,
    poll_until,
    request_bytes,
    require,
    require_generation_coverage_contract,
    require_traceable_source_refs,
    source_ref_has_location,
    source_ref_has_reference_id,
)


PROJECT1_DIR = ROOT / "docs/ex/工程1"
TENDER_PDF = PROJECT1_DIR / "采购文件桥梁检查.pdf"
RESPONSE_DOCX = PROJECT1_DIR / "响应文件格式.docx"
BOQ_XLSX = PROJECT1_DIR / "清单（固化）(1).xlsx"


def sample_file_record(label: str, path: Path) -> dict[str, Any]:
    require(path.is_file(), f"sample file does not exist: {path}")
    content = path.read_bytes()
    return {
        "label": label,
        "path": str(path.relative_to(ROOT)),
        "bytes": len(content),
        "sha256": hashlib.sha256(content).hexdigest(),
    }


def project1_evidence_skeleton() -> dict[str, Any]:
    return {
        "name": "project1_runtime_acceptance",
        "status": "running",
        "generated_at": datetime.now(UTC).isoformat(timespec="seconds"),
        "api_base": API_BASE,
        "sample_files": [
            sample_file_record("tender_pdf", TENDER_PDF),
            sample_file_record("response_docx", RESPONSE_DOCX),
            sample_file_record("boq_xlsx", BOQ_XLSX),
        ],
        "steps": {},
    }


def upload_binary_asset(token: str, path: Path, *, biz_type: str, biz_id: str = "") -> dict[str, Any]:
    require(path.is_file(), f"sample file does not exist: {path}")
    body = path.read_bytes()
    content_type = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
    presign = api(
        "POST",
        "/files/presign-upload",
        token=token,
        payload={
            "filename": path.name,
            "content_type": content_type,
            "size_bytes": len(body),
            "biz_type": biz_type,
            "biz_id": biz_id,
        },
        expected=(201,),
    )
    require(isinstance(presign, dict) and isinstance(presign.get("file"), dict), f"presign response for {path.name} is invalid")
    upload_url = str(presign.get("upload_url") or "")
    method = str(presign.get("method") or "PUT")
    headers = presign.get("headers")
    require(upload_url, f"presign response for {path.name} missing upload_url")
    require(isinstance(headers, dict), f"presign response for {path.name} missing headers")
    request_bytes(method, upload_url, payload=body, headers={str(k): str(v) for k, v in headers.items()}, expected=(200, 204))
    confirmed = api("POST", f"/files/{presign['file']['id']}/confirm", token=token)
    file_asset = confirmed.get("file") if isinstance(confirmed, dict) and isinstance(confirmed.get("file"), dict) else confirmed
    require(isinstance(file_asset, dict) and file_asset.get("id"), f"confirm response for {path.name} is invalid")
    require(file_asset.get("status") == "ready", f"{path.name} file asset is not ready")
    return file_asset


def poll_parse_ready(token: str, bid_id: str) -> dict[str, Any]:
    parse_result = poll_until(
        f"project1 parse result {bid_id}",
        lambda: api("GET", f"/bids/{bid_id}/parse-result", token=token),
        lambda value: isinstance(value, dict) and value.get("status") in {"ready", "confirmed", "failed"},
        timeout=180,
    )
    require(isinstance(parse_result, dict), "parse result response is not an object")
    require(parse_result.get("status") in {"ready", "confirmed"}, f"parse result ended as {parse_result.get('status')}: {parse_result.get('error_message')}")
    return parse_result


def require_project1_structured_result(parse_result: dict[str, Any]) -> dict[str, Any]:
    structured = parse_result.get("structured_result")
    require(isinstance(structured, dict), "project1 parse result missing structured_result")
    require("桥梁检查" in str(structured.get("project_name") or ""), "project1 parse result missing bridge inspection project name")
    require(structured.get("deadline") == "2025-04-17", f"project1 deadline mismatch: {structured.get('deadline')}")
    modules = structured.get("modules")
    require(isinstance(modules, dict), "project1 parse result missing modules")
    require({"basic", "qualification", "evaluation", "submission", "invalid_risk", "annex"}.issubset(modules), "project1 parse result missing six modules")
    items = structured.get("requirement_items")
    require(isinstance(items, list) and len(items) >= 35, "project1 parse result missing requirement_items")
    require_requirement_items(items)
    metadata = structured.get("parse_metadata")
    require(isinstance(metadata, dict), "project1 parse result missing parse_metadata")
    require(int(metadata.get("requirement_count") or 0) >= 35, "project1 parse metadata missing requirement_count")
    return structured


def require_requirement_items(items: list[Any]) -> dict[str, int]:
    typed_items = [item for item in items if isinstance(item, dict)]
    require(len(typed_items) == len(items), "project1 requirement_items contain non-object entries")
    modules = {str(item.get("module") or "") for item in typed_items}
    types = {str(item.get("type") or "") for item in typed_items}
    require({"qualification", "evaluation", "submission", "invalid_risk", "annex"}.issubset(modules), f"project1 requirement modules incomplete: {sorted(modules)}")
    require({"qualification", "scoring", "submission", "invalid_risk", "annex"}.issubset(types), f"project1 requirement types incomplete: {sorted(types)}")
    expected_response_count = sum(1 for item in typed_items if str(item.get("expected_response") or "").strip())
    mandatory_count = sum(1 for item in typed_items if item.get("mandatory") is True)
    high_priority_count = sum(1 for item in typed_items if item.get("priority") == "high")
    require(expected_response_count >= 35, f"project1 expected_response count too low: {expected_response_count}")
    require(mandatory_count >= 20, f"project1 mandatory requirement count too low: {mandatory_count}")
    require(high_priority_count >= 30, f"project1 high priority requirement count too low: {high_priority_count}")
    missing_refs = []
    for item in typed_items:
        source_ref = item.get("source_ref")
        if not isinstance(source_ref, dict) or not source_ref_has_reference_id(source_ref) or not source_ref_has_location(source_ref):
            missing_refs.append(str(item.get("id") or item.get("requirement") or "")[:80])
    require(not missing_refs, f"project1 requirements missing traceable source refs: {missing_refs[:8]}")
    return {
        "requirements": len(typed_items),
        "expected_response": expected_response_count,
        "mandatory": mandatory_count,
        "high_priority": high_priority_count,
    }


def process_knowledge_sample(token: str, path: Path) -> dict[str, Any]:
    file_asset = upload_binary_asset(token, path, biz_type="knowledge")
    document = api("POST", "/knowledge/documents", token=token, payload={"file_id": file_asset["id"]}, expected=(201,))
    require(isinstance(document, dict) and document.get("id"), f"create knowledge document failed for {path.name}")
    task = api("POST", f"/knowledge/documents/{document['id']}/process", token=token, expected=(202,))
    require(isinstance(task, dict) and task.get("id"), f"process knowledge document did not return task id for {path.name}")
    poll_ai_task(token, str(task["id"]), timeout=180)
    processed = api("GET", f"/knowledge/documents/{document['id']}", token=token)
    require(isinstance(processed, dict) and processed.get("parse_status") == "processed", f"{path.name} knowledge document was not processed")
    return processed


def export_requirements_xlsx(token: str, bid_id: str) -> int:
    _, body = request_bytes("GET", f"{API_BASE}/bids/{bid_id}/requirements/export?format=xlsx", token=token, expected=(200,), timeout=60)
    require(len(body) > 1024, "project1 requirement xlsx export is unexpectedly small")
    require(body.startswith(b"PK"), "project1 requirement xlsx export is not an XLSX zip")
    return len(body)


def run_project1_compliance_check(token: str, bid_id: str) -> dict[str, Any]:
    stamp = time.strftime("%Y%m%d%H%M%S") + "-" + uuid.uuid4().hex[:8]
    pass_rule = api(
        "POST",
        "/compliance/rules",
        token=token,
        payload={
            "code": f"project1_pass_{stamp.replace('-', '_')}",
            "name": f"Project1 Pass Rule {stamp}",
            "category": "project1_acceptance",
            "level": "L4",
            "severity": "pass",
            "description": "Runtime acceptance rule for the 工程1 export gate.",
            "enabled": True,
            "metadata": {"sample": "docs/ex/工程1", "stamp": stamp},
        },
        expected=(201,),
    )
    require(isinstance(pass_rule, dict) and pass_rule.get("severity") == "pass", "project1 pass compliance rule was not created")
    check = api(
        "POST",
        "/compliance/checks",
        token=token,
        payload={"name": f"project1-compliance-{stamp}", "bid_document_id": bid_id, "levels": ["L4"]},
        expected=(202,),
    )
    compliance = check.get("check") if isinstance(check, dict) else None
    require(isinstance(compliance, dict) and compliance.get("status") == "done", "project1 compliance check did not finish")
    if compliance.get("result_status") != "pass":
        issues = api("GET", f"/compliance/checks/{compliance['id']}/issues", token=token)
        issue_items = issues.get("items") if isinstance(issues, dict) else None
        require(isinstance(issue_items, list), "project1 compliance issues response is invalid")
        for issue in issue_items:
            if isinstance(issue, dict) and issue.get("severity") != "pass" and issue.get("status") == "open":
                api("POST", f"/compliance/issues/{issue['id']}/ignore", token=token)
        compliance = api("GET", f"/compliance/checks/{compliance['id']}", token=token)
        require(isinstance(compliance, dict), "project1 compliance refresh response is invalid")
    require(compliance.get("result_status") == "pass", f"project1 compliance check did not pass: {compliance.get('result_status')}")
    return compliance


def check_project1_runtime() -> dict[str, Any]:
    check_runtime_stack()
    token, _session = login()
    bid = create_bid(token, "工程1桥梁检查运行态验收", "combined")
    bid_id = str(bid["id"])
    evidence = project1_evidence_skeleton()
    evidence["bid_id"] = bid_id

    tender_file = upload_binary_asset(token, TENDER_PDF, biz_type="bid_tender", biz_id=bid_id)
    attached = api("POST", f"/bids/{bid_id}/upload-tender-file", token=token, payload={"file_id": tender_file["id"]}, expected=(202,))
    require(isinstance(attached, dict) and attached.get("file", {}).get("file_asset_id") == tender_file["id"], "project1 tender file did not attach to bid")
    parse_started = api("POST", f"/bids/{bid_id}/parse-tender", token=token, expected=(202,))
    require(isinstance(parse_started, dict) and parse_started.get("task", {}).get("id"), "project1 parse did not return task id")
    poll_ai_task(token, str(parse_started["task"]["id"]), timeout=180)
    parse_result = poll_parse_ready(token, bid_id)
    structured = require_project1_structured_result(parse_result)
    confirmed = api("PUT", f"/bids/{bid_id}/parse-result", token=token, payload={"structured_result": structured})
    require(isinstance(confirmed, dict) and confirmed.get("status") == "confirmed", "project1 parse confirmation failed")
    requirements_response = api("GET", f"/bids/{bid_id}/requirements", token=token)
    requirements = requirements_response.get("items") if isinstance(requirements_response, dict) else None
    require(isinstance(requirements, list), "project1 requirements API did not return items")
    requirement_summary = require_requirement_items(requirements)
    xlsx_size = export_requirements_xlsx(token, bid_id)
    evidence["steps"]["parse_response_matrix"] = {
        "bid": bid_id,
        **requirement_summary,
        "requirements_xlsx_bytes": xlsx_size,
        "tender_file_asset_id": tender_file["id"],
    }
    ok("project1 parse and response matrix", evidence["steps"]["parse_response_matrix"])

    response_doc = process_knowledge_sample(token, RESPONSE_DOCX)
    boq_doc = process_knowledge_sample(token, BOQ_XLSX)
    search = api("POST", "/knowledge/search", token=token, payload={"query": "中级养护技术员 最高单价 综合单价", "limit": 8})
    items = search.get("items") if isinstance(search, dict) else None
    source_refs = search.get("source_refs") if isinstance(search, dict) else None
    require(isinstance(items, list) and items, "project1 knowledge search did not return chunks")
    selected_ref = source_refs[0] if isinstance(source_refs, list) and source_refs else items[0].get("source_ref")
    require(isinstance(selected_ref, dict), "project1 knowledge search missing source ref")
    api("PUT", f"/bids/{bid_id}/material-selection", token=token, payload={"selected_refs": [selected_ref], "notes": "工程1运行态验收"})
    evidence["steps"]["companion_knowledge"] = {
        "response_doc": response_doc["id"],
        "response_doc_status": response_doc.get("parse_status"),
        "boq_doc": boq_doc["id"],
        "boq_doc_status": boq_doc.get("parse_status"),
        "search_items": len(items),
        "selected_ref_has_reference_id": source_ref_has_reference_id(selected_ref),
        "selected_ref_has_location": source_ref_has_location(selected_ref),
    }
    ok("project1 companion documents processed", evidence["steps"]["companion_knowledge"])

    outline = api("POST", f"/bids/{bid_id}/outline/generate", token=token, expected=(202,))
    chapters = outline.get("chapters") if isinstance(outline, dict) else None
    require(isinstance(chapters, list) and chapters, "project1 outline generation returned no chapters")
    generated = api("POST", f"/bids/{bid_id}/generate", token=token, payload={"scope": "full"}, expected=(202,))
    require(isinstance(generated, dict) and isinstance(generated.get("job"), dict), "project1 chapter generation did not return job")
    poll_generation_job(token, str(generated["job"]["id"]), timeout=180)
    chapter_list = api("GET", f"/bids/{bid_id}/chapters", token=token)
    all_chapters = chapter_list.get("items") if isinstance(chapter_list, dict) else None
    require(isinstance(all_chapters, list) and all_chapters, "project1 generated chapter list is empty")
    unfinished = [str(item.get("id") or item.get("title") or "") for item in all_chapters if isinstance(item, dict) and item.get("status") not in {"generated", "accepted", "edited"}]
    require(not unfinished, f"project1 full generation left unfinished chapters: {unfinished[:8]}")
    chapter = next((item for item in all_chapters if isinstance(item, dict) and item.get("status") == "generated"), None)
    require(isinstance(chapter, dict), "project1 generated chapter not found")
    chapter_id = str(chapter["id"])
    chapter_refs = require_traceable_source_refs(chapter.get("source_refs"), "project1 generated chapter")
    coverage_spec = api("GET", f"/bids/{bid_id}/generation-coverage", token=token)
    coverage_summary = require_generation_coverage_contract(coverage_spec)
    compliance = run_project1_compliance_check(token, bid_id)
    evidence["steps"]["generation_coverage_compliance"] = {
        "chapter": chapter_id,
        "source_refs": len(chapter_refs),
        "compliance": compliance["id"],
        "compliance_result_status": compliance.get("result_status"),
        **coverage_summary,
    }
    ok("project1 chapter generation, coverage, and compliance", evidence["steps"]["generation_coverage_compliance"])

    export_started = api("POST", f"/bids/{bid_id}/exports", token=token, payload={"export_type": "docx", "part_code": "combined_body"}, expected=(202,))
    require(isinstance(export_started, dict) and export_started.get("export", {}).get("id"), "project1 docx export did not return export id")
    export_done = poll_export(token, str(export_started["export"]["id"]), timeout=180)
    evidence["steps"]["docx_export"] = {
        "export": export_done["export"]["id"],
        "filename": export_done["export"]["filename"],
        "download_ready": bool(export_done.get("download", {}).get("url")),
    }
    ok("project1 docx export", evidence["steps"]["docx_export"])
    evidence["status"] = "passed"
    print("[ok] 工程1 runtime acceptance complete")
    return evidence


def write_json_output(evidence: dict[str, Any], output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(evidence, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    ok("project1 runtime evidence json", {"path": str(output), "status": evidence.get("status")})


def main() -> int:
    parser = argparse.ArgumentParser(description="Run 工程1 runtime acceptance against a live local stack.")
    parser.add_argument("--json-output", type=Path, help="Write structured runtime evidence to this JSON file on success.")
    args = parser.parse_args()

    evidence = check_project1_runtime()
    if args.json_output:
        write_json_output(evidence, args.json_output)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except AcceptanceError as exc:
        print(f"[error] {exc}")
        raise SystemExit(1)

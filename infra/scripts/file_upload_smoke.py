#!/usr/bin/env python3
"""Exercise public signed PUT/GET, confirmation, attachment and tender parsing.

Only creates a dedicated smoke bid; never modifies a user's bid. Tiny fixture
files and the archived smoke bid remain as deployment evidence.
"""
from __future__ import annotations

import argparse
import io
import json
import os
import time
import urllib.parse
import urllib.request
import uuid
import zipfile
from xml.sax.saxutils import escape

from chapter_generation_smoke import api_call


def require_origin(url: str, origin: str) -> None:
    parsed = urllib.parse.urlsplit(url)
    expected = urllib.parse.urlsplit(origin)
    if (parsed.scheme, parsed.netloc) != (expected.scheme, expected.netloc):
        raise RuntimeError("signed file URL does not use the configured public web origin")
    if not parsed.path.startswith("/zbt-files/"):
        raise RuntimeError("signed file URL does not target the application bucket")


def make_tender_pdf(marker: str) -> bytes:
    """Small real text-layer PDF, requiring no dependency on the runner host."""
    lines = [f"Tender project: {marker}", "Buyer: ZBT Development",
             "Scope: File upload and tender parsing acceptance", "Budget: CNY 100000",
             "Required: Business license", "Delivery: 30 days",
             "Evaluation: Technical 40 points, Price 60 points"]
    stream = ("BT /F1 12 Tf 72 720 Td " + " 0 -18 Td ".join(
        "(" + line.replace("\\", "\\\\").replace("(", "\\(").replace(")", "\\)") + ") Tj"
        for line in lines) + " ET").encode("ascii")
    objects = [b"<< /Type /Catalog /Pages 2 0 R >>",
               b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
               b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
               b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
               f"<< /Length {len(stream)} >>\nstream\n".encode() + stream + b"\nendstream"]
    pdf = b"%PDF-1.4\n"
    offsets = [0]
    for number, obj in enumerate(objects, 1):
        offsets.append(len(pdf))
        pdf += f"{number} 0 obj\n".encode() + obj + b"\nendobj\n"
    xref = len(pdf)
    pdf += f"xref\n0 {len(offsets)}\n0000000000 65535 f \n".encode()
    pdf += b"".join(f"{offset:010d} 00000 n \n".encode() for offset in offsets[1:])
    pdf += f"trailer\n<< /Size {len(offsets)} /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n".encode()
    return pdf


def make_tender_docx(marker: str) -> bytes:
    lines = ["项目名称：城南雨水管道项目 " + marker,
             "发布日期：2026-10-03", "招标人：灰度测试采购单位", "预算金额：100万元",
             "采购范围：雨水管道改造及排水导流施工。",
             "投标截止时间：2026-11-15 09:30",
             "投标人须具备市政公用工程施工总承包三级及以上资质。",
             "安全生产许可证须在有效期内。",
             "评分办法：技术方案40分；类似业绩30分；报价30分。",
             "递交要求：投标文件须按要求签章并在截止时间前提交。",
             "附件格式：报价表。", "交付期限：合同生效后30天。",
             "末页否决条款：资格证明材料缺失的投标文件将被否决。GRAY-END-CLAUSE-20261003"]
    output = io.BytesIO()
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as doc:
        doc.writestr('[Content_Types].xml', '<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>')
        doc.writestr('_rels/.rels', '<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>')
        body = ''.join('<w:p><w:r><w:t>' + escape(line) + '</w:t></w:r></w:p>' for line in lines)
        doc.writestr('word/document.xml', '<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>' + body + '<w:sectPr/></w:body></w:document>')
    return output.getvalue()


def run_smoke(base_url: str, public_origin: str, password: str, timeout: int, *, archive: bool = True, bid_type: str = 'combined') -> dict:
    login = api_call(base_url, "/auth/login", method="POST",
                     body={"email": os.getenv("ZBT_SMOKE_EMAIL", "admin@zbt.local"), "password": password})
    token = str(login["access_token"])
    marker = "ZBT-upload-smoke-" + uuid.uuid4().hex[:12]
    bid = api_call(base_url, "/bids", method="POST", token=token,
                   body={"title": marker, "project_name": marker, "bid_type": bid_type})
    bid_id = str(bid["id"])
    if bid.get('bid_type') != bid_type or bid.get('project_name') != marker or not bid.get('project_id'):
        raise RuntimeError('selected bid type and project association did not persist')
    content = make_tender_docx(marker)
    upload = api_call(base_url, "/files/presign-upload", method="POST", token=token,
                      body={"filename": marker + ".docx", "content_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
                            "size_bytes": len(content), "biz_type": "bid_tender", "biz_id": bid_id})
    require_origin(upload["upload_url"], public_origin)
    request = urllib.request.Request(upload["upload_url"], data=content,
                                     headers=upload["headers"], method="PUT")
    # Never print signed URLs or credentials, including on an HTTP error.
    with urllib.request.urlopen(request, timeout=30) as response:
        if response.status != 200:
            raise RuntimeError("signed PUT did not return HTTP 200")
    file_id = str(upload["file"]["id"])
    confirmed = api_call(base_url, f"/files/{file_id}/confirm", method="POST", token=token)
    asset = confirmed.get("file", confirmed)
    if asset.get("status") != "ready" or asset.get("size_bytes") != len(content):
        raise RuntimeError("uploaded file was not confirmed with the expected size")
    download = api_call(base_url, f"/files/{file_id}/download-url", token=token)
    require_origin(download["url"], public_origin)
    with urllib.request.urlopen(download["url"], timeout=30) as response:
        if response.read() != content:
            raise RuntimeError("signed download bytes do not match uploaded bytes")
    attached = api_call(base_url, f"/bids/{bid_id}/upload-tender-file", method="POST",
                        body={"file_id": file_id}, token=token)
    if attached["file"]["file_asset_id"] != file_id:
        raise RuntimeError("tender attachment did not persist the uploaded file")
    started = api_call(base_url, f"/bids/{bid_id}/parse-tender", method="POST", token=token)
    task_id = str(started["task"]["id"])
    queued = api_call(base_url, f"/bids/{bid_id}/parse-result", token=token)
    if queued['status'] in {'queued', 'processing'} and any(
            queued['structured_result'].get(key) for key in ('deadline', 'qualification_requirements', 'scoring_points', 'requirement_items')):
        raise RuntimeError('in-progress interpretation exposed manufactured file facts')
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        task = api_call(base_url, f"/ai-tasks/{task_id}", token=token)
        if task["status"] == "done":
            metadata = (task.get("result") or {}).get("model_metadata") or {}
            calls = metadata.get("module_calls") or []
            if len(calls) != 6 or any(call.get("status") != "done" for call in calls):
                raise RuntimeError("not all six tender modules completed real model enhancement")
            parsed = api_call(base_url, f"/bids/{bid_id}/parse-result", token=token)
            if (parsed.get("file_asset_id") != file_id or parsed.get("status") != "ready"
                    or not parsed.get("structured_result")):
                raise RuntimeError("parse callback did not persist the uploaded file result")
            structured = parsed['structured_result']
            if structured.get('deadline') != '2026-11-15 09:30' or structured.get('bid_type') != bid_type:
                raise RuntimeError('interpretation changed deadline or selected layout')
            qualifications = json.dumps(structured.get('qualification_requirements', []), ensure_ascii=False)
            if '市政公用工程施工总承包三级' not in qualifications or '安全生产许可证' not in qualifications:
                raise RuntimeError('mandatory original qualifications were not retained')
            scores = json.dumps(structured.get('scoring_points', []), ensure_ascii=False)
            if not all(item in scores for item in ('技术方案', '40', '类似业绩', '30', '报价')):
                raise RuntimeError('original 40/30/30 scoring was not retained')
            requirements = json.dumps(structured.get('requirement_items', []), ensure_ascii=False)
            if 'GRAY-END-CLAUSE-20261003' not in requirements or '工程量清单' in requirements:
                raise RuntimeError('end clause lost or absent bill of quantities invented')
            if archive:
                api_call(base_url, f"/bids/{bid_id}", method="PATCH", token=token,
                         body={"status": "archived"})
            return {"status": "passed", "bid_id": bid_id, "file_id": file_id,
                    "task_id": task_id, "bytes": len(content), "parse_status": parsed["status"]}
        if task["status"] in {"failed", "cancelled"}:
            raise RuntimeError(f"tender parse failed: {task.get('error_message')}")
        time.sleep(2)
    raise RuntimeError("tender parsing did not complete within the configured timeout")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://47.114.51.41:8080/api/v1")
    parser.add_argument("--public-origin", default="http://47.114.51.41:8080")
    parser.add_argument("--password", default=os.getenv("ZBT_SMOKE_PASSWORD", "demo-password"))
    parser.add_argument("--timeout-seconds", type=int, default=1200)
    args = parser.parse_args()
    print(json.dumps(run_smoke(args.base_url, args.public_origin, args.password,
                              args.timeout_seconds), ensure_ascii=False))

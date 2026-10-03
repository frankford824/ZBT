#!/usr/bin/env python3
"""Verify trusted IP TLS, preserved registration/login, and real signed PUT/GET.

Uses only the dedicated release-smoke account and a new archived fixture bid.
Does not change user accounts, passwords, or actual project documents.
Never prints credentials, tokens, or signed URLs.
"""
from __future__ import annotations

import argparse
import json
import uuid
import urllib.request
from pathlib import Path

from chapter_generation_smoke import api_call
from file_upload_smoke import make_tender_pdf, require_origin


def verify(origin: str, credentials: dict) -> dict:
    base = origin + "/api/v1"
    for path in ("/", "/login", "/register"):
        with urllib.request.urlopen(origin + path, timeout=20) as response:
            if response.status != 200:
                raise RuntimeError("page is unavailable: " + path)
    login = api_call(base, "/auth/login", method="POST", body={
        "email": credentials["smoke_email"], "password": credentials["smoke_password"]})
    token = login["access_token"]
    marker = "ZBT-https-smoke-" + uuid.uuid4().hex[:12]
    bid = api_call(base, "/bids", method="POST", token=token,
                   body={"title": marker, "project_name": marker, "bid_type": "combined"})
    content = make_tender_pdf(marker)
    upload = api_call(base, "/files/presign-upload", method="POST", token=token,
                      body={"filename": marker + ".pdf", "content_type": "application/pdf",
                            "size_bytes": len(content), "biz_type": "bid_tender", "biz_id": bid["id"]})
    require_origin(upload["upload_url"], origin)
    request = urllib.request.Request(upload["upload_url"], data=content,
                                     headers=upload["headers"], method="PUT")
    with urllib.request.urlopen(request, timeout=30) as response:
        if response.status != 200:
            raise RuntimeError("signed upload failed")
    file_id = upload["file"]["id"]
    confirmed = api_call(base, f"/files/{file_id}/confirm", method="POST", token=token)
    if confirmed.get("file", confirmed).get("status") != "ready":
        raise RuntimeError("file confirmation did not persist")
    download = api_call(base, f"/files/{file_id}/download-url", token=token)
    require_origin(download["url"], origin)
    with urllib.request.urlopen(download["url"], timeout=30) as response:
        if response.read() != content:
            raise RuntimeError("signed download bytes differ")
    api_call(base, f"/bids/{bid['id']}", method="PATCH", token=token, body={"status": "archived"})
    return {"origin": origin, "status": "passed", "login": "passed",
            "registration_page": "available", "signed_put_get": "byte-identical",
            "fixture_bid_id": bid["id"], "file_id": file_id}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--origin", default="https://47.114.51.41")
    parser.add_argument("--credentials", type=Path, default=Path("/opt/zbt-private/access.json"))
    args = parser.parse_args()
    try:
        print(json.dumps(verify(args.origin.rstrip("/"), json.loads(args.credentials.read_text()))))
    except Exception as error:
        # urllib failures can include signed URLs: print the class, not secrets.
        raise SystemExit("HTTPS acceptance failed: " + type(error).__name__) from None

#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import time
import urllib.error
import urllib.request


DEFAULT_CHAPTER_ID = "52000000-0000-4000-8000-000000000001"


def api_call(
    base_url: str,
    path: str,
    *,
    method: str = "GET",
    body: dict[str, object] | None = None,
    token: str = "",
) -> dict[str, object]:
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    http_request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=data,
        headers=headers,
        method=method,
    )
    try:
        with urllib.request.urlopen(http_request, timeout=20) as response:
            payload = json.load(response)
    except urllib.error.HTTPError as exc:
        detail = exc.read(4096).decode(errors="replace")
        raise RuntimeError(f"{method} {path} returned HTTP {exc.code}: {detail}") from exc
    if not isinstance(payload, dict):
        raise RuntimeError(f"{method} {path} returned non-object JSON")
    return payload


def run_smoke(
    base_url: str,
    *,
    email: str,
    password: str,
    chapter_id: str,
    timeout_seconds: int,
) -> dict[str, object]:
    login = api_call(
        base_url,
        "/auth/login",
        method="POST",
        body={"email": email, "password": password},
    )
    token = str(login.get("access_token") or "")
    if not token:
        raise RuntimeError("login response is missing access_token")

    started = api_call(
        base_url,
        f"/chapters/{chapter_id}/regenerate",
        method="POST",
        body={},
        token=token,
    )
    task = started.get("task")
    if not isinstance(task, dict) or not task.get("id"):
        raise RuntimeError("chapter regeneration response is missing task.id")
    task_id = str(task["id"])
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        task = api_call(base_url, f"/ai-tasks/{task_id}", token=token)
        status = str(task.get("status") or "")
        if status == "done":
            result = task.get("result")
            if not isinstance(result, dict):
                raise RuntimeError("completed chapter task is missing result")
            tiptap = result.get("tiptap_json")
            content = tiptap.get("content") if isinstance(tiptap, dict) else None
            if not isinstance(tiptap, dict) or tiptap.get("type") != "doc":
                raise RuntimeError("completed chapter task has invalid TipTap document")
            if not isinstance(content, list) or not content:
                raise RuntimeError("completed chapter task has no TipTap content")
            metadata = result.get("model_metadata")
            return {
                "status": status,
                "task_id": task_id,
                "model": metadata.get("model") if isinstance(metadata, dict) else None,
                "content_nodes": len(content),
                "source_refs": len(result.get("source_refs") or []),
            }
        if status in {"failed", "cancelled"}:
            message = task.get("error_message") or "no error message"
            raise RuntimeError(f"chapter generation ended with {status}: {message}")
        time.sleep(2)
    raise RuntimeError(f"chapter generation did not finish within {timeout_seconds} seconds")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Smoke-test backend to AI chapter generation and callback."
    )
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/api/v1")
    parser.add_argument("--email", default="admin@zbt.local")
    parser.add_argument("--password", default=os.getenv("ZBT_SMOKE_PASSWORD", "demo-password"))
    parser.add_argument("--chapter-id", default=DEFAULT_CHAPTER_ID)
    parser.add_argument("--timeout-seconds", type=int, default=180)
    args = parser.parse_args()
    result = run_smoke(
        args.base_url,
        email=args.email,
        password=args.password,
        chapter_id=args.chapter_id,
        timeout_seconds=max(10, args.timeout_seconds),
    )
    print(json.dumps(result, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

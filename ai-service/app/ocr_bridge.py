from __future__ import annotations

import base64
import binascii
import hmac
import io
import json
import os
import urllib.error
import urllib.request

import fitz
from fastapi import FastAPI, Header, HTTPException
from pydantic import BaseModel, Field


app = FastAPI(title="ZBT OCR Bridge", version="0.1.0")


class OCRRequest(BaseModel):
    filename: str = Field(min_length=1, max_length=255)
    content_type: str = Field(min_length=1, max_length=255)
    content_base64: str = Field(min_length=1)
    provider: str = "http_ocr"
    mode: str = "http_json"
    options: dict[str, object] = Field(default_factory=dict)


def _positive_int(name: str, default: int) -> int:
    try:
        value = int(os.getenv(name, "").strip() or default)
    except ValueError:
        return default
    return value if value > 0 else default


def _configured() -> bool:
    return bool(os.getenv("OPENAI_API_KEY", "").strip() and os.getenv("OPENAI_BASE_URL", "").strip())


def _authorize(authorization: str) -> None:
    expected = os.getenv("OCR_BRIDGE_API_KEY", "").strip()
    if not expected:
        raise HTTPException(status_code=503, detail="OCR bridge API key is not configured")
    prefix = "Bearer "
    supplied = authorization[len(prefix) :].strip() if authorization.startswith(prefix) else ""
    if not supplied or not hmac.compare_digest(supplied, expected):
        raise HTTPException(status_code=401, detail="invalid OCR bridge credentials")


def _decode_content(encoded: str) -> bytes:
    try:
        content = base64.b64decode(encoded, validate=True)
    except (binascii.Error, ValueError) as exc:
        raise HTTPException(status_code=422, detail="content_base64 is invalid") from exc
    if len(content) > _positive_int("OCR_BRIDGE_MAX_BYTES", 20 * 1024 * 1024):
        raise HTTPException(status_code=413, detail="OCR input is too large")
    return content


def _image_payloads(content: bytes, content_type: str) -> tuple[list[tuple[str, bytes]], bool]:
    if content_type.lower() != "application/pdf":
        if not content_type.lower().startswith("image/"):
            raise HTTPException(status_code=415, detail="OCR bridge accepts images and PDF files")
        return [(content_type.lower(), content)], False

    max_pages = _positive_int("OCR_BRIDGE_MAX_PAGES", 20)
    try:
        document = fitz.open(stream=content, filetype="pdf")
    except Exception as exc:
        raise HTTPException(status_code=422, detail="PDF is invalid") from exc
    try:
        pages: list[tuple[str, bytes]] = []
        for page_index in range(min(document.page_count, max_pages)):
            pixmap = document[page_index].get_pixmap(matrix=fitz.Matrix(2, 2), alpha=False)
            pages.append(("image/png", pixmap.tobytes("png")))
        return pages, document.page_count > max_pages
    finally:
        document.close()


def _recognize_image(content_type: str, content: bytes) -> str:
    base_url = os.getenv("OPENAI_BASE_URL", "").strip().rstrip("/")
    api_key = os.getenv("OPENAI_API_KEY", "").strip()
    model = os.getenv("OCR_BRIDGE_MODEL", "qwen-vl-ocr").strip() or "qwen-vl-ocr"
    if not base_url or not api_key:
        raise HTTPException(status_code=503, detail="OCR model provider is not configured")
    image_url = f"data:{content_type};base64,{base64.b64encode(content).decode('ascii')}"
    body = json.dumps(
        {
            "model": model,
            "messages": [
                {
                    "role": "user",
                    "content": [
                        {"type": "image_url", "image_url": {"url": image_url}},
                        {
                            "type": "text",
                            "text": "Extract every visible character in reading order. Return plain text only.",
                        },
                    ],
                }
            ],
            "temperature": 0,
        },
        ensure_ascii=False,
    ).encode("utf-8")
    request = urllib.request.Request(
        base_url + "/chat/completions",
        data=body,
        method="POST",
        headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=_positive_int("OCR_BRIDGE_TIMEOUT_S", 120)) as response:
            payload = json.load(io.TextIOWrapper(response, encoding="utf-8"))
        text = payload["choices"][0]["message"]["content"]
    except (urllib.error.URLError, KeyError, IndexError, TypeError, ValueError) as exc:
        raise HTTPException(status_code=502, detail="OCR model request failed") from exc
    if not isinstance(text, str) or not text.strip():
        raise HTTPException(status_code=502, detail="OCR model returned empty text")
    return text.strip()


@app.get("/healthz")
def healthz() -> dict[str, object]:
    return {
        "service": "zbt-ocr-bridge",
        "status": "ok" if _configured() else "not_configured",
        "model": os.getenv("OCR_BRIDGE_MODEL", "qwen-vl-ocr"),
    }


@app.post("/parse")
def parse_ocr(payload: OCRRequest, authorization: str = Header(default="")) -> dict[str, object]:
    _authorize(authorization)
    content = _decode_content(payload.content_base64)
    images, truncated = _image_payloads(content, payload.content_type)
    if not images:
        return {"status": "done", "text": "", "pages": [], "provider_metadata": {"page_count": 0}}
    pages = [
        {"page": index, "text": _recognize_image(content_type, image)}
        for index, (content_type, image) in enumerate(images, start=1)
    ]
    return {
        "status": "done",
        "text": "\n\n".join(page["text"] for page in pages),
        "pages": pages,
        "provider_metadata": {
            "model": os.getenv("OCR_BRIDGE_MODEL", "qwen-vl-ocr"),
            "page_count": len(pages),
            "truncated_after_page_limit": truncated,
        },
    }

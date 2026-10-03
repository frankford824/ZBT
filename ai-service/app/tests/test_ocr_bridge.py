from __future__ import annotations

import base64

import pytest
from fastapi import HTTPException

from app.ocr_bridge import _authorize, _decode_content, _image_payloads


def test_ocr_pdf_limit_rejects_instead_of_silently_truncating(monkeypatch):
    import fitz

    monkeypatch.setenv("OCR_BRIDGE_MAX_PAGES", "20")
    document = fitz.open()
    for _ in range(21):
        document.new_page(width=72, height=72)
    with pytest.raises(HTTPException) as exc:
        _image_payloads(document.tobytes(), "application/pdf")
    assert exc.value.status_code == 413


def test_truncated_ocr_response_cannot_be_accepted_as_complete():
    from app.pipelines.parse.document_parser import _merge_ocr_parse_metadata

    with pytest.raises(ValueError, match="不完整"):
        _merge_ocr_parse_metadata({}, {"provider_metadata": {"truncated_after_page_limit": True}})


def test_authorize_requires_matching_bearer_token(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("OCR_BRIDGE_API_KEY", "bridge-secret")
    _authorize("Bearer bridge-secret")
    with pytest.raises(HTTPException) as exc:
        _authorize("Bearer wrong")
    assert exc.value.status_code == 401


def test_decode_content_rejects_invalid_and_oversized_payload(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("OCR_BRIDGE_MAX_BYTES", "4")
    assert _decode_content(base64.b64encode(b"test").decode()) == b"test"
    with pytest.raises(HTTPException) as invalid:
        _decode_content("not-base64")
    assert invalid.value.status_code == 422
    with pytest.raises(HTTPException) as oversized:
        _decode_content(base64.b64encode(b"large").decode())
    assert oversized.value.status_code == 413

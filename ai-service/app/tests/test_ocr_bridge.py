from __future__ import annotations

import base64

import pytest
from fastapi import HTTPException

from app.ocr_bridge import _authorize, _decode_content


def test_authorize_requires_matching_bearer_token(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("OCR_BRIDGE_API_KEY", "bridge-secret")
    _authorize("Bearer bridge-secret")
    with pytest.raises(HTTPException) as exc:
        _authorize("Bearer wrong")
    assert exc.value.status_code == 401


def test_decode_content_rejects_invalid_and_oversized_payload(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("OCR_BRIDGE_MAX_BYTES", "4")
    assert _decode_content(base64.b64encode(b"test").decode()) == b"test"
    with pytest.raises(HTTPException) as invalid:
        _decode_content("not-base64")
    assert invalid.value.status_code == 422
    with pytest.raises(HTTPException) as oversized:
        _decode_content(base64.b64encode(b"large").decode())
    assert oversized.value.status_code == 413

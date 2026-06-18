from __future__ import annotations

import json
from datetime import datetime
from typing import Annotated

from pydantic import BaseModel, Field, StringConstraints

MAX_SOURCE_REF_ID_LENGTH = 128
MAX_SOURCE_REF_TITLE_LENGTH = 255
MAX_SOURCE_REF_PAGE = 100_000
MAX_RESPONSE_METADATA_BYTES = 32 * 1024
MAX_RESPONSE_TOKEN_USAGE_KEYS = 20
MAX_RESPONSE_TOKEN_USAGE_KEY_LENGTH = 64
MAX_RESPONSE_TOKEN_USAGE_VALUE = 1_000_000_000

SourceRefID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_SOURCE_REF_ID_LENGTH),
]
SourceRefTitle = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_SOURCE_REF_TITLE_LENGTH),
]
TaskAcceptedID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=128),
]
TaskAcceptedStatus = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=32),
]


class HealthResponse(BaseModel):
    service: str
    status: str
    time: datetime


class TaskAccepted(BaseModel):
    task_id: TaskAcceptedID
    status: TaskAcceptedStatus = "queued"
    route: dict[str, object] = Field(default_factory=dict)


class SourceRef(BaseModel):
    chunk_id: SourceRefID
    document_id: SourceRefID
    title: SourceRefTitle
    page_start: int | None = Field(default=None, ge=1, le=MAX_SOURCE_REF_PAGE)
    page_end: int | None = Field(default=None, ge=1, le=MAX_SOURCE_REF_PAGE)


def bounded_json_object(value: dict[str, object], *, max_bytes: int, label: str) -> dict[str, object]:
    try:
        encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False)
    except (TypeError, ValueError) as exc:
        raise ValueError(f"{label} contains non-serializable values") from exc
    if len(encoded.encode("utf-8")) > max_bytes:
        raise ValueError(f"{label} is too large")
    return value


def bounded_token_usage(value: dict[str, int]) -> dict[str, int]:
    if len(value) > MAX_RESPONSE_TOKEN_USAGE_KEYS:
        raise ValueError("token_usage has too many items")
    normalized: dict[str, int] = {}
    for raw_key, raw_value in value.items():
        key = str(raw_key).strip()
        if not key or len(key) > MAX_RESPONSE_TOKEN_USAGE_KEY_LENGTH:
            raise ValueError("token_usage key is invalid")
        if isinstance(raw_value, bool):
            raise ValueError("token_usage value must be a non-negative integer")
        count = int(raw_value)
        if count < 0 or count > MAX_RESPONSE_TOKEN_USAGE_VALUE:
            raise ValueError("token_usage value is out of range")
        normalized[key] = count
    return normalized

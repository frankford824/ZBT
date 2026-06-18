from __future__ import annotations

from typing import Annotated, Literal

from pydantic import BaseModel, Field, StringConstraints, field_validator

from app.schemas.common import bounded_json_object

MAX_TENDER_TASK_ID_LENGTH = 128
MAX_TENDER_TENANT_ID_LENGTH = 128
MAX_TENDER_ENTITY_ID_LENGTH = 128
MAX_TENDER_TITLE_LENGTH = 255
MAX_TENDER_OBJECT_KEY_LENGTH = 1024
MAX_TENDER_FILENAME_LENGTH = 255
MAX_TENDER_CONTENT_TYPE_LENGTH = 255
MAX_TENDER_CALLBACK_URL_LENGTH = 2048
MAX_TENDER_RESPONSE_SHORT_TEXT_LENGTH = 255
MAX_TENDER_RESPONSE_TEXT_LENGTH = 2000
MAX_TENDER_RESPONSE_SOURCE_TEXT_LENGTH = 2000
MAX_TENDER_RESPONSE_EVIDENCE = 500
MAX_TENDER_RESPONSE_REQUIREMENT_ITEMS = 300
MAX_TENDER_RESPONSE_WARNINGS = 50
MAX_TENDER_RESPONSE_FIELDS_BYTES = 128 * 1024
MAX_TENDER_RESPONSE_JSON_BYTES = 512 * 1024
MAX_TENDER_RESPONSE_LIST_ITEMS = 100
MAX_TENDER_RESPONSE_MODULES = 6
MAX_TENDER_RESPONSE_BBOX_POINTS = 8
MAX_TENDER_RESPONSE_PAGE = 100_000

TenderTaskID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_TENDER_TASK_ID_LENGTH),
]
TenderTenantID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_TENANT_ID_LENGTH),
]
TenderEntityID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_ENTITY_ID_LENGTH),
]
TenderOptionalEntityID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_TENDER_ENTITY_ID_LENGTH),
]
TenderTitle = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_TENDER_TITLE_LENGTH),
]
TenderObjectKey = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_OBJECT_KEY_LENGTH),
]
TenderFilename = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_FILENAME_LENGTH),
]
TenderContentType = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_CONTENT_TYPE_LENGTH),
]
TenderCallbackURL = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_CALLBACK_URL_LENGTH),
]
TenderResponseShortText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_RESPONSE_SHORT_TEXT_LENGTH),
]
TenderResponseOptionalText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_TENDER_RESPONSE_TEXT_LENGTH),
]
TenderResponseText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_TENDER_RESPONSE_TEXT_LENGTH),
]
TenderResponseSourceText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_TENDER_RESPONSE_SOURCE_TEXT_LENGTH),
]
TenderResponseBBoxValue = Annotated[float, Field(allow_inf_nan=False)]


TenderParseModule = Literal[
    "basic",
    "qualification",
    "evaluation",
    "submission",
    "invalid_risk",
    "annex",
]


class TenderParseRequest(BaseModel):
    task_id: TenderTaskID | None = None
    tenant_id: TenderTenantID
    bid_id: TenderOptionalEntityID | None = None
    bid_title: TenderTitle | None = None
    file_id: TenderEntityID
    object_key: TenderObjectKey
    filename: TenderFilename
    content_type: TenderContentType
    callback_url: TenderCallbackURL | None = None


class TenderParseFieldEvidence(BaseModel):
    field: TenderResponseShortText
    value: object | None = None
    confidence: float = Field(default=0.0, ge=0.0, le=1.0)
    source_text: TenderResponseSourceText = ""
    citation_id: TenderResponseShortText | None = None
    reference_id: TenderResponseShortText | None = None
    source_kind: TenderResponseShortText = "tender_document"
    document_id: TenderOptionalEntityID | None = None
    file_id: TenderOptionalEntityID | None = None
    filename: TenderTitle | None = None
    page_start: int | None = Field(default=None, ge=1, le=MAX_TENDER_RESPONSE_PAGE)
    page_end: int | None = Field(default=None, ge=1, le=MAX_TENDER_RESPONSE_PAGE)
    bbox: list[TenderResponseBBoxValue] | None = Field(
        default=None,
        max_length=MAX_TENDER_RESPONSE_BBOX_POINTS,
    )
    chunk_id: TenderOptionalEntityID | None = None
    traceable: bool = False
    needs_review: bool = False


class TenderRequirementItem(BaseModel):
    id: TenderResponseShortText
    module: TenderParseModule
    type: TenderResponseShortText
    requirement: TenderResponseText
    priority: Literal["high", "medium", "low"] = "medium"
    mandatory: bool = False
    score: float | None = Field(default=None, ge=0, le=100, allow_inf_nan=False)
    expected_response: TenderResponseOptionalText = ""
    status: Literal["unmapped", "planned", "covered", "needs_review"] = "unmapped"
    source_ref: TenderParseFieldEvidence | None = None
    needs_review: bool = False


class TenderParseModuleResult(BaseModel):
    module: TenderParseModule
    title: TenderResponseShortText
    status: Literal["done", "needs_review", "empty"] = "done"
    fields: dict[str, object] = Field(default_factory=dict)
    enhancement_error: dict[str, object] = Field(default_factory=dict)
    evidence: list[TenderParseFieldEvidence] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_EVIDENCE,
    )
    requirement_items: list[TenderRequirementItem] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_REQUIREMENT_ITEMS,
    )
    warnings: list[TenderResponseOptionalText] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_WARNINGS,
    )

    @field_validator("fields")
    @classmethod
    def fields_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        return _bounded_tender_json(value, MAX_TENDER_RESPONSE_FIELDS_BYTES, "module fields")

    @field_validator("enhancement_error")
    @classmethod
    def enhancement_error_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        return _bounded_tender_json(
            value,
            MAX_TENDER_RESPONSE_FIELDS_BYTES,
            "module enhancement error",
        )


class TenderParseStructuredResult(BaseModel):
    project_name: TenderTitle
    bid_type: Literal["combined", "separated", "custom"]
    source_file: dict[str, object]
    deadline: TenderResponseShortText | None = None
    qualification_requirements: list[TenderResponseText] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_LIST_ITEMS,
    )
    invalid_clause_risks: list[TenderResponseText] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_LIST_ITEMS,
    )
    scoring_points: list[TenderResponseText] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_LIST_ITEMS,
    )
    outline: dict[str, object] = Field(default_factory=dict)
    material_suggestions: list[dict[str, object]] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_LIST_ITEMS,
    )
    modules: dict[TenderParseModule, TenderParseModuleResult] = Field(
        default_factory=dict,
        max_length=MAX_TENDER_RESPONSE_MODULES,
    )
    field_evidence: list[TenderParseFieldEvidence] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_EVIDENCE,
    )
    requirement_items: list[TenderRequirementItem] = Field(
        default_factory=list,
        max_length=MAX_TENDER_RESPONSE_REQUIREMENT_ITEMS,
    )
    quality_gates: dict[str, object] = Field(default_factory=dict)
    parse_metadata: dict[str, object] = Field(default_factory=dict)

    @field_validator("source_file", "outline", "quality_gates", "parse_metadata")
    @classmethod
    def structured_dicts_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        return _bounded_tender_json(value, MAX_TENDER_RESPONSE_JSON_BYTES, "tender parse result")

    @field_validator("material_suggestions")
    @classmethod
    def material_suggestions_must_be_bounded(
        cls,
        value: list[dict[str, object]],
    ) -> list[dict[str, object]]:
        for item in value:
            _bounded_tender_json(item, MAX_TENDER_RESPONSE_FIELDS_BYTES, "material suggestion")
        return value


def _bounded_tender_json(value: dict[str, object], max_bytes: int, label: str) -> dict[str, object]:
    return bounded_json_object(value, max_bytes=max_bytes, label=label)

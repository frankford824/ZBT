from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


TenderParseModule = Literal[
    "basic",
    "qualification",
    "evaluation",
    "submission",
    "invalid_risk",
    "annex",
]


class TenderParseRequest(BaseModel):
    task_id: str | None = None
    tenant_id: str
    bid_id: str | None = None
    bid_title: str | None = None
    file_id: str
    object_key: str
    filename: str
    content_type: str
    callback_url: str | None = None


class TenderParseFieldEvidence(BaseModel):
    field: str
    value: object | None = None
    confidence: float = Field(default=0.0, ge=0.0, le=1.0)
    source_text: str = ""
    citation_id: str | None = None
    reference_id: str | None = None
    source_kind: str = "tender_document"
    document_id: str | None = None
    file_id: str | None = None
    filename: str | None = None
    page_start: int | None = None
    page_end: int | None = None
    bbox: list[float] | None = None
    chunk_id: str | None = None
    traceable: bool = False
    needs_review: bool = False


class TenderRequirementItem(BaseModel):
    id: str
    module: TenderParseModule
    type: str
    requirement: str
    priority: Literal["high", "medium", "low"] = "medium"
    mandatory: bool = False
    score: float | None = None
    expected_response: str = ""
    status: Literal["unmapped", "planned", "covered", "needs_review"] = "unmapped"
    source_ref: TenderParseFieldEvidence | None = None
    needs_review: bool = False


class TenderParseModuleResult(BaseModel):
    module: TenderParseModule
    title: str
    status: Literal["done", "needs_review", "empty"] = "done"
    fields: dict[str, object] = Field(default_factory=dict)
    evidence: list[TenderParseFieldEvidence] = Field(default_factory=list)
    requirement_items: list[TenderRequirementItem] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)


class TenderParseStructuredResult(BaseModel):
    project_name: str
    bid_type: Literal["combined", "separated", "custom"]
    source_file: dict[str, object]
    deadline: str | None = None
    qualification_requirements: list[str] = Field(default_factory=list)
    invalid_clause_risks: list[str] = Field(default_factory=list)
    scoring_points: list[str] = Field(default_factory=list)
    outline: dict[str, object] = Field(default_factory=dict)
    material_suggestions: list[dict[str, object]] = Field(default_factory=list)
    modules: dict[TenderParseModule, TenderParseModuleResult] = Field(default_factory=dict)
    field_evidence: list[TenderParseFieldEvidence] = Field(default_factory=list)
    requirement_items: list[TenderRequirementItem] = Field(default_factory=list)
    quality_gates: dict[str, object] = Field(default_factory=dict)
    parse_metadata: dict[str, object] = Field(default_factory=dict)

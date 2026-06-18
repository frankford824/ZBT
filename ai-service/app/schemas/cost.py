from __future__ import annotations

from typing import Annotated, Literal

from pydantic import BaseModel, Field, StringConstraints, field_validator

from app.schemas.common import MAX_RESPONSE_METADATA_BYTES, bounded_json_object, bounded_token_usage

MAX_COST_ID_LENGTH = 128
MAX_COST_NAME_LENGTH = 255
MAX_COST_SHORT_TEXT_LENGTH = 128
MAX_COST_NOTE_LENGTH = 1000
MAX_COST_RECOMMENDATION_LENGTH = 500
MAX_COST_CALLBACK_URL_LENGTH = 2048
MAX_COST_CATEGORY_TOTALS = 50
MAX_COST_OVERRUN_ITEMS = 100
MAX_COST_RECOMMENDATIONS = 20
MAX_COST_AMOUNT = 1_000_000_000_000
MIN_COST_MARGIN_RATE = -1000
MAX_COST_MARGIN_RATE = 1000
MAX_COST_RESPONSE_SUMMARY_LENGTH = 2000
MAX_COST_RESPONSE_TEXT_LENGTH = 1000
MAX_COST_RESPONSE_ITEMS = 20
MAX_COST_RESPONSE_METADATA_BYTES = MAX_RESPONSE_METADATA_BYTES

CostID = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_COST_ID_LENGTH),
]
CostName = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_COST_NAME_LENGTH),
]
CostOptionalName = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_COST_NAME_LENGTH),
]
CostShortText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_COST_SHORT_TEXT_LENGTH),
]
CostOptionalShortText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_COST_SHORT_TEXT_LENGTH),
]
CostNote = Annotated[
    str,
    StringConstraints(strip_whitespace=True, max_length=MAX_COST_NOTE_LENGTH),
]
CostRecommendation = Annotated[
    str,
    StringConstraints(
        strip_whitespace=True,
        min_length=1,
        max_length=MAX_COST_RECOMMENDATION_LENGTH,
    ),
]
CostCallbackURL = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_COST_CALLBACK_URL_LENGTH),
]
CostResponseSummary = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_COST_RESPONSE_SUMMARY_LENGTH),
]
CostResponseText = Annotated[
    str,
    StringConstraints(strip_whitespace=True, min_length=1, max_length=MAX_COST_RESPONSE_TEXT_LENGTH),
]
CostAmount = Annotated[float, Field(ge=0, le=MAX_COST_AMOUNT, allow_inf_nan=False)]
CostMarginAmount = Annotated[
    float,
    Field(ge=-MAX_COST_AMOUNT, le=MAX_COST_AMOUNT, allow_inf_nan=False),
]
CostMarginRate = Annotated[
    float,
    Field(ge=MIN_COST_MARGIN_RATE, le=MAX_COST_MARGIN_RATE, allow_inf_nan=False),
]
CostType = Literal["labor", "material", "equipment", "service", "other"]
CostStatus = Literal["planned", "committed", "actual"]


class CostCategoryTotal(BaseModel):
    category: CostShortText
    total_budget: CostAmount = 0
    total_actual: CostAmount = 0
    margin_amount: CostMarginAmount = 0


class CostOverrunItem(BaseModel):
    id: CostID
    category: CostShortText
    name: CostName
    cost_type: CostType = "other"
    budget_amount: CostAmount = 0
    actual_amount: CostAmount = 0
    status: CostStatus = "planned"
    vendor: CostOptionalName = ""
    note: CostNote = ""


class CostAdviceRequest(BaseModel):
    task_id: CostOptionalShortText | None = None
    tenant_id: CostID
    cost_project_id: CostID
    project_name: CostOptionalName = ""
    cost_project_name: CostOptionalName = ""
    budget_amount: CostAmount | None = None
    total_budget: CostAmount = 0
    total_actual: CostAmount = 0
    margin_rate: CostMarginRate = 0
    category_totals: list[CostCategoryTotal] = Field(default_factory=list, max_length=MAX_COST_CATEGORY_TOTALS)
    overrun_items: list[CostOverrunItem] = Field(default_factory=list, max_length=MAX_COST_OVERRUN_ITEMS)
    recommendations: list[CostRecommendation] = Field(default_factory=list, max_length=MAX_COST_RECOMMENDATIONS)
    callback_url: CostCallbackURL | None = None
    model_hint: CostOptionalShortText | None = None


class CostAdviceResponse(BaseModel):
    trace_id: CostShortText
    summary: CostResponseSummary
    recommendations: list[CostResponseText] = Field(default_factory=list, max_length=MAX_COST_RESPONSE_ITEMS)
    risk_flags: list[CostResponseText] = Field(default_factory=list, max_length=MAX_COST_RESPONSE_ITEMS)
    focus_items: list[CostResponseText] = Field(default_factory=list, max_length=MAX_COST_RESPONSE_ITEMS)
    model_metadata: dict[str, object]
    token_usage: dict[str, int]

    @field_validator("model_metadata")
    @classmethod
    def model_metadata_must_be_bounded(cls, value: dict[str, object]) -> dict[str, object]:
        return bounded_json_object(
            value,
            max_bytes=MAX_COST_RESPONSE_METADATA_BYTES,
            label="model_metadata",
        )

    @field_validator("token_usage")
    @classmethod
    def token_usage_must_be_bounded(cls, value: dict[str, int]) -> dict[str, int]:
        return bounded_token_usage(value)

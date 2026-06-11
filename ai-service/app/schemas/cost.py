from __future__ import annotations

from pydantic import BaseModel, Field


class CostCategoryTotal(BaseModel):
    category: str
    total_budget: float = 0
    total_actual: float = 0
    margin_amount: float = 0


class CostOverrunItem(BaseModel):
    id: str
    category: str
    name: str
    cost_type: str = "other"
    budget_amount: float = 0
    actual_amount: float = 0
    status: str = "planned"
    vendor: str = ""
    note: str = ""


class CostAdviceRequest(BaseModel):
    task_id: str | None = None
    tenant_id: str
    cost_project_id: str
    project_name: str = ""
    cost_project_name: str = ""
    budget_amount: float | None = None
    total_budget: float = 0
    total_actual: float = 0
    margin_rate: float = 0
    category_totals: list[CostCategoryTotal] = Field(default_factory=list)
    overrun_items: list[CostOverrunItem] = Field(default_factory=list)
    recommendations: list[str] = Field(default_factory=list)
    callback_url: str | None = None
    model_hint: str | None = None


class CostAdviceResponse(BaseModel):
    trace_id: str
    summary: str
    recommendations: list[str]
    risk_flags: list[str]
    focus_items: list[str]
    model_metadata: dict[str, object]
    token_usage: dict[str, int]

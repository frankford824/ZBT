from __future__ import annotations

import math

import pytest
from pydantic import ValidationError

from app.schemas.cost import (
    MAX_COST_AMOUNT,
    MAX_COST_CATEGORY_TOTALS,
    MAX_COST_NAME_LENGTH,
    MAX_COST_NOTE_LENGTH,
    MAX_COST_OVERRUN_ITEMS,
    MAX_COST_RECOMMENDATIONS,
    MAX_COST_RECOMMENDATION_LENGTH,
    CostAdviceRequest,
    CostCategoryTotal,
    CostOverrunItem,
)


def _overrun_item(index: int = 0) -> CostOverrunItem:
    return CostOverrunItem(
        id=f"item-{index}",
        category="材料",
        name=f"授权采购-{index}",
        cost_type="material",
        budget_amount=100,
        actual_amount=120,
        status="committed",
    )


def test_cost_advice_request_rejects_oversized_lists() -> None:
    with pytest.raises(ValidationError):
        CostAdviceRequest(
            tenant_id="tenant-demo",
            cost_project_id="cost-demo",
            category_totals=[
                CostCategoryTotal(category=f"分类-{index}")
                for index in range(MAX_COST_CATEGORY_TOTALS + 1)
            ],
        )

    with pytest.raises(ValidationError):
        CostAdviceRequest(
            tenant_id="tenant-demo",
            cost_project_id="cost-demo",
            overrun_items=[_overrun_item(index) for index in range(MAX_COST_OVERRUN_ITEMS + 1)],
        )

    with pytest.raises(ValidationError):
        CostAdviceRequest(
            tenant_id="tenant-demo",
            cost_project_id="cost-demo",
            recommendations=["控制成本"] * (MAX_COST_RECOMMENDATIONS + 1),
        )


def test_cost_advice_request_rejects_invalid_money_values() -> None:
    for value in (-1, MAX_COST_AMOUNT + 1, math.inf, math.nan):
        with pytest.raises(ValidationError):
            CostAdviceRequest(
                tenant_id="tenant-demo",
                cost_project_id="cost-demo",
                total_budget=value,
            )


def test_cost_overrun_item_rejects_unbounded_text_and_invalid_enums() -> None:
    oversized_cases = [
        {"name": "x" * (MAX_COST_NAME_LENGTH + 1)},
        {"note": "x" * (MAX_COST_NOTE_LENGTH + 1)},
        {"cost_type": "unexpected"},
        {"status": "unexpected"},
    ]

    for extra in oversized_cases:
        payload = {
            "id": "item-demo",
            "category": "材料",
            "name": "授权采购",
            "cost_type": "material",
            "status": "committed",
        }
        payload.update(extra)
        with pytest.raises(ValidationError):
            CostOverrunItem(**payload)


def test_cost_required_text_rejects_blank_values() -> None:
    with pytest.raises(ValidationError):
        CostAdviceRequest(tenant_id="   ", cost_project_id="cost-demo")

    with pytest.raises(ValidationError):
        CostCategoryTotal(category="   ")

    with pytest.raises(ValidationError):
        CostOverrunItem(id="item-demo", category="材料", name="   ")

    with pytest.raises(ValidationError):
        CostAdviceRequest(
            tenant_id="tenant-demo",
            cost_project_id="cost-demo",
            recommendations=[" "],
        )


def test_cost_advice_request_strips_bounded_text_fields() -> None:
    request = CostAdviceRequest(
        task_id=" task-demo ",
        tenant_id=" tenant-demo ",
        cost_project_id=" cost-demo ",
        project_name=" 项目 ",
        cost_project_name=" 成本测算 ",
        callback_url=" http://backend:8080/api/v1/ai/callbacks/tasks ",
        model_hint=" model-a ",
        category_totals=[CostCategoryTotal(category=" 材料 ")],
        overrun_items=[
            CostOverrunItem(
                id=" item-demo ",
                category=" 材料 ",
                name=" 授权采购 ",
                vendor=" 供应商 ",
                note=" 备注 ",
            )
        ],
        recommendations=[" 控制采购成本 "],
    )

    assert request.task_id == "task-demo"
    assert request.tenant_id == "tenant-demo"
    assert request.cost_project_id == "cost-demo"
    assert request.project_name == "项目"
    assert request.cost_project_name == "成本测算"
    assert request.callback_url == "http://backend:8080/api/v1/ai/callbacks/tasks"
    assert request.model_hint == "model-a"
    assert request.category_totals[0].category == "材料"
    assert request.overrun_items[0].id == "item-demo"
    assert request.overrun_items[0].category == "材料"
    assert request.overrun_items[0].name == "授权采购"
    assert request.overrun_items[0].vendor == "供应商"
    assert request.overrun_items[0].note == "备注"
    assert request.recommendations == ["控制采购成本"]


def test_cost_advice_request_rejects_oversized_recommendation() -> None:
    with pytest.raises(ValidationError):
        CostAdviceRequest(
            tenant_id="tenant-demo",
            cost_project_id="cost-demo",
            recommendations=["x" * (MAX_COST_RECOMMENDATION_LENGTH + 1)],
        )

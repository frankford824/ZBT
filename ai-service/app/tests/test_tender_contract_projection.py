import pytest
from pydantic import ValidationError

from app.pipelines.parse.tender_parser import (
    _compatible_field_value,
    merge_tender_module_result,
)
from app.schemas.tender import TenderParseStructuredResult


def test_structured_scores_project_to_strings_without_losing_raw_module_records():
    records = [{"name": "Technical", "score": 40, "chunk_id": "parse-chunk-0001"},
               {"name": "Price", "score": 60}]
    base = {
        "project_name": "PDF acceptance",
        "bid_type": "combined",
        "source_file": {},
        "modules": {"evaluation": {"module": "evaluation", "title": "评分",
                                   "fields": {}, "status": "empty"}},
    }
    merged = merge_tender_module_result(
        base, "evaluation", {"module": "evaluation", "fields": {"scoring_points": records}}
    )
    validated = TenderParseStructuredResult(**merged)
    assert validated.scoring_points == ["Technical（40分）", "Price（60分）"]
    assert validated.modules["evaluation"].fields["scoring_points"] == records
    assert "scoring_points" not in base


@pytest.mark.parametrize("key", ["qualification_requirements", "invalid_clause_risks"])
def test_structured_requirements_project_to_readable_strings(key):
    assert _compatible_field_value(key, [{"requirement": "营业执照"}, "安全生产许可"]) == [
        "营业执照", "安全生产许可"
    ]


@pytest.mark.parametrize("key,value", [
    ("bid_type", "not-a-bid-type"), ("deadline", {"date": "2026-12-31"}),
    ("scoring_points", [None]), ("qualification_requirements", [123]),
    ("project_name", "x" * 256),
])
def test_invalid_compatible_field_is_rejected_before_callback(key, value):
    with pytest.raises(ValidationError):
        _compatible_field_value(key, value)


def test_zero_score_is_preserved():
    assert _compatible_field_value("scoring_points", [{"name": "Bonus", "score": 0}]) == [
        "Bonus（0分）"
    ]

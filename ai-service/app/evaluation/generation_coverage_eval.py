from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any


DEFAULT_MIN_COVERAGE_RATIO = 1.0
DEFAULT_MIN_SOURCE_REF_RESOLUTION_RATIO = 0.95
DEFAULT_MIN_SOURCE_REF_REFERENCE_ID_RATIO = 1.0
DEFAULT_MIN_SOURCE_REF_LOCATION_RATIO = 1.0


def evaluate_generation_coverage(spec_path: Path) -> dict[str, Any]:
    spec = json.loads(spec_path.read_text(encoding="utf-8"))
    checks: list[dict[str, Any]] = []
    requirements = _dict_list(spec.get("requirements") or spec.get("requirement_items"))
    chapters = _dict_list(spec.get("chapters"))
    known_refs = _known_source_refs(spec)
    thresholds = spec.get("thresholds") if isinstance(spec.get("thresholds"), dict) else {}
    min_coverage_ratio = _ratio(thresholds.get("min_mandatory_coverage_ratio"), DEFAULT_MIN_COVERAGE_RATIO)
    min_source_ref_resolution_ratio = _ratio(
        thresholds.get("min_source_ref_resolution_ratio"),
        DEFAULT_MIN_SOURCE_REF_RESOLUTION_RATIO,
    )
    min_source_ref_reference_id_ratio = _ratio(
        thresholds.get("min_source_ref_reference_id_ratio"),
        DEFAULT_MIN_SOURCE_REF_REFERENCE_ID_RATIO,
    )
    min_source_ref_location_ratio = _ratio(
        thresholds.get("min_source_ref_location_ratio"),
        DEFAULT_MIN_SOURCE_REF_LOCATION_RATIO,
    )

    coverage = _coverage_by_requirement(chapters)
    mandatory_requirements = [item for item in requirements if _is_mandatory(item)]
    covered_mandatory = [
        requirement
        for requirement in mandatory_requirements
        if _coverage_satisfied(coverage.get(_requirement_id(requirement), []))
    ]
    mandatory_ratio = _safe_ratio(len(covered_mandatory), len(mandatory_requirements))
    _add_check(
        checks,
        "generation.requirements.mandatory_coverage_ratio",
        mandatory_ratio >= min_coverage_ratio,
        f">={min_coverage_ratio}",
        mandatory_ratio,
    )

    for requirement in mandatory_requirements:
        requirement_id = _requirement_id(requirement)
        items = coverage.get(requirement_id, [])
        _add_check(
            checks,
            f"generation.requirements.{requirement_id}.covered",
            _coverage_satisfied(items),
            "covered",
            _coverage_summary(items),
        )

    covered_items = [
        item
        for items in coverage.values()
        for item in items
        if _coverage_item_satisfied(item)
    ]
    covered_without_source = [
        str(item.get("requirement_id") or item.get("id") or "")
        for item in covered_items
        if not _source_refs(item)
    ]
    _add_check(
        checks,
        "generation.coverage.covered_items_have_sources",
        not covered_without_source,
        "all covered requirement_coverage items include source_refs",
        covered_without_source,
    )

    source_refs = _all_source_refs(chapters)
    resolved_source_refs = [ref for ref in source_refs if _source_ref_resolved(ref, known_refs)]
    source_ref_resolution_ratio = _safe_ratio(len(resolved_source_refs), len(source_refs))
    source_refs_with_reference_id = [ref for ref in source_refs if _source_ref_has_reference_id(ref)]
    source_ref_reference_id_ratio = _safe_ratio(len(source_refs_with_reference_id), len(source_refs))
    source_refs_with_location = [ref for ref in source_refs if _source_ref_has_location(ref)]
    source_ref_location_ratio = _safe_ratio(len(source_refs_with_location), len(source_refs))
    _add_check(
        checks,
        "generation.source_refs.resolution_ratio",
        source_ref_resolution_ratio >= min_source_ref_resolution_ratio,
        f">={min_source_ref_resolution_ratio}",
        source_ref_resolution_ratio,
    )
    _add_check(
        checks,
        "generation.source_refs.reference_id_ratio",
        source_ref_reference_id_ratio >= min_source_ref_reference_id_ratio,
        f">={min_source_ref_reference_id_ratio}",
        source_ref_reference_id_ratio,
    )
    _add_check(
        checks,
        "generation.source_refs.location_ratio",
        source_ref_location_ratio >= min_source_ref_location_ratio,
        f">={min_source_ref_location_ratio}",
        source_ref_location_ratio,
    )

    if spec.get("require_source_refs", True):
        _add_check(
            checks,
            "generation.source_refs.present",
            bool(source_refs),
            "non-empty",
            len(source_refs),
        )

    passed = sum(1 for check in checks if check["passed"])
    total = len(checks)
    status = "passed" if total and passed == total else "failed"
    return {
        "name": spec.get("name") or spec_path.stem,
        "status": status,
        "passed_checks": passed,
        "failed_checks": total - passed,
        "total_checks": total,
        "mandatory_requirement_count": len(mandatory_requirements),
        "covered_mandatory_requirement_count": len(covered_mandatory),
        "mandatory_coverage_ratio": mandatory_ratio,
        "source_ref_count": len(source_refs),
        "resolved_source_ref_count": len(resolved_source_refs),
        "source_ref_resolution_ratio": source_ref_resolution_ratio,
        "source_ref_reference_id_ratio": source_ref_reference_id_ratio,
        "source_ref_location_ratio": source_ref_location_ratio,
        "checks": checks,
    }


def _dict_list(value: Any) -> list[dict[str, Any]]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def _known_source_refs(spec: dict[str, Any]) -> set[tuple[str, str]]:
    refs: set[tuple[str, str]] = set()
    for key in ("knowledge_chunks", "source_chunks"):
        for item in _dict_list(spec.get(key)):
            chunk_id = _text(item.get("chunk_id") or item.get("id"))
            document_id = _text(item.get("document_id") or item.get("source_document_id"))
            if chunk_id:
                refs.add((chunk_id, document_id))
                refs.add((chunk_id, ""))
    return refs


def _coverage_by_requirement(chapters: list[dict[str, Any]]) -> dict[str, list[dict[str, Any]]]:
    by_requirement: dict[str, list[dict[str, Any]]] = {}
    for chapter in chapters:
        metadata = chapter.get("model_metadata")
        metadata = metadata if isinstance(metadata, dict) else {}
        self_check = metadata.get("self_check")
        self_check = self_check if isinstance(self_check, dict) else {}
        coverage_items = _dict_list(chapter.get("requirement_coverage"))
        if not coverage_items:
            coverage_items = _dict_list(metadata.get("requirement_coverage"))
        if not coverage_items:
            coverage_items = _dict_list(self_check.get("requirement_coverage"))
        for item in coverage_items:
            requirement_id = _text(item.get("requirement_id") or item.get("id"))
            if not requirement_id:
                continue
            by_requirement.setdefault(requirement_id, []).append(item)
    return by_requirement


def _requirement_id(requirement: dict[str, Any]) -> str:
    return _text(requirement.get("id") or requirement.get("external_id") or requirement.get("reference_id"))


def _is_mandatory(requirement: dict[str, Any]) -> bool:
    if requirement.get("mandatory") is True:
        return True
    priority = _text(requirement.get("priority")).lower()
    return priority in {"must", "mandatory", "high", "required"}


def _coverage_satisfied(items: list[dict[str, Any]]) -> bool:
    return any(_coverage_item_satisfied(item) for item in items)


def _coverage_item_satisfied(item: dict[str, Any]) -> bool:
    status = _text(item.get("status") or item.get("coverage_status")).lower()
    if status in {"covered", "satisfied", "pass", "passed"}:
        return item.get("needs_review") is not True
    return item.get("satisfied") is True and item.get("needs_review") is not True


def _coverage_summary(items: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [
        {
            "satisfied": item.get("satisfied"),
            "status": item.get("status") or item.get("coverage_status"),
            "needs_review": item.get("needs_review"),
            "source_ref_count": len(_source_refs(item)),
        }
        for item in items
    ]


def _all_source_refs(chapters: list[dict[str, Any]]) -> list[dict[str, Any]]:
    refs: list[dict[str, Any]] = []
    for chapter in chapters:
        refs.extend(_source_refs(chapter))
        for item in _coverage_by_requirement([chapter]).values():
            for coverage in item:
                refs.extend(_source_refs(coverage))
    return refs


def _source_refs(item: dict[str, Any]) -> list[dict[str, Any]]:
    values = item.get("source_refs")
    if isinstance(values, list):
        return [value for value in values if isinstance(value, dict)]
    value = item.get("source_ref")
    return [value] if isinstance(value, dict) else []


def _source_ref_resolved(ref: dict[str, Any], known_refs: set[tuple[str, str]]) -> bool:
    if ref.get("resolved") is True:
        return True
    nested = _nested_source_ref(ref)
    if nested is not ref and nested.get("resolved") is True:
        return True
    chunk_id = _text(ref.get("chunk_id") or ref.get("chunkId") or nested.get("chunk_id") or nested.get("chunkId"))
    document_id = _text(
        ref.get("document_id")
        or ref.get("documentId")
        or ref.get("source_document_id")
        or nested.get("document_id")
        or nested.get("documentId")
        or nested.get("source_document_id")
    )
    if not chunk_id:
        return False
    return (chunk_id, document_id) in known_refs or (chunk_id, "") in known_refs


def _source_ref_has_reference_id(ref: dict[str, Any]) -> bool:
    nested = _nested_source_ref(ref)
    return any(
        _text(source.get(key))
        for source in (ref, nested)
        for key in ("citation_id", "citationId", "reference_id", "referenceId", "locator", "source_locator")
    )


def _source_ref_has_location(ref: dict[str, Any]) -> bool:
    nested = _nested_source_ref(ref)
    return any(
        _text(source.get(key))
        for source in (ref, nested)
        for key in (
            "chunk_id",
            "chunkId",
            "document_id",
            "documentId",
            "source_document_id",
            "file_id",
            "fileId",
            "source_file_id",
            "page",
            "page_start",
            "pageStart",
            "page_number",
            "source_locator",
            "locator",
        )
    )


def _nested_source_ref(ref: dict[str, Any]) -> dict[str, Any]:
    metadata = ref.get("metadata")
    if isinstance(metadata, dict):
        source_ref = metadata.get("source_ref")
        if isinstance(source_ref, dict):
            return source_ref
    return ref


def _ratio(value: Any, default: float) -> float:
    try:
        ratio = float(value)
    except (TypeError, ValueError):
        return default
    return min(max(ratio, 0.0), 1.0)


def _safe_ratio(numerator: int, denominator: int) -> float:
    if denominator <= 0:
        return 1.0
    return round(numerator / denominator, 4)


def _text(value: Any) -> str:
    return "" if value is None else str(value).strip()


def _add_check(
    checks: list[dict[str, Any]],
    name: str,
    passed: bool,
    expected: Any,
    actual: Any,
) -> None:
    checks.append(
        {
            "name": name,
            "passed": bool(passed),
            "expected": expected,
            "actual": actual,
        }
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Evaluate bid generation coverage and source attribution.")
    parser.add_argument("--input", required=True, type=Path, help="Path to generation coverage JSON.")
    parser.add_argument("--json", action="store_true", help="Print full JSON result.")
    args = parser.parse_args()

    result = evaluate_generation_coverage(args.input)
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(
            f"{result['status']} mandatory_coverage={result['mandatory_coverage_ratio']} "
            f"source_ref_resolution={result['source_ref_resolution_ratio']} "
            f"source_ref_reference_id={result['source_ref_reference_id_ratio']} "
            f"source_ref_location={result['source_ref_location_ratio']} "
            f"passed={result['passed_checks']}/{result['total_checks']}"
        )
        for check in result["checks"]:
            if not check["passed"]:
                print(
                    f"- {check['name']}: expected={check['expected']!r} "
                    f"actual={check['actual']!r}"
                )
    return 0 if result["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())

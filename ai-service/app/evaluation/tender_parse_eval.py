from __future__ import annotations

import argparse
import json
import mimetypes
from collections.abc import Callable
from pathlib import Path
from typing import Any

from app.pipelines.parse.document_parser import parse_document
from app.pipelines.parse.tender_parser import build_tender_structured_result
from app.schemas.knowledge import KnowledgeProcessRequest, KnowledgeProcessResult
from app.schemas.tender import TenderParseRequest


DEFAULT_REQUIRED_MODULES = (
    "basic",
    "qualification",
    "evaluation",
    "submission",
    "invalid_risk",
    "annex",
)


def evaluate_golden(golden_path: Path, repo_root: Path | None = None) -> dict[str, Any]:
    root = repo_root or _repo_root()
    golden = json.loads(golden_path.read_text(encoding="utf-8"))
    checks: list[dict[str, Any]] = []
    parsed_documents: dict[str, KnowledgeProcessResult] = {}
    document_specs = golden.get("documents", [])
    if not isinstance(document_specs, list):
        raise ValueError("golden.documents must be a list")

    for document_spec in document_specs:
        if not isinstance(document_spec, dict):
            continue
        document_id = str(document_spec.get("id") or document_spec.get("path") or "").strip()
        if not document_id:
            _add_check(checks, "document.id", False, "non-empty id", document_spec.get("id"))
            continue
        document_path = _resolve_document_path(root, document_spec)
        _add_check(checks, f"document.{document_id}.exists", document_path.is_file(), True, str(document_path))
        if not document_path.is_file():
            continue
        parsed = _parse_sample_document(document_id, document_path, document_spec)
        parsed_documents[document_id] = parsed
        _evaluate_document(document_id, parsed, document_spec, checks)

    tender_spec = golden.get("tender_parse")
    if isinstance(tender_spec, dict):
        _evaluate_tender_parse(tender_spec, parsed_documents, checks)

    passed = sum(1 for check in checks if check["passed"])
    total = len(checks)
    score = round(passed / total, 4) if total else 0.0
    minimum_score = float(golden.get("minimum_score") or 1.0)
    status = "passed" if total and score >= minimum_score and passed == total else "failed"
    return {
        "name": golden.get("name") or golden_path.stem,
        "status": status,
        "score": score,
        "minimum_score": minimum_score,
        "passed_checks": passed,
        "failed_checks": total - passed,
        "total_checks": total,
        "checks": checks,
    }


def _evaluate_document(
    document_id: str,
    parsed: KnowledgeProcessResult,
    spec: dict[str, Any],
    checks: list[dict[str, Any]],
) -> None:
    metadata = parsed.metadata
    if "parser" in spec:
        _add_check(
            checks,
            f"document.{document_id}.parser",
            metadata.get("parser") == spec["parser"],
            spec["parser"],
            metadata.get("parser"),
        )
    for key, metadata_key in (
        ("min_chunks", "chunk_count"),
        ("min_pages", "page_count"),
        ("min_table_blocks", "table_block_count"),
    ):
        if key not in spec:
            continue
        fallback = len(parsed.chunks) if metadata_key == "chunk_count" else 0
        actual = _int_value(metadata.get(metadata_key), fallback)
        expected = int(spec[key])
        _add_check(
            checks,
            f"document.{document_id}.{metadata_key}",
            actual >= expected,
            f">={expected}",
            actual,
        )
    if "ocr_required" in spec:
        _add_check(
            checks,
            f"document.{document_id}.ocr_required",
            bool(metadata.get("ocr_required")) == bool(spec["ocr_required"]),
            bool(spec["ocr_required"]),
            bool(metadata.get("ocr_required")),
        )
    for index, expected_text in enumerate(_string_list(spec.get("text_contains")), start=1):
        source_text = "\n".join(chunk.content for chunk in parsed.chunks)
        _add_check(
            checks,
            f"document.{document_id}.text_contains[{index}]",
            expected_text in source_text,
            expected_text,
            _excerpt(source_text, expected_text),
        )
    table_spec = spec.get("table_blocks")
    if isinstance(table_spec, dict):
        _evaluate_table_blocks(document_id, metadata, table_spec, checks)


def _evaluate_table_blocks(
    document_id: str,
    metadata: dict[str, Any],
    table_spec: dict[str, Any],
    checks: list[dict[str, Any]],
) -> None:
    raw_blocks = metadata.get("table_blocks")
    blocks = [block for block in raw_blocks if isinstance(block, dict)] if isinstance(raw_blocks, list) else []
    if "required_sources" in table_spec:
        sources = {str(block.get("source") or "") for block in blocks}
        for source in _string_list(table_spec.get("required_sources")):
            _add_check(
                checks,
                f"document.{document_id}.table_blocks.source.{source}",
                source in sources,
                "present",
                sorted(sources),
            )
    if "min_total_rows" in table_spec:
        expected = int(table_spec["min_total_rows"])
        actual = sum(_table_row_count(block) for block in blocks)
        _add_check(
            checks,
            f"document.{document_id}.table_blocks.total_rows",
            actual >= expected,
            f">={expected}",
            actual,
        )
    if "min_blocks_with_rows" in table_spec:
        expected = int(table_spec["min_blocks_with_rows"])
        actual = sum(1 for block in blocks if _table_row_count(block) > 0)
        _add_check(
            checks,
            f"document.{document_id}.table_blocks.with_rows",
            actual >= expected,
            f">={expected}",
            actual,
        )
    if "min_blocks_with_bbox" in table_spec:
        expected = int(table_spec["min_blocks_with_bbox"])
        actual = sum(1 for block in blocks if _valid_bbox(block.get("bbox")))
        _add_check(
            checks,
            f"document.{document_id}.table_blocks.with_bbox",
            actual >= expected,
            f">={expected}",
            actual,
        )
    if "min_cells_with_bbox" in table_spec:
        expected = int(table_spec["min_cells_with_bbox"])
        actual = sum(_cell_bbox_count(block) for block in blocks)
        _add_check(
            checks,
            f"document.{document_id}.table_blocks.cells_with_bbox",
            actual >= expected,
            f">={expected}",
            actual,
        )
    if table_spec.get("require_md_table") is True:
        blocks_with_rows = [block for block in blocks if _table_row_count(block) > 0]
        missing = [
            block.get("index")
            for block in blocks_with_rows
            if not str(block.get("md_table") or "").strip()
        ]
        _add_check(
            checks,
            f"document.{document_id}.table_blocks.md_table_present",
            bool(blocks_with_rows) and not missing,
            "all row-backed table blocks include md_table",
            {"blocks_with_rows": len(blocks_with_rows), "missing_indexes": missing},
        )
    table_text = json.dumps(blocks, ensure_ascii=False)
    for index, expected_text in enumerate(_string_list(table_spec.get("must_contain")), start=1):
        _add_check(
            checks,
            f"document.{document_id}.table_blocks.must_contain[{index}]",
            expected_text in table_text,
            expected_text,
            _excerpt(table_text, expected_text),
        )
    md_table_text = "\n\n".join(str(block.get("md_table") or "") for block in blocks)
    for index, expected_text in enumerate(_string_list(table_spec.get("md_table_must_contain")), start=1):
        _add_check(
            checks,
            f"document.{document_id}.table_blocks.md_table_must_contain[{index}]",
            expected_text in md_table_text,
            expected_text,
            _excerpt(md_table_text, expected_text),
        )


def _table_row_count(block: dict[str, Any]) -> int:
    rows = block.get("rows")
    if isinstance(rows, list):
        return len(rows)
    return _int_value(block.get("row_count"), 0)


def _valid_bbox(value: Any) -> bool:
    if not isinstance(value, list) or len(value) != 4:
        return False
    try:
        x0, y0, x1, y1 = (float(item) for item in value)
    except (TypeError, ValueError):
        return False
    return x1 > x0 and y1 > y0


def _cell_bbox_count(block: dict[str, Any]) -> int:
    cell_bboxes = block.get("cell_bboxes")
    if not isinstance(cell_bboxes, list):
        return 0
    total = 0
    for row in cell_bboxes:
        if not isinstance(row, list):
            continue
        total += sum(1 for cell in row if _valid_bbox(cell))
    return total


def _evaluate_tender_parse(
    tender_spec: dict[str, Any],
    parsed_documents: dict[str, KnowledgeProcessResult],
    checks: list[dict[str, Any]],
) -> None:
    document_id = str(tender_spec.get("document_id") or "").strip()
    parsed = parsed_documents.get(document_id)
    _add_check(checks, "tender_parse.document", parsed is not None, document_id, sorted(parsed_documents))
    if parsed is None:
        return
    source_file = str(tender_spec.get("filename") or parsed.processed_title)
    payload = TenderParseRequest(
        tenant_id=str(tender_spec.get("tenant_id") or "eval-tenant"),
        bid_id=str(tender_spec.get("bid_id") or "eval-bid"),
        bid_title=str(tender_spec.get("bid_title") or "").strip() or None,
        file_id=document_id,
        object_key=str(tender_spec.get("object_key") or f"eval/{source_file}"),
        filename=source_file,
        content_type=str(tender_spec.get("content_type") or "application/pdf"),
    )
    structured = build_tender_structured_result(payload, parsed)
    for module in tender_spec.get("required_modules") or DEFAULT_REQUIRED_MODULES:
        modules = structured.get("modules")
        _add_check(
            checks,
            f"tender_parse.module.{module}",
            isinstance(modules, dict) and module in modules,
            "present",
            "present" if isinstance(modules, dict) and module in modules else "missing",
        )
    for field_spec in tender_spec.get("fields", []):
        if isinstance(field_spec, dict):
            _evaluate_field(field_spec, structured, checks)
    requirements_spec = tender_spec.get("requirements")
    if isinstance(requirements_spec, dict):
        _evaluate_requirements(requirements_spec, structured, checks)
    evidence_spec = tender_spec.get("evidence")
    if isinstance(evidence_spec, dict):
        _evaluate_evidence(evidence_spec, structured, checks)


def _evaluate_field(
    field_spec: dict[str, Any],
    structured: dict[str, Any],
    checks: list[dict[str, Any]],
) -> None:
    field_path = str(field_spec.get("path") or "").strip()
    if not field_path:
        return
    actual = _path_value(structured, field_path)
    label = str(field_spec.get("name") or field_path)
    if "equals" in field_spec:
        _add_check(
            checks,
            f"tender_parse.field.{label}.equals",
            actual == field_spec["equals"],
            field_spec["equals"],
            actual,
        )
    for index, expected_text in enumerate(_string_list(field_spec.get("contains")), start=1):
        _add_check(
            checks,
            f"tender_parse.field.{label}.contains[{index}]",
            _contains_text(actual, expected_text),
            expected_text,
            actual,
        )


def _evaluate_requirements(
    requirements_spec: dict[str, Any],
    structured: dict[str, Any],
    checks: list[dict[str, Any]],
) -> None:
    requirements = structured.get("requirement_items")
    if not isinstance(requirements, list):
        requirements = []
    if "min_count" in requirements_spec:
        expected = int(requirements_spec["min_count"])
        _add_check(
            checks,
            "tender_parse.requirements.count",
            len(requirements) >= expected,
            f">={expected}",
            len(requirements),
        )
    for module in _string_list(requirements_spec.get("required_modules")):
        _add_check(
            checks,
            f"tender_parse.requirements.module.{module}",
            any(isinstance(item, dict) and item.get("module") == module for item in requirements),
            "present",
            _requirement_module_counts(requirements),
        )
    for index, item_spec in enumerate(requirements_spec.get("must_contain") or [], start=1):
        if not isinstance(item_spec, dict):
            continue
        module = str(item_spec.get("module") or "").strip()
        text = str(item_spec.get("text") or "").strip()
        matched = any(
            isinstance(item, dict)
            and (not module or item.get("module") == module)
            and text in str(item.get("requirement") or "")
            for item in requirements
        )
        _add_check(
            checks,
            f"tender_parse.requirements.must_contain[{index}]",
            matched,
            item_spec,
            _requirements_excerpt(requirements, module),
        )
    if requirements_spec.get("require_traceable_source_refs") is True:
        missing = _missing_source_ref_indexes(
            requirements,
            lambda item: item.get("source_ref") if isinstance(item, dict) else None,
            require_traceable=True,
        )
        _add_check(
            checks,
            "tender_parse.requirements.traceable_source_refs",
            not missing and bool(requirements),
            "all requirement items include traceable source_ref",
            _source_ref_check_summary(requirements, missing),
        )
    if requirements_spec.get("require_reference_ids") is True:
        missing = _missing_source_ref_indexes(
            requirements,
            lambda item: item.get("source_ref") if isinstance(item, dict) else None,
            require_reference_id=True,
        )
        _add_check(
            checks,
            "tender_parse.requirements.source_reference_ids",
            not missing and bool(requirements),
            "all requirement source_ref values include citation_id or reference_id",
            _source_ref_check_summary(requirements, missing),
        )
    if requirements_spec.get("require_source_locations") is True:
        missing = _missing_source_ref_indexes(
            requirements,
            lambda item: item.get("source_ref") if isinstance(item, dict) else None,
            require_location=True,
        )
        _add_check(
            checks,
            "tender_parse.requirements.source_locations",
            not missing and bool(requirements),
            "all requirement source_ref values include page, chunk, file, or document location",
            _source_ref_check_summary(requirements, missing),
        )


def _evaluate_evidence(
    evidence_spec: dict[str, Any],
    structured: dict[str, Any],
    checks: list[dict[str, Any]],
) -> None:
    evidence = structured.get("field_evidence")
    if not isinstance(evidence, list):
        evidence = []
    if "min_count" in evidence_spec:
        expected = int(evidence_spec["min_count"])
        _add_check(
            checks,
            "tender_parse.evidence.count",
            len(evidence) >= expected,
            f">={expected}",
            len(evidence),
        )
    if "max_missing_source_count" in evidence_spec:
        expected = int(evidence_spec["max_missing_source_count"])
        actual = sum(
            1 for item in evidence if isinstance(item, dict) and not str(item.get("source_text") or "").strip()
        )
        _add_check(
            checks,
            "tender_parse.evidence.missing_source_count",
            actual <= expected,
            f"<={expected}",
            actual,
        )
    if evidence_spec.get("require_traceable") is True:
        missing = _missing_source_ref_indexes(evidence, lambda item: item, require_traceable=True)
        _add_check(
            checks,
            "tender_parse.evidence.traceable",
            not missing and bool(evidence),
            "all evidence items are traceable",
            _source_ref_check_summary(evidence, missing),
        )
    if evidence_spec.get("require_reference_ids") is True:
        missing = _missing_source_ref_indexes(evidence, lambda item: item, require_reference_id=True)
        _add_check(
            checks,
            "tender_parse.evidence.reference_ids",
            not missing and bool(evidence),
            "all evidence items include citation_id or reference_id",
            _source_ref_check_summary(evidence, missing),
        )
    if evidence_spec.get("require_source_locations") is True:
        missing = _missing_source_ref_indexes(evidence, lambda item: item, require_location=True)
        _add_check(
            checks,
            "tender_parse.evidence.source_locations",
            not missing and bool(evidence),
            "all evidence items include page, chunk, file, or document location",
            _source_ref_check_summary(evidence, missing),
        )


def _parse_sample_document(
    document_id: str,
    path: Path,
    spec: dict[str, Any],
) -> KnowledgeProcessResult:
    content_type = str(spec.get("content_type") or _content_type(path))
    request = KnowledgeProcessRequest(
        tenant_id=str(spec.get("tenant_id") or "eval-tenant"),
        document_id=document_id,
        file_id=document_id,
        object_key=str(spec.get("object_key") or f"eval/{path.name}"),
        filename=path.name,
        content_type=content_type,
    )
    return parse_document(request, path.read_bytes())


def _resolve_document_path(repo_root: Path, spec: dict[str, Any]) -> Path:
    raw = Path(str(spec.get("path") or ""))
    return raw if raw.is_absolute() else repo_root / raw


def _content_type(path: Path) -> str:
    known = {
        ".pdf": "application/pdf",
        ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    }
    return known.get(path.suffix.lower()) or mimetypes.guess_type(path.name)[0] or "text/plain"


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


def _int_value(value: Any, default: int) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def _path_value(value: Any, path: str) -> Any:
    current = value
    for part in path.split("."):
        if isinstance(current, dict):
            current = current.get(part)
        elif isinstance(current, list):
            try:
                current = current[int(part)]
            except (ValueError, IndexError):
                return None
        else:
            return None
    return current


def _contains_text(actual: Any, expected: str) -> bool:
    if isinstance(actual, list):
        return any(expected in str(item) for item in actual)
    if isinstance(actual, dict):
        return expected in json.dumps(actual, ensure_ascii=False)
    return expected in str(actual or "")


def _string_list(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        return [value]
    if isinstance(value, list):
        return [str(item) for item in value if str(item).strip()]
    return [str(value)]


def _requirement_module_counts(requirements: list[Any]) -> dict[str, int]:
    counts: dict[str, int] = {}
    for item in requirements:
        if not isinstance(item, dict):
            continue
        module = str(item.get("module") or "")
        counts[module] = counts.get(module, 0) + 1
    return counts


def _requirements_excerpt(requirements: list[Any], module: str) -> list[str]:
    values: list[str] = []
    for item in requirements:
        if not isinstance(item, dict):
            continue
        if module and item.get("module") != module:
            continue
        values.append(str(item.get("requirement") or "")[:120])
        if len(values) >= 8:
            break
    return values


def _missing_source_ref_indexes(
    items: list[Any],
    source_ref_for: Callable[[Any], Any],
    *,
    require_traceable: bool = False,
    require_reference_id: bool = False,
    require_location: bool = False,
) -> list[str]:
    missing: list[str] = []
    for index, item in enumerate(items, start=1):
        source_ref = source_ref_for(item)
        if not isinstance(source_ref, dict):
            missing.append(_item_identifier(item, index))
            continue
        if require_traceable and not _traceable_source_ref(source_ref):
            missing.append(_item_identifier(item, index))
            continue
        if require_reference_id and not _has_reference_id(source_ref):
            missing.append(_item_identifier(item, index))
            continue
        if require_location and not _has_source_location(source_ref):
            missing.append(_item_identifier(item, index))
            continue
    return missing


def _traceable_source_ref(source_ref: dict[str, Any]) -> bool:
    return bool(str(source_ref.get("source_text") or "").strip()) and (
        bool(source_ref.get("traceable"))
        or _has_reference_id(source_ref)
        or _has_source_location(source_ref)
    )


def _has_reference_id(source_ref: dict[str, Any]) -> bool:
    return bool(str(source_ref.get("citation_id") or source_ref.get("reference_id") or "").strip())


def _has_source_location(source_ref: dict[str, Any]) -> bool:
    for key in ("page_start", "page_end", "chunk_id", "document_id", "file_id", "bbox"):
        value = source_ref.get(key)
        if value not in (None, "", []):
            return True
    return False


def _item_identifier(item: Any, index: int) -> str:
    if isinstance(item, dict):
        for key in ("id", "field", "requirement"):
            value = str(item.get(key) or "").strip()
            if value:
                return value[:80]
    return f"item-{index}"


def _source_ref_check_summary(items: list[Any], missing: list[str]) -> dict[str, Any]:
    return {
        "total": len(items),
        "missing_count": len(missing),
        "missing": missing[:12],
    }


def _excerpt(source_text: str, expected_text: str) -> str:
    index = source_text.find(expected_text)
    if index < 0:
        return source_text[:240]
    start = max(index - 80, 0)
    end = min(index + len(expected_text) + 80, len(source_text))
    return source_text[start:end]


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def main() -> int:
    parser = argparse.ArgumentParser(description="Evaluate tender parse output against golden JSON.")
    parser.add_argument("--golden", required=True, type=Path, help="Path to golden evaluation JSON.")
    parser.add_argument("--repo-root", type=Path, default=None, help="Repository root for relative document paths.")
    parser.add_argument("--json", action="store_true", help="Print full JSON result.")
    args = parser.parse_args()

    result = evaluate_golden(args.golden, args.repo_root)
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(
            f"{result['status']} score={result['score']} "
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

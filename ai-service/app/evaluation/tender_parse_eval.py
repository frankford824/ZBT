from __future__ import annotations

import argparse
import json
import mimetypes
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

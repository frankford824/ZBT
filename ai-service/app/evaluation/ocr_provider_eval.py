from __future__ import annotations

import argparse
import json
import mimetypes
import os
from contextlib import contextmanager
from pathlib import Path
from typing import Any

import fitz

from app.pipelines.parse.document_parser import parse_document
from app.schemas.knowledge import KnowledgeProcessRequest, KnowledgeProcessResult


DEFAULT_SAMPLE = Path("docs/ex/工程1/采购文件桥梁检查.pdf")
SUPPORTED_PROVIDERS = {"http_ocr", "mineru", "paddleocr"}


def evaluate_ocr_provider(
    provider: str,
    sample_path: Path,
    *,
    repo_root: Path | None = None,
    render_pdf_page: int | None = 1,
    min_text_chars: int = 20,
    min_table_blocks: int = 0,
    min_layout_blocks: int = 0,
) -> dict[str, Any]:
    provider = _normalize_provider(provider)
    root = repo_root or _repo_root()
    sample = sample_path if sample_path.is_absolute() else root / sample_path
    checks: list[dict[str, Any]] = []
    _add_check(checks, "provider.supported", provider in SUPPORTED_PROVIDERS, sorted(SUPPORTED_PROVIDERS), provider)
    if provider not in SUPPORTED_PROVIDERS:
        return _result(provider, sample, "failed", checks)

    endpoint = _provider_endpoint(provider)
    if not endpoint:
        _add_check(checks, "provider.endpoint_configured", False, _provider_endpoint_env(provider), "")
        return _result(provider, sample, "skipped", checks)
    _add_check(checks, "provider.endpoint_configured", True, _provider_endpoint_env(provider), _configured_endpoint_source(provider))
    _add_check(checks, "sample.exists", sample.is_file(), True, str(sample))
    if not sample.is_file():
        return _result(provider, sample, "failed", checks)

    content, filename, content_type = _sample_content(sample, render_pdf_page)
    with _temporary_env({"OCR_PROVIDER": provider}):
        parsed = parse_document(
            KnowledgeProcessRequest(
                tenant_id="ocr-eval-tenant",
                document_id="ocr-eval-document",
                file_id="ocr-eval-file",
                object_key="ocr-eval/sample",
                filename=filename,
                content_type=content_type,
            ),
            content,
        )
    _evaluate_parsed_result(provider, parsed, checks, min_text_chars, min_table_blocks, min_layout_blocks)
    status = "passed" if checks and all(check["passed"] for check in checks) else "failed"
    result = _result(provider, sample, status, checks)
    result["metadata"] = _safe_metadata(parsed.metadata)
    return result


def _evaluate_parsed_result(
    provider: str,
    parsed: KnowledgeProcessResult,
    checks: list[dict[str, Any]],
    min_text_chars: int,
    min_table_blocks: int,
    min_layout_blocks: int,
) -> None:
    metadata = parsed.metadata
    ocr = metadata.get("ocr")
    ocr_record = ocr if isinstance(ocr, dict) else {}
    text = "\n".join(chunk.content for chunk in parsed.chunks)
    _add_check(checks, "ocr.status", ocr_record.get("status") == "done", "done", ocr_record.get("status"))
    _add_check(checks, "ocr.provider", ocr_record.get("provider") == provider, provider, ocr_record.get("provider"))
    _add_check(checks, "ocr.text_chars", len(text.strip()) >= min_text_chars, f">={min_text_chars}", len(text.strip()))
    _add_check(checks, "ocr.chunk_count", len(parsed.chunks) > 0, ">0", len(parsed.chunks))
    profile = ocr_record.get("provider_profile")
    profile_record = profile if isinstance(profile, dict) else {}
    _add_check(
        checks,
        "ocr.provider_profile.endpoint_env",
        profile_record.get("endpoint_env") == _provider_endpoint_env(provider),
        _provider_endpoint_env(provider),
        profile_record.get("endpoint_env"),
    )
    if min_table_blocks > 0:
        actual = _int_value(metadata.get("table_block_count"), 0)
        _add_check(checks, "ocr.table_blocks", actual >= min_table_blocks, f">={min_table_blocks}", actual)
    if min_layout_blocks > 0:
        actual = _int_value(metadata.get("layout_block_count"), 0)
        _add_check(checks, "ocr.layout_blocks", actual >= min_layout_blocks, f">={min_layout_blocks}", actual)


def _sample_content(sample: Path, render_pdf_page: int | None) -> tuple[bytes, str, str]:
    if sample.suffix.lower() == ".pdf" and render_pdf_page is not None:
        page_index = max(render_pdf_page, 1) - 1
        doc = fitz.open(sample)
        try:
            if page_index >= doc.page_count:
                raise ValueError(f"PDF page {render_pdf_page} is outside sample page count {doc.page_count}")
            pixmap = doc[page_index].get_pixmap(matrix=fitz.Matrix(2, 2), alpha=False)
            return pixmap.tobytes("png"), f"{sample.stem}-page-{render_pdf_page}.png", "image/png"
        finally:
            doc.close()
    content_type = mimetypes.guess_type(sample.name)[0] or "application/octet-stream"
    return sample.read_bytes(), sample.name, content_type


def _normalize_provider(provider: str) -> str:
    return (provider or os.getenv("OCR_PROVIDER") or "http_ocr").strip().lower() or "http_ocr"


def _provider_endpoint(provider: str) -> str:
    return os.getenv(_provider_endpoint_env(provider), "").strip() or os.getenv("OCR_HTTP_ENDPOINT", "").strip()


def _configured_endpoint_source(provider: str) -> str:
    if os.getenv(_provider_endpoint_env(provider), "").strip():
        return _provider_endpoint_env(provider)
    if os.getenv("OCR_HTTP_ENDPOINT", "").strip():
        return "OCR_HTTP_ENDPOINT"
    return ""


def _provider_endpoint_env(provider: str) -> str:
    if provider == "mineru":
        return "MINERU_HTTP_ENDPOINT"
    if provider == "paddleocr":
        return "PADDLEOCR_HTTP_ENDPOINT"
    return "OCR_HTTP_ENDPOINT"


def _safe_metadata(metadata: dict[str, Any]) -> dict[str, Any]:
    ocr = metadata.get("ocr")
    ocr_record = dict(ocr) if isinstance(ocr, dict) else {}
    for key in ("text", "pages", "blocks", "layout_blocks", "table_blocks", "tables"):
        if key in ocr_record:
            value = ocr_record[key]
            if isinstance(value, list):
                ocr_record[key] = {"count": len(value)}
            elif isinstance(value, str):
                ocr_record[key] = {"length": len(value)}
    return {
        "parser": metadata.get("parser"),
        "ocr_required": metadata.get("ocr_required"),
        "chunk_count": metadata.get("chunk_count"),
        "table_block_count": metadata.get("table_block_count"),
        "layout_block_count": metadata.get("layout_block_count"),
        "ocr": ocr_record,
    }


def _result(provider: str, sample: Path, status: str, checks: list[dict[str, Any]]) -> dict[str, Any]:
    passed = sum(1 for check in checks if check["passed"])
    return {
        "name": "ocr_provider_eval",
        "status": status,
        "provider": provider,
        "sample": str(sample),
        "passed_checks": passed,
        "failed_checks": len(checks) - passed,
        "total_checks": len(checks),
        "checks": checks,
    }


def _add_check(checks: list[dict[str, Any]], name: str, passed: bool, expected: Any, actual: Any) -> None:
    checks.append({"name": name, "passed": bool(passed), "expected": expected, "actual": actual})


def _int_value(value: Any, default: int) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


@contextmanager
def _temporary_env(values: dict[str, str]):
    previous = {key: os.environ.get(key) for key in values}
    try:
        for key, value in values.items():
            os.environ[key] = value
        yield
    finally:
        for key, value in previous.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def main() -> int:
    parser = argparse.ArgumentParser(description="Evaluate configured OCR Provider against a sample document.")
    parser.add_argument("--provider", default=os.getenv("OCR_PROVIDER", "http_ocr"), choices=sorted(SUPPORTED_PROVIDERS))
    parser.add_argument("--sample", type=Path, default=DEFAULT_SAMPLE, help="Sample file path. PDF defaults to rendered page OCR.")
    parser.add_argument("--repo-root", type=Path, default=None, help="Repository root for relative sample paths.")
    parser.add_argument("--render-pdf-page", type=int, default=1, help="Render this PDF page to PNG before OCR; set 0 to parse file directly.")
    parser.add_argument("--min-text-chars", type=int, default=20)
    parser.add_argument("--min-table-blocks", type=int, default=0)
    parser.add_argument("--min-layout-blocks", type=int, default=0)
    parser.add_argument("--json", action="store_true", help="Print full JSON result.")
    parser.add_argument("--allow-skip", action="store_true", help="Exit 0 when the provider endpoint is not configured.")
    args = parser.parse_args()

    render_pdf_page = args.render_pdf_page if args.render_pdf_page > 0 else None
    result = evaluate_ocr_provider(
        args.provider,
        args.sample,
        repo_root=args.repo_root,
        render_pdf_page=render_pdf_page,
        min_text_chars=args.min_text_chars,
        min_table_blocks=args.min_table_blocks,
        min_layout_blocks=args.min_layout_blocks,
    )
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(
            f"{result['status']} provider={result['provider']} "
            f"passed={result['passed_checks']}/{result['total_checks']} sample={result['sample']}"
        )
        for check in result["checks"]:
            if not check["passed"]:
                print(f"- {check['name']}: expected={check['expected']!r} actual={check['actual']!r}")
    if result["status"] == "passed" or (result["status"] == "skipped" and args.allow_skip):
        return 0
    if result["status"] == "skipped":
        return 2
    return 1


if __name__ == "__main__":
    raise SystemExit(main())

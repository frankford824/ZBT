from __future__ import annotations

import argparse
import json
from pathlib import Path
from tempfile import TemporaryDirectory
from typing import Any
from zipfile import BadZipFile, ZipFile

import fitz
from docx import Document

from app.pipelines.export.docx_exporter import export_bid_docx, export_bid_pdf, export_bid_zip
from app.schemas.export import ExportAttachment, ExportChapter, ExportLayoutOptions, ExportPart


def evaluate_export_format(spec_path: Path) -> dict[str, Any]:
    spec = json.loads(spec_path.read_text(encoding="utf-8"))
    checks: list[dict[str, Any]] = []
    with TemporaryDirectory() as tmpdir:
        tmp_path = Path(tmpdir)
        docx_result = _evaluate_docx(spec, tmp_path, checks)
        zip_result = _evaluate_zip(spec, tmp_path, checks)
        pdf_result = _evaluate_pdf(spec, tmp_path, checks)
    passed = sum(1 for check in checks if check["passed"])
    total = len(checks)
    return {
        "name": spec.get("name") or spec_path.stem,
        "status": "passed" if total and passed == total else "failed",
        "passed_checks": passed,
        "failed_checks": total - passed,
        "total_checks": total,
        "docx": docx_result,
        "zip": zip_result,
        "pdf": pdf_result,
        "checks": checks,
    }


def _evaluate_docx(spec: dict[str, Any], tmp_path: Path, checks: list[dict[str, Any]]) -> dict[str, Any]:
    docx_spec = _dict_value(spec.get("docx"))
    output = tmp_path / "export.docx"
    export_bid_docx(
        _text(spec.get("bid_title"), "智标通投标文件"),
        _text(spec.get("part_title"), "技术标"),
        _chapters(spec),
        output,
        layout=_layout(spec),
    )
    package = _docx_package(output)
    document_text = _docx_text(output)
    result = {
        "path": str(output),
        "size_bytes": output.stat().st_size,
        "table_count": _docx_table_count(output),
    }
    _add_check(checks, "export.docx.openable", bool(package), "valid docx zip", output.name)
    _add_check(checks, "export.docx.non_empty", output.stat().st_size > 0, ">0 bytes", output.stat().st_size)
    for text in _list_value(docx_spec.get("required_text")):
        expected = str(text)
        _add_check(
            checks,
            f"export.docx.text.{_check_slug(expected)}",
            expected in document_text,
            expected,
            _short_actual(document_text),
        )
    min_tables = _int_value(docx_spec.get("min_tables"), 0)
    if min_tables > 0:
        _add_check(checks, "export.docx.tables", result["table_count"] >= min_tables, f">={min_tables}", result["table_count"])
    package_xml = "\n".join(package.values())
    settings_xml = package.get("word/settings.xml", "")
    if docx_spec.get("require_toc", True):
        _add_check(checks, "export.docx.toc_field", "TOC" in package_xml, "TOC field", "TOC" in package_xml)
    if docx_spec.get("require_update_fields", True):
        _add_check(
            checks,
            "export.docx.update_fields",
            "updateFields" in settings_xml,
            "updateFields in settings.xml",
            "updateFields" in settings_xml,
        )
    if docx_spec.get("require_page_fields", True):
        has_page_fields = "PAGE" in package_xml and "NUMPAGES" in package_xml
        _add_check(checks, "export.docx.page_fields", has_page_fields, "PAGE and NUMPAGES fields", has_page_fields)
    if docx_spec.get("require_header_footer", True):
        has_header = any(name.startswith("word/header") for name in package)
        has_footer = any(name.startswith("word/footer") for name in package)
        _add_check(checks, "export.docx.header_footer", has_header and has_footer, "header and footer parts", {"header": has_header, "footer": has_footer})
    return result


def _evaluate_zip(spec: dict[str, Any], tmp_path: Path, checks: list[dict[str, Any]]) -> dict[str, Any]:
    zip_spec = _dict_value(spec.get("zip"))
    if zip_spec.get("enabled") is False:
        return {"status": "disabled"}
    output = tmp_path / "export.zip"
    parts = _parts(spec)
    export_bid_zip(
        _text(spec.get("bid_title"), "智标通投标文件"),
        parts,
        output,
        layout=_layout(spec),
        attachments=_attachments(zip_spec.get("attachments")),
        boq_files=_attachments(zip_spec.get("boq_files")),
    )
    with ZipFile(output) as archive:
        names = sorted(archive.namelist())
        manifest = json.loads(archive.read("manifest.json").decode("utf-8")) if "manifest.json" in names else {}
        docx_entries = [name for name in names if name.endswith(".docx")]
        docx_openable = all(_zip_docx_openable(archive, name) for name in docx_entries)
    result = {
        "path": str(output),
        "size_bytes": output.stat().st_size,
        "entry_count": len(names),
        "docx_entry_count": len(docx_entries),
    }
    _add_check(checks, "export.zip.openable", output.stat().st_size > 0, ">0 bytes", output.stat().st_size)
    if zip_spec.get("require_manifest", True):
        _add_check(checks, "export.zip.manifest", bool(manifest), "manifest.json", "manifest.json" in names)
        _add_check(checks, "export.zip.manifest_part_count", manifest.get("part_count") == len(parts), len(parts), manifest.get("part_count"))
    min_docx_entries = _int_value(zip_spec.get("min_docx_entries"), len(parts))
    _add_check(checks, "export.zip.docx_entries", len(docx_entries) >= min_docx_entries, f">={min_docx_entries}", len(docx_entries))
    _add_check(checks, "export.zip.docx_entries_openable", docx_openable, "all docx entries open", docx_entries)
    for entry in _list_value(zip_spec.get("expected_entries")):
        expected = str(entry)
        _add_check(checks, f"export.zip.entry.{_check_slug(expected)}", expected in names, expected, names)
    return result


def _evaluate_pdf(spec: dict[str, Any], tmp_path: Path, checks: list[dict[str, Any]]) -> dict[str, Any]:
    pdf_spec = _dict_value(spec.get("pdf"))
    if pdf_spec.get("enabled") is False:
        return {"status": "disabled"}
    output = tmp_path / "export.pdf"
    allow_skip = bool(pdf_spec.get("allow_skip", False))
    try:
        export_bid_pdf(
            _text(spec.get("bid_title"), "智标通投标文件"),
            _text(spec.get("part_title"), "技术标"),
            _chapters(spec),
            output,
            layout=_layout(spec),
        )
    except RuntimeError as exc:
        skipped = allow_skip and "LibreOffice" in str(exc)
        _add_check(checks, "export.pdf.generated", skipped, "generated PDF or explicit skip", "skipped: LibreOffice unavailable" if skipped else str(exc))
        return {"status": "skipped" if skipped else "failed", "reason": str(exc)}
    with fitz.open(output) as document:
        page_count = document.page_count
        text = "\n".join(page.get_text("text") for page in document)
        first_page_nonblank = _first_page_nonblank(document)
    result = {
        "status": "generated",
        "path": str(output),
        "size_bytes": output.stat().st_size,
        "page_count": page_count,
        "text_chars": len(text.strip()),
        "first_page_nonblank": first_page_nonblank,
    }
    min_pages = _int_value(pdf_spec.get("min_pages"), 1)
    _add_check(checks, "export.pdf.generated", output.stat().st_size > 0, ">0 bytes", output.stat().st_size)
    _add_check(checks, "export.pdf.openable", page_count >= min_pages, f">={min_pages} page(s)", page_count)
    if pdf_spec.get("require_text", True):
        _add_check(checks, "export.pdf.text_layer", bool(text.strip()), "non-empty text layer", len(text.strip()))
    if pdf_spec.get("require_first_page_nonblank", True):
        _add_check(checks, "export.pdf.first_page_nonblank", first_page_nonblank, True, first_page_nonblank)
    return result


def _docx_package(path: Path) -> dict[str, str]:
    try:
        with ZipFile(path) as archive:
            return {
                name: archive.read(name).decode("utf-8", errors="ignore")
                for name in archive.namelist()
                if name.endswith(".xml")
            }
    except BadZipFile:
        return {}


def _docx_text(path: Path) -> str:
    document = Document(path)
    parts = [paragraph.text for paragraph in document.paragraphs]
    for table in document.tables:
        for row in table.rows:
            parts.extend(cell.text for cell in row.cells)
    return "\n".join(parts)


def _docx_table_count(path: Path) -> int:
    return len(Document(path).tables)


def _zip_docx_openable(archive: ZipFile, name: str) -> bool:
    try:
        with ZipFile(_bytes_buffer(archive.read(name))) as docx_archive:
            return "word/document.xml" in docx_archive.namelist()
    except BadZipFile:
        return False


def _bytes_buffer(value: bytes):
    from io import BytesIO

    return BytesIO(value)


def _first_page_nonblank(document: fitz.Document) -> bool:
    if document.page_count <= 0:
        return False
    pixmap = document[0].get_pixmap(matrix=fitz.Matrix(0.5, 0.5), alpha=False)
    samples = pixmap.samples
    if not samples:
        return False
    step = max(len(samples) // 12000, 1)
    return any(byte < 245 for byte in samples[::step])


def _layout(spec: dict[str, Any]) -> ExportLayoutOptions:
    return ExportLayoutOptions(**_dict_value(spec.get("layout")))


def _chapters(spec: dict[str, Any]) -> list[ExportChapter]:
    return [ExportChapter(**item) for item in _dict_items(spec.get("chapters"))]


def _parts(spec: dict[str, Any]) -> list[ExportPart]:
    parts = _dict_items(_dict_value(spec.get("zip")).get("parts"))
    if parts:
        return [ExportPart(**item) for item in parts]
    return [
        ExportPart(
            code="main",
            title=_text(spec.get("part_title"), "技术标"),
            chapters=_chapters(spec),
        )
    ]


def _attachments(value: Any) -> list[ExportAttachment]:
    return [ExportAttachment(**item) for item in _dict_items(value)]


def _dict_value(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def _dict_items(value: Any) -> list[dict[str, Any]]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def _list_value(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def _int_value(value: Any, default: int) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def _text(value: Any, default: str = "") -> str:
    text = "" if value is None else str(value).strip()
    return text or default


def _check_slug(value: str) -> str:
    slug = "".join(char if char.isalnum() else "-" for char in value.strip().lower())
    return slug.strip("-")[:48] or "value"


def _short_actual(value: str) -> str:
    normalized = " ".join(value.split())
    return normalized[:220]


def _add_check(checks: list[dict[str, Any]], name: str, passed: bool, expected: Any, actual: Any) -> None:
    checks.append({"name": name, "passed": bool(passed), "expected": expected, "actual": actual})


def main() -> int:
    parser = argparse.ArgumentParser(description="Evaluate DOCX/PDF/ZIP export format regression.")
    parser.add_argument("--input", required=True, type=Path, help="Path to export format JSON.")
    parser.add_argument("--json", action="store_true", help="Print full JSON result.")
    args = parser.parse_args()

    result = evaluate_export_format(args.input)
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(f"{result['status']} passed={result['passed_checks']}/{result['total_checks']} name={result['name']}")
        for check in result["checks"]:
            if not check["passed"]:
                print(f"- {check['name']}: expected={check['expected']!r} actual={check['actual']!r}")
    return 0 if result["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())

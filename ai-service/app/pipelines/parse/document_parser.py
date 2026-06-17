from __future__ import annotations

import base64
import json
import os
import re
import shutil
import subprocess
from itertools import zip_longest
from io import BytesIO
from pathlib import Path
from tempfile import TemporaryDirectory
from urllib import error, request

import fitz
from openpyxl import load_workbook
from docx import Document as DocxDocument
from pptx import Presentation

from app.schemas.knowledge import KnowledgeChunk, KnowledgeProcessRequest, KnowledgeProcessResult


IMAGE_SUFFIXES = {".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff"}
LEGACY_OFFICE_TARGETS = {
    ".doc": ".docx",
    ".xls": ".xlsx",
    ".ppt": ".pptx",
}
DEFAULT_OCR_RESPONSE_MAX_BYTES = 8 * 1024 * 1024


class OCRResponseTooLargeError(Exception):
    pass


def _env_int(name: str, default: int, minimum: int = 1) -> int:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        value = int(raw)
    except ValueError:
        return default
    return value if value >= minimum else default


def parse_document(payload: KnowledgeProcessRequest, content: bytes) -> KnowledgeProcessResult:
    suffix = Path(payload.filename).suffix.lower()
    content_type = payload.content_type.lower()
    metadata: dict[str, object] = {
        "content_type": payload.content_type,
        "object_key": payload.object_key,
    }
    if "pdf" in content_type or suffix == ".pdf":
        text, page_count, pdf_metadata = _parse_pdf(payload, content)
        metadata.update(pdf_metadata)
        if not text.strip():
            ocr_result = _try_http_ocr(payload, content)
            if ocr_result["status"] == "done":
                text = str(ocr_result.get("text") or "")
                ocr_result = {key: value for key, value in ocr_result.items() if key != "text"}
            metadata["ocr"] = ocr_result
            metadata["ocr_required"] = not bool(text.strip())
        parser = "pymupdf"
    elif suffix in LEGACY_OFFICE_TARGETS:
        text, page_count, legacy_metadata = _parse_legacy_office(payload, content, suffix)
        metadata.update(legacy_metadata)
        parser = str(legacy_metadata.get("parser") or "libreoffice-conversion")
    elif "word" in content_type or suffix == ".docx":
        text, parse_metadata = _parse_docx(content)
        page_count = None
        metadata.update(parse_metadata)
        parser = "python-docx"
    elif "spreadsheet" in content_type or suffix in {".xlsx", ".xlsm"}:
        text, parse_metadata = _parse_xlsx(content)
        page_count = None
        metadata.update(parse_metadata)
        parser = "openpyxl"
    elif "presentation" in content_type or suffix in {".pptx", ".pptm"}:
        text, parse_metadata = _parse_pptx(content)
        page_count = None
        metadata.update(parse_metadata)
        parser = "python-pptx"
    elif content_type.startswith("image/") or suffix in IMAGE_SUFFIXES:
        ocr_result = _try_http_ocr(payload, content)
        text = str(ocr_result.get("text") or "") if ocr_result["status"] == "done" else ""
        metadata["ocr"] = {key: value for key, value in ocr_result.items() if key != "text"}
        metadata["ocr_required"] = not bool(text.strip())
        page_count = None
        parser = "image-ocr"
    else:
        text, page_count = _parse_text(content), None
        parser = "plain-text"

    chunk_limit = _env_int("KNOWLEDGE_PARSE_MAX_CHUNKS", 300)
    chunks = _chunk_text(text, payload.filename, page_count, chunk_limit)
    summary = _summary(text, payload.filename)
    metadata.update(
        {
            "parser": parser,
            "page_count": page_count,
            "chunk_count": len(chunks),
            "chunk_limit": chunk_limit,
            "truncated_after_chunk_limit": any(
                bool(chunk.metadata.get("truncated_after_chunk_limit")) for chunk in chunks
            ),
        }
    )
    return KnowledgeProcessResult(
        processed_title=Path(payload.filename).stem or payload.filename,
        summary=summary,
        chunks=chunks,
        metadata=metadata,
    )


def _parse_text(content: bytes) -> str:
    for encoding in ("utf-8", "utf-16", "gb18030"):
        try:
            return content.decode(encoding)
        except UnicodeDecodeError:
            continue
    return content.decode("utf-8", errors="replace")


def _parse_pdf(payload: KnowledgeProcessRequest, content: bytes) -> tuple[str, int, dict[str, object]]:
    doc = fitz.open(stream=content, filetype="pdf")
    try:
        page_limit = _env_int("KNOWLEDGE_PARSE_MAX_PDF_PAGES", 300)
        parsed_page_count = min(doc.page_count, page_limit)
        page_texts: list[str] = []
        layout_blocks: list[dict[str, object]] = []
        tables: list[dict[str, object]] = []
        table_errors: list[dict[str, object]] = []
        for page_index in range(1, parsed_page_count + 1):
            page = doc[page_index - 1]
            page_text = page.get_text("text").strip()
            page_texts.append(page_text)
            layout_blocks.extend(_extract_pdf_layout_blocks(page, page_index))
            page_tables, page_table_errors = _extract_pdf_tables(page, page_index, page_text)
            tables.extend(page_tables)
            table_errors.extend(page_table_errors)
        pages: list[str] = []
        page_ocr_results: list[dict[str, object]] = []
        has_text_layer = any(text.strip() for text in page_texts)
        for page_index, page_text in enumerate(page_texts, start=1):
            if page_text:
                pages.append(f"[Page {page_index}]\n{page_text}")
                continue
            if not has_text_layer:
                continue
            ocr_result = _try_pdf_page_ocr(payload, doc[page_index - 1], page_index)
            ocr_text = str(ocr_result.get("text") or "")
            ocr_metadata = {key: value for key, value in ocr_result.items() if key != "text"}
            ocr_metadata["page"] = page_index
            page_ocr_results.append(ocr_metadata)
            if ocr_result.get("status") == "done" and ocr_text.strip():
                pages.append(f"[Page {page_index} OCR]\n{ocr_text.strip()}")
        text = "\n\n".join(pages)
        metadata = {
            "layout_blocks": layout_blocks[:200],
            "layout_block_count": len(layout_blocks),
            "tables": tables[:50],
            "table_count": len(tables),
            "table_extraction_errors": table_errors[:20],
            "table_extraction_error_count": len(table_errors),
            "ocr_pages": page_ocr_results[:50],
            "ocr_page_count": len(page_ocr_results),
            "ocr_required": _pdf_ocr_required(text, page_ocr_results),
            "parsed_page_count": parsed_page_count,
            "page_limit": page_limit,
            "truncated_after_page_limit": doc.page_count > parsed_page_count,
        }
        return text, doc.page_count, metadata
    finally:
        doc.close()


def _pdf_ocr_required(text: str, page_ocr_results: list[dict[str, object]]) -> bool:
    if not text.strip():
        return True
    return any(result.get("status") != "done" for result in page_ocr_results)


def _try_pdf_page_ocr(payload: KnowledgeProcessRequest, page: fitz.Page, page_index: int) -> dict[str, object]:
    if not os.getenv("OCR_HTTP_ENDPOINT", "").strip():
        return {"status": "provider_not_configured", "provider": _ocr_provider_name()}
    try:
        pixmap = page.get_pixmap(matrix=fitz.Matrix(2, 2), alpha=False)
        page_image = pixmap.tobytes("png")
    except Exception as exc:
        return {
            "status": "failed",
            "provider": _ocr_provider_name(),
            "error": "pdf page render failed",
            "error_type": type(exc).__name__,
        }
    page_payload = payload.model_copy(
        update={
            "filename": f"{Path(payload.filename).stem or 'page'}-page-{page_index}.png",
            "content_type": "image/png",
        }
    )
    return _try_http_ocr(page_payload, page_image)


def _extract_pdf_layout_blocks(page: fitz.Page, page_index: int) -> list[dict[str, object]]:
    raw = page.get_text("dict")
    blocks: list[dict[str, object]] = []
    for block in raw.get("blocks", []):
        if block.get("type") != 0:
            continue
        lines: list[str] = []
        for line in block.get("lines", []):
            line_text = "".join(span.get("text", "") for span in line.get("spans", [])).strip()
            if line_text:
                lines.append(line_text)
        text = "\n".join(lines).strip()
        if not text:
            continue
        blocks.append(
            {
                "page": page_index,
                "bbox": [round(float(value), 2) for value in block.get("bbox", [])],
                "text": text[:1000],
            }
        )
    return blocks


def _extract_pdf_tables(
    page: fitz.Page,
    page_index: int,
    page_text: str,
) -> tuple[list[dict[str, object]], list[dict[str, object]]]:
    tables: list[dict[str, object]] = []
    errors: list[dict[str, object]] = []
    if hasattr(page, "find_tables"):
        try:
            max_rows = _env_int("KNOWLEDGE_PARSE_MAX_TABLE_ROWS", 200)
            found = page.find_tables()
            for table_index, table in enumerate(found.tables, start=1):
                rows = table.extract()
                cleaned_rows: list[list[str]] = []
                for row in rows[:max_rows]:
                    values = [str(cell or "").strip() for cell in row]
                    if any(values):
                        cleaned_rows.append(values)
                if cleaned_rows:
                    table_payload: dict[str, object] = {"page": page_index, "index": table_index, "rows": cleaned_rows}
                    if len(rows) > max_rows:
                        table_payload["truncated_after_row_limit"] = True
                        table_payload["row_limit"] = max_rows
                    tables.append(table_payload)
        except Exception as exc:
            errors.append(
                {
                    "page": page_index,
                    "extractor": "pymupdf",
                    "error_type": type(exc).__name__,
                    "message": "PDF 表格结构识别失败，已改用文本行识别",
                }
            )
    if tables:
        return tables, errors

    heuristic_rows: list[list[str]] = []
    max_rows = _env_int("KNOWLEDGE_PARSE_MAX_TABLE_ROWS", 200)
    for line in page_text.splitlines():
        cells = [cell.strip() for cell in re.split(r"\t+|\s{2,}|\s*\|\s*", line.strip()) if cell.strip()]
        if len(cells) >= 2:
            heuristic_rows.append(cells)
            if len(heuristic_rows) >= max_rows:
                break
    if len(heuristic_rows) >= 2:
        table_payload = {"page": page_index, "index": 1, "rows": heuristic_rows, "extraction": "heuristic"}
        if len(heuristic_rows) >= max_rows:
            table_payload["truncated_after_row_limit"] = True
            table_payload["row_limit"] = max_rows
        tables.append(table_payload)
    return tables, errors


def _try_http_ocr(payload: KnowledgeProcessRequest, content: bytes) -> dict[str, object]:
    endpoint = os.getenv("OCR_HTTP_ENDPOINT", "").strip()
    provider = _ocr_provider_name()
    if not endpoint:
        return {"status": "provider_not_configured", "provider": provider}
    max_bytes = _env_int("OCR_MAX_BYTES", 20 * 1024 * 1024)
    if len(content) > max_bytes:
        return {
            "status": "skipped_too_large",
            "provider": provider,
            "size_bytes": len(content),
            "max_bytes": max_bytes,
        }
    body = json.dumps(
        {
            "filename": payload.filename,
            "content_type": payload.content_type,
            "provider": provider,
            "content_base64": base64.b64encode(content).decode("ascii"),
        },
        ensure_ascii=False,
    ).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    api_key = os.getenv("OCR_API_KEY", "").strip()
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    req = request.Request(
        endpoint,
        data=body,
        method="POST",
        headers=headers,
    )
    try:
        with request.urlopen(req, timeout=_env_int("OCR_HTTP_TIMEOUT_S", 120)) as response:
            max_response_bytes = _env_int(
                "OCR_MAX_RESPONSE_BYTES",
                DEFAULT_OCR_RESPONSE_MAX_BYTES,
                minimum=1024,
            )
            result = json.loads(_read_limited_response(response, max_response_bytes).decode("utf-8"))
        if not isinstance(result, dict):
            return {
                "status": "failed",
                "provider": provider,
                "error": "ocr response invalid",
            }
        text = str(result.get("text") or "")
        return {
            "status": "done" if text.strip() else "empty_result",
            "provider": provider,
            "text": text,
            "metadata": result.get("metadata", {}),
        }
    except error.HTTPError as exc:
        return {
            "status": "failed",
            "provider": provider,
            "error": "ocr request failed",
            "http_status": exc.code,
        }
    except OCRResponseTooLargeError:
        return {
            "status": "failed",
            "provider": provider,
            "error": "ocr response too large",
        }
    except Exception:
        return {"status": "failed", "provider": provider, "error": "ocr request failed"}


def _read_limited_response(response: object, max_bytes: int) -> bytes:
    content = response.read(max_bytes + 1)
    if len(content) > max_bytes:
        raise OCRResponseTooLargeError
    return content


def _ocr_provider_name() -> str:
    return os.getenv("OCR_PROVIDER", "http_ocr").strip() or "http_ocr"


def _parse_docx(content: bytes) -> tuple[str, dict[str, object]]:
    document = DocxDocument(BytesIO(content))
    paragraph_limit = _env_int("KNOWLEDGE_PARSE_MAX_DOCX_PARAGRAPHS", 3000)
    table_row_limit = _env_int("KNOWLEDGE_PARSE_MAX_DOCX_TABLE_ROWS", 3000)
    paragraphs: list[str] = []
    paragraph_count = len(document.paragraphs)
    for paragraph in document.paragraphs[:paragraph_limit]:
        text = paragraph.text.strip()
        if text:
            paragraphs.append(text)
    table_rows: list[str] = []
    table_count = len(document.tables)
    parsed_table_rows = 0
    total_table_rows = sum(len(table.rows) for table in document.tables)
    for table_index, table in enumerate(document.tables, start=1):
        for row_index, row in enumerate(table.rows, start=1):
            if parsed_table_rows >= table_row_limit:
                break
            parsed_table_rows += 1
            values = [cell.text.strip().replace("\n", " ") for cell in row.cells if cell.text.strip()]
            if values:
                table_rows.append(f"[Table {table_index} Row {row_index}] " + " | ".join(values))
        if parsed_table_rows >= table_row_limit:
            break
    return "\n\n".join([*paragraphs, *table_rows]), {
        "docx_paragraph_count": paragraph_count,
        "docx_parsed_paragraph_count": min(paragraph_count, paragraph_limit),
        "docx_paragraph_limit": paragraph_limit,
        "docx_table_count": table_count,
        "docx_table_row_count": total_table_rows,
        "docx_parsed_table_row_count": parsed_table_rows,
        "docx_table_row_limit": table_row_limit,
        "truncated_after_parse_limit": paragraph_count > paragraph_limit or total_table_rows > table_row_limit,
    }


def _parse_xlsx(content: bytes) -> tuple[str, dict[str, object]]:
    workbook = load_workbook(BytesIO(content), read_only=True, data_only=True)
    formula_workbook = None
    lines: list[str] = []
    max_sheets = _env_int("KNOWLEDGE_PARSE_MAX_XLSX_SHEETS", 20)
    max_rows = _env_int("KNOWLEDGE_PARSE_MAX_XLSX_ROWS_PER_SHEET", 5000)
    max_columns = _env_int("KNOWLEDGE_PARSE_MAX_XLSX_COLUMNS", 80)
    truncated = False
    parsed_sheets = 0
    parsed_rows = 0
    try:
        formula_workbook = load_workbook(BytesIO(content), read_only=True, data_only=False)
        sheet_count = len(workbook.worksheets)
        sheets = workbook.worksheets
        formula_sheets = {sheet.title: sheet for sheet in formula_workbook.worksheets}
        for sheet in sheets[:max_sheets]:
            formula_sheet = formula_sheets.get(sheet.title)
            parsed_sheets += 1
            lines.append(f"[Sheet] {sheet.title}")
            data_rows = sheet.iter_rows(max_row=max_rows, max_col=max_columns, values_only=True)
            formula_rows = (
                formula_sheet.iter_rows(max_row=max_rows, max_col=max_columns, values_only=True)
                if formula_sheet
                else []
            )
            for row, formula_row in zip_longest(data_rows, formula_rows, fillvalue=()):
                values = [
                    _cell_text(value, formula_value)
                    for value, formula_value in zip_longest(row or (), formula_row or (), fillvalue=None)
                ]
                values = [value for value in values if value]
                if values:
                    lines.append(" | ".join(values))
                    parsed_rows += 1
            formula_max_row = (formula_sheet.max_row or 0) if formula_sheet else 0
            formula_max_column = (formula_sheet.max_column or 0) if formula_sheet else 0
            if (
                max(sheet.max_row or 0, formula_max_row) > max_rows
                or max(sheet.max_column or 0, formula_max_column) > max_columns
            ):
                truncated = True
        if sheet_count > max_sheets:
            truncated = True
    finally:
        workbook.close()
        if formula_workbook is not None:
            formula_workbook.close()
    return "\n".join(lines), {
        "xlsx_sheet_count": sheet_count,
        "xlsx_parsed_sheet_count": parsed_sheets,
        "xlsx_parsed_row_count": parsed_rows,
        "xlsx_sheet_limit": max_sheets,
        "xlsx_row_limit_per_sheet": max_rows,
        "xlsx_column_limit": max_columns,
        "truncated_after_parse_limit": truncated,
    }


def _parse_pptx(content: bytes) -> tuple[str, dict[str, object]]:
    presentation = Presentation(BytesIO(content))
    lines: list[str] = []
    max_slides = _env_int("KNOWLEDGE_PARSE_MAX_PPTX_SLIDES", 200)
    max_table_rows = _env_int("KNOWLEDGE_PARSE_MAX_PPTX_TABLE_ROWS", 1000)
    slide_count = len(presentation.slides)
    parsed_slides = 0
    parsed_table_rows = 0
    truncated = slide_count > max_slides
    for slide_index, slide in enumerate(presentation.slides, start=1):
        if parsed_slides >= max_slides:
            break
        parsed_slides += 1
        lines.append(f"[Slide {slide_index}]")
        for shape in slide.shapes:
            if getattr(shape, "has_text_frame", False) and shape.text_frame:
                text = shape.text_frame.text.strip()
                if text:
                    lines.append(text)
            if getattr(shape, "has_table", False):
                for row in shape.table.rows:
                    if parsed_table_rows >= max_table_rows:
                        truncated = True
                        break
                    parsed_table_rows += 1
                    values = [cell.text.strip().replace("\n", " ") for cell in row.cells if cell.text.strip()]
                    if values:
                        lines.append(" | ".join(values))
    return "\n".join(lines), {
        "pptx_slide_count": slide_count,
        "pptx_parsed_slide_count": parsed_slides,
        "pptx_slide_limit": max_slides,
        "pptx_parsed_table_row_count": parsed_table_rows,
        "pptx_table_row_limit": max_table_rows,
        "truncated_after_parse_limit": truncated,
    }


def _parse_legacy_office(
    payload: KnowledgeProcessRequest,
    content: bytes,
    suffix: str,
) -> tuple[str, int | None, dict[str, object]]:
    target_suffix = LEGACY_OFFICE_TARGETS[suffix]
    conversion = _convert_with_libreoffice(payload.filename, content, target_suffix)
    metadata: dict[str, object] = {"legacy_conversion": conversion}
    if conversion.get("status") != "done":
        metadata["parser"] = "legacy-office-unconverted"
        metadata["needs_human_input"] = True
        return "", None, metadata
    converted = conversion.get("content")
    if not isinstance(converted, bytes):
        metadata["parser"] = "legacy-office-unconverted"
        metadata["needs_human_input"] = True
        return "", None, metadata
    conversion = {key: value for key, value in conversion.items() if key != "content"}
    metadata["legacy_conversion"] = conversion
    if target_suffix == ".docx":
        metadata["parser"] = "libreoffice-python-docx"
        text, parse_metadata = _parse_docx(converted)
        metadata.update(parse_metadata)
        return text, None, metadata
    if target_suffix == ".xlsx":
        metadata["parser"] = "libreoffice-openpyxl"
        text, parse_metadata = _parse_xlsx(converted)
        metadata.update(parse_metadata)
        return text, None, metadata
    if target_suffix == ".pptx":
        metadata["parser"] = "libreoffice-python-pptx"
        text, parse_metadata = _parse_pptx(converted)
        metadata.update(parse_metadata)
        return text, None, metadata
    metadata["parser"] = "legacy-office-unconverted"
    metadata["needs_human_input"] = True
    return "", None, metadata


def _convert_with_libreoffice(filename: str, content: bytes, target_suffix: str) -> dict[str, object]:
    executable = _libreoffice_convert_executable()
    if not executable:
        return {"status": "converter_not_configured", "target_suffix": target_suffix}
    source_name = Path(filename).name or f"input{target_suffix}"
    timeout = _env_int("LIBREOFFICE_CONVERT_TIMEOUT_S", 120)
    with TemporaryDirectory(prefix="zbt-ai-parse-") as tmpdir:
        source_path = Path(tmpdir) / source_name
        source_path.write_bytes(content)
        try:
            completed = subprocess.run(
                [
                    executable,
                    "--headless",
                    "--convert-to",
                    target_suffix.lstrip("."),
                    "--outdir",
                    tmpdir,
                    str(source_path),
                ],
                check=False,
                capture_output=True,
                text=True,
                timeout=timeout,
            )
        except Exception as exc:
            return {
                "status": "failed",
                "target_suffix": target_suffix,
                "error": "conversion_failed",
                "error_type": type(exc).__name__,
            }
        if completed.returncode != 0:
            return {
                "status": "failed",
                "target_suffix": target_suffix,
                "error": "conversion_failed",
                "return_code": completed.returncode,
            }
        converted_files = [
            path
            for path in Path(tmpdir).iterdir()
            if path.suffix.lower() == target_suffix and path.name != source_path.name
        ]
        if not converted_files:
            return {"status": "failed", "target_suffix": target_suffix, "error": "converted file not found"}
        converted_path = max(converted_files, key=lambda path: path.stat().st_mtime)
        return {
            "status": "done",
            "target_suffix": target_suffix,
            "filename": converted_path.name,
            "content": converted_path.read_bytes(),
        }


def _libreoffice_convert_executable() -> str | None:
    return (
        os.getenv("LIBREOFFICE_BIN", "").strip()
        or os.getenv("LIBREOFFICE_PATH", "").strip()
        or shutil.which("soffice")
        or shutil.which("libreoffice")
    )


def _cell_text(value: object, formula_value: object | None = None) -> str:
    if value is None:
        value = formula_value
    if value is None:
        return ""
    return str(value).strip()


def _chunk_text(
    text: str,
    filename: str,
    page_count: int | None,
    max_chunks: int,
) -> list[KnowledgeChunk]:
    normalized = "\n".join(line.strip() for line in text.splitlines() if line.strip())
    if not normalized:
        return [
            KnowledgeChunk(
                title=filename,
                content=f"{filename} 暂未解析出文本内容，需要人工确认或 OCR。",
                section_path=filename,
                metadata={"needs_human_input": True, "reason": "empty_parse_result"},
            )
        ]

    chunks: list[KnowledgeChunk] = []
    max_chars = 1200
    cursor = 0
    while cursor < len(normalized) and len(chunks) < max_chunks:
        end = min(len(normalized), cursor + max_chars)
        if end < len(normalized):
            boundary = normalized.rfind("\n", cursor, end)
            if boundary > cursor + 300:
                end = boundary
        chunk_text = normalized[cursor:end].strip()
        if chunk_text:
            index = len(chunks) + 1
            page_start = 1 if page_count else None
            page_end = page_count if page_count else None
            chunks.append(
                KnowledgeChunk(
                    title=f"{Path(filename).stem or filename} #{index}",
                    content=chunk_text,
                    section_path=Path(filename).stem or filename,
                    page_start=page_start,
                    page_end=page_end,
                    metadata={"chunk_index": index - 1, "parser_max_chars": max_chars},
                )
            )
        cursor = max(end, cursor + 1)
    if cursor < len(normalized) and chunks:
        chunks[-1].metadata["truncated_after_chunk_limit"] = True
        chunks[-1].metadata["remaining_chars_estimate"] = len(normalized) - cursor
    return chunks


def _summary(text: str, filename: str) -> str:
    compact = " ".join(text.split())
    if not compact:
        return f"{filename} 暂未解析出文本内容，需要人工确认或 OCR。"
    return compact[:180]

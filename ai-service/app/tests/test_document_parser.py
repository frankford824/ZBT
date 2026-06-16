from io import BytesIO
import urllib.error

import fitz
from docx import Document
from openpyxl import Workbook
from pptx import Presentation

from app.pipelines.parse.document_parser import _env_int, _extract_pdf_tables, parse_document
from app.schemas.knowledge import KnowledgeProcessRequest


def test_pdf_parser_extracts_layout_blocks_and_table_candidates() -> None:
    pdf = fitz.open()
    page = pdf.new_page()
    page.insert_text((72, 72), "Item    Amount")
    page.insert_text((72, 96), "Equipment    1200")
    page.insert_text((72, 140), "Implementation plan")
    content = pdf.tobytes()
    pdf.close()

    result = parse_document(_request("layout.pdf"), content)
    text = "\n".join(chunk.content for chunk in result.chunks)

    assert result.metadata["parser"] == "pymupdf"
    assert result.metadata["layout_block_count"] >= 1
    assert result.metadata["table_count"] >= 1
    assert result.metadata["ocr_required"] is False
    assert "Equipment" in text


def test_pdf_table_extraction_error_keeps_heuristic_tables_without_sensitive_details() -> None:
    class BrokenTablePage:
        def find_tables(self) -> object:
            raise RuntimeError("secret table payload fragment")

    tables, errors = _extract_pdf_tables(
        BrokenTablePage(),
        2,
        "Item    Amount\nEquipment    1200",
    )

    assert tables == [
        {
            "page": 2,
            "index": 1,
            "rows": [["Item", "Amount"], ["Equipment", "1200"]],
            "extraction": "heuristic",
        }
    ]
    assert errors == [
        {
            "page": 2,
            "extractor": "pymupdf",
            "error_type": "RuntimeError",
            "message": "PDF 表格结构识别失败，已改用文本行识别",
        }
    ]
    assert "secret table" not in str(errors)


def test_empty_pdf_marks_ocr_required_without_claiming_success(monkeypatch) -> None:
    monkeypatch.delenv("OCR_HTTP_ENDPOINT", raising=False)
    pdf = fitz.open()
    pdf.new_page()
    content = pdf.tobytes()
    pdf.close()

    result = parse_document(_request("scan.pdf"), content)

    assert result.metadata["parser"] == "pymupdf"
    assert result.metadata["ocr_required"] is True
    assert result.metadata["ocr"]["status"] == "provider_not_configured"
    assert result.chunks[0].metadata["needs_human_input"] is True


def test_empty_pdf_clears_ocr_required_after_successful_ocr(monkeypatch) -> None:
    monkeypatch.setattr(
        "app.pipelines.parse.document_parser._try_http_ocr",
        lambda _payload, _content: {"status": "done", "provider": "fake_ocr", "text": "扫描件识别文本"},
    )
    pdf = fitz.open()
    pdf.new_page()
    content = pdf.tobytes()
    pdf.close()

    result = parse_document(_request("scan.pdf"), content)
    text = "\n".join(chunk.content for chunk in result.chunks)

    assert result.metadata["ocr_required"] is False
    assert result.metadata["ocr"]["status"] == "done"
    assert "扫描件识别文本" in text


def test_image_parser_uses_ocr_boundary_without_falling_back_to_plain_text(monkeypatch) -> None:
    monkeypatch.delenv("OCR_HTTP_ENDPOINT", raising=False)

    result = parse_document(_request("scan.png"), b"not-a-real-image")

    assert result.metadata["parser"] == "image-ocr"
    assert result.metadata["ocr_required"] is True
    assert result.metadata["ocr"]["status"] == "provider_not_configured"
    assert result.chunks[0].metadata["needs_human_input"] is True


def test_image_parser_clears_ocr_required_after_successful_ocr(monkeypatch) -> None:
    monkeypatch.setattr(
        "app.pipelines.parse.document_parser._try_http_ocr",
        lambda _payload, _content: {"status": "done", "provider": "fake_ocr", "text": "图片识别文本"},
    )

    result = parse_document(_request("scan.png"), b"image-bytes")
    text = "\n".join(chunk.content for chunk in result.chunks)

    assert result.metadata["parser"] == "image-ocr"
    assert result.metadata["ocr_required"] is False
    assert result.metadata["ocr"]["status"] == "done"
    assert "图片识别文本" in text


def test_ocr_http_error_metadata_does_not_expose_response_body(monkeypatch) -> None:
    monkeypatch.setenv("OCR_HTTP_ENDPOINT", "https://ocr.example.test/parse")

    def raise_http_error(*_args, **_kwargs):
        raise urllib.error.HTTPError(
            "https://ocr.example.test/parse",
            502,
            "Bad Gateway",
            {},
            BytesIO(b'{"error":"secret OCR payload fragment"}'),
        )

    monkeypatch.setattr("app.pipelines.parse.document_parser.request.urlopen", raise_http_error)

    result = parse_document(_request("scan.png"), b"image-bytes")

    assert result.metadata["ocr"]["status"] == "failed"
    assert result.metadata["ocr"]["error"] == "ocr request failed"
    assert result.metadata["ocr"]["http_status"] == 502
    assert "secret OCR" not in str(result.metadata["ocr"])


def test_ocr_skips_oversized_content_without_external_request(monkeypatch) -> None:
    monkeypatch.setenv("OCR_HTTP_ENDPOINT", "https://ocr.example.test/parse")
    monkeypatch.setenv("OCR_MAX_BYTES", "4")

    def fail_if_called(*_args, **_kwargs):
        raise AssertionError("oversized OCR content must not be sent")

    monkeypatch.setattr("app.pipelines.parse.document_parser.request.urlopen", fail_if_called)

    result = parse_document(_request("scan.png"), b"image-bytes")

    assert result.metadata["ocr_required"] is True
    assert result.metadata["ocr"]["status"] == "skipped_too_large"
    assert result.metadata["ocr"]["size_bytes"] == len(b"image-bytes")
    assert result.metadata["ocr"]["max_bytes"] == 4


def test_legacy_office_marks_human_input_when_converter_missing(monkeypatch) -> None:
    monkeypatch.delenv("LIBREOFFICE_BIN", raising=False)
    monkeypatch.setattr("app.pipelines.parse.document_parser.shutil.which", lambda _name: None)

    result = parse_document(_request("legacy.doc"), b"legacy-binary")

    assert result.metadata["parser"] == "legacy-office-unconverted"
    assert result.metadata["legacy_conversion"]["status"] == "converter_not_configured"
    assert result.metadata["needs_human_input"] is True
    assert result.chunks[0].metadata["needs_human_input"] is True


def test_docx_parser_includes_paragraphs_and_tables() -> None:
    document = Document()
    document.add_paragraph("项目总体方案")
    table = document.add_table(rows=1, cols=2)
    table.cell(0, 0).text = "工期"
    table.cell(0, 1).text = "90天"
    content = BytesIO()
    document.save(content)

    result = parse_document(_request("plan.docx"), content.getvalue())
    text = "\n".join(chunk.content for chunk in result.chunks)

    assert result.metadata["parser"] == "python-docx"
    assert "项目总体方案" in text
    assert "工期 | 90天" in text


def test_xlsx_parser_extracts_sheet_rows() -> None:
    workbook = Workbook()
    sheet = workbook.active
    sheet.title = "报价"
    sheet.append(["科目", "金额"])
    sheet.append(["设备", 1200])
    content = BytesIO()
    workbook.save(content)

    result = parse_document(_request("quote.xlsx"), content.getvalue())
    text = "\n".join(chunk.content for chunk in result.chunks)

    assert result.metadata["parser"] == "openpyxl"
    assert "[Sheet] 报价" in text
    assert "设备 | 1200" in text


def test_pptx_parser_extracts_slide_text() -> None:
    presentation = Presentation()
    slide = presentation.slides.add_slide(presentation.slide_layouts[5])
    slide.shapes.title.text = "实施路线"
    textbox = slide.shapes.add_textbox(0, 0, 1000000, 1000000)
    textbox.text = "里程碑计划"
    content = BytesIO()
    presentation.save(content)

    result = parse_document(_request("deck.pptx"), content.getvalue())
    text = "\n".join(chunk.content for chunk in result.chunks)

    assert result.metadata["parser"] == "python-pptx"
    assert "实施路线" in text
    assert "里程碑计划" in text


def test_parse_document_marks_chunk_limit_truncation(monkeypatch) -> None:
    monkeypatch.setenv("KNOWLEDGE_PARSE_MAX_CHUNKS", "2")
    content = ("\n".join(f"段落 {index} " + "内容" * 40 for index in range(80))).encode()

    result = parse_document(_request("large.txt"), content)

    assert len(result.chunks) == 2
    assert result.metadata["chunk_count"] == 2
    assert result.metadata["chunk_limit"] == 2
    assert result.metadata["truncated_after_chunk_limit"] is True
    assert result.chunks[-1].metadata["truncated_after_chunk_limit"] is True


def test_timeout_env_parsing_falls_back_for_invalid_values(monkeypatch) -> None:
    monkeypatch.setenv("OCR_HTTP_TIMEOUT_S", "bad")
    assert _env_int("OCR_HTTP_TIMEOUT_S", 120) == 120

    monkeypatch.setenv("OCR_HTTP_TIMEOUT_S", "0")
    assert _env_int("OCR_HTTP_TIMEOUT_S", 120) == 120

    monkeypatch.setenv("OCR_HTTP_TIMEOUT_S", "30")
    assert _env_int("OCR_HTTP_TIMEOUT_S", 120) == 30


def _request(filename: str) -> KnowledgeProcessRequest:
    return KnowledgeProcessRequest(
        tenant_id="tenant-demo",
        document_id="doc-demo",
        file_id="file-demo",
        object_key=f"demo/{filename}",
        filename=filename,
        content_type="application/octet-stream",
    )

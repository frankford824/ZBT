from io import BytesIO

from docx import Document
from openpyxl import Workbook
from pptx import Presentation

from app.pipelines.parse.document_parser import parse_document
from app.schemas.knowledge import KnowledgeProcessRequest


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


def _request(filename: str) -> KnowledgeProcessRequest:
    return KnowledgeProcessRequest(
        tenant_id="tenant-demo",
        document_id="doc-demo",
        file_id="file-demo",
        object_key=f"demo/{filename}",
        filename=filename,
        content_type="application/octet-stream",
    )

from __future__ import annotations

import json
from io import BytesIO
from zipfile import ZipFile

from docx import Document

from app.pipelines.export.docx_exporter import export_bid_docx, export_bid_pdf, export_bid_zip
from app.schemas.export import ExportChapter, ExportLayoutOptions, ExportPart


def test_export_bid_docx_applies_master_layout(tmp_path, monkeypatch) -> None:
    monkeypatch.delenv("BID_EXPORT_TEMPLATE_PATH", raising=False)
    output = tmp_path / "bid.docx"
    layout = ExportLayoutOptions(watermark_text="内部评审", generated_at="2026-06-11")
    chapters = [
        ExportChapter(
            title="项目实施方案",
            plain_text=(
                "# 总体安排\n"
                "| 项目 | 内容 |\n"
                "| --- | --- |\n"
                "| 工期 | 90天 |\n\n"
                "- 进度控制\n"
                "1. 质量控制"
            ),
        )
    ]

    export_bid_docx("智慧交通平台", "技术标", chapters, output, layout=layout)

    document = Document(output)
    paragraph_text = "\n".join(paragraph.text for paragraph in document.paragraphs)
    header_text = "\n".join(paragraph.text for paragraph in document.sections[0].header.paragraphs)
    footer_text = "\n".join(paragraph.text for paragraph in document.sections[0].footer.paragraphs)
    package_xml = _package_xml(output)

    assert "智慧交通平台" in paragraph_text
    assert "投标文件" in paragraph_text
    assert "目录" in paragraph_text
    assert "技术标" in paragraph_text
    assert "项目实施方案" in paragraph_text
    assert "总体安排" in paragraph_text
    assert "智慧交通平台 - 技术标" in header_text
    assert "智标通投标文件导出" in footer_text
    assert len(document.tables) == 1
    assert document.tables[0].cell(1, 0).text == "工期"
    assert "TOC" in package_xml
    assert "updateFields" in package_xml
    assert "PAGE" in package_xml
    assert "NUMPAGES" in package_xml
    assert "内部评审" in package_xml


def test_export_bid_zip_uses_master_layout_for_each_part(tmp_path, monkeypatch) -> None:
    monkeypatch.delenv("BID_EXPORT_TEMPLATE_PATH", raising=False)
    output = tmp_path / "bid.zip"
    parts = [
        ExportPart(
            code="tech",
            title="技术标",
            chapters=[ExportChapter(title="实施计划", plain_text="项目实施内容。")],
        ),
        ExportPart(
            code="business",
            title="商务标",
            chapters=[ExportChapter(title="报价说明", plain_text="商务报价内容。")],
        ),
    ]

    export_bid_zip("智慧交通平台", parts, output)

    with ZipFile(output) as archive:
        names = sorted(archive.namelist())
        assert names == ["01-技术标.docx", "02-商务标.docx", "manifest.json"]
        first_docx = archive.read("01-技术标.docx")
        manifest = json.loads(archive.read("manifest.json").decode("utf-8"))

    with ZipFile(BytesIO(first_docx)) as document_archive:
        document_xml = document_archive.read("word/document.xml").decode("utf-8")
        settings_xml = document_archive.read("word/settings.xml").decode("utf-8")

    assert manifest["bid_title"] == "智慧交通平台"
    assert manifest["part_count"] == 2
    assert manifest["parts"][0]["filename"] == "01-技术标.docx"
    assert manifest["parts"][0]["chapter_count"] == 1
    assert len(manifest["parts"][0]["sha256"]) == 64
    assert "TOC" in document_xml
    assert "实施计划" in document_xml
    assert "updateFields" in settings_xml


def test_export_bid_docx_renders_template_placeholders_and_body_anchor(tmp_path, monkeypatch) -> None:
    template_path = tmp_path / "template.docx"
    template = Document()
    template.add_paragraph("项目：{{ bid_title }}")
    template.add_paragraph("分册：{{ part_title }}")
    template.add_paragraph("日期：{{ generated_at }}")
    template.add_paragraph("{{ZBT_BODY}}")
    template.save(template_path)
    monkeypatch.setenv("BID_EXPORT_TEMPLATE_PATH", str(template_path))
    output = tmp_path / "templated.docx"

    export_bid_docx(
        "智慧交通平台",
        "技术标",
        [ExportChapter(title="实施计划", plain_text="模板正文内容。")],
        output,
        layout=ExportLayoutOptions(include_cover=False, include_toc=False, generated_at="2026-06-12"),
    )

    document = Document(output)
    paragraph_text = "\n".join(paragraph.text for paragraph in document.paragraphs)

    assert "项目：智慧交通平台" in paragraph_text
    assert "分册：技术标" in paragraph_text
    assert "日期：2026-06-12" in paragraph_text
    assert "{{ZBT_BODY}}" not in paragraph_text
    assert "实施计划" in paragraph_text
    assert "模板正文内容。" in paragraph_text


def test_export_bid_pdf_reuses_master_docx_before_conversion(tmp_path, monkeypatch) -> None:
    monkeypatch.delenv("BID_EXPORT_TEMPLATE_PATH", raising=False)
    fake_soffice = tmp_path / "fake-soffice.sh"
    fake_soffice.write_text(
        """#!/bin/sh
outdir=""
input=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--outdir" ]; then
    shift
    outdir="$1"
  else
    input="$1"
  fi
  shift
done
cp "$input" "$outdir/source.pdf"
""",
        encoding="utf-8",
    )
    fake_soffice.chmod(0o755)
    monkeypatch.setenv("LIBREOFFICE_PATH", str(fake_soffice))
    output = tmp_path / "bid.pdf"

    export_bid_pdf(
        "智慧交通平台",
        "技术标",
        [ExportChapter(title="实施计划", plain_text="项目实施内容。")],
        output,
    )

    assert output.exists()
    assert "TOC" in _docx_xml(output, "word/document.xml")


def _package_xml(path) -> str:
    with ZipFile(path) as archive:
        return "\n".join(
            archive.read(name).decode("utf-8", errors="ignore")
            for name in archive.namelist()
            if name.endswith(".xml")
        )


def _docx_xml(path, name: str) -> str:
    with ZipFile(path) as archive:
        return archive.read(name).decode("utf-8")

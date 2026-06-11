from __future__ import annotations

from pathlib import Path
from tempfile import TemporaryDirectory
from zipfile import ZIP_DEFLATED, ZipFile

from docx import Document

from app.schemas.export import ExportChapter, ExportPart


def export_minimal_docx(title: str, paragraphs: list[str], output_path: Path) -> Path:
    document = Document()
    document.add_heading(title, level=1)
    for paragraph in paragraphs:
        document.add_paragraph(paragraph)
    document.save(output_path)
    return output_path


def export_bid_docx(title: str, part_title: str, chapters: list[ExportChapter], output_path: Path) -> Path:
    document = Document()
    document.add_heading(title, level=1)
    document.add_heading(part_title, level=2)
    for chapter in chapters:
        document.add_heading(chapter.title, level=3)
        text = chapter.plain_text.strip() or "本章节暂无内容，需要人工补充。"
        for paragraph in text.splitlines():
            if paragraph.strip():
                document.add_paragraph(paragraph.strip())
    document.save(output_path)
    return output_path


def export_bid_zip(title: str, parts: list[ExportPart], output_path: Path) -> Path:
    with TemporaryDirectory() as tmpdir:
        tmp_path = Path(tmpdir)
        with ZipFile(output_path, mode="w", compression=ZIP_DEFLATED) as archive:
            for index, part in enumerate(parts, start=1):
                docx_name = f"{index:02d}-{_safe_filename(part.title or part.code)}.docx"
                docx_path = tmp_path / docx_name
                export_bid_docx(title, part.title, part.chapters, docx_path)
                archive.write(docx_path, docx_name)
    return output_path


def _safe_filename(value: str) -> str:
    cleaned = value.strip() or "document"
    for char in '/\\:*?"<>|':
        cleaned = cleaned.replace(char, "-")
    return cleaned

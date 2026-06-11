from __future__ import annotations

from pathlib import Path

from docx import Document

from app.schemas.export import ExportChapter


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

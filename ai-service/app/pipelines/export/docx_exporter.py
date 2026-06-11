from __future__ import annotations

from pathlib import Path

from docx import Document


def export_minimal_docx(title: str, paragraphs: list[str], output_path: Path) -> Path:
    document = Document()
    document.add_heading(title, level=1)
    for paragraph in paragraphs:
        document.add_paragraph(paragraph)
    document.save(output_path)
    return output_path

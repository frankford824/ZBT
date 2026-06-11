from __future__ import annotations

from io import BytesIO
from pathlib import Path

import fitz
from docx import Document as DocxDocument

from app.schemas.knowledge import KnowledgeChunk, KnowledgeProcessRequest, KnowledgeProcessResult


def parse_document(payload: KnowledgeProcessRequest, content: bytes) -> KnowledgeProcessResult:
    suffix = Path(payload.filename).suffix.lower()
    content_type = payload.content_type.lower()
    if "pdf" in content_type or suffix == ".pdf":
        text, page_count = _parse_pdf(content)
        parser = "pymupdf"
    elif "word" in content_type or suffix in {".docx", ".doc"}:
        text, page_count = _parse_docx(content), None
        parser = "python-docx"
    else:
        text, page_count = _parse_text(content), None
        parser = "plain-text"

    chunks = _chunk_text(text, payload.filename, page_count)
    summary = _summary(text, payload.filename)
    return KnowledgeProcessResult(
        processed_title=Path(payload.filename).stem or payload.filename,
        summary=summary,
        chunks=chunks,
        metadata={
            "parser": parser,
            "content_type": payload.content_type,
            "object_key": payload.object_key,
            "page_count": page_count,
            "chunk_count": len(chunks),
        },
    )


def _parse_text(content: bytes) -> str:
    for encoding in ("utf-8", "utf-16", "gb18030"):
        try:
            return content.decode(encoding)
        except UnicodeDecodeError:
            continue
    return content.decode("utf-8", errors="replace")


def _parse_pdf(content: bytes) -> tuple[str, int]:
    doc = fitz.open(stream=content, filetype="pdf")
    pages: list[str] = []
    for page_index, page in enumerate(doc, start=1):
        page_text = page.get_text("text").strip()
        if page_text:
            pages.append(f"[Page {page_index}]\n{page_text}")
    return "\n\n".join(pages), doc.page_count


def _parse_docx(content: bytes) -> str:
    document = DocxDocument(BytesIO(content))
    paragraphs = [paragraph.text.strip() for paragraph in document.paragraphs if paragraph.text.strip()]
    return "\n\n".join(paragraphs)


def _chunk_text(text: str, filename: str, page_count: int | None) -> list[KnowledgeChunk]:
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
    while cursor < len(normalized):
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
    return chunks


def _summary(text: str, filename: str) -> str:
    compact = " ".join(text.split())
    if not compact:
        return f"{filename} 暂未解析出文本内容，需要人工确认或 OCR。"
    return compact[:180]

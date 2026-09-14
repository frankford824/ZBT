from __future__ import annotations

import json
from io import BytesIO

from PIL import Image, ImageDraw

from app.pipelines.parse.document_parser import parse_document
from app.schemas.knowledge import KnowledgeProcessRequest


def main() -> int:
    image = Image.new("RGB", (720, 180), "white")
    ImageDraw.Draw(image).text((30, 55), "ZBT OCR TEST 2026\nTender Document", fill="black", font_size=32)
    buffer = BytesIO()
    image.save(buffer, format="PNG")
    result = parse_document(
        KnowledgeProcessRequest(
            tenant_id="ocr-smoke-tenant",
            document_id="ocr-smoke-document",
            file_id="ocr-smoke-file",
            object_key="ocr-smoke/sample.png",
            filename="ocr-smoke.png",
            content_type="image/png",
        ),
        buffer.getvalue(),
    )
    text = "\n".join(chunk.content for chunk in result.chunks)
    passed = "ZBT OCR TEST 2026" in text and result.metadata.get("ocr_required") is False
    print(
        json.dumps(
            {
                "status": "passed" if passed else "failed",
                "recognized_marker": "ZBT OCR TEST 2026" in text,
                "chunk_count": len(result.chunks),
                "ocr_required": result.metadata.get("ocr_required"),
            },
            ensure_ascii=False,
        )
    )
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())

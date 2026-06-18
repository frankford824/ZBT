from __future__ import annotations

import json
from io import BytesIO
from pathlib import Path

from app.evaluation.ocr_provider_eval import evaluate_ocr_provider


class _FakeHTTPResponse:
    def __init__(self, body: bytes, status: int = 200) -> None:
        self._body = BytesIO(body)
        self.status = status
        self.code = status

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return None

    def read(self, size: int = -1) -> bytes:
        return self._body.read(size)

    def getcode(self) -> int:
        return self.status


def test_ocr_provider_eval_skips_when_endpoint_is_missing(monkeypatch, tmp_path: Path) -> None:
    sample = tmp_path / "scan.png"
    sample.write_bytes(b"image-bytes")
    monkeypatch.delenv("OCR_HTTP_ENDPOINT", raising=False)
    monkeypatch.delenv("MINERU_HTTP_ENDPOINT", raising=False)

    result = evaluate_ocr_provider("mineru", sample)

    assert result["status"] == "skipped"
    assert result["checks"][1]["name"] == "provider.endpoint_configured"
    assert result["checks"][1]["passed"] is False


def test_ocr_provider_eval_passes_against_mineru_http_response(monkeypatch, tmp_path: Path) -> None:
    sample = tmp_path / "scan.png"
    sample.write_bytes(b"image-bytes")
    captured: dict[str, object] = {}

    def fake_urlopen(req, timeout):
        captured["url"] = req.full_url
        captured["body"] = json.loads(req.data.decode("utf-8"))
        captured["authorization"] = req.get_header("Authorization")
        return _FakeHTTPResponse(
            """
            {
              "data": {
                "markdown": "# OCR\\n识别后的招标文件文本",
                "pages": [{"page": 1, "text": "识别后的招标文件文本", "confidence": 0.96}],
                "layout_blocks": [{"page": 1, "type": "title", "text": "OCR", "bbox": [0, 0, 200, 40]}],
                "tables": [
                  {
                    "page": 1,
                    "rows": [["项目", "分值"], ["方案", "20"]],
                    "bbox": [0, 0, 120, 60],
                    "cell_bboxes": [
                      [[0, 0, 60, 30], [60, 0, 120, 30]],
                      [[0, 30, 60, 60], [60, 30, 120, 60]]
                    ]
                  }
                ],
                "metadata": {"request_id": "mineru-eval"}
              }
            }
            """.encode("utf-8")
        )

    monkeypatch.setenv("MINERU_HTTP_ENDPOINT", "https://mineru.example.test/file_parse")
    monkeypatch.setenv("MINERU_API_KEY", "secret-token")
    monkeypatch.setattr("app.pipelines.parse.document_parser.request.urlopen", fake_urlopen)

    result = evaluate_ocr_provider(
        "mineru",
        sample,
        min_text_chars=8,
        min_table_blocks=1,
        min_page_confidence=0.9,
        min_layout_bbox_count=1,
        min_table_bbox_count=1,
        min_cell_bbox_count=4,
    )

    assert result["status"] == "passed"
    assert captured["url"] == "https://mineru.example.test/file_parse"
    assert captured["authorization"] == "Bearer secret-token"
    assert captured["body"]["provider"] == "mineru"
    assert result["metadata"]["ocr"]["provider_profile"]["endpoint_env"] == "MINERU_HTTP_ENDPOINT"
    assert result["passed_checks"] == result["total_checks"]


def test_ocr_provider_eval_fails_when_required_table_is_missing(monkeypatch, tmp_path: Path) -> None:
    sample = tmp_path / "scan.png"
    sample.write_bytes(b"image-bytes")

    monkeypatch.setenv("PADDLEOCR_HTTP_ENDPOINT", "https://paddle.example.test/parse")
    monkeypatch.setattr(
        "app.pipelines.parse.document_parser.request.urlopen",
        lambda *_args, **_kwargs: _FakeHTTPResponse('{"data":{"text":"只有文本没有表格"}}'.encode("utf-8")),
    )

    result = evaluate_ocr_provider("paddleocr", sample, min_text_chars=4, min_table_blocks=1)

    assert result["status"] == "failed"
    table_check = next(check for check in result["checks"] if check["name"] == "ocr.table_blocks")
    assert table_check["passed"] is False
    assert table_check["actual"] == 0


def test_ocr_provider_eval_fails_when_required_bbox_evidence_is_missing(monkeypatch, tmp_path: Path) -> None:
    sample = tmp_path / "scan.png"
    sample.write_bytes(b"image-bytes")

    monkeypatch.setenv("PADDLEOCR_HTTP_ENDPOINT", "https://paddle.example.test/parse")
    monkeypatch.setattr(
        "app.pipelines.parse.document_parser.request.urlopen",
        lambda *_args, **_kwargs: _FakeHTTPResponse(
            """
            {
              "data": {
                "pages": [{"page": 1, "text": "有文本但没有坐标", "confidence": 0.91}],
                "layout_blocks": [{"page": 1, "type": "title", "text": "标题"}],
                "tables": [{"page": 1, "rows": [["项目", "分值"], ["方案", "20"]]}]
              }
            }
            """.encode("utf-8")
        ),
    )

    result = evaluate_ocr_provider(
        "paddleocr",
        sample,
        min_text_chars=4,
        min_table_blocks=1,
        min_page_confidence=0.9,
        min_layout_bbox_count=1,
        min_table_bbox_count=1,
        min_cell_bbox_count=1,
    )

    assert result["status"] == "failed"
    failed = {check["name"]: check for check in result["checks"] if not check["passed"]}
    assert failed["ocr.layout_bbox_count"]["actual"] == 0
    assert failed["ocr.table_bbox_count"]["actual"] == 0
    assert failed["ocr.cell_bbox_count"]["actual"] == 0

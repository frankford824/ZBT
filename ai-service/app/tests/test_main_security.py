import asyncio
import hashlib
import hmac

import pytest
from fastapi import BackgroundTasks

from app.main import (
    DEFAULT_AI_HMAC_SECRET,
    DEFAULT_MINIO_ACCESS_KEY,
    DEFAULT_MINIO_SECRET_KEY,
    ai_service_hmac_secret,
    callback_allowed_hosts,
    ensure_callback_url_allowed,
    export_docx,
    knowledge_process,
    production_mode,
    safe_output_filename,
    tender_parse,
    validate_production_config,
    verify_request_signature,
)
from app.gateway.model_router import ModelRouter
from app.pipelines.parse.tender_parser import build_tender_structured_result
from app.schemas.export import DocumentExportRequest
from app.schemas.knowledge import KnowledgeChunk, KnowledgeProcessRequest, KnowledgeProcessResult
from app.schemas.tender import TenderParseRequest


def test_safe_output_filename_keeps_task_output_in_temp_directory() -> None:
    assert safe_output_filename("/etc/passwd", "pdf") == "passwd.pdf"
    assert safe_output_filename("..\\..\\投标文件?.docx", "docx") == "投标文件.docx"
    assert safe_output_filename("", "zip") == "export.zip"


def test_safe_output_filename_preserves_suffix_when_truncated() -> None:
    filename = safe_output_filename("a" * 200, "pdf")

    assert len(filename) == 120
    assert filename.endswith(".pdf")


def test_verify_request_signature_accepts_valid_body_signature() -> None:
    body = b'{"task":"demo"}'
    timestamp = "1800000000"
    signature = hmac.new(b"secret", timestamp.encode() + b"." + body, hashlib.sha256).hexdigest()

    assert verify_request_signature(timestamp, signature, body, "secret", now=1800000000)


def test_verify_request_signature_rejects_invalid_or_expired_signature() -> None:
    body = b'{"task":"demo"}'
    timestamp = "1800000000"

    assert not verify_request_signature(timestamp, "bad", body, "secret", now=1800000000)
    assert not verify_request_signature(timestamp, "bad", body, "secret", now=1800000400)


def test_ai_service_hmac_secret_has_development_default(monkeypatch) -> None:
    monkeypatch.delenv("AI_SERVICE_HMAC_SECRET", raising=False)

    assert ai_service_hmac_secret() == DEFAULT_AI_HMAC_SECRET


def test_ai_service_hmac_secret_treats_empty_value_as_unset(monkeypatch) -> None:
    monkeypatch.setenv("AI_SERVICE_HMAC_SECRET", "")

    assert ai_service_hmac_secret() == DEFAULT_AI_HMAC_SECRET


def test_ai_service_hmac_secret_allows_override(monkeypatch) -> None:
    monkeypatch.setenv("AI_SERVICE_HMAC_SECRET", "custom-secret")

    assert ai_service_hmac_secret() == "custom-secret"


def test_callback_url_defaults_allow_backend_and_local_hosts(monkeypatch) -> None:
    monkeypatch.delenv("AI_CALLBACK_ALLOWED_HOSTS", raising=False)

    assert "backend" in callback_allowed_hosts()
    ensure_callback_url_allowed("http://backend:8080/api/v1/ai/callbacks/tasks")
    ensure_callback_url_allowed("http://127.0.0.1:8080/api/v1/ai/callbacks/tasks")


def test_callback_url_rejects_non_http_or_unlisted_hosts(monkeypatch) -> None:
    monkeypatch.setenv("AI_CALLBACK_ALLOWED_HOSTS", "backend,internal.example")

    with pytest.raises(RuntimeError, match="absolute http"):
        ensure_callback_url_allowed("file:///etc/passwd")
    with pytest.raises(RuntimeError, match="not allowed"):
        ensure_callback_url_allowed("http://169.254.169.254/latest/meta-data")
    with pytest.raises(RuntimeError, match="not allowed"):
        ensure_callback_url_allowed("http://evil.example/callback")
    ensure_callback_url_allowed("https://internal.example/callback")


def test_knowledge_process_accepts_backend_task_id() -> None:
    payload = KnowledgeProcessRequest(
        task_id="task-knowledge-backend-owned",
        tenant_id="tenant-demo",
        document_id="doc-demo",
        file_id="file-demo",
        object_key="demo/doc.pdf",
        filename="doc.pdf",
        content_type="application/pdf",
    )

    accepted = asyncio.run(knowledge_process(payload, BackgroundTasks()))

    assert accepted.task_id == "task-knowledge-backend-owned"


def test_document_export_accepts_backend_task_id() -> None:
    payload = DocumentExportRequest(
        task_id="task-export-backend-owned",
        tenant_id="tenant-demo",
        export_id="export-demo",
        bid_id="bid-demo",
        bid_title="测试项目",
        part_code="tech",
        part_title="技术标",
        filename="测试项目.docx",
        object_key="tenant/bid_export/demo.docx",
    )

    accepted = asyncio.run(export_docx(payload, BackgroundTasks()))

    assert accepted.task_id == "task-export-backend-owned"


def test_tender_parse_accepts_backend_task_id() -> None:
    payload = TenderParseRequest(
        task_id="task-tender-parse-backend-owned",
        tenant_id="tenant-demo",
        bid_id="bid-demo",
        file_id="file-demo",
        object_key="tenant/bid_tender/demo.pdf",
        filename="demo.pdf",
        content_type="application/pdf",
    )

    accepted = asyncio.run(tender_parse(payload, BackgroundTasks()))

    assert accepted.task_id == "task-tender-parse-backend-owned"


def test_build_tender_structured_result_extracts_business_fields() -> None:
    payload = TenderParseRequest(
        tenant_id="tenant-demo",
        bid_id="bid-demo",
        bid_title="智慧交通平台建设",
        file_id="file-demo",
        object_key="tenant/bid_tender/demo.pdf",
        filename="demo.pdf",
        content_type="application/pdf",
    )
    parsed = KnowledgeProcessResult(
        processed_title="demo",
        summary="summary",
        metadata={"parser": "pymupdf", "page_count": 3, "table_count": 1},
        chunks=[
            KnowledgeChunk(
                title="招标公告",
                content=(
                    "项目名称：智慧交通平台建设\n"
                    "投标截止时间：2026年7月15日\n"
                    "资格要求：具备类似项目业绩和安全资质证书\n"
                    "无效投标：未按要求签章或投标有效期不足\n"
                    "评分标准：技术方案完整性和项目团队能力\n"
                    "第一章 技术方案\n第二章 商务响应"
                ),
                section_path="公告",
            )
        ],
    )

    result = build_tender_structured_result(payload, parsed)

    assert result["project_name"] == "智慧交通平台建设"
    assert result["deadline"] == "2026-07-15"
    assert result["bid_type"] == "separated"
    assert result["qualification_requirements"]
    assert result["invalid_clause_risks"]
    assert result["scoring_points"]
    assert isinstance(result["outline"], dict)


def test_production_mode_detects_release_environment(monkeypatch) -> None:
    monkeypatch.setenv("GIN_MODE", "release")

    assert production_mode()


def test_validate_production_config_rejects_development_secret(monkeypatch) -> None:
    monkeypatch.setenv("APP_ENV", "production")
    monkeypatch.delenv("AI_SERVICE_HMAC_SECRET", raising=False)

    with pytest.raises(RuntimeError, match="AI_SERVICE_HMAC_SECRET"):
        validate_production_config()


def test_validate_production_config_rejects_development_minio_credentials(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("MINIO_ACCESS_KEY", DEFAULT_MINIO_ACCESS_KEY)
    monkeypatch.setenv("MINIO_SECRET_KEY", DEFAULT_MINIO_SECRET_KEY)

    with pytest.raises(RuntimeError, match="MINIO_ACCESS_KEY"):
        validate_production_config()


def test_validate_production_config_rejects_mock_provider_mode(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "true")

    with pytest.raises(RuntimeError, match="USE_MOCK_PROVIDERS"):
        validate_production_config()


def test_validate_production_config_rejects_mock_model_routes(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")

    with pytest.raises(RuntimeError, match="MockProvider"):
        validate_production_config()


def test_validate_production_config_allows_explicit_production_config(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")
    monkeypatch.setattr(
        "app.main.router",
        ModelRouter(
            {
                "providers": {
                    "openai_compatible_primary": {
                        "type": "openai_compatible",
                        "base_url_env": "OPENAI_BASE_URL",
                        "api_key_env": "OPENAI_API_KEY",
                        "default_base_url": "https://example.test/v1",
                    }
                },
                "routes": {
                    "chapter_generate": {
                        "primary": {"provider": "openai_compatible_primary", "model": "real-model"}
                    }
                },
            }
        ),
    )

    validate_production_config()


def _set_production_security_env(monkeypatch) -> None:
    monkeypatch.setenv("APP_ENV", "production")
    monkeypatch.setenv("AI_SERVICE_HMAC_SECRET", "prod-ai-secret")
    monkeypatch.setenv("MINIO_ACCESS_KEY", "prod-minio-access")
    monkeypatch.setenv("MINIO_SECRET_KEY", "prod-minio-secret")
    monkeypatch.setenv("OPENAI_API_KEY", "prod-openai-key")

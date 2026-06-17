import asyncio
import hashlib
import hmac
import json
import threading
import time
from pathlib import Path
from zipfile import ZipFile

import pytest
from fastapi import BackgroundTasks
from starlette.requests import Request
from starlette.responses import JSONResponse

from app.main import (
    CallbackResponseTooLargeError,
    DEFAULT_AI_SERVICE_MAX_BODY_BYTES,
    MAX_AI_SERVICE_MAX_BODY_BYTES,
    DEFAULT_AI_HMAC_SECRET,
    DEFAULT_MINIO_ACCESS_KEY,
    DEFAULT_MINIO_SECRET_KEY,
    ai_service_max_body_bytes,
    ai_service_hmac_secret,
    callback_allowed_hosts,
    download_minio_object_base64,
    embed_knowledge_inputs,
    ensure_callback_url_allowed,
    ensure_tenant_object_key_allowed,
    export_docx,
    knowledge_embeddings,
    knowledge_process,
    knowledge_rerank,
    minio_client,
    normalize_minio_endpoint,
    process_document_export,
    process_knowledge_document,
    process_tender_parse,
    post_callback,
    production_mode,
    require_backend_signature,
    safe_output_filename,
    tender_parse,
    tender_parse_module_concurrency,
    validate_production_config,
    verify_request_signature,
)
from app.gateway.model_router import ModelRouter, RouteTarget
from app.pipelines.parse.tender_parser import build_tender_structured_result, merge_tender_structured_result
from app.schemas.export import DocumentExportRequest, ExportAttachment, ExportChapter, ExportPart
from app.schemas.knowledge import (
    KnowledgeChunk,
    KnowledgeEmbeddingRequest,
    KnowledgeProcessRequest,
    KnowledgeProcessResult,
    KnowledgeRerankDocument,
    KnowledgeRerankRequest,
)
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


def test_ai_service_middleware_keeps_only_public_paths_unsigned() -> None:
    assert asyncio.run(middleware_status("GET", "/healthz")) == 209
    assert asyncio.run(middleware_status("GET", "/not-found")) == 401
    assert asyncio.run(middleware_status("GET", "/not-found", signed_headers(b""))) == 209
    body = b'{"task":"demo"}'
    assert asyncio.run(middleware_status("POST", "/tasks/demo", signed_headers(body), body=body)) == 209


def test_ai_service_max_body_bytes_uses_safe_env_bounds(monkeypatch) -> None:
    monkeypatch.setenv("AI_SERVICE_MAX_BODY_BYTES", "2097152")
    assert ai_service_max_body_bytes() == 2 * 1024 * 1024

    monkeypatch.setenv("AI_SERVICE_MAX_BODY_BYTES", "1")
    assert ai_service_max_body_bytes() == DEFAULT_AI_SERVICE_MAX_BODY_BYTES

    monkeypatch.setenv("AI_SERVICE_MAX_BODY_BYTES", "999999999")
    assert ai_service_max_body_bytes() == MAX_AI_SERVICE_MAX_BODY_BYTES


def test_ai_service_middleware_rejects_known_oversized_body(monkeypatch) -> None:
    monkeypatch.setenv("AI_SERVICE_MAX_BODY_BYTES", "1048576")
    headers = signed_headers(b"")
    headers["Content-Length"] = str(1024 * 1024 + 1)

    assert asyncio.run(middleware_status("POST", "/tasks/demo", headers, body=b"")) == 413


def test_ai_service_middleware_rejects_unknown_length_oversized_body(monkeypatch) -> None:
    monkeypatch.setenv("AI_SERVICE_MAX_BODY_BYTES", "1048576")
    body = b"x" * (1024 * 1024 + 1)

    assert asyncio.run(middleware_status("POST", "/tasks/demo", signed_headers(body), body=body)) == 413


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


def test_callback_allowed_hosts_accepts_hosts_and_origins(monkeypatch) -> None:
    monkeypatch.setenv("AI_CALLBACK_ALLOWED_HOSTS", "backend:8080,https://Internal.Example:9443/")

    assert callback_allowed_hosts() == {"backend:8080", "internal.example:9443"}
    ensure_callback_url_allowed("http://backend:8080/api/v1/ai/callbacks/tasks")
    ensure_callback_url_allowed("https://internal.example:9443/api/v1/ai/callbacks/tasks")

    with pytest.raises(RuntimeError, match="not allowed"):
        ensure_callback_url_allowed("https://internal.example/api/v1/ai/callbacks/tasks")


@pytest.mark.parametrize(
    "configured",
    [
        "token@backend",
        "http://token@backend",
        "backend/api",
        "https://backend/api",
        "backend?debug=1",
        "backend#fragment",
        "backend:bad",
        "backend\nX-Injected: yes",
    ],
)
def test_callback_allowed_hosts_rejects_ambiguous_entries(monkeypatch, configured) -> None:
    monkeypatch.setenv("AI_CALLBACK_ALLOWED_HOSTS", configured)

    with pytest.raises(RuntimeError, match="AI_CALLBACK_ALLOWED_HOSTS"):
        callback_allowed_hosts()


def test_callback_url_rejects_non_http_or_unlisted_hosts(monkeypatch) -> None:
    monkeypatch.setenv("AI_CALLBACK_ALLOWED_HOSTS", "backend,internal.example")

    with pytest.raises(RuntimeError, match="absolute http"):
        ensure_callback_url_allowed("file:///etc/passwd")
    with pytest.raises(RuntimeError, match="credentials"):
        ensure_callback_url_allowed("http://token@backend:8080/api/v1/ai/callbacks/tasks")
    with pytest.raises(RuntimeError, match="invalid characters"):
        ensure_callback_url_allowed("http://backend:8080/api/v1/ai/callbacks/tasks\nX-Injected: yes")
    with pytest.raises(RuntimeError, match="fragment"):
        ensure_callback_url_allowed("http://backend:8080/api/v1/ai/callbacks/tasks#secret")
    with pytest.raises(RuntimeError, match="not allowed"):
        ensure_callback_url_allowed("http://169.254.169.254/latest/meta-data")
    with pytest.raises(RuntimeError, match="not allowed"):
        ensure_callback_url_allowed("http://evil.example/callback")
    ensure_callback_url_allowed("https://internal.example/callback")


def test_tenant_object_key_rejects_ambiguous_paths() -> None:
    ensure_tenant_object_key_allowed("tenant-demo", "tenant-demo/assets/file.txt")

    for object_key in [
        "",
        "tenant-demo",
        "tenant-demo//assets/file.txt",
        "tenant-demo/./assets/file.txt",
        "tenant-demo/../assets/file.txt",
        "/tenant-demo/assets/file.txt",
        "tenant-demo\\assets\\file.txt",
        "tenant-demo/assets/file.txt ",
        "tenant-demo/assets/ file.txt",
        "tenant-demo/assets/file\n.txt",
        "tenant-demo/assets/file.txt?download=1",
        "tenant-demo/assets/file.txt#preview",
        "http://tenant-demo/assets/file.txt",
        "other-tenant/assets/file.txt",
    ]:
        with pytest.raises(RuntimeError, match="outside tenant scope"):
            ensure_tenant_object_key_allowed("tenant-demo", object_key)


def test_post_callback_retries_transient_delivery_failures(monkeypatch) -> None:
    calls: list[object] = []

    class FakeResponse:
        def __enter__(self) -> "FakeResponse":
            return self

        def __exit__(self, _exc_type, _exc, _traceback) -> None:
            return None

        def read(self, size: int = -1) -> bytes:
            _ = size
            return b"ok"

    def flaky_urlopen(req: object, timeout: int) -> FakeResponse:
        assert timeout == 10
        calls.append(req)
        if len(calls) < 3:
            raise RuntimeError("temporary backend outage")
        return FakeResponse()

    monkeypatch.setenv("AI_CALLBACK_ALLOWED_HOSTS", "backend")
    monkeypatch.setenv("AI_CALLBACK_RETRY_DELAY_SECONDS", "0")
    monkeypatch.setattr("app.main.request.urlopen", flaky_urlopen)

    post_callback(
        "http://backend:8080/api/v1/ai/callbacks/tasks",
        {"tenant_id": "tenant-demo", "task_id": "task-demo", "status": "done"},
    )

    assert len(calls) == 3


def test_post_callback_rejects_oversized_response_without_leaking_body(monkeypatch) -> None:
    class FakeResponse:
        def __init__(self) -> None:
            self._body = b"secret backend response body"

        def __enter__(self) -> "FakeResponse":
            return self

        def __exit__(self, _exc_type, _exc, _traceback) -> None:
            return None

        def read(self, size: int = -1) -> bytes:
            return self._body[:size]

    monkeypatch.setenv("AI_CALLBACK_ALLOWED_HOSTS", "backend")
    monkeypatch.setenv("AI_CALLBACK_MAX_ATTEMPTS", "1")
    monkeypatch.setenv("AI_CALLBACK_MAX_RESPONSE_BYTES", "4")
    monkeypatch.setattr("app.main.request.urlopen", lambda *_args, **_kwargs: FakeResponse())

    with pytest.raises(CallbackResponseTooLargeError) as exc_info:
        post_callback(
            "http://backend:8080/api/v1/ai/callbacks/tasks",
            {"tenant_id": "tenant-demo", "task_id": "task-demo", "status": "done"},
        )

    message = str(exc_info.value)
    assert "too large" in message
    assert "secret backend" not in message


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


def test_process_knowledge_document_batches_embeddings(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeResponse:
        def read(self) -> bytes:
            return b"large document"

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse()

    class FakeEmbeddingProvider:
        name = "fake-embedding"

        def __init__(self) -> None:
            self.batch_sizes: list[int] = []

        def embed_batch(self, texts: list[str]) -> list[list[float]]:
            self.batch_sizes.append(len(texts))
            return [[1.0] + [0.0] * 1023 for _ in texts]

        def get_dimensions(self) -> int:
            return 1024

    provider = FakeEmbeddingProvider()

    class FakeRouter:
        def resolve(self, _task_type: str, tenant_id: str) -> RouteTarget:
            assert tenant_id == "tenant-demo"
            return RouteTarget(provider="fake-embedding", model="fake-embedding-model", dimensions=1024)

        def get_embedding(self, _task_type: str, tenant_id: str) -> FakeEmbeddingProvider:
            assert tenant_id == "tenant-demo"
            return provider

    chunks = [
        KnowledgeChunk(title=f"chunk {index}", content=f"content {index}", section_path="section")
        for index in range(65)
    ]
    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.router", FakeRouter())
    monkeypatch.setattr(
        "app.main.parse_document",
        lambda _payload, _content: KnowledgeProcessResult(
            processed_title="large",
            summary="large summary",
            metadata={"parser": "test"},
            chunks=chunks,
        ),
    )
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_knowledge_document(
        "task-knowledge-demo",
        KnowledgeProcessRequest(
            tenant_id="tenant-demo",
            document_id="doc-demo",
            file_id="file-demo",
            object_key="tenant-demo/knowledge/file-demo",
            filename="large.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert provider.batch_sizes == [32, 32, 1]
    assert callbacks[0]["status"] == "done", callbacks[0]
    result = callbacks[0]["result"]
    assert isinstance(result, dict)
    assert result["chunk_count"] == 65
    callback_chunks = callbacks[0]["chunks"]
    assert isinstance(callback_chunks, list)
    assert all(chunk["embedding"] for chunk in callback_chunks)


def test_knowledge_embeddings_falls_back_when_primary_provider_call_fails(monkeypatch) -> None:
    class FailingEmbeddingProvider:
        name = "primary-embedding"

        def embed_batch(self, _texts: list[str]) -> list[list[float]]:
            raise RuntimeError("primary embedding unavailable")

        def get_dimensions(self) -> int:
            return 3

    class FallbackEmbeddingProvider:
        name = "fallback-embedding"

        def embed_batch(self, texts: list[str]) -> list[list[float]]:
            return [[1.0, 0.0, 0.0] for _ in texts]

        def get_dimensions(self) -> int:
            return 3

    class FakeRouter:
        def resolve_candidates(self, _task_type: str, tenant_id: str) -> list[RouteTarget]:
            assert tenant_id == "tenant-demo"
            return [
                RouteTarget(provider="primary-embedding", model="primary-embedding-model", dimensions=3),
                RouteTarget(
                    provider="fallback-embedding",
                    model="fallback-embedding-model",
                    dimensions=3,
                    fallback_from="primary-embedding",
                ),
            ]

        def provider_for_target(self, target: RouteTarget) -> object:
            if target.provider == "primary-embedding":
                return FailingEmbeddingProvider()
            return FallbackEmbeddingProvider()

    monkeypatch.setattr("app.main.router", FakeRouter())

    response = asyncio.run(
        knowledge_embeddings(KnowledgeEmbeddingRequest(tenant_id="tenant-demo", texts=["智慧交通"]))
    )

    assert response.provider == "fallback-embedding"
    assert response.model == "fallback-embedding-model"
    assert response.dimensions == 3
    assert response.embeddings == [[1.0, 0.0, 0.0]]
    assert response.route["fallback_from"] == "primary-embedding"


def test_knowledge_rerank_falls_back_when_primary_provider_call_fails(monkeypatch) -> None:
    class FailingRerankProvider:
        name = "primary-rerank"

        def rerank(self, _query: str, _documents: list[str]) -> list[int]:
            raise RuntimeError("primary rerank unavailable")

    class FallbackRerankProvider:
        name = "fallback-rerank"

        def rerank(self, _query: str, _documents: list[str]) -> list[int]:
            return [1, 0]

    class FakeRouter:
        def resolve_candidates(self, _task_type: str, tenant_id: str) -> list[RouteTarget]:
            assert tenant_id == "tenant-demo"
            return [
                RouteTarget(provider="primary-rerank", model="primary-rerank-model"),
                RouteTarget(
                    provider="fallback-rerank",
                    model="fallback-rerank-model",
                    fallback_from="primary-rerank",
                ),
            ]

        def provider_for_target(self, target: RouteTarget) -> object:
            if target.provider == "primary-rerank":
                return FailingRerankProvider()
            return FallbackRerankProvider()

    monkeypatch.setattr("app.main.router", FakeRouter())

    response = asyncio.run(
        knowledge_rerank(
            KnowledgeRerankRequest(
                tenant_id="tenant-demo",
                query="智慧交通",
                documents=[
                    KnowledgeRerankDocument(id="doc-1", title="财务报表", content="成本核算"),
                    KnowledgeRerankDocument(id="doc-2", title="交通方案", content="智慧交通实施"),
                ],
                top_k=2,
            )
        )
    )

    assert response.provider == "fallback-rerank"
    assert response.model == "fallback-rerank-model"
    assert [item.id for item in response.results] == ["doc-2", "doc-1"]
    assert response.route["fallback_from"] == "primary-rerank"


def test_process_knowledge_document_falls_back_when_embedding_provider_call_fails(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeResponse:
        def read(self) -> bytes:
            return b"document"

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse()

    class FailingEmbeddingProvider:
        name = "primary-embedding"

        def embed_batch(self, _texts: list[str]) -> list[list[float]]:
            raise RuntimeError("primary embedding unavailable")

        def get_dimensions(self) -> int:
            return 3

    class FallbackEmbeddingProvider:
        name = "fallback-embedding"

        def embed_batch(self, texts: list[str]) -> list[list[float]]:
            return [[0.0, 1.0, 0.0] for _ in texts]

        def get_dimensions(self) -> int:
            return 3

    class FakeRouter:
        def resolve_candidates(self, _task_type: str, tenant_id: str) -> list[RouteTarget]:
            assert tenant_id == "tenant-demo"
            return [
                RouteTarget(provider="primary-embedding", model="primary-embedding-model", dimensions=3),
                RouteTarget(
                    provider="fallback-embedding",
                    model="fallback-embedding-model",
                    dimensions=3,
                    fallback_from="primary-embedding",
                ),
            ]

        def provider_for_target(self, target: RouteTarget) -> object:
            if target.provider == "primary-embedding":
                return FailingEmbeddingProvider()
            return FallbackEmbeddingProvider()

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.router", FakeRouter())
    monkeypatch.setattr(
        "app.main.parse_document",
        lambda _payload, _content: KnowledgeProcessResult(
            processed_title="doc",
            summary="summary",
            metadata={"parser": "test"},
            chunks=[KnowledgeChunk(title="chunk", content="content", section_path="section")],
        ),
    )
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_knowledge_document(
        "task-knowledge-demo",
        KnowledgeProcessRequest(
            tenant_id="tenant-demo",
            document_id="doc-demo",
            file_id="file-demo",
            object_key="tenant-demo/knowledge/file-demo",
            filename="doc.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert callbacks[0]["status"] == "done", callbacks[0]
    result = callbacks[0]["result"]
    assert isinstance(result, dict)
    assert result["embedding_provider"] == "fallback-embedding"
    assert result["embedding_model"] == "fallback-embedding-model"
    assert result["model_metadata"]["fallback_from"] == "primary-embedding"
    assert callbacks[0]["chunks"][0]["embedding"] == [0.0, 1.0, 0.0]


def test_process_knowledge_document_rejects_cross_tenant_object_key(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> object:
            raise AssertionError("cross-tenant object should be rejected before MinIO fetch")

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_knowledge_document(
        "task-knowledge-demo",
        KnowledgeProcessRequest(
            tenant_id="tenant-demo",
            document_id="doc-demo",
            file_id="file-demo",
            object_key="other-tenant/knowledge/file-demo",
            filename="large.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert callbacks[0]["status"] == "failed", callbacks[0]
    assert callbacks[0]["error_message"] == "知识库文档整理失败，请稍后重试"
    assert "outside tenant scope" not in str(callbacks[0])


def test_embed_knowledge_inputs_rejects_dimension_mismatch() -> None:
    class BadProvider:
        def embed_batch(self, texts: list[str]) -> list[list[float]]:
            return [[1.0, 0.0] for _ in texts]

        def get_dimensions(self) -> int:
            return 1024

    with pytest.raises(RuntimeError, match="dimension mismatch"):
        embed_knowledge_inputs(BadProvider(), ["a", "b"])


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


def test_process_document_export_hydrates_object_key_attachments(monkeypatch, tmp_path) -> None:
    callbacks: list[dict[str, object]] = []
    stored: dict[str, bytes] = {}
    objects = {
        "tenant-demo/assets/qualification.txt": "资质文件内容".encode(),
        "tenant-demo/assets/boq.xlsx": b"boq-content",
    }

    class FakeResponse:
        def __init__(self, content: bytes) -> None:
            self.content = content

        def read(self) -> bytes:
            return self.content

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, object_key: str) -> FakeResponse:
            return FakeResponse(objects[object_key])

        def fput_object(
            self,
            _bucket: str,
            object_key: str,
            file_path: str,
            content_type: str | None = None,
        ) -> None:
            stored[object_key] = Path(file_path).read_bytes()
            assert content_type == "application/zip"

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_document_export(
        "task-export-zip",
        DocumentExportRequest(
            tenant_id="tenant-demo",
            export_id="export-demo",
            bid_id="bid-demo",
            bid_title="测试项目",
            export_type="zip",
            part_code="all",
            part_title="投标文件全套",
            filename="测试项目.zip",
            object_key="tenant-demo/bid_export/export.zip",
            parts=[
                ExportPart(
                    code="tech",
                    title="技术标",
                    chapters=[ExportChapter(title="实施计划", plain_text="实施正文")],
                    attachments=[
                        ExportAttachment(
                            filename="资质文件.txt",
                            object_key="tenant-demo/assets/qualification.txt",
                        )
                    ],
                )
            ],
            boq_files=[
                ExportAttachment(filename="清单.xlsx", object_key="tenant-demo/assets/boq.xlsx")
            ],
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
        "zip",
    )

    assert callbacks[0]["status"] == "done", callbacks[0]
    output_path = tmp_path / "stored.zip"
    output_path.write_bytes(stored["tenant-demo/bid_export/export.zip"])
    with ZipFile(output_path) as archive:
        names = set(archive.namelist())
        manifest = json.loads(archive.read("manifest.json").decode("utf-8"))
        assert "02_附件/tech/资质文件.txt" in names
        assert "03_工程量清单/清单.xlsx" in names
        assert archive.read("02_附件/tech/资质文件.txt") == objects["tenant-demo/assets/qualification.txt"]
        assert archive.read("03_工程量清单/清单.xlsx") == objects["tenant-demo/assets/boq.xlsx"]
        assert manifest["parts"][0]["attachments"][0]["object_key"] == "tenant-demo/assets/qualification.txt"
        assert manifest["boq_files"][0]["object_key"] == "tenant-demo/assets/boq.xlsx"


def test_process_document_export_rejects_cross_tenant_attachment_object_key(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> object:
            raise AssertionError("cross-tenant object should be rejected before MinIO fetch")

        def fput_object(self, *_args, **_kwargs) -> None:
            raise AssertionError("failed export must not upload an output file")

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_document_export(
        "task-export-zip",
        DocumentExportRequest(
            tenant_id="tenant-demo",
            export_id="export-demo",
            bid_id="bid-demo",
            bid_title="测试项目",
            export_type="zip",
            part_code="all",
            part_title="投标文件全套",
            filename="测试项目.zip",
            object_key="tenant-demo/bid_export/export.zip",
            parts=[
                ExportPart(
                    code="tech",
                    title="技术标",
                    chapters=[ExportChapter(title="实施计划", plain_text="实施正文")],
                    attachments=[
                        ExportAttachment(
                            filename="跨租户文件.txt",
                            object_key="other-tenant/assets/secret.txt",
                        )
                    ],
                )
            ],
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
        "zip",
    )

    assert callbacks[0]["status"] == "failed", callbacks[0]
    assert callbacks[0]["error_message"] == "导出文件生成失败，请检查内容后重试"
    assert callbacks[0]["result"]["error"] == "导出文件生成失败，请检查内容后重试"
    assert callbacks[0]["result"]["export_id"] == "export-demo"
    assert "outside tenant scope" not in str(callbacks[0])


def test_download_minio_object_base64_rejects_oversized_attachment_object(monkeypatch) -> None:
    class FakeResponse:
        def __init__(self, content: bytes) -> None:
            self.content = content
            self.offset = 0

        def read(self, size: int = -1) -> bytes:
            if size < 0:
                size = len(self.content) - self.offset
            chunk = self.content[self.offset : self.offset + size]
            self.offset += len(chunk)
            return chunk

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse(b"abcde")

    monkeypatch.setenv("AI_EXPORT_ATTACHMENT_MAX_BYTES", "4")

    with pytest.raises(RuntimeError, match="export attachment object exceeds"):
        download_minio_object_base64(FakeMinio(), "tenant-demo/assets/big.bin")


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
    assert set(result["modules"]) == {
        "basic",
        "qualification",
        "evaluation",
        "submission",
        "invalid_risk",
        "annex",
    }
    assert result["modules"]["basic"]["fields"]["project_name"] == "智慧交通平台建设"
    assert result["modules"]["qualification"]["requirement_items"]
    assert result["modules"]["submission"]["requirement_items"]
    assert result["modules"]["annex"]["requirement_items"]
    assert result["field_evidence"]
    assert result["requirement_items"]
    assert result["quality_gates"]["interpret"]["module_count"] == 6
    assert result["parse_metadata"]["module_count"] == 6
    assert result["parse_metadata"]["requirement_count"] == len(result["requirement_items"])


def test_build_tender_structured_result_joins_cover_project_title_without_bid_title() -> None:
    payload = TenderParseRequest(
        tenant_id="tenant-demo",
        bid_id="bid-demo",
        file_id="file-demo",
        object_key="tenant/bid_tender/demo.pdf",
        filename="采购文件桥梁检查.pdf",
        content_type="application/pdf",
    )
    parsed = KnowledgeProcessResult(
        processed_title="demo",
        summary="summary",
        metadata={"parser": "pymupdf", "page_count": 90, "table_count": 36},
        chunks=[
            KnowledgeChunk(
                title="封面",
                content=(
                    "[Page 1]\n"
                    "2025 年度江苏东方路桥建设养护有限公司\n"
                    "桥梁检查劳务合作项目\n"
                    "询比采购文件\n"
                    "采 购 人：江苏东方路桥建设养护有限公司\n"
                    "采购代理：江苏交通工程投资咨询有限公司"
                ),
                section_path="封面",
            )
        ],
    )

    result = build_tender_structured_result(payload, parsed)

    assert result["project_name"] == "2025年度江苏东方路桥建设养护有限公司桥梁检查劳务合作项目"


def test_merge_tender_structured_result_preserves_source_file_and_uses_model_fields() -> None:
    base = {
        "project_name": "基础项目",
        "bid_type": "combined",
        "source_file": {"file_asset_id": "file-demo"},
        "qualification_requirements": ["基础资格"],
    }
    merged = merge_tender_structured_result(
        base,
        {
            "project_name": "模型项目",
            "source_file": {"file_asset_id": "malicious-overwrite"},
            "qualification_requirements": ["模型资格"],
            "empty_value": "",
        },
    )

    assert merged["project_name"] == "模型项目"
    assert merged["qualification_requirements"] == ["模型资格"]
    assert merged["source_file"] == {"file_asset_id": "file-demo"}


def test_process_tender_parse_uses_model_provider_and_callback(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeResponse:
        def read(self) -> bytes:
            return (
                "项目名称：桥梁检查服务\n"
                "投标截止时间：2026年7月20日\n"
                "资格要求：具备桥梁检测资质\n"
            ).encode()

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse()

    class FakeProvider:
        name = "fake-llm"

        def generate_json(self, prompt: str, schema_name: str) -> dict[str, object]:
            assert schema_name == "TenderParseModuleResult"
            module = json.loads(prompt)["module"]
            if module == "basic":
                return {
                    "module": "basic",
                    "fields": {"project_name": "模型增强桥梁检查服务"},
                    "evidence": [
                        {
                            "field": "project_name",
                            "value": "模型增强桥梁检查服务",
                            "confidence": 0.86,
                            "source_text": "项目名称：桥梁检查服务",
                        }
                    ],
                }
            if module == "qualification":
                return {
                    "module": "qualification",
                    "fields": {"qualification_requirements": ["模型识别资质要求"]},
                    "requirement_items": [
                        {
                            "id": "qualification-001",
                            "module": "qualification",
                            "type": "qualification",
                            "requirement": "模型识别资质要求",
                            "priority": "high",
                            "mandatory": True,
                            "expected_response": "提供对应资质证明。",
                            "source_ref": {
                                "field": "qualification_requirements",
                                "value": "模型识别资质要求",
                                "confidence": 0.72,
                                "source_text": "资格要求：具备桥梁检测资质",
                            },
                        }
                    ],
                }
            return {"module": module, "fields": {}}

        def count_tokens(self, text: str) -> int:
            return max(1, len(text) // 4)

    class FakeRouter:
        def resolve(self, _task_type: str, tenant_id: str) -> RouteTarget:
            assert tenant_id == "tenant-demo"
            return RouteTarget(provider="fake-llm", model="fake-model", schema_name="TenderParseModuleResult")

        def get_llm(self, _task_type: str, tenant_id: str) -> FakeProvider:
            assert tenant_id == "tenant-demo"
            return FakeProvider()

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.router", FakeRouter())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_tender_parse(
        "task-tender-demo",
        TenderParseRequest(
            tenant_id="tenant-demo",
            bid_id="bid-demo",
            file_id="file-demo",
            object_key="tenant-demo/bid_tender/file-demo",
            filename="采购文件.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert callbacks
    assert callbacks[0]["status"] == "done", callbacks[0]
    result = callbacks[0]["result"]
    assert isinstance(result, dict)
    assert result["structured_result"]["project_name"] == "模型增强桥梁检查服务"
    assert set(result["structured_result"]["modules"]) == {
        "basic",
        "qualification",
        "evaluation",
        "submission",
        "invalid_risk",
        "annex",
    }
    assert result["structured_result"]["modules"]["basic"]["fields"]["project_name"] == "模型增强桥梁检查服务"
    assert result["structured_result"]["modules"]["qualification"]["fields"]["qualification_requirements"] == [
        "模型识别资质要求"
    ]
    assert result["structured_result"]["modules"]["qualification"]["requirement_items"][0]["needs_review"] is False
    assert result["structured_result"]["requirement_items"]
    assert result["structured_result"]["field_evidence"]
    assert result["structured_result"]["quality_gates"]["interpret"]["module_count"] == 6
    assert result["model_metadata"]["provider"] == "fake-llm"
    assert result["model_metadata"]["model"] == "fake-model"
    assert result["model_metadata"]["module_call_count"] == 6
    assert len(result["model_metadata"]["module_calls"]) == 6
    assert result["token_usage"]["input_tokens"] > 0


def test_process_tender_parse_runs_modules_with_configured_concurrency(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []
    lock = threading.Lock()
    active_calls = 0
    max_active_calls = 0

    class FakeResponse:
        def read(self) -> bytes:
            return (
                "项目名称：桥梁检查服务\n"
                "资格要求：具备桥梁检测资质\n"
                "评分标准：技术方案 20 分\n"
                "响应文件格式：按附件签章提交\n"
            ).encode()

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse()

    class SlowProvider:
        name = "slow-llm"

        def generate_json(self, prompt: str, schema_name: str) -> dict[str, object]:
            nonlocal active_calls, max_active_calls
            assert schema_name == "TenderParseModuleResult"
            module = json.loads(prompt)["module"]
            with lock:
                active_calls += 1
                max_active_calls = max(max_active_calls, active_calls)
            try:
                time.sleep(0.03)
                return {"module": module, "fields": {}}
            finally:
                with lock:
                    active_calls -= 1

        def count_tokens(self, text: str) -> int:
            return max(1, len(text) // 4)

    provider = SlowProvider()

    class FakeRouter:
        def resolve_candidates(self, _task_type: str, tenant_id: str) -> list[RouteTarget]:
            assert tenant_id == "tenant-demo"
            return [
                RouteTarget(
                    provider="slow-llm",
                    model="slow-model",
                    schema_name="TenderParseModuleResult",
                )
            ]

        def provider_for_target(self, _target: RouteTarget) -> SlowProvider:
            return provider

    monkeypatch.setenv("TENDER_PARSE_MODULE_CONCURRENCY", "2")
    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.router", FakeRouter())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_tender_parse(
        "task-tender-demo",
        TenderParseRequest(
            tenant_id="tenant-demo",
            bid_id="bid-demo",
            file_id="file-demo",
            object_key="tenant-demo/bid_tender/file-demo",
            filename="采购文件.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    result = callbacks[0]["result"]
    assert isinstance(result, dict)
    assert result["model_metadata"]["module_concurrency"] == 2
    assert result["model_metadata"]["module_call_count"] == 6
    assert [call["module"] for call in result["model_metadata"]["module_calls"]] == [
        "basic",
        "qualification",
        "evaluation",
        "submission",
        "invalid_risk",
        "annex",
    ]
    assert max_active_calls == 2


def test_tender_parse_module_concurrency_uses_safe_bounds(monkeypatch) -> None:
    monkeypatch.setenv("TENDER_PARSE_MODULE_CONCURRENCY", "4")
    assert tender_parse_module_concurrency() == 4

    monkeypatch.setenv("TENDER_PARSE_MODULE_CONCURRENCY", "0")
    assert tender_parse_module_concurrency() == 1

    monkeypatch.setenv("TENDER_PARSE_MODULE_CONCURRENCY", "999")
    assert tender_parse_module_concurrency() == 6

    monkeypatch.setenv("TENDER_PARSE_MODULE_CONCURRENCY", "invalid")
    assert tender_parse_module_concurrency() == 3


def test_process_tender_parse_falls_back_when_primary_provider_call_fails(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeResponse:
        def read(self) -> bytes:
            return "项目名称：桥梁检查服务\n资格要求：具备桥梁检测资质\n".encode()

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse()

    class FailingProvider:
        name = "primary-llm"

        def generate_json(self, _prompt: str, _schema_name: str) -> dict[str, object]:
            raise RuntimeError("primary unavailable")

        def count_tokens(self, text: str) -> int:
            return max(1, len(text) // 4)

    class FallbackProvider:
        name = "fallback-llm"

        def generate_json(self, prompt: str, schema_name: str) -> dict[str, object]:
            assert schema_name == "TenderParseModuleResult"
            module = json.loads(prompt)["module"]
            if module == "basic":
                return {"module": "basic", "fields": {"project_name": "fallback 解析项目"}}
            return {"module": module, "fields": {}}

        def count_tokens(self, text: str) -> int:
            return max(1, len(text) // 4)

    class FakeRouter:
        def resolve_candidates(self, _task_type: str, tenant_id: str) -> list[RouteTarget]:
            assert tenant_id == "tenant-demo"
            return [
                RouteTarget(provider="primary-llm", model="primary-model", schema_name="TenderParseModuleResult"),
                RouteTarget(
                    provider="fallback-llm",
                    model="fallback-model",
                    schema_name="TenderParseModuleResult",
                    fallback_from="primary-llm",
                ),
            ]

        def provider_for_target(self, target: RouteTarget) -> object:
            if target.provider == "primary-llm":
                return FailingProvider()
            return FallbackProvider()

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.router", FakeRouter())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_tender_parse(
        "task-tender-demo",
        TenderParseRequest(
            tenant_id="tenant-demo",
            bid_id="bid-demo",
            file_id="file-demo",
            object_key="tenant-demo/bid_tender/file-demo",
            filename="采购文件.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert callbacks[0]["status"] == "done", callbacks[0]
    result = callbacks[0]["result"]
    assert isinstance(result, dict)
    assert result["structured_result"]["project_name"] == "fallback 解析项目"
    assert result["model_metadata"]["provider"] == "fallback-llm"
    assert result["model_metadata"]["model"] == "fallback-model"
    assert result["model_metadata"]["fallback_from"] == "primary-llm"


def test_process_tender_parse_keeps_result_when_one_module_enhancement_fails(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeResponse:
        def read(self) -> bytes:
            return (
                "项目名称：桥梁检查服务\n"
                "资格要求：具备桥梁检测资质\n"
                "评分标准：技术方案 20 分\n"
            ).encode()

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse()

    class PartiallyFailingProvider:
        name = "module-llm"

        def generate_json(self, prompt: str, schema_name: str) -> dict[str, object]:
            assert schema_name == "TenderParseModuleResult"
            module = json.loads(prompt)["module"]
            if module == "evaluation":
                raise RuntimeError("evaluation unavailable")
            return {"module": module, "fields": {}}

        def count_tokens(self, text: str) -> int:
            return max(1, len(text) // 4)

    class FakeRouter:
        def resolve_candidates(self, _task_type: str, tenant_id: str) -> list[RouteTarget]:
            assert tenant_id == "tenant-demo"
            return [RouteTarget(provider="module-llm", model="module-model", schema_name="TenderParseModuleResult")]

        def provider_for_target(self, _target: RouteTarget) -> object:
            return PartiallyFailingProvider()

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.router", FakeRouter())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_tender_parse(
        "task-tender-demo",
        TenderParseRequest(
            tenant_id="tenant-demo",
            bid_id="bid-demo",
            file_id="file-demo",
            object_key="tenant-demo/bid_tender/file-demo",
            filename="采购文件.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert callbacks[0]["status"] == "done", callbacks[0]
    result = callbacks[0]["result"]
    evaluation = result["structured_result"]["modules"]["evaluation"]
    assert evaluation["status"] == "needs_review"
    assert evaluation["enhancement_error"]["type"] == "RuntimeError"
    failed_calls = [
        call
        for call in result["model_metadata"]["module_calls"]
        if call["module"] == "evaluation"
    ]
    assert failed_calls == [{"module": "evaluation", "status": "failed", "error_type": "RuntimeError"}]


def test_process_tender_parse_rejects_oversized_source_object(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeResponse:
        def __init__(self, content: bytes) -> None:
            self.content = content
            self.offset = 0

        def read(self, size: int = -1) -> bytes:
            if size < 0:
                size = len(self.content) - self.offset
            chunk = self.content[self.offset : self.offset + size]
            self.offset += len(chunk)
            return chunk

        def close(self) -> None:
            pass

        def release_conn(self) -> None:
            pass

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> FakeResponse:
            return FakeResponse(b"abcde")

    monkeypatch.setenv("AI_TASK_OBJECT_MAX_BYTES", "4")
    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.parse_document", lambda _payload, _content: pytest.fail("parse should not run"))
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_tender_parse(
        "task-tender-demo",
        TenderParseRequest(
            tenant_id="tenant-demo",
            bid_id="bid-demo",
            file_id="file-demo",
            object_key="tenant-demo/bid_tender/file-demo",
            filename="采购文件.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert callbacks[0]["status"] == "failed", callbacks[0]
    assert callbacks[0]["error_message"] == "招标文件解读失败，请检查文件后重试"
    assert "byte limit" not in str(callbacks[0])


def test_process_tender_parse_rejects_cross_tenant_object_key(monkeypatch) -> None:
    callbacks: list[dict[str, object]] = []

    class FakeMinio:
        def get_object(self, _bucket: str, _object_key: str) -> object:
            raise AssertionError("cross-tenant object should be rejected before MinIO fetch")

    monkeypatch.setattr("app.main.minio_client", lambda: FakeMinio())
    monkeypatch.setattr("app.main.post_callback", lambda _url, payload: callbacks.append(payload))

    process_tender_parse(
        "task-tender-demo",
        TenderParseRequest(
            tenant_id="tenant-demo",
            bid_id="bid-demo",
            file_id="file-demo",
            object_key="other-tenant/bid_tender/file-demo",
            filename="采购文件.txt",
            content_type="text/plain",
            callback_url="http://backend:8080/api/v1/ai/callbacks/tasks",
        ),
    )

    assert callbacks[0]["status"] == "failed", callbacks[0]
    assert callbacks[0]["error_message"] == "招标文件解读失败，请检查文件后重试"
    assert "outside tenant scope" not in str(callbacks[0])


def test_production_mode_detects_release_environment(monkeypatch) -> None:
    monkeypatch.setenv("GIN_MODE", "release")

    assert production_mode()


def test_validate_production_config_rejects_development_secret(monkeypatch) -> None:
    monkeypatch.setenv("APP_ENV", "production")
    monkeypatch.delenv("AI_SERVICE_HMAC_SECRET", raising=False)

    with pytest.raises(RuntimeError, match="AI_SERVICE_HMAC_SECRET"):
        validate_production_config()


def test_validate_production_config_rejects_short_secret(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("AI_SERVICE_HMAC_SECRET", "short")

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


def test_validate_production_config_rejects_mock_provider_escape_hatch(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("ALLOW_MOCK_PROVIDERS_IN_PRODUCTION", "true")
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "true")

    with pytest.raises(RuntimeError, match="USE_MOCK_PROVIDERS"):
        validate_production_config()


def test_validate_production_config_rejects_mock_model_routes(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")

    with pytest.raises(RuntimeError, match="MockProvider"):
        validate_production_config()


def test_validate_production_config_rejects_mock_environment_override(monkeypatch) -> None:
    _set_production_security_env(monkeypatch)
    monkeypatch.setenv("USE_MOCK_PROVIDERS", "false")
    monkeypatch.setenv("ALLOW_MOCK_FALLBACK", "false")
    monkeypatch.setenv("AI_LLM_PROVIDER", "mock")
    monkeypatch.setenv("AI_LLM_MODEL", "mock-model")
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

    with pytest.raises(RuntimeError, match="chapter_generate.primary"):
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


def test_normalize_minio_endpoint_supports_cloudflare_r2_https_url() -> None:
    endpoint, secure = normalize_minio_endpoint(
        " https://example-account.r2.cloudflarestorage.com/ ",
        fallback_secure=False,
    )

    assert endpoint == "example-account.r2.cloudflarestorage.com"
    assert secure is True


def test_normalize_minio_endpoint_supports_http_url() -> None:
    endpoint, secure = normalize_minio_endpoint("http://127.0.0.1:9000", fallback_secure=True)

    assert endpoint == "127.0.0.1:9000"
    assert secure is False


def test_normalize_minio_endpoint_uses_fallback_for_host_only_endpoint() -> None:
    endpoint, secure = normalize_minio_endpoint("minio:9000", fallback_secure=True)

    assert endpoint == "minio:9000"
    assert secure is True


def test_normalize_minio_endpoint_rejects_paths_and_unsupported_schemes() -> None:
    for raw in (
        "",
        "ftp://example-account.r2.cloudflarestorage.com",
        "https://access:secret@example-account.r2.cloudflarestorage.com",
        "https://example-account.r2.cloudflarestorage.com/bucket",
        "minio:9000/bucket",
        "user:secret@minio:9000",
        "minio\\9000",
        "min io:9000",
        "minio:9000\r\nX-Injected: yes",
        "https://example-account.r2.cloudflarestorage.com?bucket=zbt",
    ):
        with pytest.raises(RuntimeError, match="MINIO_ENDPOINT"):
            normalize_minio_endpoint(raw, fallback_secure=False)


def test_minio_client_passes_configured_region(monkeypatch) -> None:
    created: dict[str, object] = {}

    class FakeMinio:
        def __init__(self, endpoint: str, **kwargs: object) -> None:
            created["endpoint"] = endpoint
            created.update(kwargs)

    monkeypatch.setenv("MINIO_ENDPOINT", "https://example-account.r2.cloudflarestorage.com")
    monkeypatch.setenv("MINIO_ACCESS_KEY", "r2-access-key")
    monkeypatch.setenv("MINIO_SECRET_KEY", "r2-secret-key")
    monkeypatch.setenv("MINIO_REGION", "auto")
    monkeypatch.setattr("app.main.Minio", FakeMinio)

    client = minio_client()

    assert isinstance(client, FakeMinio)
    assert created == {
        "endpoint": "example-account.r2.cloudflarestorage.com",
        "access_key": "r2-access-key",
        "secret_key": "r2-secret-key",
        "secure": True,
        "region": "auto",
    }


def _set_production_security_env(monkeypatch) -> None:
    monkeypatch.setenv("APP_ENV", "production")
    monkeypatch.setenv("AI_SERVICE_HMAC_SECRET", "prod-ai-secret-value")
    monkeypatch.setenv("MINIO_ACCESS_KEY", "prod-minio-access-value")
    monkeypatch.setenv("MINIO_SECRET_KEY", "prod-minio-secret-value")
    monkeypatch.setenv("OPENAI_API_KEY", "prod-openai-key")


def signed_headers(body: bytes) -> dict[str, str]:
    timestamp = str(int(time.time()))
    signature = hmac.new(
        ai_service_hmac_secret().encode("utf-8"),
        timestamp.encode("utf-8") + b"." + body,
        hashlib.sha256,
    ).hexdigest()
    return {
        "X-ZBT-Timestamp": timestamp,
        "X-ZBT-Signature": signature,
    }


async def middleware_status(
    method: str,
    path: str,
    headers: dict[str, str] | None = None,
    *,
    body: bytes = b"",
) -> int:
    raw_headers = [
        (key.lower().encode("latin-1"), value.encode("latin-1"))
        for key, value in (headers or {}).items()
    ]

    async def receive() -> dict[str, object]:
        return {"type": "http.request", "body": body, "more_body": False}

    request = Request(
        {
            "type": "http",
            "method": method,
            "path": path,
            "raw_path": path.encode("utf-8"),
            "query_string": b"",
            "headers": raw_headers,
            "scheme": "http",
            "server": ("testserver", 80),
            "client": ("testclient", 1234),
        },
        receive=receive,
    )

    async def call_next(_request: Request) -> JSONResponse:
        return JSONResponse({"status": "passed"}, status_code=209)

    response = await require_backend_signature(request, call_next)
    return response.status_code

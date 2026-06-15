from __future__ import annotations

import hashlib
import hmac
import json
import os
import re
import time
import uuid
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory
from urllib import request

from fastapi import BackgroundTasks, FastAPI, Request
from minio import Minio
from starlette.responses import JSONResponse

from app.gateway.model_router import ModelRouter
from app.pipelines.export.docx_exporter import export_bid_docx, export_bid_pdf, export_bid_zip
from app.pipelines.parse.document_parser import parse_document
from app.schemas.common import HealthResponse, TaskAccepted
from app.schemas.cost import CostAdviceRequest
from app.schemas.export import DocumentExportRequest
from app.schemas.generation import ChapterActionRequest, ChapterGenerateRequest
from app.schemas.knowledge import (
    KnowledgeEmbeddingRequest,
    KnowledgeEmbeddingResponse,
    KnowledgeProcessRequest,
    KnowledgeRerankRequest,
    KnowledgeRerankResponse,
    KnowledgeRerankResult,
)

def routing_config_path() -> Path:
    configured = os.getenv("MODEL_ROUTING_FILE", "").strip()
    if configured:
        return Path(configured)
    return Path(__file__).parent / "config" / "model_routing.yaml"


CONFIG_PATH = routing_config_path()


@asynccontextmanager
async def lifespan(_app: FastAPI):
    validate_production_config()
    yield


app = FastAPI(title="ZhiBiaoTong AI Service", version="0.1.0", lifespan=lifespan)
router = ModelRouter.from_yaml(CONFIG_PATH)
PUBLIC_PATHS = {"/healthz", "/models/health"}
DEFAULT_AI_HMAC_SECRET = "dev-only-zbt-ai-callback-secret"


@app.middleware("http")
async def require_backend_signature(request: Request, call_next):
    if request.method in {"GET", "HEAD", "OPTIONS"} or request.url.path in PUBLIC_PATHS:
        return await call_next(request)
    secret = ai_service_hmac_secret()
    if not secret:
        return await call_next(request)
    body = await request.body()
    if not verify_request_signature(
        request.headers.get("X-ZBT-Timestamp", ""),
        request.headers.get("X-ZBT-Signature", ""),
        body,
        secret,
    ):
        return JSONResponse(
            status_code=401,
            content={"code": "invalid_signature", "error": "AI service request signature invalid"},
        )

    async def receive() -> dict[str, object]:
        return {"type": "http.request", "body": body, "more_body": False}

    return await call_next(Request(request.scope, receive))


def minio_client() -> Minio:
    return Minio(
        os.getenv("MINIO_ENDPOINT", "minio:9000"),
        access_key=os.getenv("MINIO_ACCESS_KEY", "zbt_minio"),
        secret_key=os.getenv("MINIO_SECRET_KEY", "zbt_minio_secret"),
        secure=os.getenv("MINIO_USE_SSL", "").lower() in {"1", "true", "yes"},
    )


@app.get("/healthz", response_model=HealthResponse)
async def healthz() -> HealthResponse:
    return HealthResponse(service="zbt-ai-service", status="ok", time=datetime.now(UTC))


@app.get("/models/health")
async def model_health() -> dict[str, object]:
    return {"status": "ok", "providers": router.health_check()}


@app.post("/tasks/tender-parse", response_model=TaskAccepted, status_code=202)
async def tender_parse() -> TaskAccepted:
    route = router.resolve("tender_parse", tenant_id="tenant-demo")
    return TaskAccepted(task_id="task-tender-parse-demo", status="queued", route=route.model_dump())


@app.post("/tasks/knowledge-process", response_model=TaskAccepted, status_code=202)
async def knowledge_process(
    payload: KnowledgeProcessRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route = router.resolve("knowledge_process", tenant_id=payload.tenant_id)
    task_suffix = payload.document_id.replace("-", "")[:12]
    task_id = f"task-knowledge-{task_suffix}"
    background_tasks.add_task(process_knowledge_document, task_id, payload)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


@app.post("/embeddings/knowledge", response_model=KnowledgeEmbeddingResponse)
async def knowledge_embeddings(payload: KnowledgeEmbeddingRequest) -> KnowledgeEmbeddingResponse:
    route = router.resolve("knowledge_embedding", tenant_id=payload.tenant_id)
    provider = router.get_embedding("knowledge_embedding", tenant_id=payload.tenant_id)
    embeddings = provider.embed_batch(payload.texts)
    return KnowledgeEmbeddingResponse(
        provider=provider.name,
        model=route.model,
        dimensions=provider.get_dimensions(),
        embeddings=embeddings,
        route=route.model_dump(),
    )


@app.post("/rerank/knowledge", response_model=KnowledgeRerankResponse)
async def knowledge_rerank(payload: KnowledgeRerankRequest) -> KnowledgeRerankResponse:
    route = router.resolve("knowledge_rerank", tenant_id=payload.tenant_id)
    provider = router.get_rerank("knowledge_rerank", tenant_id=payload.tenant_id)
    document_texts = [
        f"{document.title}\n{document.section_path}\n{document.content}" for document in payload.documents
    ]
    ordered_indexes = provider.rerank(payload.query, document_texts)
    seen: set[int] = set()
    results: list[KnowledgeRerankResult] = []
    for index in ordered_indexes:
        if index < 0 or index >= len(payload.documents) or index in seen:
            continue
        seen.add(index)
        document = payload.documents[index]
        results.append(
            KnowledgeRerankResult(
                id=document.id,
                index=index,
                score=1.0 / (len(results) + 1),
            )
        )
        if len(results) >= payload.top_k:
            break
    for index, document in enumerate(payload.documents):
        if len(results) >= payload.top_k:
            break
        if index in seen:
            continue
        results.append(
            KnowledgeRerankResult(
                id=document.id,
                index=index,
                score=1.0 / (len(results) + 1),
            )
        )
    return KnowledgeRerankResponse(
        provider=provider.name,
        model=route.model,
        results=results,
        route=route.model_dump(),
    )


def process_knowledge_document(task_id: str, payload: KnowledgeProcessRequest) -> None:
    try:
        client = minio_client()
        response = client.get_object(os.getenv("MINIO_BUCKET", "zbt-files"), payload.object_key)
        try:
            content = response.read()
        finally:
            response.close()
            response.release_conn()
        parsed = parse_document(payload, content)
        embedding_provider = router.get_embedding("knowledge_embedding", tenant_id=payload.tenant_id)
        embedding_route = router.resolve("knowledge_embedding", tenant_id=payload.tenant_id)
        embedding_inputs = [
            f"{chunk.title}\n{chunk.section_path}\n{chunk.content}" for chunk in parsed.chunks
        ]
        embeddings = embedding_provider.embed_batch(embedding_inputs) if embedding_inputs else []
        input_tokens = sum(estimate_tokens(text) for text in embedding_inputs)
        for chunk, embedding in zip(parsed.chunks, embeddings, strict=False):
            chunk.embedding = embedding
            chunk.metadata["embedding_model"] = embedding_route.model
            chunk.metadata["embedding_provider"] = embedding_provider.name
            chunk.metadata["embedding_dimensions"] = embedding_provider.get_dimensions()
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "processed_title": parsed.processed_title,
            "summary": parsed.summary,
            "chunks": [chunk.model_dump() for chunk in parsed.chunks],
            "result": {
                "summary": parsed.summary,
                "metadata": parsed.metadata,
                "chunk_count": len(parsed.chunks),
                "embedding_model": embedding_route.model,
                "embedding_provider": embedding_provider.name,
                "embedding_dimensions": embedding_provider.get_dimensions(),
                "model_metadata": {
                    "provider": embedding_provider.name,
                    "model": embedding_route.model,
                    "embedding_dimensions": embedding_provider.get_dimensions(),
                },
                "token_usage": {
                    "input_tokens": input_tokens,
                    "output_tokens": 0,
                },
            },
        }
    except Exception as exc:  # pragma: no cover - defensive task boundary
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "failed",
            "error_message": str(exc),
            "result": {"error": str(exc)},
        }
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


def post_callback(callback_url: str, payload: dict[str, object]) -> None:
    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    timestamp = str(int(time.time()))
    secret = ai_service_hmac_secret()
    signature = hmac.new(secret.encode("utf-8"), timestamp.encode("utf-8") + b"." + body, hashlib.sha256).hexdigest()
    req = request.Request(
        callback_url,
        data=body,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "X-ZBT-Timestamp": timestamp,
            "X-ZBT-Signature": signature,
        },
    )
    with request.urlopen(req, timeout=10) as response:
        response.read()


def verify_request_signature(
    timestamp_header: str,
    signature_header: str,
    body: bytes,
    secret: str,
    *,
    now: int | None = None,
) -> bool:
    secret = secret.strip()
    if not secret or not timestamp_header or not signature_header:
        return False
    try:
        timestamp = int(timestamp_header)
    except ValueError:
        return False
    current = int(time.time()) if now is None else now
    if abs(current - timestamp) > 300:
        return False
    expected = hmac.new(
        secret.encode("utf-8"),
        timestamp_header.encode("utf-8") + b"." + body,
        hashlib.sha256,
    ).hexdigest()
    return hmac.compare_digest(expected, signature_header)


def ai_service_hmac_secret() -> str:
    return os.getenv("AI_SERVICE_HMAC_SECRET", "").strip() or DEFAULT_AI_HMAC_SECRET


def estimate_tokens(text: str) -> int:
    value = text.strip()
    if not value:
        return 0
    return max(1, len(value) // 4)


@app.post("/tasks/chapter-generate", response_model=TaskAccepted, status_code=202)
async def chapter_generate(
    payload: ChapterGenerateRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route = router.resolve("chapter_generate", tenant_id=payload.tenant_id)
    task_suffix = payload.chapter_id.replace("-", "")[:8]
    task_id = payload.task_id or f"task-chapter-{task_suffix}-{uuid.uuid4().hex[:8]}"
    background_tasks.add_task(process_chapter_generate, task_id, payload)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


def process_chapter_generate(task_id: str, payload: ChapterGenerateRequest) -> None:
    try:
        route = router.resolve("chapter_generate", tenant_id=payload.tenant_id)
        provider = router.get_llm("chapter_generate", tenant_id=payload.tenant_id)
        payload.model_hint = route.model
        generation = provider.generate_chapter(payload)
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": generation.model_dump(),
        }
    except Exception as exc:  # pragma: no cover - defensive task boundary
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "failed",
            "error_message": str(exc),
            "result": {"error": str(exc), "chapter_id": payload.chapter_id},
        }
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


@app.post("/tasks/chapter-action", response_model=TaskAccepted, status_code=202)
async def chapter_action(
    payload: ChapterActionRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route_name = "chapter_self_check" if payload.action == "self_check" else "rewrite_assistant"
    route = router.resolve(route_name, tenant_id=payload.tenant_id)
    task_suffix = payload.chapter_id.replace("-", "")[:8]
    action_suffix = payload.action.replace("_", "-")[:16]
    task_id = payload.task_id or f"task-chapter-action-{task_suffix}-{action_suffix}-{uuid.uuid4().hex[:8]}"
    background_tasks.add_task(process_chapter_action, task_id, payload, route_name, route.model)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


def process_chapter_action(task_id: str, payload: ChapterActionRequest, route_name: str, model_hint: str) -> None:
    try:
        provider = router.get_llm(route_name, tenant_id=payload.tenant_id)
        payload.model_hint = model_hint
        generation = provider.chapter_action(payload)
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": generation.model_dump(),
        }
    except Exception as exc:  # pragma: no cover - defensive task boundary
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "failed",
            "error_message": str(exc),
            "result": {"error": str(exc), "chapter_id": payload.chapter_id, "action": payload.action},
        }
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


@app.post("/tasks/cost-advice", response_model=TaskAccepted, status_code=202)
async def cost_advice(
    payload: CostAdviceRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route = router.resolve("cost_advice", tenant_id=payload.tenant_id)
    task_suffix = payload.cost_project_id.replace("-", "")[:8]
    task_id = payload.task_id or f"task-cost-advice-{task_suffix}-{uuid.uuid4().hex[:8]}"
    background_tasks.add_task(process_cost_advice, task_id, payload, route.model)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


def process_cost_advice(task_id: str, payload: CostAdviceRequest, model_hint: str) -> None:
    try:
        provider = router.get_llm("cost_advice", tenant_id=payload.tenant_id)
        payload.model_hint = model_hint
        result = provider.cost_advice(payload)
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": result.model_dump(),
        }
    except Exception as exc:  # pragma: no cover - defensive task boundary
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "failed",
            "error_message": str(exc),
            "result": {"error": str(exc), "cost_project_id": payload.cost_project_id},
        }
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


@app.post("/tasks/export/docx", response_model=TaskAccepted, status_code=202)
async def export_docx(
    payload: DocumentExportRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    return enqueue_document_export("docx", payload, background_tasks)


@app.post("/tasks/export/pdf", response_model=TaskAccepted, status_code=202)
async def export_pdf(
    payload: DocumentExportRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    return enqueue_document_export("pdf", payload, background_tasks)


@app.post("/tasks/export/zip", response_model=TaskAccepted, status_code=202)
async def export_zip(
    payload: DocumentExportRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    return enqueue_document_export("zip", payload, background_tasks)


def enqueue_document_export(
    export_type: str,
    payload: DocumentExportRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route = router.resolve("document_export", tenant_id=payload.tenant_id)
    task_suffix = payload.export_id.replace("-", "")[:12]
    task_id = f"task-export-{task_suffix}"
    background_tasks.add_task(process_document_export, task_id, payload, export_type)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


def process_document_export(task_id: str, payload: DocumentExportRequest, export_type: str) -> None:
    with TemporaryDirectory(prefix="zbt-ai-export-") as tmpdir:
        output_path = Path(tmpdir) / safe_output_filename(payload.filename, export_type)
        try:
            content_type = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
            if export_type == "zip":
                export_bid_zip(
                    payload.bid_title,
                    payload.parts,
                    output_path,
                    layout=payload.layout,
                    attachments=payload.attachments,
                    boq_files=payload.boq_files,
                )
                content_type = "application/zip"
            elif export_type == "pdf":
                export_bid_pdf(
                    payload.bid_title,
                    payload.part_title,
                    payload.chapters,
                    output_path,
                    layout=payload.layout,
                )
                content_type = "application/pdf"
            else:
                export_bid_docx(
                    payload.bid_title,
                    payload.part_title,
                    payload.chapters,
                    output_path,
                    layout=payload.layout,
                )
            client = minio_client()
            client.fput_object(
                os.getenv("MINIO_BUCKET", "zbt-files"),
                payload.object_key,
                str(output_path),
                content_type=content_type,
            )
            callback_payload = {
                "tenant_id": payload.tenant_id,
                "task_id": task_id,
                "status": "done",
                "result": {
                    "export_id": payload.export_id,
                    "bid_id": payload.bid_id,
                    "export_type": export_type,
                    "filename": payload.filename,
                    "object_key": payload.object_key,
                    "part_code": payload.part_code,
                    "part_count": len(payload.parts) if export_type == "zip" else 1,
                    "chapter_count": sum(len(part.chapters) for part in payload.parts)
                    if export_type == "zip"
                    else len(payload.chapters),
                    "size_bytes": output_path.stat().st_size,
                    "content_type": content_type,
                    "layout": payload.layout.model_dump(),
                    "manifest_filename": "manifest.json"
                    if export_type == "zip" and payload.layout.include_manifest
                    else None,
                    "pdf_validation": "enabled"
                    if export_type == "pdf" and payload.layout.validate_pdf
                    else "disabled",
                },
            }
        except Exception as exc:  # pragma: no cover - defensive task boundary
            callback_payload = {
                "tenant_id": payload.tenant_id,
                "task_id": task_id,
                "status": "failed",
                "error_message": str(exc),
                "result": {"error": str(exc), "export_id": payload.export_id},
            }
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


def safe_output_filename(filename: str, export_type: str) -> str:
    suffix = {
        "docx": ".docx",
        "pdf": ".pdf",
        "zip": ".zip",
    }.get(export_type, ".docx")
    basename = Path(filename.replace("\\", "/")).name.strip()
    basename = re.sub(r"[^0-9A-Za-z._\-\u4e00-\u9fff]+", "-", basename).strip(".-")
    if not basename:
        basename = "export"
    path = Path(basename)
    stem = path.stem.strip(".-") or "export"
    basename = stem + path.suffix
    if Path(basename).suffix.lower() != suffix:
        basename = Path(basename).with_suffix(suffix).name
    if len(basename) > 120:
        basename = Path(basename).stem[: 120 - len(suffix)] + suffix
    return basename


def validate_production_config() -> None:
    if not production_mode():
        return
    if ai_service_hmac_secret() == DEFAULT_AI_HMAC_SECRET:
        raise RuntimeError("AI_SERVICE_HMAC_SECRET must be set to a non-development value in production")


def production_mode() -> bool:
    return any(
        os.getenv(key, "").strip().lower() in {"prod", "production", "release"}
        for key in ("APP_ENV", "ZBT_ENV", "ENVIRONMENT", "GIN_MODE")
    )

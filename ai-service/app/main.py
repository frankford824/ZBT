from __future__ import annotations

import base64
import hashlib
import hmac
import json
import logging
import os
import re
import time
import uuid
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from pathlib import Path
from tempfile import TemporaryDirectory
from urllib import request
from urllib.parse import urlparse

from fastapi import BackgroundTasks, FastAPI, Request
from minio import Minio
from starlette.responses import JSONResponse

from app.gateway.model_router import ModelRouter, RouteTarget
from app.pipelines.export.docx_exporter import export_bid_docx, export_bid_pdf, export_bid_zip
from app.pipelines.parse.document_parser import parse_document
from app.pipelines.parse.tender_parser import (
    build_tender_parse_prompt,
    build_tender_structured_result,
    merge_tender_structured_result,
)
from app.schemas.common import HealthResponse, TaskAccepted
from app.schemas.cost import CostAdviceRequest
from app.schemas.export import (
    MAX_EXPORT_INLINE_ATTACHMENT_BYTES,
    DocumentExportRequest,
    ExportAttachment,
)
from app.schemas.generation import ChapterActionRequest, ChapterGenerateRequest
from app.schemas.knowledge import (
    KnowledgeEmbeddingRequest,
    KnowledgeEmbeddingResponse,
    KnowledgeProcessRequest,
    KnowledgeRerankRequest,
    KnowledgeRerankResponse,
    KnowledgeRerankResult,
)
from app.schemas.tender import TenderParseRequest

logger = logging.getLogger(__name__)


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
DEFAULT_MINIO_ACCESS_KEY = "zbt_minio"
DEFAULT_MINIO_SECRET_KEY = "zbt_minio_secret"
MIN_PRODUCTION_SECRET_LENGTH = 16
DEFAULT_CALLBACK_ALLOWED_HOSTS = {"backend", "localhost", "127.0.0.1", "host.docker.internal"}
DEFAULT_CALLBACK_MAX_ATTEMPTS = 3
DEFAULT_CALLBACK_RETRY_DELAY_SECONDS = 0.25
DEFAULT_EMBEDDING_BATCH_SIZE = 32
DEFAULT_TASK_OBJECT_MAX_BYTES = 128 * 1024 * 1024
MAX_TASK_OBJECT_MAX_BYTES = 256 * 1024 * 1024
MINIO_READ_CHUNK_BYTES = 1024 * 1024


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
async def tender_parse(
    payload: TenderParseRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route = router.resolve("tender_parse", tenant_id=payload.tenant_id)
    task_suffix = (payload.bid_id or payload.file_id).replace("-", "")[:12]
    task_id = payload.task_id or f"task-tender-parse-{task_suffix}-{uuid.uuid4().hex[:8]}"
    background_tasks.add_task(process_tender_parse, task_id, payload)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


def process_tender_parse(task_id: str, payload: TenderParseRequest) -> None:
    try:
        client = minio_client()
        ensure_tenant_object_key_allowed(payload.tenant_id, payload.object_key)
        content = download_minio_object_bytes(
            client,
            payload.object_key,
            max_bytes=task_object_max_bytes(),
            limit_name="task source object",
        )
        parse_payload = KnowledgeProcessRequest(
            tenant_id=payload.tenant_id,
            document_id=payload.bid_id or payload.file_id,
            file_id=payload.file_id,
            object_key=payload.object_key,
            filename=payload.filename,
            content_type=payload.content_type,
        )
        parsed = parse_document(parse_payload, content)
        base_structured = build_tender_structured_result(payload, parsed)
        prompt = build_tender_parse_prompt(payload, parsed, base_structured)

        def generate(route: RouteTarget, provider: object) -> tuple[dict[str, object], int, int]:
            model_result = provider.generate_json(prompt, route.schema_name or "TenderParseResult")
            structured = merge_tender_structured_result(base_structured, model_result)
            output_text = json.dumps(structured, ensure_ascii=False)
            input_tokens = provider.count_tokens(prompt) if hasattr(provider, "count_tokens") else estimate_tokens(prompt)
            output_tokens = provider.count_tokens(output_text) if hasattr(provider, "count_tokens") else estimate_tokens(output_text)
            return structured, input_tokens, output_tokens

        route, provider, generated = run_llm_task("tender_parse", payload.tenant_id, generate)
        structured, input_tokens, output_tokens = generated
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": {
                "bid_id": payload.bid_id,
                "file_id": payload.file_id,
                "filename": payload.filename,
                "structured_result": structured,
                "summary": parsed.summary,
                "metadata": parsed.metadata,
                "chunk_count": len(parsed.chunks),
                "model_metadata": {
                    "provider": provider_name(provider, route),
                    "model": route.model,
                    "fallback_from": route.fallback_from,
                    "parser": parsed.metadata.get("parser"),
                },
                "token_usage": {
                    "input_tokens": input_tokens,
                    "output_tokens": output_tokens,
                },
            },
        }
    except Exception:  # pragma: no cover - defensive task boundary
        callback_payload = task_failure_callback(
            payload.tenant_id,
            task_id,
            "招标文件解读失败，请检查文件后重试",
            {"bid_id": payload.bid_id, "file_id": payload.file_id},
        )
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


@app.post("/tasks/knowledge-process", response_model=TaskAccepted, status_code=202)
async def knowledge_process(
    payload: KnowledgeProcessRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route = router.resolve("knowledge_process", tenant_id=payload.tenant_id)
    task_suffix = payload.document_id.replace("-", "")[:12]
    task_id = payload.task_id or f"task-knowledge-{task_suffix}"
    background_tasks.add_task(process_knowledge_document, task_id, payload)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


@app.post("/embeddings/knowledge", response_model=KnowledgeEmbeddingResponse)
async def knowledge_embeddings(payload: KnowledgeEmbeddingRequest) -> KnowledgeEmbeddingResponse:
    route, provider, embeddings = run_provider_task(
        "knowledge_embedding",
        payload.tenant_id,
        lambda _route, candidate: candidate.embed_batch(payload.texts),
        provider_kind="embedding",
    )
    return KnowledgeEmbeddingResponse(
        provider=provider_name(provider, route),
        model=route.model,
        dimensions=provider_dimensions(provider, route, embeddings),
        embeddings=embeddings,
        route=route.model_dump(),
    )


@app.post("/rerank/knowledge", response_model=KnowledgeRerankResponse)
async def knowledge_rerank(payload: KnowledgeRerankRequest) -> KnowledgeRerankResponse:
    document_texts = [
        f"{document.title}\n{document.section_path}\n{document.content}" for document in payload.documents
    ]
    route, provider, ordered_indexes = run_provider_task(
        "knowledge_rerank",
        payload.tenant_id,
        lambda _route, candidate: candidate.rerank(payload.query, document_texts),
        provider_kind="rerank",
    )
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
        provider=provider_name(provider, route),
        model=route.model,
        results=results,
        route=route.model_dump(),
    )


def process_knowledge_document(task_id: str, payload: KnowledgeProcessRequest) -> None:
    try:
        client = minio_client()
        ensure_tenant_object_key_allowed(payload.tenant_id, payload.object_key)
        content = download_minio_object_bytes(
            client,
            payload.object_key,
            max_bytes=task_object_max_bytes(),
            limit_name="task source object",
        )
        parsed = parse_document(payload, content)
        embedding_inputs = [
            f"{chunk.title}\n{chunk.section_path}\n{chunk.content}" for chunk in parsed.chunks
        ]
        embedding_route, embedding_provider, embeddings = run_provider_task(
            "knowledge_embedding",
            payload.tenant_id,
            lambda _route, candidate: embed_knowledge_inputs(candidate, embedding_inputs),
            provider_kind="embedding",
        )
        input_tokens = sum(estimate_tokens(text) for text in embedding_inputs)
        if len(embeddings) != len(parsed.chunks):
            raise RuntimeError(f"embedding count mismatch: got {len(embeddings)} want {len(parsed.chunks)}")
        embedding_provider_name = provider_name(embedding_provider, embedding_route)
        embedding_dimensions = provider_dimensions(embedding_provider, embedding_route, embeddings)
        for chunk, embedding in zip(parsed.chunks, embeddings, strict=True):
            chunk.embedding = embedding
            chunk.metadata["embedding_model"] = embedding_route.model
            chunk.metadata["embedding_provider"] = embedding_provider_name
            chunk.metadata["embedding_dimensions"] = embedding_dimensions
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
                "embedding_provider": embedding_provider_name,
                "embedding_dimensions": embedding_dimensions,
                "model_metadata": {
                    "provider": embedding_provider_name,
                    "model": embedding_route.model,
                    "embedding_dimensions": embedding_dimensions,
                    "fallback_from": embedding_route.fallback_from,
                },
                "token_usage": {
                    "input_tokens": input_tokens,
                    "output_tokens": 0,
                },
            },
        }
    except Exception:  # pragma: no cover - defensive task boundary
        callback_payload = task_failure_callback(
            payload.tenant_id,
            task_id,
            "知识库文档整理失败，请稍后重试",
            {},
        )
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


def embed_knowledge_inputs(provider: object, texts: list[str]) -> list[list[float]]:
    if not texts:
        return []
    batch_size = embedding_batch_size()
    expected_dimensions = provider.get_dimensions() if hasattr(provider, "get_dimensions") else 0
    embeddings: list[list[float]] = []
    for start in range(0, len(texts), batch_size):
        batch = texts[start : start + batch_size]
        batch_embeddings = provider.embed_batch(batch)
        if len(batch_embeddings) != len(batch):
            raise RuntimeError(
                f"embedding batch count mismatch: got {len(batch_embeddings)} want {len(batch)}"
            )
        if expected_dimensions:
            for index, embedding in enumerate(batch_embeddings):
                if len(embedding) != expected_dimensions:
                    absolute_index = start + index
                    raise RuntimeError(
                        "embedding dimension mismatch: "
                        f"chunk {absolute_index} got {len(embedding)} want {expected_dimensions}"
                    )
        embeddings.extend(batch_embeddings)
    return embeddings


def embedding_batch_size() -> int:
    configured = os.getenv("KNOWLEDGE_EMBEDDING_BATCH_SIZE", "").strip()
    if not configured:
        return DEFAULT_EMBEDDING_BATCH_SIZE
    try:
        value = int(configured)
    except ValueError:
        return DEFAULT_EMBEDDING_BATCH_SIZE
    return min(max(value, 1), DEFAULT_EMBEDDING_BATCH_SIZE)


def post_callback(callback_url: str, payload: dict[str, object]) -> None:
    ensure_callback_url_allowed(callback_url)
    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    attempts = callback_max_attempts()
    delay = callback_retry_delay_seconds()
    host = urlparse(callback_url).hostname or ""
    for attempt in range(1, attempts + 1):
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
        try:
            with request.urlopen(req, timeout=10) as response:
                response.read()
            return
        except Exception:
            if attempt >= attempts:
                raise
            logger.warning("AI task callback delivery retrying: host=%s attempt=%s/%s", host, attempt, attempts)
            if delay > 0:
                time.sleep(delay)


def callback_max_attempts() -> int:
    configured = os.getenv("AI_CALLBACK_MAX_ATTEMPTS", "").strip()
    if not configured:
        return DEFAULT_CALLBACK_MAX_ATTEMPTS
    try:
        value = int(configured)
    except ValueError:
        return DEFAULT_CALLBACK_MAX_ATTEMPTS
    return min(max(value, 1), 5)


def callback_retry_delay_seconds() -> float:
    configured = os.getenv("AI_CALLBACK_RETRY_DELAY_SECONDS", "").strip()
    if not configured:
        return DEFAULT_CALLBACK_RETRY_DELAY_SECONDS
    try:
        value = float(configured)
    except ValueError:
        return DEFAULT_CALLBACK_RETRY_DELAY_SECONDS
    return min(max(value, 0.0), 5.0)


def ensure_callback_url_allowed(callback_url: str) -> None:
    parsed = urlparse(callback_url)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise RuntimeError("callback_url must be an absolute http(s) URL")
    host = parsed.hostname.rstrip(".").lower()
    if host not in callback_allowed_hosts():
        raise RuntimeError(f"callback_url host is not allowed: {host}")


def callback_allowed_hosts() -> set[str]:
    configured = os.getenv("AI_CALLBACK_ALLOWED_HOSTS", "").strip()
    if not configured:
        return set(DEFAULT_CALLBACK_ALLOWED_HOSTS)
    hosts: set[str] = set()
    for item in configured.split(","):
        value = item.strip()
        if not value:
            continue
        parsed = urlparse(value if "://" in value else f"//{value}", scheme="http")
        host = parsed.hostname or value
        hosts.add(host.rstrip(".").lower())
    return hosts


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


def provider_attempts(task_type: str, tenant_id: str, provider_kind: str) -> list[tuple[RouteTarget, object]]:
    if hasattr(router, "resolve_candidates") and hasattr(router, "provider_for_target"):
        return [
            (target, router.provider_for_target(target))
            for target in router.resolve_candidates(task_type, tenant_id=tenant_id)
        ]
    target = router.resolve(task_type, tenant_id=tenant_id)
    if provider_kind == "embedding":
        provider = router.get_embedding(task_type, tenant_id=tenant_id)
    elif provider_kind == "rerank":
        provider = router.get_rerank(task_type, tenant_id=tenant_id)
    else:
        provider = router.get_llm(task_type, tenant_id=tenant_id)
    return [(target, provider)]


def provider_name(provider: object, route: RouteTarget) -> str:
    name = getattr(provider, "name", "")
    return str(name).strip() or route.provider


def provider_dimensions(provider: object, route: RouteTarget, embeddings: list[list[float]] | None = None) -> int:
    if hasattr(provider, "get_dimensions"):
        return int(provider.get_dimensions())
    if route.dimensions:
        return route.dimensions
    if embeddings:
        return len(embeddings[0])
    return 0


def run_provider_task(
    task_type: str,
    tenant_id: str,
    operation,
    *,
    provider_kind: str = "llm",
) -> tuple[RouteTarget, object, object]:
    last_error: Exception | None = None
    for route, provider in provider_attempts(task_type, tenant_id, provider_kind):
        try:
            return route, provider, operation(route, provider)
        except Exception as exc:
            last_error = exc
            logger.warning(
                "AI provider call failed; task_type=%s provider=%s model=%s error_type=%s",
                task_type,
                provider_name(provider, route),
                route.model,
                type(exc).__name__,
            )
    raise RuntimeError(f"all configured providers failed for {task_type}") from last_error


def run_llm_task(
    task_type: str,
    tenant_id: str,
    operation,
) -> tuple[RouteTarget, object, object]:
    return run_provider_task(task_type, tenant_id, operation)


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
        def generate(route: RouteTarget, provider: object) -> object:
            payload.model_hint = route.model
            return provider.generate_chapter(payload)

        _, _, generation = run_llm_task("chapter_generate", payload.tenant_id, generate)
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": generation.model_dump(),
        }
    except Exception:  # pragma: no cover - defensive task boundary
        callback_payload = task_failure_callback(
            payload.tenant_id,
            task_id,
            "章节生成失败，请稍后重试",
            {"chapter_id": payload.chapter_id},
        )
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
        _ = model_hint

        def generate(route: RouteTarget, provider: object) -> object:
            payload.model_hint = route.model
            return provider.chapter_action(payload)

        _, _, generation = run_llm_task(route_name, payload.tenant_id, generate)
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": generation.model_dump(),
        }
    except Exception:  # pragma: no cover - defensive task boundary
        callback_payload = task_failure_callback(
            payload.tenant_id,
            task_id,
            "章节处理失败，请稍后重试",
            {"chapter_id": payload.chapter_id, "action": payload.action},
        )
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
        _ = model_hint

        def generate(route: RouteTarget, provider: object) -> object:
            payload.model_hint = route.model
            return provider.cost_advice(payload)

        _, _, result = run_llm_task("cost_advice", payload.tenant_id, generate)
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": result.model_dump(),
        }
    except Exception:  # pragma: no cover - defensive task boundary
        callback_payload = task_failure_callback(
            payload.tenant_id,
            task_id,
            "成本建议生成失败，请稍后重试",
            {"cost_project_id": payload.cost_project_id},
        )
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
    task_id = payload.task_id or f"task-export-{task_suffix}"
    background_tasks.add_task(process_document_export, task_id, payload, export_type)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


def process_document_export(task_id: str, payload: DocumentExportRequest, export_type: str) -> None:
    with TemporaryDirectory(prefix="zbt-ai-export-") as tmpdir:
        output_path = Path(tmpdir) / safe_output_filename(payload.filename, export_type)
        try:
            client = minio_client()
            ensure_tenant_object_key_allowed(payload.tenant_id, payload.object_key)
            payload = hydrate_export_attachment_content(client, payload)
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
        except Exception:  # pragma: no cover - defensive task boundary
            callback_payload = task_failure_callback(
                payload.tenant_id,
                task_id,
                "导出文件生成失败，请检查内容后重试",
                {"export_id": payload.export_id},
            )
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)


def task_failure_callback(
    tenant_id: str,
    task_id: str,
    public_message: str,
    result_refs: dict[str, object],
) -> dict[str, object]:
    logger.exception("AI background task failed: task_id=%s", task_id)
    return {
        "tenant_id": tenant_id,
        "task_id": task_id,
        "status": "failed",
        "error_message": public_message,
        "result": {"error": public_message, **result_refs},
    }


def hydrate_export_attachment_content(client: Minio, payload: DocumentExportRequest) -> DocumentExportRequest:
    return payload.model_copy(
        update={
            "attachments": [
                hydrate_export_attachment(client, payload.tenant_id, attachment)
                for attachment in payload.attachments
            ],
            "boq_files": [
                hydrate_export_attachment(client, payload.tenant_id, attachment)
                for attachment in payload.boq_files
            ],
            "parts": [
                part.model_copy(
                    update={
                        "attachments": [
                            hydrate_export_attachment(client, payload.tenant_id, attachment)
                            for attachment in part.attachments
                        ]
                    }
                )
                for part in payload.parts
            ],
        }
    )


def hydrate_export_attachment(
    client: Minio,
    tenant_id: str,
    attachment: ExportAttachment,
) -> ExportAttachment:
    if attachment.object_key:
        ensure_tenant_object_key_allowed(tenant_id, attachment.object_key)
    if attachment.content_base64 or attachment.local_path or not attachment.object_key:
        return attachment
    return attachment.model_copy(
        update={"content_base64": download_minio_object_base64(client, attachment.object_key)}
    )


def ensure_tenant_object_key_allowed(tenant_id: str, object_key: str) -> None:
    tenant = tenant_id.strip()
    key = object_key.strip()
    key_parts = key.split("/")
    if (
        not tenant
        or tenant != tenant.strip("/")
        or "/" in tenant
        or "\\" in tenant
        or not key
        or key != object_key
        or key.startswith("/")
        or "\\" in key
        or "://" in key
        or len(key_parts) < 2
        or key_parts[0] != tenant
        or any(part in {"", ".", ".."} for part in key_parts)
    ):
        raise RuntimeError("object_key is outside tenant scope")


def task_object_max_bytes() -> int:
    return bounded_env_int(
        "AI_TASK_OBJECT_MAX_BYTES",
        DEFAULT_TASK_OBJECT_MAX_BYTES,
        minimum=1,
        maximum=MAX_TASK_OBJECT_MAX_BYTES,
    )


def export_attachment_max_bytes() -> int:
    return bounded_env_int(
        "AI_EXPORT_ATTACHMENT_MAX_BYTES",
        MAX_EXPORT_INLINE_ATTACHMENT_BYTES,
        minimum=1,
        maximum=MAX_EXPORT_INLINE_ATTACHMENT_BYTES,
    )


def bounded_env_int(name: str, default: int, *, minimum: int, maximum: int) -> int:
    configured = os.getenv(name, "").strip()
    if not configured:
        return default
    try:
        value = int(configured)
    except ValueError:
        return default
    return min(max(value, minimum), maximum)


def download_minio_object_bytes(
    client: Minio,
    object_key: str,
    *,
    max_bytes: int,
    limit_name: str,
) -> bytes:
    response = client.get_object(os.getenv("MINIO_BUCKET", "zbt-files"), object_key)
    try:
        return read_response_bytes(response, max_bytes=max_bytes, limit_name=limit_name)
    finally:
        response.close()
        response.release_conn()


def read_response_bytes(response: object, *, max_bytes: int, limit_name: str) -> bytes:
    chunks: list[bytes] = []
    total = 0
    while True:
        remaining = max_bytes + 1 - total
        try:
            chunk = response.read(min(MINIO_READ_CHUNK_BYTES, max(remaining, 1)))
        except TypeError:
            chunk = response.read()
            if isinstance(chunk, str):
                chunk = chunk.encode()
            if len(chunk) > max_bytes:
                raise RuntimeError(f"{limit_name} exceeds configured {max_bytes} byte limit")
            return chunk
        if not chunk:
            break
        if isinstance(chunk, str):
            chunk = chunk.encode()
        total += len(chunk)
        if total > max_bytes:
            raise RuntimeError(f"{limit_name} exceeds configured {max_bytes} byte limit")
        chunks.append(chunk)
    return b"".join(chunks)


def download_minio_object_base64(client: Minio, object_key: str) -> str:
    content = download_minio_object_bytes(
        client,
        object_key,
        max_bytes=export_attachment_max_bytes(),
        limit_name="export attachment object",
    )
    return base64.b64encode(content).decode("ascii")


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
    if insecure_config_value(ai_service_hmac_secret(), DEFAULT_AI_HMAC_SECRET):
        raise RuntimeError("AI_SERVICE_HMAC_SECRET must be set to a non-development value in production")
    if insecure_config_value(os.getenv("MINIO_ACCESS_KEY", DEFAULT_MINIO_ACCESS_KEY), DEFAULT_MINIO_ACCESS_KEY):
        raise RuntimeError("MINIO_ACCESS_KEY must be set to a non-development value in production")
    if insecure_config_value(os.getenv("MINIO_SECRET_KEY", DEFAULT_MINIO_SECRET_KEY), DEFAULT_MINIO_SECRET_KEY):
        raise RuntimeError("MINIO_SECRET_KEY must be set to a non-development value in production")
    if allow_mock_providers_in_production():
        return
    if os.getenv("USE_MOCK_PROVIDERS", "true").strip().lower() not in {"0", "false", "no"}:
        raise RuntimeError("USE_MOCK_PROVIDERS must be false in production")
    if os.getenv("ALLOW_MOCK_FALLBACK", "true").strip().lower() not in {"0", "false", "no"}:
        raise RuntimeError("ALLOW_MOCK_FALLBACK must be false in production")
    mock_routes = router.provider_backed_mock_routes()
    if mock_routes:
        preview = ", ".join(mock_routes[:8])
        raise RuntimeError(f"MockProvider is not allowed in production model routes: {preview}")


def production_mode() -> bool:
    return any(
        os.getenv(key, "").strip().lower() in {"prod", "production", "release"}
        for key in ("APP_ENV", "ZBT_ENV", "ENVIRONMENT", "GIN_MODE")
    )


def allow_mock_providers_in_production() -> bool:
    return os.getenv("ALLOW_MOCK_PROVIDERS_IN_PRODUCTION", "").strip().lower() in {"1", "true", "yes"}


def insecure_config_value(value: str, development_default: str) -> bool:
    value = value.strip()
    return value == "" or value == development_default or len(value) < MIN_PRODUCTION_SECRET_LENGTH

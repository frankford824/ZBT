from __future__ import annotations

import hashlib
import hmac
import json
import os
import time
from datetime import UTC, datetime
from pathlib import Path
from urllib import request

from fastapi import BackgroundTasks, FastAPI
from minio import Minio

from app.gateway.model_router import ModelRouter
from app.pipelines.export.docx_exporter import export_bid_docx
from app.pipelines.parse.document_parser import parse_document
from app.schemas.common import HealthResponse, TaskAccepted
from app.schemas.export import DocxExportRequest
from app.schemas.generation import ChapterGenerateRequest, ChapterGenerateResponse
from app.schemas.knowledge import KnowledgeProcessRequest

CONFIG_PATH = Path(__file__).parent / "config" / "model_routing.yaml"

app = FastAPI(title="ZhiBiaoTong AI Service", version="0.1.0")
router = ModelRouter.from_yaml(CONFIG_PATH)


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
    secret = os.getenv("AI_SERVICE_HMAC_SECRET", "")
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


@app.post("/tasks/chapter-generate", response_model=ChapterGenerateResponse)
async def chapter_generate(payload: ChapterGenerateRequest) -> ChapterGenerateResponse:
    provider = router.get_llm("chapter_generate", tenant_id=payload.tenant_id)
    return provider.generate_chapter(payload)


@app.post("/tasks/export/docx", response_model=TaskAccepted, status_code=202)
async def export_docx(
    payload: DocxExportRequest,
    background_tasks: BackgroundTasks,
) -> TaskAccepted:
    route = router.resolve("document_export", tenant_id=payload.tenant_id)
    task_suffix = payload.export_id.replace("-", "")[:12]
    task_id = f"task-export-{task_suffix}"
    background_tasks.add_task(process_docx_export, task_id, payload)
    return TaskAccepted(task_id=task_id, status="queued", route=route.model_dump())


def process_docx_export(task_id: str, payload: DocxExportRequest) -> None:
    output_path = Path("/tmp") / payload.filename
    try:
        export_bid_docx(payload.bid_title, payload.part_title, payload.chapters, output_path)
        client = minio_client()
        client.fput_object(
            os.getenv("MINIO_BUCKET", "zbt-files"),
            payload.object_key,
            str(output_path),
            content_type="application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        )
        callback_payload = {
            "tenant_id": payload.tenant_id,
            "task_id": task_id,
            "status": "done",
            "result": {
                "export_id": payload.export_id,
                "bid_id": payload.bid_id,
                "filename": payload.filename,
                "object_key": payload.object_key,
                "part_code": payload.part_code,
                "chapter_count": len(payload.chapters),
                "size_bytes": output_path.stat().st_size,
                "content_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
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
    finally:
        try:
            output_path.unlink(missing_ok=True)
        except OSError:
            pass
    if payload.callback_url:
        post_callback(payload.callback_url, callback_payload)

from __future__ import annotations

from datetime import UTC, datetime
from pathlib import Path

from fastapi import FastAPI

from app.gateway.model_router import ModelRouter
from app.schemas.common import HealthResponse, TaskAccepted
from app.schemas.generation import ChapterGenerateRequest, ChapterGenerateResponse

CONFIG_PATH = Path(__file__).parent / "config" / "model_routing.yaml"

app = FastAPI(title="ZhiBiaoTong AI Service", version="0.1.0")
router = ModelRouter.from_yaml(CONFIG_PATH)


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


@app.post("/tasks/chapter-generate", response_model=ChapterGenerateResponse)
async def chapter_generate(payload: ChapterGenerateRequest) -> ChapterGenerateResponse:
    provider = router.get_llm("chapter_generate", tenant_id=payload.tenant_id)
    return provider.generate_chapter(payload)


@app.post("/tasks/export/docx", response_model=TaskAccepted, status_code=202)
async def export_docx() -> TaskAccepted:
    route = router.resolve("document_export", tenant_id="tenant-demo")
    return TaskAccepted(task_id="task-export-docx-demo", status="queued", route=route.model_dump())

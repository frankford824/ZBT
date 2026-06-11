# 智标通 ZBT

智标通是面向企业投标团队的 B2B 智能标书生成 SaaS 平台雏形。本仓库按 `x.md` 的 Loop 协议推进，固定采用 React/Vite 前端、Go 主业务后端、Python AI/RAG/文档服务、PostgreSQL + RLS + pgvector、Redis 和 MinIO。

## 当前状态

- `docs/input`：PRD V2.0-V2.4、HTML 高保真原型、技术方案评估。
- `docs/blueprint`：Loop-0 蓝图、裁决表、路由、数据库、API、状态机、权限、AI/RAG/导出设计。
- `frontend`：React + TypeScript + Vite + Ant Design 后台，已覆盖 V2.4 的 14 个页面和详情路由。
- `backend`：Go + Gin 模块化单体骨架，包含租户上下文、RBAC、审计中间件、状态机、API stub、pgx、goose、Asynq 和 RLS 迁移。
- `ai-service`：Python FastAPI 服务，包含 ModelRouter、Provider 抽象、MockProvider、Pydantic schema、RAG VectorStore 抽象和最小 docx 导出函数。

## 本地检查

```bash
cd frontend && pnpm install && pnpm build
cd ../backend && GOTOOLCHAIN=local go test ./...
cd ../ai-service && python3 -m compileall app
docker compose config
```

## Docker 启动

```bash
cp .env.example .env
docker compose up -d --build
curl http://localhost:8080/healthz
curl http://localhost:8000/healthz
curl http://localhost:5173/api/v1/meta/routes
curl http://localhost:8000/models/health
```

服务端口：

- 前端：`http://localhost:5173`
- Go 后端：`http://localhost:8080`
- Python AI 服务：`http://localhost:8000`
- MinIO Console：`http://localhost:9001`

说明：AI 镜像内置 LibreOffice Writer 和 WenQuanYi 中文字体，用于后续 Word/PDF/ZIP 导出链路验证。

## 开发约束

1. 不删除 8 大模块，不少于 V2.4 的 14 个页面。
2. 主业务后端使用 Go，不降级为全 Python 单体。
3. AI、RAG、文档解析和导出在 Python 服务，所有模型调用经过 ModelRouter。
4. PostgreSQL 一期使用 pgvector，并保留 VectorStore 抽象。
5. 所有业务表必须带 tenant_id，并启用 RLS。
6. 禁止 GORM、Drools、Rust 主后端、重 LangChain。
7. API Key 只走 `.env` 或密钥管理，不写入代码、prompt、日志或数据库。

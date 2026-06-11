# 开发循环日志

## Loop-0 / Loop-1 - 2026-06-10

### 本轮目标

1. 整理输入资料到 docs/input。
2. 固化 V2.4 页面、Go + Python 双服务、pgvector、ModelRouter、RLS、RBAC 等硬约束。
3. 创建 docs/blueprint 全套文档。
4. 搭建 frontend、backend、ai-service 和 Docker 基础骨架。
5. 运行可用检查命令并记录结果。

### 输入文件检查

原始文件位于 doc/，已移动并重命名到 docs/input。未发现缺失输入文件。

### 偏离蓝图

否。本轮允许先用 MockProvider 和内存/样例数据建立可运行骨架，后续 Loop 继续补齐真实业务流。

### 代码交付

1. 前端：创建 React + TypeScript + Vite + Ant Design 后台骨架，覆盖 V2.4 的 8 个主页面、6 个子页面和详情级路由。
2. 后端：创建 Go + Gin 主业务服务，接入租户上下文、RBAC、审计中间件、pgx、goose、Asynq 客户端、状态机测试和 API stub。
3. AI 服务：创建 FastAPI 服务，接入 ModelRouter、Provider 抽象、MockProvider、Pydantic schema、VectorStore 抽象和最小 docx 导出函数。
4. 基础设施：创建 Dockerfile、docker-compose、Nginx 前端代理、环境变量样例和一键检查脚本。

### 检查结果

2026-06-11 已运行：

```bash
./infra/scripts/check.sh
docker compose up -d
curl -sS http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8000/healthz
curl -sS http://127.0.0.1:5173/api/v1/meta/routes
curl -sS http://127.0.0.1:8000/models/health
curl -sS -X POST http://127.0.0.1:8000/tasks/chapter-generate ...
docker exec -i love-ai-service-1 python - <<'PY'
from pathlib import Path
from app.pipelines.export.docx_exporter import export_minimal_docx
out = export_minimal_docx('智标通导出验证', ['技术标章节示例', '商务标章节示例'], Path('/tmp/zbt-smoke.docx'))
print(out.exists(), out.stat().st_size)
PY
```

结果：

1. `./infra/scripts/check.sh` 通过：前端生产构建、Go 测试、AI `compileall`、compose config 均成功。
2. `docker compose up -d` 成功启动 postgres、redis、minio、backend、ai-service、frontend。
3. 后端 `/healthz`、AI `/healthz`、AI `/models/health` 均返回 ok。
4. 前端 Nginx 代理 GET `/api/v1/meta/routes` 成功转发到后端。
5. AI 逐章生成接口返回符合 schema 的 Tiptap JSON、source_refs、needs_human_input、模型元数据和 token_usage。
6. AI 容器内确认 `/usr/bin/soffice`、LibreOffice 25.2.3.2 和 WenQuanYi Zen Hei 字体可用。
7. `export_minimal_docx` 已在 AI 容器内生成 `/tmp/zbt-smoke.docx`，文件大小 36685 bytes。

### 构建问题与处理

1. DockerHub 元数据请求曾出现 EOF，重试 `docker pull python:3.12-slim` 后恢复。
2. Aliyun Debian 镜像下载 `libreoffice-core` 大包失败，已将 `ai-service/Dockerfile` 改为 HTTPS `deb.debian.org`，并加入 apt retry、timeout、no-cache、no-pipeline 选项。
3. 前端代理用 GET 验证通过；HEAD `/api/v1/meta/routes` 返回 404 是后端未注册 HEAD 方法，不影响浏览器 API 调用。

### 下一轮建议

Loop-2 应优先把 SaaS 底座从骨架推进到可用：执行数据库迁移、补齐真实登录/租户/角色/成员 API、让前端从 API 获取真实当前用户与权限，再进入标讯解析和项目创建链路。

## Loop-2 - 2026-06-11

### 本轮目标

1. 将 SaaS 底座从 stub 推进到真实数据库 API。
2. 落地 JWT 登录、当前用户、当前租户、成员、角色、通知接口。
3. 将 RBAC 中间件改为读取数据库模块权限。
4. 使用非超级应用账号验证 RLS，避免 owner/superuser 绕过。
5. 前端登录页和团队页接入真实 API。

### 代码交付

1. 后端启动时通过嵌入 goose migration 自动执行迁移。
2. 新增 `zbt_app` 非超级应用账号；迁移使用 owner 连接，业务查询使用 app 连接。
3. 新增 `tenant_member_roles`、模块权限唯一约束、RLS helper、FORCE RLS、6 个预置角色和 demo 成员种子。
4. 新增最小 HS256 JWT 签发/解析和单元测试。
5. `/api/v1/auth/login`、`/me`、`/tenant`、`/tenant/members`、`/roles`、`/notifications` 改为真实 DB 响应。
6. 非 GET stub 统一要求 full 权限，GET 要求 read 权限。
7. 前端登录页调用真实登录接口，Zustand/localStorage 保存 token、租户、用户和权限。
8. 前端请求拦截器自动携带 Bearer token 和 X-Tenant-ID。
9. 团队页成员、角色、通知读取真实 API；邀请成员调用后端创建成员。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
./infra/scripts/check.sh
docker compose build backend frontend
docker compose up -d backend frontend
curl -sS -X POST http://127.0.0.1:5173/api/v1/auth/login ...
curl -sS http://127.0.0.1:5173/api/v1/me -H "Authorization: Bearer <admin>"
curl -sS http://127.0.0.1:5173/api/v1/roles -H "Authorization: Bearer <admin>"
curl -sS http://127.0.0.1:5173/api/v1/tenant/members -H "Authorization: Bearer <admin>"
curl -sS http://127.0.0.1:5173/api/v1/cost-projects -H "Authorization: Bearer <viewer>"
docker compose exec -T postgres psql 'postgres://zbt_app:zbt_app@localhost:5432/zbt?sslmode=disable' ...
```

结果：

1. `./infra/scripts/check.sh` 通过。
2. backend 自动执行到 goose version 2。
3. `zbt_app` 存在且 `rolsuper=false`。
4. admin 登录返回 JWT，`/me` 返回真实 user、tenant、role、permissions。
5. admin 读取角色数 6、成员数 4。
6. viewer 访问 `/api/v1/cost-projects` 返回 403。
7. 使用 `zbt_app` 验证 RLS：tenant1 角色数 6，tenant2 角色数 1，未设置租户时角色数 0。

### 偏离蓝图

1. JWT refresh token 尚未实现，本轮只实现 8 小时 access token。
2. MinIO 预签名上传下载仍是下一步重点；当前只完成 DB 表和权限边界。
3. 文件越权下载测试尚未覆盖，因为真实 presign API 尚未落地。

### 下一轮建议

继续 Loop-2 收尾：实现 MinIO bucket 初始化、文件资产创建、预签名上传、confirm、下载/预览 URL，并补文件归属和权限测试。随后进入 Loop-3/4 的真实页面数据和标书主链路。

## Loop-2 文件闭环 - 2026-06-11

### 本轮目标

1. 补齐 MinIO bucket 初始化和私有对象存储访问。
2. 实现 file_assets 创建、预签名上传、confirm、下载和预览 URL。
3. 将知识库文档页从静态列表改为真实 API 数据。
4. 验证 object key 租户前缀、RLS 隔离、文件上传权限和跨租户下载不可见。

### 代码交付

1. 新增 `platform/file` 服务，封装 MinIO client、bucket 初始化、file_assets 事务和预签名 URL。
2. 新增 `00003_file_asset_status.sql`，为 file_assets 增加 status、confirmed_at、object_key 唯一索引和查询索引。
3. 新增 `00004_fix_module_permission_tenant_ids.sql`，修复历史种子中 tenant2 角色权限 tenant_id 错误；同时修正 00001 新库种子逻辑。
4. 后端接入 `/files/presign-upload`、`/files/:id/confirm`、`/files/:id/download-url`、`/files/:id/preview-url` 和 `/knowledge/documents` 真实 handler。
5. 前端文档库页接真实文件列表，上传流程为：请求预签名 URL -> PUT MinIO -> confirm -> 刷新列表。
6. 文件预览页通过 Go 鉴权后获取 inline 预签名 URL。
7. Docker Compose 为 backend 增加 MinIO 健康依赖、访问密钥、region 和 public endpoint 配置。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
./infra/scripts/check.sh
docker compose build backend frontend
docker compose up -d backend frontend
curl -X POST /api/v1/files/presign-upload ...
curl -X PUT "<presigned-url>" --data-binary @/tmp/zbt-minio-check.txt
curl -X POST /api/v1/files/<id>/confirm
curl /api/v1/files/<id>/download-url
docker compose exec -T postgres psql 'postgres://zbt_app:zbt_app@localhost:5432/zbt?sslmode=disable' ...
```

结果：

1. `./infra/scripts/check.sh` 通过。
2. backend 自动执行到 goose version 4。
3. MinIO 上传 PUT 返回 200，confirm 后文件状态为 ready，size_bytes 为 23。
4. 下载预签名 URL 返回内容 `zbt minio runtime check`。
5. viewer 调用 `/files/presign-upload` 返回 403。
6. tenant2 管理员具有 knowledge full，但访问 tenant1 文件 `/download-url` 返回 404。
7. RLS 下 tenant1 可见 object_key：`00000000-0000-4000-8000-000000000001/knowledge/6bcabfc8-4d92-4b79-a6b9-6858ac261afb`；tenant2 对同一 file_id 可见数量为 0。

### 偏离蓝图

1. 文件解析、OCR、切片、embedding 和 knowledge_documents 业务表尚未落地，本轮只完成文件资产与对象存储闭环。
2. 浏览器端上传依赖 MinIO CORS，当前通过 curl 验证了预签名 URL；后续进入文档解析前应补 Playwright 上传冒烟。

### 下一轮建议

进入 Loop-3：围绕知识库文档处理流水线落地 knowledge_documents、分类/标签、解析任务创建，以及 Go -> Python AI 服务任务编排和回调验签。

## Loop-3 知识库文档处理底座 - 2026-06-11

### 本轮目标

1. 将知识库从 file_asset 列表推进为真实 knowledge_documents。
2. 补齐分类、标签、文档详情、文档处理任务和统计接口。
3. 建立 Go -> Python AI 服务任务编排，并提供 HMAC 回调验签入口。
4. 前端知识库首页、文档库、标签页接入真实知识库 API。

### 代码交付

1. 新增 `00005_knowledge_documents_tasks.sql`，创建 `knowledge_categories`、`knowledge_tags`、`knowledge_documents`、`knowledge_document_tags`、`knowledge_references`、`ai_tasks`，并启用 FORCE RLS。
2. 新增 `platform/knowledge` 服务，封装分类/标签 CRUD、文档列表/详情/更新、file_asset -> knowledge_document、处理任务、统计和回调落库。
3. 后端实现 `/knowledge/categories`、`/knowledge/tags`、`/knowledge/documents`、`/knowledge/documents/:id/process`、`/knowledge/stats`、`/ai-tasks/:taskId`。
4. 新增公开但 HMAC 验签的 `/api/v1/ai/callbacks/tasks`，签名内容为 `timestamp.body`。
5. Python AI 服务新增 `/tasks/knowledge-process`，通过 ModelRouter 解析 `knowledge_process` 路由后返回外部 task_id。
6. 前端知识库首页读取统计，文档库读取真实文档、分类树和处理任务，标签页读取真实标签和关联文档。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
cd ai-service && python3 -m compileall app
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
curl -X POST /api/v1/files/presign-upload ...
curl -X PUT "<presigned-url>" --data-binary @/tmp/zbt-knowledge-loop3.txt
curl -X POST /api/v1/files/<id>/confirm
curl /api/v1/knowledge/categories
curl /api/v1/knowledge/tags
curl /api/v1/knowledge/stats
curl -X POST /api/v1/knowledge/documents/<id>/process
curl -X POST /api/v1/ai/callbacks/tasks -H X-ZBT-Timestamp -H X-ZBT-Signature ...
curl /api/v1/ai-tasks/<taskId>
curl /api/v1/knowledge/documents/<id>
```

结果：

1. backend 自动执行到 goose version 5。
2. AI `/healthz` 与 `/models/health` 返回 ok，mock provider 健康。
3. 文件上传 PUT 返回 200，confirm 后生成 knowledge_document，初始 parse_status 为 ready。
4. 分类数 4，标签数 3，知识库统计 document_count 为 1。
5. `POST /knowledge/documents/:id/process` 创建本地任务 `b4ec7f38-bdcf-4976-9a09-ef80499e536d`，外部任务 `task-knowledge-ba4fd961048f`，状态 queued。
6. HMAC 回调返回 200；回调后 ai_task 状态为 done，knowledge_document 状态为 processed，summary 为 `知识库解析回调验证完成`。
7. tenant2 访问 tenant1 的 knowledge_document 返回 404。

### 偏离蓝图

1. Python 端尚未真正解析 PDF/Word/Excel/PPT，也未生成 chunks 与 embedding；当前为受 ModelRouter 管理的任务入口和回调闭环。
2. `knowledge_search` 仍是接口占位；混合检索、RRF、rerank 和 source_refs 落库将在后续 Loop 实现。

### 下一轮建议

继续 Loop-3：实现最小文档解析和 chunk 入库，将 text/plain / PDF / Word 的文本抽取结果写入 `knowledge_chunks`，再接入 `/knowledge/search` 的关键词检索与 source_refs 返回。

## Loop-3 最小解析与检索 - 2026-06-11

### 本轮目标

1. Python 后台任务从 MinIO 读取知识库文件，完成 text/plain、PDF 和 Word 的最小文本抽取。
2. AI 回调携带切片结果，Go 验签后写入 knowledge_chunks。
3. 将 `POST /knowledge/search` 从占位接口推进为真实租户内检索。
4. 前端知识库首页接入搜索结果展示，返回 chunk 和 source_refs 信息。

### 代码交付

1. 新增 `00006_knowledge_chunk_search.sql`，补充 knowledge_chunks 文档维度索引、创建时间索引和 PostgreSQL 全文检索 GIN 索引。
2. Python AI 服务新增 MinIO SDK 依赖和文档解析模块，支持 text/plain、PDF、Word 的最小抽取、摘要和约 1200 字符切片。
3. `/tasks/knowledge-process` 改为启动后台任务，读取 MinIO 对象后通过 HMAC 回调 Go。
4. Go 回调处理新增 chunks 替换写入逻辑，任务完成时更新文档状态并重建该文档切片。
5. 后端实现 `/knowledge/search`，按租户过滤 knowledge_chunks，返回搜索 items 和 source_refs。
6. 前端知识库首页接入真实搜索框、结果列表、文档标题、section_path、chunk_id 和内容摘要。
7. Docker Compose 为 ai-service 补齐 MinIO endpoint、bucket 和访问密钥环境变量。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd ai-service && python3 -m compileall app
cd frontend && pnpm build
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
curl http://127.0.0.1:8000/healthz
curl http://127.0.0.1:8000/models/health
```

结果：

1. backend 测试通过，AI compileall 通过，frontend build 通过。
2. backend 自动执行到 goose version 6。
3. AI `/healthz` 与 `/models/health` 返回 ok。
4. 端到端上传测试中，MinIO 预签名 PUT 返回 200。
5. Python 后台任务自动完成解析并回调，ai_task 状态为 done，knowledge_document 状态为 processed。
6. 搜索 `alpha_search_token` 返回 1 个 item 和 1 个 source_ref。
7. 返回 chunk id 为 `38e95835-a216-4a97-8e40-69ab3af935c5`，source_ref document id 为 `ba2fd5f8-e886-470e-8969-797854804e63`。
8. RLS 验证中，tenant1 对该文档可见 chunk 数为 1，tenant2 对同一 chunk 可见数量为 0。

### 偏离蓝图

1. 当前检索仍是 PostgreSQL 关键词检索，不是向量召回、RRF 融合和 rerank。
2. PDF 和 Word 只完成最小文本抽取，尚未处理表格、版式、图片、扫描件 OCR 和复杂结构。
3. 本轮返回 source_refs，但还没有将生成引用写入 knowledge_references。

### 下一轮建议

继续补 MockEmbeddingProvider、pgvector embedding 入库、混合检索、knowledge_references，以及生成链路的 source_refs 解析和落库。

## Loop-4 标书 docx 导出闭环 - 2026-06-11

### 本轮目标

1. 将标书模块从页面/stub 推进到最小真实数据模型。
2. 支持分离标书技术标、商务标 parts 和 chapters。
3. 打通 Go -> Python -> MinIO -> HMAC 回调 -> Go 落库的 docx 导出链路。
4. 前端 7 步向导第 7 步可创建导出任务、查看导出历史并下载完成文件。

### 代码交付

1. 新增 `00007_bid_export_foundation.sql`，创建 `bid_parts`、`bid_chapters`、`bid_exports`，启用 FORCE RLS，并种入一条可验证的分离标书。
2. 新增 `platform/bid` 服务，实现标书列表、创建标书、parts、chapters、exports、导出任务创建和导出回调处理。
3. 后端将 `/bids`、`/bids/:id`、`/bids/:id/parts`、`/bids/:id/chapters`、`POST /bids/:id/exports`、`GET /bids/:id/exports`、`GET /bid-exports/:exportId` 从 stub 切到真实 handler。
4. AI 服务新增 `DocxExportRequest`，`/tasks/export/docx` 后台生成 docx 并上传 MinIO，再通过 HMAC 回调 Go。
5. AI 回调入口改为先读取 `ai_tasks.resource_type`，再分派到 knowledge_document 或 bid_export 处理。
6. 前端标书列表接入真实 `/bids`，新建标书调用真实 `POST /bids`，向导导出步骤接入 docx 导出和下载。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd ai-service && python3 -m compileall app
cd frontend && pnpm build
git diff --check
./infra/scripts/check.sh
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
```

结果：

1. backend 测试通过，AI compileall 通过，frontend build 通过，`./infra/scripts/check.sh` 通过。
2. backend 自动执行到 goose version 7。
3. AI `/healthz` 与 `/models/health` 返回 ok。
4. 端到端导出使用 bid `50000000-0000-4000-8000-000000000001`，parts 返回 `tech` 和 `business`。
5. `POST /bids/:id/exports` 创建导出 `52c5f4d6-210b-470b-b1c0-dde9b089ad2d`，本地任务 `59c13507-8442-433d-8eab-01264aa13312`，外部任务 `task-export-52c5f4d6210b`。
6. Python 后台任务完成后，`GET /bid-exports/:exportId` 返回 status `done` 和下载 URL。
7. 下载文件名为 `智慧交通平台分离标书-技术标.docx`，文件大小 36891 bytes，文件头为 docx ZIP 包格式。
8. tenant2 访问 tenant1 导出详情返回 404。
9. RLS 验证中，tenant1 对该 export 可见数量为 1，关联 file_asset 状态为 ready 且 size_bytes 为 36891；tenant2 对同一 export 可见数量为 0。

### 偏离蓝图

1. 本轮只实现 docx 最小导出；PDF 转换、ZIP 打包、封面、页眉页脚、目录和模板排版尚未实现。
2. chapters 仍是最小样例内容，尚未接入逐章生成、版本对比、采纳/重新生成和 source_refs 落库。
3. `bid_exports` 已落地，但 `bid_chapter_versions`、`bid_generation_jobs`、`bid_generation_steps`、`bid_templates` 仍待真实实现。

### 下一轮建议

继续推进标书主链路：实现章节编辑保存、章节版本、逐章生成任务和 source_refs 落库；或补 ZIP 打包使分离标书技术标/商务标导出链路更完整。

## Loop-4 ZIP 打包闭环 - 2026-06-11

### 本轮目标

1. 补齐 `x.md` 中分离标书必须支持投标文件全套 ZIP 打包的要求。
2. ZIP 打包沿用 Go 编排、Python 导出、MinIO 私有存储、HMAC 回调和 Go 落库链路。
3. 前端第 7 步导出页启用“打包全套 ZIP”操作。
4. 验证 ZIP 至少包含技术标和商务标两个 docx，并保持租户隔离。

### 代码交付

1. Go `platform/bid` 的 `CreateExport` 支持 `export_type=zip`，自动汇总 `tech` 和 `business` parts 及其 chapters。
2. ZIP 导出创建 `bid_exports.export_type='zip'`、待确认 `file_assets.content_type='application/zip'`，并通过 `ai_tasks` 调用 Python `/tasks/export/zip`。
3. Python AI 服务新增 `/tasks/export/zip`，复用 docx 导出器为每个 part 生成 docx，再写入 ZIP 包并上传 MinIO。
4. HMAC 回调结果包含 export_type、part_count、chapter_count、size_bytes 和 content_type，Go 回调后将 file_asset 标记为 ready。
5. 前端标书向导第 7 步启用 ZIP 打包按钮，导出历史中显示“全套 ZIP”，完成后使用同一下载入口。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd ai-service && python3 -m compileall app
cd frontend && pnpm build
git diff --check
./infra/scripts/check.sh
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
```

结果：

1. backend 测试通过，AI compileall 通过，frontend build 通过，`./infra/scripts/check.sh` 通过。
2. backend 启动确认 goose current version 7，AI `/healthz` 与 `/models/health` 返回 ok。
3. 端到端 ZIP 导出使用 bid `50000000-0000-4000-8000-000000000001`。
4. `POST /bids/:id/exports` 创建 ZIP 导出 `fabe0dfc-8ca5-4743-8b10-d66dbe5681a4`，本地任务 `abb29f62-b6ab-4fcc-9143-81f6c4d8a49d`，外部任务 `task-export-fabe0dfc8ca5`。
5. Python 后台任务完成后，`GET /bid-exports/:exportId` 返回 status `done` 和下载 URL。
6. 下载文件名为 `智慧交通平台分离标书-投标文件全套.zip`，大小 69066 bytes，ZIP 内包含 `01-技术标.docx` 和 `02-商务标.docx`，两个条目均为 docx ZIP 包格式。
7. tenant2 访问 tenant1 ZIP 导出详情返回 404。
8. RLS 验证中，tenant1 对该 ZIP export 可见数量为 1，关联 file_asset 状态为 ready、content_type 为 `application/zip`、size_bytes 为 69066；tenant2 对同一 export 可见数量为 0。

### 偏离蓝图

1. ZIP 目前只打包生成出的 docx 文件，尚未包含附件、工程量清单、电子标特殊格式或导出清单 manifest。
2. PDF 转换、封面、页眉页脚、目录和模板排版仍待增强。
3. 章节仍是最小样例内容，尚未接入逐章生成、版本对比、采纳/重新生成和 source_refs 落库。

### 下一轮建议

继续推进标书主链路：章节编辑保存、章节版本、逐章生成任务、生成结果 source_refs 落库，以及编辑器按技术标/商务标分别进入和保存。

## Loop-5 章节编辑与版本闭环 - 2026-06-11

### 本轮目标

1. 将章节保存、采纳、版本查询和重新生成从 stub 推进为真实 API。
2. 保存和采纳章节时写入 `bid_chapter_versions`。
3. 重新生成章节时通过 Python `/tasks/chapter-generate` 走 ModelRouter，回写 Tiptap JSON、source_refs 和 needs_human_input。
4. 将生成返回的 source_refs 写入 `knowledge_references`，无法解析到真实知识库文档的引用保留 metadata 并标记 unresolved。
5. 前端标书编辑器读取真实 chapters，支持保存、采纳、重新生成和版本列表。

### 代码交付

1. 新增 `00008_bid_chapter_versions.sql`，创建 `bid_chapter_versions` 并启用 FORCE RLS；同时允许 `knowledge_references.source_document_id` 为空，用于记录未解析 source_ref。
2. `platform/bid` 新增章节保存、采纳、重新生成、版本列表和 diff 方法。
3. 后端将 `PUT /chapters/:chapterId/content`、`PATCH /chapters/:chapterId`、`POST /chapters/:chapterId/accept`、`POST /chapters/:chapterId/regenerate`、`GET /chapters/:chapterId/versions`、`GET /chapters/:chapterId/diff` 从 stub 切到真实 handler。
4. 章节重新生成调用 Python `/tasks/chapter-generate`，并将返回的 `trace_id`、`source_refs`、`needs_human_input`、`model_metadata`、`token_usage` 写入章节和版本。
5. 前端编辑器页读取真实 bid、parts、chapters；当前章节内容载入 Tiptap；保存、采纳、重新生成后刷新章节和版本列表。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
docker compose build backend frontend
docker compose up -d backend frontend
curl http://127.0.0.1:8000/healthz
curl http://127.0.0.1:8000/models/health
```

结果：

1. backend 测试通过，frontend build 通过。
2. backend 自动执行到 goose version 8。
3. AI `/healthz` 与 `/models/health` 返回 ok。
4. 端到端章节验证使用 bid `50000000-0000-4000-8000-000000000001`、chapter `52000000-0000-4000-8000-000000000001`。
5. `PUT /chapters/:chapterId/content` 生成 manual_edit 版本 v1。
6. `POST /chapters/:chapterId/accept` 生成 accepted 版本 v2。
7. `POST /chapters/:chapterId/regenerate` 返回 202，章节状态更新为 generated，生成 ai_regenerate 版本 v3，trace_id 为 `trace-mock-chapter`。
8. 重新生成后的章节包含 1 条 source_ref，并返回 needs_human_input：`企业资质证书编号`、`项目经理证书有效期`。
9. `GET /chapters/:chapterId/versions` 返回 3 条版本，`GET /chapters/:chapterId/diff` 返回 current 和 previous。
10. tenant2 调用同一章节 regenerate 返回 404。
11. RLS 验证中 tenant1 对该章节版本可见数量为 3、knowledge_references 为 1；tenant2 两者均为 0。

### 偏离蓝图

1. 重新生成目前是同步调用 Python `/tasks/chapter-generate` 后立即落库，尚未改成异步 ai_tasks + SSE 进度。
2. source_ref 来自 MockProvider，当前落库为 unresolved；后续需要接真实 RAG 检索，使 source_document_id/chunk_id 可解析到 knowledge_documents/knowledge_chunks。
3. diff 目前返回 current 和 previous 快照，尚未做结构化逐段差异。

### 下一轮建议

继续补章节生成任务异步化、真实 RAG 检索输入、source_ref 解析为真实知识库 chunk，以及 bid_chapter_versions 的可视化差异。

## Loop-5 章节生成异步化闭环 - 2026-06-11

### 本轮目标

1. 修正章节重新生成同步调用 Python 的偏离，使逐章生成符合 202 + task_id 的异步约束。
2. 复用 `ai_tasks` 和 HMAC 回调，将章节生成结果在回调阶段写入章节、版本和引用表。
3. 前端编辑器在发起重新生成后轮询任务状态，完成后刷新章节内容和版本列表。

### 代码交付

1. Python `/tasks/chapter-generate` 改为返回 `TaskAccepted`，后台执行 ModelRouter，并通过 `callback_url` HMAC 回调 Go。
2. Go `RegenerateChapter` 预生成外部 task_id，先写入 `ai_tasks(resource_type='bid_chapter')` 并将章节置为 `generating`，再调用 Python，避免快速回调和 task 绑定之间的竞态。
3. Go AI 回调入口支持 `bid_chapter`，`bidStore.ApplyCallback` 按资源类型分派；章节生成完成后写入 `bid_chapters`、`bid_chapter_versions` 和 `knowledge_references`。
4. 前端 `regenerateChapter` 响应改为 `{ chapter, task }`，编辑器页使用 `GET /ai-tasks/:taskId` 轮询任务状态，并在 done/failed/cancelled 后刷新页面数据。
5. 文档同步更新 API、AI_PIPELINE 和 RAG 当前状态。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
python3 -m compileall ai-service/app
python3 -m pytest ai-service/app/tests
./infra/scripts/check.sh
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
```

结果：

1. backend 测试通过。
2. frontend build 通过。
3. AI compileall 通过。
4. 当前系统 Python 缺少 pytest 模块，`python3 -m pytest ai-service/app/tests` 未运行成功；后续以 `infra/scripts/check.sh` 或容器内依赖继续验证。
5. `./infra/scripts/check.sh` 通过。
6. Docker 重新构建并启动 backend、frontend、ai-service 成功；backend goose current version 8，AI `/healthz` 与 `/models/health` 返回 ok。
7. 运行时异步章节验证使用 bid `50000000-0000-4000-8000-000000000001`、chapter `52000000-0000-4000-8000-000000000001`。
8. `POST /chapters/:chapterId/regenerate` 返回 202，章节立即进入 `generating`，返回本地 task `1d6d311b-abc7-4917-95bd-a9e5d5df37fd` 和外部 task `task-chapter-079d954e-6c59-4aef-88ad-888cce73d1d4`。
9. `GET /ai-tasks/:taskId` 轮询到 `done`，result 中 `trace_id=trace-mock-chapter`。
10. 回调后章节状态为 `generated`，source_refs 数量为 1，needs_human_input 为 `企业资质证书编号`、`项目经理证书有效期`。
11. `GET /chapters/:chapterId/versions` 从 3 条增长为 4 条，最新版本 `change_reason=ai_regenerate`。
12. tenant2 访问同一 task 返回 404，tenant2 重新生成 tenant1 章节返回 404。
13. RLS 验证中 tenant1 可见 bid_chapter task 为 1 且 done 为 1、章节版本为 4、knowledge_references 为 1；tenant2 对同一章节任务、版本、引用均为 0。

### 偏离蓝图

1. 本轮已修正“章节重新生成同步调用 Python”的偏离。
2. 前端目前是轮询兜底，尚未实现真正 SSE 流式进度。
3. source_ref 仍主要来自 MockProvider，尚未接真实 RAG source_ref 解析到 `knowledge_chunks`。

### 下一轮建议

继续完成章节生成 SSE 进度、真实 RAG 检索输入和 source_ref 解析；随后推进 Loop-6 知识库三子页面、文档预览、embedding MockProvider 与 pgvector 检索。

## Loop-5 章节生成 SSE 进度 - 2026-06-11

### 本轮目标

1. 将 `GET /bids/:id/generation/stream` 从 stub 切到真实 SSE。
2. SSE 事件输出标书下章节状态、章节生成 task 状态和汇总进度。
3. 前端编辑器用带鉴权头的 fetch stream 订阅 SSE，并保留轮询兜底。

### 代码交付

1. `platform/bid` 新增 `GenerationSnapshot`，按 bid 聚合 chapters 和 `ai_tasks(resource_type='bid_chapter')`，输出 total/generating/generated/accepted/needs_fix 章节数和 queued/running/done/failed task 数。
2. 后端新增 `streamBidGeneration`，鉴权和资源存在检查后返回 `text/event-stream`；首次发送 `generation` 事件，后续仅在章节/task 快照变化时推送，空闲时发送 heartbeat。
3. 前端 `shared/sse/client.ts` 从原生 EventSource 改为 fetch stream，自动携带 JWT `Authorization` 和 `X-Tenant-ID`。
4. 标书编辑器接入 `/bids/:id/generation/stream`，在右侧 AI 助手显示 SSE 状态、章节进度和当前章节最近 task 状态；收到 generation 事件后刷新 chapters 和 versions。
5. API 和技术架构文档同步记录认证 SSE 设计。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
./infra/scripts/check.sh
docker compose build backend frontend
docker compose up -d backend frontend ai-service
```

结果：

1. backend 测试通过。
2. frontend build 通过。
3. `./infra/scripts/check.sh` 通过。
4. Docker 重新构建并启动 backend/frontend 成功；backend `/healthz`、AI `/healthz` 返回 ok。
5. 运行时 SSE 验证使用 bid `50000000-0000-4000-8000-000000000001`、chapter `52000000-0000-4000-8000-000000000001`。
6. 打开 `/bids/:id/generation/stream` 后收到初始 `generation` 事件：total_chapters=4、done_tasks=1。
7. 订阅期间触发 `POST /chapters/:chapterId/regenerate`，随后 stream 收到包含新 task `f4a012b2-18ff-413b-adee-323488abb374` / `task-chapter-3241b47d-e304-4a09-9cea-5589fcf13037` 的 `generation` 事件，task_status=done、done_tasks=2。
8. SSE 鉴权验证：无 token 返回 401，tenant1 访问 stream 返回 200，tenant2 访问 tenant1 bid stream 返回 404。

### 偏离蓝图

1. 本轮补齐了 Loop-5 的 SSE 进度通道。
2. SSE 当前推送快照级事件，尚未做 token-by-token 章节文本流式输出。
3. source_ref 仍主要来自 MockProvider，真实 RAG 检索输入和 source_ref 解析仍待继续。

### 下一轮建议

继续做真实 RAG 检索输入、source_ref 解析到真实 `knowledge_chunks`，或进入 Loop-6 的知识库三子页面、文档预览和 pgvector 语义搜索。

## Loop-5 / RAG 引用解析闭环 - 2026-06-11

### 本轮目标

1. 章节重新生成前从当前租户知识库 chunk 检索可引用素材。
2. 将真实 chunk/document UUID 传给 Python `/tasks/chapter-generate`。
3. 生成回调后把 source_refs 解析为真实 `knowledge_references.source_document_id` 与 `chunk_id`。
4. 修复 `/knowledge/search` 对中文空格关键词组合不命中的问题。

### 代码交付

1. `platform/bid` 新增 `retrieved_knowledge_refs` payload，创建章节生成 task 前按章节标题和正文检索 `knowledge_chunks`，最多传入 5 条真实引用上下文。
2. `MockProvider.generate_chapter` 优先使用 `retrieved_knowledge_refs` 返回 source_refs；无检索结果时才回退 demo source_ref。
3. `replaceKnowledgeReferences` 改为验证 chunk 与 document 是否同租户存在，存在时写入真实 `source_document_id`、`chunk_id`，metadata 标记 `resolved=true`、`resolved_by=knowledge_chunk`。
4. `/knowledge/search` 和章节检索 SQL 支持整句匹配或空格拆分关键词匹配，改善中文查询 `智慧交通 项目理解` 的召回。
5. AI 测试补充 MockProvider 使用 retrieved refs 的断言；API、RAG、AI_PIPELINE、DATABASE_SCHEMA 文档同步更新。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
python3 -m compileall ai-service/app
./infra/scripts/check.sh
docker compose build backend ai-service
docker compose up -d backend ai-service frontend
docker compose build backend
docker compose up -d backend frontend ai-service
```

结果：

1. backend 测试通过。
2. AI compileall 通过。
3. `./infra/scripts/check.sh` 通过。
4. Docker 重新构建并启动 backend、ai-service 成功；backend `/healthz`、AI `/healthz` 与 `/models/health` 返回 ok。
5. AI 容器未安装 pytest，`python -m pytest app/tests` 无法运行，错误为 `No module named pytest`；本轮新增测试已通过 compileall。
6. 运行时创建并处理 text/plain 知识库文档，写入真实 chunk。随后 `/knowledge/search` 查询 `智慧交通 项目理解` 返回 2 条 source_refs。
7. 章节重新生成 task `cc538d61-788e-41fd-a23b-a418bf14771c` 的 payload 包含 3 条 `retrieved_knowledge_refs`，Python 回调结果返回 3 条 source_refs，版本数从 6 增长到 7。
8. RLS 验证中 tenant1 对章节 `52000000-0000-4000-8000-000000000001` 的 `knowledge_references` 为 3 条，`source_document_id` 和 `chunk_id` 均为 3，`all_resolved=true`；tenant2 对同一章节引用数为 0。

### 偏离蓝图

1. 本轮补齐了真实 chunk 引用输入与解析落库，但还不是 pgvector 向量召回、RRF 融合或 rerank。
2. MockProvider 仍负责生成文本；真实 LLM Provider 和章节自检仍待 Loop-7 继续。

### 下一轮建议

进入 Loop-6：完善知识库三子页面、文档预览、标签/分类管理交互、embedding MockProvider 入库和 pgvector 语义搜索；或继续推进章节 diff 的结构化可视化。

## Loop-6 / Mock embedding 与 pgvector 语义搜索 - 2026-06-11

### 本轮目标

1. 让知识库文档处理结果携带 embedding，并写入 `knowledge_chunks.embedding`。
2. 为 `knowledge_chunks.embedding` 建立 pgvector 索引。
3. 将 `/knowledge/search` 从关键词检索推进为租户内语义向量召回 + 关键词融合排序。
4. 保持所有 embedding 调用经过 Python AI 服务 ModelRouter。

### 代码交付

1. Python AI 服务新增 `/embeddings/knowledge`，通过 `knowledge_embedding` 路由返回 MockProvider 1024 维 embedding。
2. MockProvider 的 embedding 从长度常量改为确定性 token-hash 向量，并做 L2 归一化，支持基本中文字符/词片相似度冒烟。
3. `/tasks/knowledge-process` 后台任务在切片后生成 embedding，回调 Go 时每个 chunk 携带 `embedding` 和 embedding 元数据。
4. Go `platform/knowledge` 在回调写入 chunk 时校验 1024 维向量并写入 pgvector；无 embedding 的旧回调仍可写入 null。
5. 新增迁移 `00009_knowledge_chunk_embedding_index.sql`，为 `knowledge_chunks.embedding` 创建 HNSW cosine 索引。
6. `/knowledge/search` 会调用 AI 服务生成 query embedding，将 pgvector cosine 分数与全文/关键词分数融合排序；AI embedding 调用失败时保留关键词兜底。
7. 新增 Go 单测覆盖向量字面量校验，新增 Python 测试覆盖 Mock embedding 的确定性、维度和基本相似度排序。
8. AI_PIPELINE、RAG_DESIGN、API_SPEC、DATABASE_SCHEMA、MODEL_GATEWAY 文档同步更新。

### 检查结果

已运行：

```bash
python3 -m compileall ai-service/app
cd backend && GOTOOLCHAIN=local go test ./...
python3 -m pytest app/tests
docker compose build backend ai-service
docker compose up -d backend ai-service frontend
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8000/healthz
curl http://127.0.0.1:8000/models/health
curl -X POST http://127.0.0.1:8000/embeddings/knowledge ...
./infra/scripts/check.sh
git diff --check
```

结果：

1. AI compileall 通过。
2. backend Go 测试通过，新增 `platform/knowledge` 单测通过。
3. 本机和 AI 容器均未安装 pytest，`python -m pytest app/tests` 返回 `No module named pytest`；对应 Python 测试已通过 compileall，待后续镜像安装 dev extra 后执行。
4. 首次 `docker compose build backend ai-service` 因 DockerHub `golang:1.25-alpine` TLS handshake timeout 失败；重试后 backend 与 ai-service 镜像构建成功。
5. backend、ai-service、frontend 启动成功；backend `/healthz`、AI `/healthz`、AI `/models/health` 均返回 ok。
6. goose 迁移版本为 9，`idx_knowledge_chunks_embedding_hnsw` 存在。
7. `/embeddings/knowledge` 返回 provider=mock、model=configurable-embedding-model、dimensions=1024，向量 L2 norm=1.0。
8. 运行时上传并处理 `zbt-pgvector-semantic.txt`，生成文档 `8057186a-cd9d-4d7d-ae3d-30382e34f592`，该文档 1 个 chunk、1 个 embedding，`vector_dims(embedding)=1024`。
9. `/knowledge/search` 查询 `星瀚治理` 返回 1 条结果，首条为该上传文档，score=0.176506，source_refs=1；查询词并非原文完整子串，用于验证 pgvector 语义召回参与排序。
10. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询该文档 chunk，返回 0，确认向量入库后仍受租户隔离约束。
11. `./infra/scripts/check.sh` 通过；前端构建仍有既有大 chunk warning，无失败。
12. `git diff --check` 通过。

### 偏离蓝图

1. 本轮落地的是 Mock embedding + pgvector 最小语义召回，不是生产级 BGE/OpenAI embedding。
2. 检索排序为 pgvector cosine 分数与关键词分数直接加权，尚未实现正式 RRF 融合和 RerankProvider 精排。
3. 历史已处理 chunk 没有自动补 embedding；后续可加批量 reindex 任务。

### 下一轮建议

继续 Loop-6：完善知识库三子页面、文档预览、标签/分类管理交互；或进入检索增强的下一步，实现 RRF + RerankProvider 与历史 chunk embedding reindex。

## Loop-6 / 文档引用追踪闭环 - 2026-06-11

### 本轮目标

1. 将 `GET /knowledge/documents/:id/references` 从空实现切换为真实反向引用查询。
2. 在知识库文档库页面提供可操作的引用追踪入口。
3. 验证引用追踪仍受租户 RLS 隔离。

### 代码交付

1. `platform/knowledge` 新增 `DocumentReference` DTO 和 `DocumentReferences` 查询，先验证文档属于当前租户，再从 `knowledge_references` 反查 bid、chapter、chunk 和 metadata。
2. API handler `knowledgeDocumentReferences` 返回真实 `{items}`。
3. 前端 API client 新增 `KnowledgeDocumentReferenceDTO` 与 `fetchKnowledgeDocumentReferences`。
4. 文档库页面操作列新增“引用”按钮，打开抽屉展示引用标书、章节、chunk、解析状态和引用时间。
5. API_SPEC、DATABASE_SCHEMA 和 DEV_LOOP_LOG 同步更新。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/knowledge/store.go internal/api/routes.go
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
docker compose build backend frontend
docker compose up -d backend frontend ai-service
curl http://127.0.0.1:8080/healthz
curl /api/v1/knowledge/documents/363729dc-65fb-4778-9a81-31f8489fe7ab/references
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. Docker 重新构建并启动 backend/frontend 成功；backend `/healthz` 返回 ok。
4. 新引用 API 对文档 `363729dc-65fb-4778-9a81-31f8489fe7ab` 返回 1 条引用。
5. 返回内容包含 `bid_title=智慧交通平台分离标书`、`chapter_title=一、项目理解`、`chunk_id_present=true`、`resolved=true`。
6. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询同一 `source_document_id` 的 `knowledge_references` 返回 0。

### 偏离蓝图

1. 引用抽屉目前展示引用记录，不做跳转到章节定位；后续可接 `/bids/:bidId/editor?chapter=...`。
2. 文档引用只覆盖已解析到真实 `source_document_id` 的记录，unresolved 引用仍只能在章节版本 source_refs 中查看。

### 下一轮建议

继续完善知识库三子页面：标签/分类管理表单、文档元数据编辑、模板库真实 API，或推进章节引用到编辑器定位。

## Loop-6 / 文档模板库真实 API - 2026-06-11

### 本轮目标

1. 将 `/knowledge/templates` 从 stub 和静态前端表格推进为真实租户内数据。
2. 落地 `document_templates` 表、RLS 和种子模板。
3. 让文档模板页支持真实列表和新建模板。

### 代码交付

1. 新增迁移 `00010_document_templates.sql`，创建 `document_templates`，包含 name、category、description、version、content、usage_count、status，并启用 FORCE RLS。
2. 为每个租户种子 3 个文档模板：项目实施方案、售后服务承诺、数据安全响应。
3. `platform/knowledge` 新增 `DocumentTemplate`、`CreateDocumentTemplateRequest`、列表和创建方法。
4. 后端注册真实 `GET /knowledge/templates` 和 `POST /knowledge/templates`，替换原 stub。
5. 前端 API client 新增模板 DTO、列表和创建函数。
6. `/knowledge/templates` 页面改为真实 API 表格，并提供“新建模板”弹窗，支持名称、分类、版本、说明和章节结构输入。
7. API_SPEC、DATABASE_SCHEMA 和 DEV_LOOP_LOG 同步更新。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/knowledge/store.go internal/api/routes.go
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
curl http://127.0.0.1:8080/healthz
curl /api/v1/knowledge/templates
curl -X POST /api/v1/knowledge/templates ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker 重新构建并启动 backend/frontend 成功；backend `/healthz` 返回 ok。
5. goose 迁移版本为 10。
6. `GET /knowledge/templates` 初始返回 3 条种子模板。
7. `POST /knowledge/templates` 创建 `运行时模板验证-1781186252.docx` 成功，返回 id `74259391-ebda-43ac-9acb-2d8929042cab`、category `验证模板`、sections 数量 2。
8. 再次 `GET /knowledge/templates` 返回 4 条，包含新建模板。
9. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询 tenant1 新建模板，返回 0。

### 偏离蓝图

1. 当前模板正文以 JSON section 结构保存，尚未关联真实 docx 模板文件和预览。
2. 还未实现模板编辑、归档、下载和“使用模板创建文档/标书”的业务动作。

### 下一轮建议

继续补知识库管理交互：标签/分类创建编辑删除表单、文档元数据编辑，或将文档模板关联 file_assets 实现模板文件上传/预览。

## Loop-6 / 分类标签管理交互闭环 - 2026-06-11

### 本轮目标

1. 将已有分类/标签 CRUD 后端接口接入前端真实操作。
2. 在文档库侧栏提供分类创建、编辑、删除。
3. 在标签管理页提供标签创建、编辑、删除和颜色选择。
4. 验证分类/标签 CRUD 仍受租户 RLS 隔离。

### 代码交付

1. 前端 API client 新增 `createKnowledgeCategory`、`updateKnowledgeCategory`、`deleteKnowledgeCategory`。
2. 前端 API client 新增 `createKnowledgeTag`、`updateKnowledgeTag`、`deleteKnowledgeTag`。
3. 文档库页面分类侧栏从只读 Tree 改为分类列表，支持新建、编辑和删除；删除分类后后端会将关联文档置为未分类。
4. 标签管理页新增标签表单弹窗，支持新建/编辑标签名称和颜色；列表操作支持删除确认。
5. 分类/标签变更后刷新分类、标签、文档和知识库统计查询。
6. API_SPEC 同步记录分类和标签 CRUD 已接入前端管理交互。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build frontend
docker compose up -d frontend backend ai-service
curl -X POST /api/v1/knowledge/categories ...
curl -X PATCH /api/v1/knowledge/categories/:id ...
curl -X DELETE /api/v1/knowledge/categories/:id
curl -X POST /api/v1/knowledge/tags ...
curl -X PATCH /api/v1/knowledge/tags/:id ...
curl -X DELETE /api/v1/knowledge/tags/:id
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker 重新构建并启动 frontend 成功。
5. 运行时创建分类 `运行分类-1781187016`，PATCH 后变为 `运行分类-1781187016-改`，description 为 `更新验证`。
6. 运行时创建标签 `运行标签-1781187016`，PATCH 后变为 `运行标签-1781187016-改`，color 为 `red`。
7. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询上述分类和标签，均返回 0。
8. DELETE 分类和标签均返回 204；再次列表确认 `category_removed=true`、`tag_removed=true`。

### 偏离蓝图

1. 分类当前只支持一级分类，尚未做父子层级编辑。
2. 标签与文档的关联仍通过文档编辑接口预留，前端尚未提供文档标签批量调整。

### 下一轮建议

继续做文档元数据编辑：标题、分类、标签、摘要的前端表单；或推进模板文件上传/预览和模板使用动作。

## Loop-6 / 文档元数据编辑闭环 - 2026-06-11

### 本轮目标

1. 将已有 `PATCH /knowledge/documents/:id` 接入前端文档库页面。
2. 支持编辑文档标题、类型、分类、标签和摘要。
3. 验证文档分类/标签关联写入与恢复路径。

### 代码交付

1. 前端 API client 新增 `updateKnowledgeDocument`。
2. 文档库页面新增文档编辑 Modal。
3. 编辑表单支持 title、doc_type、category_id、tag_ids、summary。
4. 文档更新成功后刷新文档列表、分类统计和知识库统计。
5. API_SPEC 同步记录文档库页已支持元数据编辑。

### 检查结果

已运行：

```bash
cd frontend && pnpm build
git diff --check
docker compose build frontend
docker compose up -d frontend backend ai-service
curl -X PATCH /api/v1/knowledge/documents/:id ...
```

结果：

1. frontend build 通过；仍有既有大 chunk warning，无失败。
2. `git diff --check` 通过。
3. Docker 重新构建并启动 frontend 成功。
4. 运行时选取文档 `zbt-pgvector-semantic`，临时创建分类和标签后 PATCH 文档成功。
5. PATCH 返回 title 以 `元数据验证-1781187701` 结尾，category_id 为临时分类，tag_ids 包含临时标签，summary 为 `元数据编辑运行时验证 1781187701`。
6. 验证后已将文档恢复到原 title、doc_type、category、tags 和 summary，并删除临时分类/标签。
7. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询临时分类和标签，均返回 0。

### 偏离蓝图

1. 暂未做文档批量编辑。
2. 文档编辑不直接触发 reprocess/reindex，后续可在分类或标签变化后补 reindex 任务。

### 下一轮建议

推进模板文件上传/预览和模板使用动作，或继续补知识库文档详情页中的 chunk/embedding/references 可视化。

## Loop-6 / 标书模板库真实 API - 2026-06-11

### 本轮目标

1. 将 `/bids/templates` 从静态卡片改为真实模板库数据。
2. 落地 `GET /bid-templates` 和 `POST /bid-templates/:templateId/use`。
3. 新增 `bid_templates` 表、租户 RLS 和默认模板种子。

### 代码交付

1. 新增迁移 `00011_bid_templates.sql`，创建 `bid_templates`，包含 name、bid_type、category、description、version、content、usage_count、status，并启用 FORCE RLS。
2. 后端 bid store 新增 `ListTemplates` 和 `UseTemplate`，使用模板时同一事务创建 draft 标书、默认分册章节并递增模板使用次数。
3. API 路由新增真实 `GET /bid-templates` 和 `POST /bid-templates/:templateId/use` handler，替换原 stub。
4. 前端 API client 新增标书模板 DTO、模板列表和使用模板调用。
5. 标书模板页展示真实分类、版本、描述、章节标签和使用次数，点击“使用模板”后进入新建标书向导。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
curl -X GET /api/v1/bid-templates ...
curl -X POST /api/v1/bid-templates/:templateId/use ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker 重新构建并启动 backend/frontend 成功。
5. 运行时 `GET /bid-templates` 返回 4 个租户模板。
6. 运行时使用模板 `综合项目投标模板` 创建标书 `模板使用验证-1781188837`，返回 bid_id `6f7c80f5-b9e4-44c8-9631-479c7e4caab5`，bid_type 为 `combined`。
7. 模板 `usage_count` 从 158 增加到 159，`GET /bids` 可查到新建标书。
8. goose 迁移版本为 11。
9. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询验证标书返回 0，tenant1 返回 1。

### 偏离蓝图

1. 当前模板使用动作先创建默认分册和章节，尚未按模板 content 动态生成完整大纲。
2. 模板预览、上传 docx 模板文件和模板编辑仍待后续实现。

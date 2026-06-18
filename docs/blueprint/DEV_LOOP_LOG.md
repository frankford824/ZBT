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

## Loop-7 / 标讯大厅真实 API - 2026-06-11

### 本轮目标

1. 将 `/tenders` 和 `/tenders/:tenderId` 从静态页面/stub 推进到真实租户数据。
2. 落地标讯列表、详情、收藏、数据源配置和 URL 可达性检测。
3. 支持从标讯创建项目和生成标书。

### 代码交付

1. 新增迁移 `00012_tender_foundation.sql`，创建 `tender_sources`、`tenders`、`tender_user_states`、`tender_parse_results`，启用 FORCE RLS，并写入默认数据源和标讯种子。
2. 新增 `platform/tender` store，包含列表筛选、详情、创建/更新标讯、收藏、数据源 CRUD、URL 检测和从标讯创建项目。
3. API 路由新增真实 Tender handlers，替换 `GET /tenders`、`POST /tenders`、`GET /tenders/:id`、`PATCH /tenders/:id`、收藏、创建项目/标书、数据源和检测相关 stub。
4. 前端 API client 新增 Tender DTO 和接口方法。
5. 标讯大厅页改为真实列表，支持全部、AI 推荐、监控、收藏和监控设置；详情页展示真实要求/风险，并可创建项目或生成标书。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
curl -X GET /api/v1/tenders ...
curl -X POST /api/v1/tenders/:id/favorite ...
curl -X POST /api/v1/tenders/:id/create-project ...
curl -X POST /api/v1/tenders/:id/create-bid ...
curl -X POST /api/v1/tender-sources/:id/verify ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker 重新构建并启动 backend/frontend 成功。
5. 运行时 `GET /tenders` 返回 3 条租户标讯，`GET /tenders?recommended=true` 返回 3 条推荐标讯。
6. 运行时收藏首条标讯后返回 `favorite=true`，`GET /tenders?favorite=true` 可查到该标讯。
7. 运行时详情返回 `某市智慧交通综合治理平台建设项目`。
8. 运行时从标讯创建项目 `dc3689e8-594f-446e-afe3-e4e9c1c80f21`，从标讯生成标书 `7e36ccea-c123-41a0-b029-ad097682e206`。
9. 运行时创建临时数据源并检测 `http://127.0.0.1:8080/healthz`，检测结果为 `ok / 200 OK`，验证后删除临时数据源返回 204。
10. goose 迁移版本为 12。
11. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询新建项目和标书均返回 0，tenant1 均返回 1。

### 偏离蓝图

1. 标讯自动抓取、Cookie 登录和自定义 CSS 选择器抓取仍按蓝图放到二期。
2. `tender_parse_results` 仅落表预留，尚未接入招标文件解析任务。
3. 从标讯生成标书当前创建 combined draft 标书，尚未把标讯解析结果写入标书大纲。

## Loop-8 / 项目管理真实 API - 2026-06-11

### 本轮目标

1. 将 `/projects` 和 `/projects/:projectId` 从静态页面/stub 推进到真实租户数据。
2. 落地项目状态流转、里程碑、成员、活动日志和中标后创建成本项目。
3. 让标讯创建出来的项目能在项目管理页继续推进。

### 代码交付

1. 新增迁移 `00013_project_foundation.sql`，创建 `project_milestones`、`project_members`、`project_logs`、最小 `cost_projects`，启用 FORCE RLS，并为既有示例项目补 owner、里程碑和日志。
2. 新增 `platform/project` store，包含项目列表/详情/创建/更新/删除、状态流转、里程碑 CRUD、成员添加/删除、活动列表和成本项目创建。
3. API 路由新增真实 Project handlers，替换项目列表、详情、状态流转、里程碑、成员、活动和 create-cost-project 相关 stub。
4. 前端 API client 新增 Project、Milestone、Activity、CostProject DTO 和接口方法。
5. 项目管理页改为真实看板/列表；项目详情页接入真实状态推进、标记中标、里程碑创建、活动时间线和成本项目创建。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
curl -X GET /api/v1/projects ...
curl -X POST /api/v1/projects ...
curl -X POST /api/v1/projects/:id/transition ...
curl -X POST /api/v1/projects/:id/milestones ...
curl -X POST /api/v1/projects/:id/members ...
curl -X POST /api/v1/projects/:id/create-cost-project ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker 重新构建并启动 backend/frontend 成功。
5. 运行时 `GET /projects` 初始返回 2 个项目。
6. 运行时创建项目 `项目管理验证-1781193173`，project_id 为 `d88a694e-0563-46ee-80d7-9ebffb26c5b3`，状态为 `opportunity`。
7. 运行时新增里程碑 `39bc021e-1d79-4367-8429-e16705bb1aa6`，随后列表返回 1 条。
8. 运行时添加项目成员 `df18c636-866c-4cfa-9c1d-b63c48b77f23`。
9. 运行时状态流转到 `bidding`，再标记为 `closed:won`。
10. 运行时创建成本项目 `ec5e3981-fa40-4223-98d2-a59f97c8200e`，活动列表返回 6 条。
11. goose 迁移版本为 13。
12. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询新建项目、里程碑、成员和成本项目均返回 0，tenant1 分别返回 1、1、2、1。

### 偏离蓝图

1. 成本项目目前仅落地最小 `cost_projects` 表和创建动作，成本项、分析、AI 建议和报告仍待成本模块继续实现。
2. 项目成员当前通过 API 支持添加/删除，前端详情页本轮只展示负责人，未做成员管理面板。
3. 项目删除在存在关联标书时仍会受外键约束保护，后续可补归档/软删除策略。

## Loop-9 / 成本管理真实 API - 2026-06-11

### 本轮目标

1. 将 `/costs` 和 `/costs/:costProjectId` 从静态页面/stub 推进到真实租户数据。
2. 落地成本项目、成本项、成本分析、AI 建议占位和报告生成。
3. 补齐项目中标后创建成本项目之后的成本管理闭环。

### 代码交付

1. 新增迁移 `00014_cost_foundation.sql`，创建 `cost_items` 和 `cost_reports`，启用 FORCE RLS，并为既有示例成本项目写入默认成本项。
2. 新增 `platform/cost` store，包含成本项目列表/创建/详情/更新、成本项 CRUD、成本分析、建议和报告生成。
3. API 路由新增真实 Cost handlers，替换 `GET /cost-projects`、`POST /cost-projects`、`GET /cost-projects/:id`、`PATCH /cost-projects/:id`、成本项、analysis、ai-advice 和 report 相关 stub。
4. 前端 API client 新增 CostItem、CostAnalysis、CostReport DTO 和接口方法。
5. 成本管理页改为真实列表；成本详情页接入预算/实际/利润率、成本构成图、成本明细、新增成本项、AI 优化建议和报告生成。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
curl -X GET /api/v1/cost-projects ...
curl -X POST /api/v1/cost-projects ...
curl -X POST /api/v1/cost-projects/:id/items ...
curl -X PATCH /api/v1/cost-items/:id ...
curl -X GET /api/v1/cost-projects/:id/analysis ...
curl -X POST /api/v1/cost-projects/:id/ai-advice ...
curl -X POST /api/v1/cost-projects/:id/report ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker 后端重建并启动成功；首次完整 build 因 Docker Hub token/metadata TLS handshake timeout 失败，重试后 frontend build 和启动成功。
5. 运行时 `GET /cost-projects` 初始返回 2 个成本项目。
6. 运行时创建项目 `c80007ef-014f-4d53-a17c-9c51be7dca51`，并创建成本项目 `e5c7f0b3-13b9-4d25-96b2-95d9ff6d110c`。
7. 运行时新增成本项 `392a0b6b-fc49-4cfe-93ae-9a90eac66247`，PATCH 后 actual_amount 为 310000，列表返回 1 条。
8. 运行时 analysis 返回 1 个超预算项，首条建议为 `利润率低于 20%，建议复核人力投入和外采成本。`。
9. 运行时 `POST /cost-projects/:id/ai-advice` 返回 2 条建议。
10. 运行时 `POST /cost-projects/:id/report` 生成报告 `b765d945-040b-40b0-9ed5-cc9ca27fedc9`，status 为 `generated`。
11. goose 迁移版本为 14。
12. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询新建成本项目、成本项和报告均返回 0，tenant1 均返回 1。

### 偏离蓝图

1. `ai-advice` 当前是规则化建议占位，尚未接入 Python AI 服务异步任务。
2. `report` 当前生成数据库报告摘要，尚未导出 docx/pdf 文件资产。
3. 成本项目删除和成本报告列表接口尚未展开，后续可随成本模块深化补齐。

## Loop-10 / 合规检查真实 API - 2026-06-11

### 本轮目标

1. 将 `/compliance` 和 `/compliance/:checkId` 从静态页面/stub 推进到真实租户数据。
2. 落地合规规则库、检查任务、问题、修复日志和报告摘要。
3. 支持 pass / warn / fail_candidate / fail 严重度，其中语义类问题先进入 fail_candidate，再由人工确认 fail。

### 代码交付

1. 新增迁移 `00015_compliance_foundation.sql`，创建 `compliance_rules`、`compliance_checks`、`compliance_issues`、`compliance_reports`、`compliance_fix_logs`，启用 FORCE RLS，并为每个租户写入 5 条默认规则。
2. 新增 `platform/compliance` store，包含检查列表/创建/详情、问题列表、SSE 快照、autofix、ignore、confirm-fail、报告生成和规则 CRUD。
3. API 路由新增真实 Compliance handlers，替换 `/compliance/checks`、`/compliance/issues`、`/compliance/rules` 相关 stub。
4. 前端 API client 新增 Compliance DTO 和接口方法。
5. 合规检查页改为真实检查历史、规则库、新增规则和开始检查；检查详情页接入问题分组、状态动作和报告生成。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X GET /api/v1/compliance/rules ...
curl -X POST /api/v1/compliance/checks ...
curl -X GET /api/v1/compliance/checks/:id/issues ...
curl -X POST /api/v1/compliance/issues/:id/confirm-fail ...
curl -X POST /api/v1/compliance/issues/:id/ignore ...
curl -X POST /api/v1/compliance/issues/:id/autofix ...
curl -X POST /api/v1/compliance/checks/:id/report ...
curl -N /api/v1/compliance/checks/:id/stream ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker backend/frontend 构建成功，backend/frontend/ai-service 启动成功。
5. `./infra/scripts/check.sh` 通过。
6. 运行时 `GET /compliance/rules` 返回 5 条默认规则。
7. 运行时创建检查 `16c99267-9831-45d7-82ae-3c5b9b17fabd`，初始结果为 `fail`，score 为 33，问题数为 4。
8. 运行时将 fail_candidate 问题人工确认后返回 `confirmed_fail:fail`。
9. 运行时忽略 warn 问题后返回 `ignored:warn`。
10. 运行时一键修复 fail 问题后返回 `fixed:fail`，检查回算结果为 `fail`，score 为 50。
11. 运行时报告 `1cacd276-e2ed-4925-b859-6be9ee422c2a` 生成成功，summary 为 `合规得分 50，结果 fail，问题 4 项`。
12. 运行时 SSE 返回 `event: compliance`。
13. goose 迁移版本为 15。
14. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询新建检查、问题和报告均返回 0，tenant1 分别返回 1、4、1。

### 偏离蓝图

1. 合规检查当前同步生成数据库快照并返回 202，尚未接入 Python AI 服务异步任务。
2. `autofix` 当前记录修复动作并回算状态，尚未接入编辑器自动定位和内容改写。
3. `report` 当前生成数据库报告摘要，尚未导出 docx/pdf 文件资产。

## Loop-11 / 审批与通知真实 API - 2026-06-11

### 本轮目标

1. 将团队协作中的审批链、审批实例和通知从 stub/静态内容推进到真实租户数据。
2. 落地标书提交审批、审批通过、审批驳回和状态回写。
3. 补齐通知已读和通知 SSE 快照接口。

### 代码交付

1. 新增迁移 `00016_approval_foundation.sql`，创建 `approval_chains`、`approval_instances`、`approval_actions`、`comments`，启用 FORCE RLS，并为每个租户写入默认标书审批链。
2. 新增 `platform/approval` store，包含审批链 CRUD、标书提交审批、审批列表/详情、审批通过和审批驳回。
3. API 路由新增真实 Approval handlers，替换 `/approval-chains`、`/approvals`、`/bids/:id/submit-for-approval` 相关 stub。
4. 通知 store 新增 `POST /notifications/read`，API 新增 `GET /notifications/stream` SSE 快照。
5. 前端 API client 新增 Approval DTO 和接口方法；`/team` 接入真实审批链、审批实例、审批动作、通知列表和已读动作；`/bids` 增加提交审批入口。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X GET /api/v1/approval-chains ...
curl -X POST /api/v1/approval-chains ...
curl -X PATCH /api/v1/approval-chains/:id ...
curl -X DELETE /api/v1/approval-chains/:id ...
curl -X POST /api/v1/bids/:id/submit-for-approval ...
curl -X POST /api/v1/approvals/:id/approve ...
curl -X POST /api/v1/approvals/:id/reject ...
curl -X POST /api/v1/notifications/read ...
curl -N /api/v1/notifications/stream ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker backend/frontend 构建成功，backend/frontend/ai-service 启动成功。
5. `./infra/scripts/check.sh` 通过。
6. 运行时 `GET /approval-chains` 初始返回 1 条默认审批链，默认链 id 为 `cf00605e-1fe8-4f56-8057-284dd333ac3e`。
7. 运行时创建临时审批链 `e02d066e-0fe4-4117-8403-8cacbf9e8006`，PATCH 后名称为 `临时验证审批链-已更新`，DELETE 返回 204。
8. 运行时创建标书 `e342164c-8f63-4c4d-b25c-e6a937344e88` 并提交审批，审批实例 `c4f80a1b-34fb-49a6-9191-e7669c9bc089` 初始状态为 `pending`。
9. 第一次 approve 后实例仍为 `pending` 且 current_step 为 2；第二次 approve 后实例为 `approved`，标书状态为 `approved`，动作数为 3。
10. 运行时创建标书 `172d0cda-24ee-4c0a-bc0b-a9857a07a495` 并提交审批，随后 reject，审批实例 `959474f4-51c4-40e4-8661-bd314cbbce00` 为 `rejected`，标书状态回到 `editing`。
11. 运行时 `GET /approvals` 返回 2 条审批实例。
12. 运行时通知列表返回 5 条，`POST /notifications/read` 标记 5 条已读。
13. 运行时通知 SSE 返回 `event: notifications`。
14. goose 迁移版本为 16。
15. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询默认审批链、审批实例和审批动作均返回 0，tenant1 分别返回 1、2、3。

### 偏离蓝图

1. 审批链 steps 当前以 JSON 保存，尚未拆成独立审批级表和拖拽排序 API。
2. 审批人校验当前依赖模块权限，尚未按当前审批级角色强约束执行人。
3. 通知 SSE 当前返回快照事件，尚未接入 Redis pub/sub 实时增量推送。

## Loop-12 / 工作台真实聚合 API - 2026-06-11

### 本轮目标

1. 将 `/dashboard` 从静态统计卡片推进到真实租户聚合数据。
2. 落地工作台 summary API，集中返回统计、趋势、推荐标讯、最近项目、待审批和通知快照。
3. 验证 summary API 在不同租户下只返回当前租户数据。

### 代码交付

1. 新增 `platform/dashboard` store，使用当前租户上下文聚合项目、标书、合规、审批、AI 任务、知识库、标讯和通知数据。
2. API 新增 `GET /api/v1/dashboard/summary`，纳入 `routeSpecs` 和 RBAC `dashboard:read` 控制。
3. 后端启动流程注入 `dashboard.NewStore(pool)`。
4. 前端 API client 新增 `PlatformSummaryDTO` 和 `fetchPlatformSummary`。
5. `/dashboard` 页面改为消费真实 summary API，展示真实统计、6 个月趋势、推荐标讯、待审批、通知和最近项目。
6. API 文档补充 dashboard summary 契约说明。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X GET /api/v1/dashboard/summary ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker backend/frontend 构建成功，backend/frontend/ai-service 启动成功。
5. `./infra/scripts/check.sh` 通过。
6. 运行时 tenant1/admin `GET /dashboard/summary` 返回 active_projects=2、monthly_bids=5、knowledge_docs=5、trends=6、recommended_tenders=3、recent_projects=4、notifications=5。
7. 运行时 tenant1/admin top recommended tender 为 `某市智慧交通综合治理平台建设项目`。
8. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询 projects=4、bid_documents=5、tenders=3、notifications=7。
9. 使用 `zbt_app` 设置 tenant2 RLS 上下文查询 projects=0、bid_documents=0、tenders=3、notifications=0。
10. 运行时 tenant2/other `GET /dashboard/summary` 返回 active_projects=0、monthly_bids=0、recommended_tenders=3、notifications=0。

### 偏离蓝图

1. 工作台 summary 当前是只读聚合快照，尚未接入 Redis 或 WebSocket 实时增量刷新。
2. 趋势图当前聚合标书数量和项目中标率，尚未加入成本偏差、风险热度等深度经营指标。
3. 待办当前聚合审批、严重合规问题和 AI 任务数量，尚未拆分成可单项跳转的任务中心。

## Loop-13 / AI 调用审计日志 - 2026-06-11

### 本轮目标

1. 补齐 x.md 中所有 AI 调用必须写入 `ai_call_logs` 的底座要求。
2. 将知识库 embedding 搜索和 Python HMAC 回调完成的 AI 任务接入真实租户审计日志。
3. 将团队协作日志页从审批时间线推进到真实 AI 调用审计表。

### 代码交付

1. 新增 `platform/aicall` store，支持 AI 调用日志记录、回调任务去重记录和租户内列表查询。
2. 后端启动流程创建并注入 `aicall.Store`；API 新增 `GET /api/v1/ai-call-logs`，纳入 `routeSpecs` 和 `team:read` 权限。
3. `POST /ai/callbacks/tasks` 在 knowledge_document、bid_chapter、bid_export 回调更新业务任务后追加 `ai_call_logs`。
4. `POST /knowledge/search` 调用 Python `/embeddings/knowledge` 成功后写入 `knowledge_embedding` 审计日志，并保留 AI 服务不可用时的关键词检索兜底。
5. Python knowledge_process 回调结果补充 `model_metadata` 和 `token_usage`，便于 Go 统一落库。
6. 前端 API client 新增 `AICallLogDTO` 和 `fetchAICallLogs`；`/team?tab=logs` 改为展示真实 AI 调用日志。
7. API 和数据库蓝图补充 AI 调用审计说明。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
cd ai-service && python3 -m compileall app
git diff --check
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X POST /api/v1/knowledge/search ...
curl -X POST /api/v1/chapters/:chapterId/regenerate ...
curl -X GET /api/v1/ai-call-logs ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `python3 -m compileall app` 通过；本机 Python 未安装 pytest，额外 `python3 -m pytest app/tests` 未运行。
4. `git diff --check` 通过。
5. Docker backend/frontend/ai-service 构建成功，并启动成功。
6. `./infra/scripts/check.sh` 通过。
7. 运行时 tenant1/admin 调用 `/knowledge/search` 后，`GET /ai-call-logs` 返回新增 `knowledge_embedding` 日志：provider=`mock`，model=`configurable-embedding-model`，status=`done`，input_tokens=2。
8. 运行时 tenant1/admin 触发章节 `52000000-0000-4000-8000-000000000001` 重新生成，任务 `17d2ea65-6441-482a-bb69-705bf3b51160` 回调完成为 `done`，`GET /ai-call-logs` 返回 `chapter_generate` 日志：provider=`mock`，model=`mock-model`，input_tokens=220，output_tokens=256。
9. 运行时 tenant2/other `GET /ai-call-logs` 返回 0 条。
10. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询 `ai_call_logs` 可见 `chapter_generate` 和 `knowledge_embedding` 各 1 条；tenant2 RLS 上下文可见 0 条。

### 偏离蓝图

1. `estimated_cost` 当前仍为 0，尚未按 provider/model 配置单价计算费用。
2. Python `ModelRouter.log_call()` 仍是轻量内存返回；持久化审计由 Go 在业务回调和 embedding 查询完成后统一落库。
3. 失败的 Python AI 服务调用当前只有进入 HMAC 回调或成功返回 Go 后才会写日志；HTTP 级不可达失败仍作为调用方错误处理。

## Loop-14 / 标书 PDF 导出闭环 - 2026-06-11

### 本轮目标

1. 补齐 x.md 第 13 节和 Loop-10 中 PDF 导出的硬要求。
2. 复用现有 Go 编排、Python 导出、MinIO 私有存储、HMAC 回调和 Go 落库链路。
3. 前端第 7 步导出页支持单个分册 docx / PDF 生成和下载。

### 代码交付

1. Go `platform/bid` 的 `CreateExport` 支持 `export_type=pdf`，按单个 part 汇总章节内容，写入 `bid_exports`、`ai_tasks` 和 `file_assets(content_type='application/pdf')`。
2. Python AI 服务新增 `/tasks/export/pdf`，先用 `python-docx` 生成 docx 中间文件，再调用 LibreOffice Headless 转换为 PDF 并上传 MinIO。
3. Python PDF 回调结果包含 export_type、part_count、chapter_count、size_bytes 和 `content_type=application/pdf`。
4. 前端标书向导第 7 步为每个可导出分册提供 `.docx` 和 `.pdf` 两个动作；导出历史中用类型标签区分 DOCX / PDF / ZIP。
5. API、数据库和导出设计文档同步更新 PDF 导出状态。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
cd ai-service && python3 -m compileall app
git diff --check
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X POST /api/v1/bids/:bidId/exports -d '{"export_type":"pdf","part_code":"tech"}'
curl -X GET /api/v1/bid-exports/:exportId ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `python3 -m compileall app` 通过。
4. `git diff --check` 通过。
5. Docker backend/frontend/ai-service 构建成功，并启动成功。
6. `./infra/scripts/check.sh` 通过。
7. 运行时使用 bid `50000000-0000-4000-8000-000000000001` 创建 PDF 导出 `c90cd2aa-40ab-4462-9db7-3929293c681e`，本地任务 `4c0483cc-337f-4824-9e18-427f018c1dba`，外部任务 `task-export-c90cd2aa40ab`。
8. 运行时 PDF 导出完成为 `done`，文件名为 `智慧交通平台分离标书-技术标.pdf`，file_asset 为 `a4d0c2e5-9bac-47af-ad4c-0a427d711d79`，content_type 为 `application/pdf`，size_bytes 为 34211。
9. 下载文件头为 `%PDF-`。
10. 运行时 tenant2/other 访问 tenant1 PDF 导出详情返回 404。
11. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询该 PDF export 可见 export_type=`pdf`、status=`done`、content_type=`application/pdf`、size_bytes=34211；tenant2 RLS 上下文可见 0 条。
12. `GET /ai-call-logs` 返回该 PDF document_export 日志，provider=`mock`、model=`docx-export-engine`、status=`done`。

### 偏离蓝图

1. PDF 当前以最小 docx 中间文件转换生成，封面、目录、页眉页脚、水印和模板版式保真仍待增强。
2. ZIP 当前仍打包 docx 文件，尚未包含 PDF 副本、附件、工程量清单、电子标特殊格式或导出清单 manifest。

## Loop-15 / 招标文件解析与目录大纲闭环 - 2026-06-11

### 本轮目标

1. 补齐 x.md 最终验收中“可上传招标文件、可触发解析任务、可人工确认解析结果、可生成目录大纲、可编辑目录”的主链路。
2. 保持文件私有存储、租户 RLS、AI task 编排和现有标书章节模型一致。
3. 前端 7 步向导第 1-4 步从静态占位切换为真实接口。

### 代码交付

1. 新增 `bid_tender_files`、`bid_parse_results`、`bid_material_selections` RLS 表。
2. Go API 新增真实 `POST /bids/:id/upload-tender-file`、`POST /bids/:id/parse-tender`、`GET/PUT /bids/:id/parse-result`、`POST /bids/:id/outline/generate`、`GET/PUT /bids/:id/parts/:partId/outline`、`GET/PUT /bids/:id/material-selection`。
3. 解析和目录生成当前使用确定性 bootstrap 结果，同时写入 done 状态 `ai_tasks`，接口仍返回 202 以保持后续异步 AI/OCR 契约。
4. 前端向导支持选择文件、预签名上传、绑定标书、触发解析、编辑 JSON 解析结果、确认解析、生成目录、编辑/新增章节和保存素材选择。
5. API 与数据库蓝图同步更新新增接口和表。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
cd ai-service && python3 -m compileall app
git diff --check
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X POST /api/v1/files/presign-upload ...
curl -X POST /api/v1/bids/:id/upload-tender-file ...
curl -X POST /api/v1/bids/:id/parse-tender ...
curl -X PUT /api/v1/bids/:id/parse-result ...
curl -X POST /api/v1/bids/:id/outline/generate ...
curl -X PUT /api/v1/bids/:id/parts/:partId/outline ...
curl -X PUT /api/v1/bids/:id/material-selection ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `python3 -m compileall app` 通过。
4. `git diff --check` 通过。
5. Docker backend/frontend/ai-service 构建成功，并启动成功；前端 client 归一化修正后已单独重建并重启 frontend 容器。
6. `./infra/scripts/check.sh` 通过。
7. 运行时新建 bid `62596c59-a084-4a8f-b758-1da522c201ac`，上传并确认招标文件 `3583c823-0007-460f-891c-88aadb2ab740`，`POST /bids/:id/upload-tender-file` 返回 parse_result.status=`queued`。
8. 运行时 `POST /bids/:id/parse-tender` 返回 task.status=`done`；`GET /parse-result` 返回 status=`ready`；`PUT /parse-result` 后 status=`confirmed`。
9. 运行时 `POST /outline/generate` 生成 2 个 part；`PUT /parts/:partId/outline` 新增“运行时验证章节”后该 part 章节数为 4。
10. 运行时 `GET/PUT /material-selection` 成功保存 notes=`runtime material verification`。
11. tenant2/other 访问 tenant1 新建 bid 的 parse-result 返回 404。
12. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询新 bid：`bid_parse_results=1`、`bid_tender_files=1`、`bid_material_selections=1`；tenant2 RLS 上下文三项均为 0。

### 偏离蓝图

1. 招标文件解析和目录生成当前为 Go 侧确定性 bootstrap，实现了接口契约、任务记录和人工确认闭环；尚未接入真实 OCR/LLM 解析 worker。
2. 素材选择当前保存用户确认结果和备注，后续逐章生成仍需将该选择纳入知识库检索过滤与提示词上下文。

## Loop-16 / 逐章生成 Job 与暂停继续闭环 - 2026-06-11

### 本轮目标

1. 补齐 x.md 中 `POST /bids/:id/generate`、`GET /generation-jobs/:jobId`、pause/resume/cancel 和“可逐章生成标书”的主链路。
2. 每章独立任务，复用现有 ModelRouter、AI service、HMAC callback、source_refs、needs_human_input 和版本快照链路。
3. 前端 7 步向导第 5 步从静态进度切换为真实整标/分册生成控制台。

### 代码交付

1. 新增 `bid_generation_jobs`、`bid_generation_steps` RLS 表，记录 job 进度、step 状态、关联 `ai_tasks` 和 trace_id。
2. Go API 新增真实 `POST /bids/:id/generate`、`GET /bids/:id/generation-jobs`、`GET /generation-jobs/:jobId`、`POST /generation-jobs/:jobId/pause|resume|cancel`。
3. Go 生成 job 后只派发一个章节任务；章节 HMAC 回调完成后刷新 step/job 进度，并在 job 仍 running 时自动派发下一章。pause 在章节边界生效，resume 继续派发下一 queued step，cancel 取消未开始 step。
4. 前端向导第 5 步支持启动整标逐章生成、按分册生成、查看 job 进度和暂停/继续/取消。
5. API 与数据库蓝图同步更新新增接口和表。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
cd ai-service && python3 -m compileall app
git diff --check
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X POST /api/v1/bids/:id/generate ...
curl -X GET /api/v1/generation-jobs/:jobId ...
curl -X POST /api/v1/generation-jobs/:jobId/pause ...
curl -X POST /api/v1/generation-jobs/:jobId/resume ...
curl -X POST /api/v1/generation-jobs/:jobId/cancel ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `python3 -m compileall app` 通过。
4. `git diff --check` 通过。
5. Docker backend/frontend/ai-service 构建成功，并启动成功。
6. `./infra/scripts/check.sh` 通过。
7. 运行时新建 bid `17f35433-974e-49b1-9db0-ff6a11e89956`，生成目录后启动技术标逐章生成 job `8f88573f-4ea2-4ee6-9bf9-ed65684ed119`，自动推进完成为 status=`done`，steps=`3/3`。
8. 运行时启动商务标 job `e2430c3b-a9c4-4bcb-b454-89bc0273aea9` 后立即 pause，返回 status=`paused`；resume 后自动推进完成为 status=`done`。
9. 运行时新建 cancel 验证 bid 并启动 full job `6bb84e5a-6d0f-48ba-ad44-5646855b7682`，调用 cancel 后 status=`cancelled`。
10. 完整生成验证 bid 的章节中 generated/accepted 数量为 6。
11. tenant2/other 访问 tenant1 generation job 返回 404。
12. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询 generation jobs 可见 2 条、tech job steps 可见 3 条、cancel job status=`cancelled`；tenant2 RLS 上下文 jobs/steps 均为 0。

### 偏离蓝图

1. pause 在章节边界生效；当前正在执行的 AI chapter task 不做强制中断，完成后不会继续派发下一章，符合本阶段任务边界控制。
2. cancel 会取消未开始 step；已经派发给 AI service 的 running step 仍可能回调完成，但 job 保持 cancelled 且不会继续派发。

## Loop-17 / 章节 AI 改写与自检闭环 - 2026-06-11

### 本轮目标

1. 补齐 x.md 中 `POST /chapters/:chapterId/ai-action`、章节自检真实接口和改写助手真实接口。
2. AI 动作必须通过 Python AI 服务 ModelRouter，不在 Go 业务代码里直连模型或本地伪造最终结果。
3. 前端三栏编辑器的 AI 助手提供优化、扩写、缩写、加细节和自检动作。

### 代码交付

1. Python AI 服务新增 `ChapterActionRequest` 和 `/tasks/chapter-action`，根据 action 走 `rewrite_assistant` 或 `chapter_self_check` 路由。
2. MockProvider 新增 `chapter_action()`，返回 Tiptap JSON、source_refs、self_check、needs_human_input、model_metadata 和 token_usage。
3. Go 新增 `ChapterAIAction`，创建 `chapter_ai_action` ai_task，调用 Python AI 服务，HMAC 回调后回写章节、版本快照、source_refs 和 knowledge_references。
4. API 新增真实 `POST /chapters/:chapterId/ai-action` handler 并移出 stub。
5. 前端编辑器 AI 助手新增优化、扩写、缩写、加细节、自检按钮，复用 task 轮询刷新章节。
6. API 蓝图同步更新章节 AI 动作状态。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
cd ai-service && python3 -m compileall app
git diff --check
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X POST /api/v1/chapters/:chapterId/ai-action {"action":"optimize"}
curl -X POST /api/v1/chapters/:chapterId/ai-action {"action":"self_check"}
curl -X GET /api/v1/ai-tasks/:taskId
curl -X GET /api/v1/chapters/:chapterId/versions
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `python3 -m compileall app` 通过。
4. `git diff --check` 通过。
5. Docker backend/frontend/ai-service 构建成功，并启动成功。
6. `./infra/scripts/check.sh` 通过。
7. 运行时新建 bid `404e92b9-75c6-4b33-ab27-8eba0bc08410`，生成默认大纲后选取章节 `8997758b-ebac-404f-99af-c27baa828fc2`。
8. `optimize` 动作创建 task `2888a139-244f-4a76-9fca-216ac2c4d6a6`，Go 轮询状态为 `done`，task_type=`chapter_ai_action`，result.model_metadata.action=`optimize`。
9. `self_check` 动作创建 task `1a38542b-836c-44dd-91dd-1cd4c5b9fb8d`，Go 轮询状态为 `done`，task_type=`chapter_ai_action`，result.model_metadata.action=`self_check`，result.self_check 存在。
10. 回查章节状态为 `generated`，source_refs 数量为 4，needs_human_input 数量为 1，最新版本 change_reason=`ai_action`，版本数量为 2。
11. tenant2/other 调用 tenant1 章节 `POST /chapters/:chapterId/ai-action` 返回 404。
12. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询该章节 chapter_ai_action task 数量为 2、ai_action 版本数量为 2；tenant2 RLS 上下文两者均为 0。

### 偏离蓝图

1. 本轮仍使用 MockProvider 作为无 API Key 环境下的模型提供方，但调用入口、路由选择、Pydantic 校验和 HMAC 回调均经过 Python AI 服务 ModelRouter。

## Loop-18 / 团队成员管理真实接口 - 2026-06-11

### 本轮目标

1. 补齐 `PATCH /tenant/members/:id` 和 `DELETE /tenant/members/:id`，避免团队成员管理仍落入 stub。
2. 前端团队页成员 Tab 支持编辑成员状态、角色和禁用成员。
3. 继续验证多租户隔离，确保 tenant2 不能枚举或修改 tenant1 成员。

### 代码交付

1. SaaS Store 新增 `UpdateMemberRequest`、`UpdateMember` 和 `DeleteMember`。
2. `PATCH /tenant/members/:id` 支持更新姓名、状态 active/invited/disabled 和角色集合。
3. `DELETE /tenant/members/:id` 采用软禁用语义，将成员状态置为 disabled，保留历史审批、审计和角色引用。
4. API 路由新增真实成员更新/禁用 handler，并从 stub 注册表移除。
5. 前端 API client 新增 `updateMember` 和 `deleteMember`。
6. `/team?tab=members` 增加成员设置弹窗、角色多选、状态选择和禁用操作。
7. API 蓝图同步更新成员管理接口状态。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/saas/store.go internal/api/routes.go && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend
./infra/scripts/check.sh
curl -X POST /api/v1/tenant/members/invite
curl -X PATCH /api/v1/tenant/members/:id
curl -X DELETE /api/v1/tenant/members/:id
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker backend/frontend 构建成功，并启动成功。
5. `./infra/scripts/check.sh` 通过。
6. 运行时邀请临时成员 `7c54e554-a31f-412f-aca5-a2200ca19423`，邮箱 `member-20260611131119@zbt.local`，初始 status=`active`，role=`viewer`。
7. `PATCH /tenant/members/:id` 将姓名改为“临时成员已更新”、status=`active`、role=`bid_specialist`，接口返回更新后的成员快照。
8. tenant2/other 调用 tenant1 成员 `PATCH /tenant/members/:id` 返回 404。
9. `DELETE /tenant/members/:id` 返回 204，回查成员 status=`disabled`，角色仍保留为 `bid_specialist`。
10. 禁用成员再次登录返回 401。
11. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询该成员 disabled 记录数量为 1、角色记录数量为 1；tenant2 RLS 上下文两者均为 0。

### 偏离蓝图

1. `DELETE /tenant/members/:id` 采用软禁用而非物理删除，以避免破坏审批、审计和历史项目成员引用；禁用后登录链路已验证会被拒绝。

## Loop-19 / 注册与租户创建闭环 - 2026-06-11

### 本轮目标

1. 补齐 x.md 最终验收中的“可以注册 / 登录”和“可以创建企业租户”。
2. 将 `POST /auth/register`、`POST /auth/refresh`、`POST /auth/logout` 从 API 文档声明推进到真实后端接口。
3. 前端 `/register` 从静态表单切到真实注册接口。

### 代码交付

1. SaaS Store 新增 `RegisterRequest` 和 `Register`，注册时生成 tenant UUID 并在同一事务中设置 RLS 上下文。
2. 注册链路创建 tenant、管理员 user、tenant_member、tenant_member_roles、默认角色矩阵、module_permissions 和欢迎通知。
3. 新租户默认包含 company_admin、department_admin、project_manager、bid_specialist 和 viewer 角色。
4. API 新增 `POST /auth/register`、`POST /auth/refresh` 和 `POST /auth/logout`。
5. 登录、注册和刷新复用统一 session/token 响应结构。
6. 前端 API client 新增 `registerTenant`、`refreshSession` 和 `logoutSession`。
7. `/register` 页面接入真实注册接口，注册成功后保存 session 并进入 `/onboarding`。
8. Shell 退出登录会调用后端 logout 后清理本地 session。
9. API 蓝图和 README 同步更新注册说明。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/saas/store.go internal/api/routes.go && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
docker compose build backend frontend
docker compose up -d backend frontend
./infra/scripts/check.sh
curl -X POST /api/v1/auth/register
curl -X POST /api/v1/auth/refresh
curl -X POST /api/v1/auth/logout
curl -X GET /api/v1/me
curl -X GET /api/v1/tenant/members
curl -X GET /api/v1/roles
curl -X GET /api/v1/notifications
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. Docker backend/frontend 构建成功，并启动成功。
5. `./infra/scripts/check.sh` 通过。
6. 运行时注册新租户 `9229ce60-4e3a-4140-8ee2-5c55b762826a`，管理员邮箱 `register-20260611132924@zbt.local`，用户 `843c1d38-7442-47bd-822f-2cd510b4bd91`。
7. 注册响应直接返回 company_admin session，权限模块数量为 8。
8. 新 token 访问 `/me` 返回同一 tenant；`GET /tenant/members` 返回 1 个 active company_admin 成员；`GET /roles` 返回 bid_specialist、company_admin、department_admin、project_manager、viewer 5 个默认角色；`GET /notifications` 返回 1 条欢迎通知。
9. `POST /auth/refresh` 返回有效 access_token；`POST /auth/logout` 返回 200。
10. 使用注册邮箱和密码再次 `POST /auth/login` 成功，返回同一 tenant。
11. 使用 `zbt_app` 设置新租户 RLS 上下文查询 tenant/roles/member/notification 数量分别为 1/5/1/1；设置默认租户 RLS 上下文查询该新租户数据均为 0。

### 偏离蓝图

1. 当前 logout 为 stateless JWT 语义，不维护服务端 token 黑名单；前端会调用后端 logout 后清理本地 session。
2. `/onboarding` 本轮仍是前端初始化提示页，注册链路已创建 tenant 和默认角色矩阵；后续可继续接入知识库分类、审批链模板等初始化动作。

## Loop-20 / 最终 API Stub 清零 - 2026-06-11

### 本轮目标

1. 清理 `routeSpecs` 中最后 3 条仍可能落入通用 stub 的接口：`GET /knowledge`、`PATCH /bids/:id`、`DELETE /bids/:id`。
2. 标书列表支持真实更新与软归档，避免破坏审批、导出和审计历史。
3. 知识库首页入口返回真实统计、分类、标签、近期文档和模板数据。

### 代码交付

1. Bid Store 新增 `UpdateDocumentRequest`、`UpdateDocument` 和 `DeleteDocument`。
2. `PATCH /bids/:id` 支持更新标题、状态和关联项目，并校验跨租户项目引用。
3. `DELETE /bids/:id` 将标书状态置为 `archived`，`GET /bids` 默认过滤 archived，`GET /bids/:id` 保留审计回查能力。
4. `GET /knowledge` 汇总 `stats`、`categories`、`tags`、`recent_documents` 和 `templates`。
5. 前端 API client 新增 `updateBid` 和 `deleteBid`，标书列表增加归档操作。
6. API 蓝图同步更新最终 stub 清理状态。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/bid/store.go internal/api/routes.go && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
git diff --check
python3 routeSpecs/custom-map stub 检查
docker compose build backend frontend
docker compose up -d backend frontend
./infra/scripts/check.sh
curl -X POST /api/v1/bids
curl -X PATCH /api/v1/bids/:id
curl -X GET /api/v1/knowledge
curl -X DELETE /api/v1/bids/:id
curl -X GET /api/v1/bids
curl -X GET /api/v1/bids/:id
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `git diff --check` 通过。
4. `routeSpecs` 与 custom map 对比输出 `NO_STUB_ROUTES`。
5. Docker backend/frontend 构建成功，并启动成功。
6. `./infra/scripts/check.sh` 通过。
7. 运行时新建 bid `7d93f41d-b6d0-491d-8e76-89a7e3e98f06`，初始 status=`draft`。
8. `PATCH /bids/:id` 将标题改为“归档验证已更新”、status=`editing`，接口返回更新后的标书快照。
9. `GET /knowledge` 返回完整总览 keys：`stats`、`categories`、`tags`、`recent_documents`、`templates`；本地数据计数为 categories=4、tags=3、recent_documents=5、templates=4。
10. tenant2/other 调用 tenant1 bid 的 `PATCH /bids/:id` 返回 404。
11. `DELETE /bids/:id` 返回 204；`GET /bids` 不再返回该 bid；`GET /bids/:id` 仍可回查 status=`archived`。
12. tenant2/other 调用 tenant1 archived bid 的 `DELETE /bids/:id` 返回 404。
13. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询该 archived bid 数量为 1；tenant2 RLS 上下文数量为 0。

### 偏离蓝图

1. `DELETE /bids/:id` 采用软归档而非物理删除，以保留生成任务、审批、导出、章节和审计追踪。

## Loop-21 / 成本 AI 建议异步化 - 2026-06-11

### 本轮目标

1. 修正 `POST /cost-projects/:id/ai-advice` 在 `routeSpecs` 标记 async 但实现仍同步返回规则化分析的问题。
2. 成本 AI 建议必须经过 Python AI 服务 ModelRouter，不在 Go 成本模块里伪造 AI 结果。
3. 前端成本详情页支持提交 AI 建议任务并轮询 `/ai-tasks/:taskId` 展示结果。

### 代码交付

1. Cost Store 接入 `config.Config` 和 AI service client，新增 `Task`、`Advice`、`ApplyAdviceCallback`、任务状态更新与失败标记。
2. `POST /cost-projects/:id/ai-advice` 改为创建 `task_type=cost_advice`、`resource_type=cost_project` 的 `ai_tasks`，再调用 Python `/tasks/cost-advice`。
3. AI callback 分发新增 `cost_project` resource type，回调后更新任务结果并通过 `ai_call_logs` 记录 provider、model、token_usage 和 trace_id。
4. Python AI 服务新增 `CostAdviceRequest`、`CostAdviceResponse` 和 `/tasks/cost-advice`，通过 `cost_advice` ModelRouter 路由调用 MockProvider。
5. MockProvider 新增 `cost_advice()`，返回 summary、recommendations、risk_flags、focus_items、model_metadata 和 token_usage。
6. 前端 `createCostAdvice` 返回 `AITaskDTO`，成本详情页提交任务后轮询 `/ai-tasks/:taskId` 并展示 AI 建议。
7. API 蓝图同步更新成本 AI 建议链路。

### 检查结果

已运行：

```bash
cd backend && gofmt -w cmd/server/main.go internal/api/routes.go internal/platform/cost/store.go && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
cd ai-service && python3 -m compileall app
git diff --check
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
./infra/scripts/check.sh
curl -X POST /api/v1/cost-projects/:id/ai-advice
curl -X GET /api/v1/ai-tasks/:taskId
curl -X GET /api/v1/ai-call-logs
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. ai-service `python3 -m compileall app` 通过。
4. `git diff --check` 通过。
5. Docker backend/frontend/ai-service 构建成功，并启动成功。
6. `./infra/scripts/check.sh` 通过。
7. 运行时 tenant1/admin 对成本项目 `e5c7f0b3-13b9-4d25-96b2-95d9ff6d110c` 调用 `POST /cost-projects/:id/ai-advice`，返回本地任务 `a297ea78-5cf7-4ef9-9348-457cc1609436`、外部任务 `task-cost-advice-496fe75e-e18e-49dc-a942-a88e2b3f7d1f`。
8. `GET /ai-tasks/:taskId` 轮询到 status=`done`，result.trace_id=`trace-mock-cost-advice-e5c7f0b3`，provider=`mock`，model=`configurable-low-cost-json-model`，recommendations 数量为 4。
9. `GET /ai-call-logs` 可查到同 trace_id 的 `cost_advice` 日志，provider=`mock`。
10. tenant2/other 调用 tenant1 成本项目 `POST /cost-projects/:id/ai-advice` 返回 404。
11. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询该 `cost_advice` task 数量为 1、ai_call_logs 数量为 1；tenant2 RLS 上下文两者均为 0。

### 偏离蓝图

1. 本轮仍使用 MockProvider 作为无 API Key 环境下的成本建议模型提供方，但调用入口、路由选择、Pydantic 校验、HMAC 回调和 ai_call_logs 均已经过 Python AI 服务 ModelRouter。

## Loop-22 / 中标案例回流知识库 - 2026-06-11

### 本轮目标

1. 补齐 x.md 最终验收项“中标案例可回流知识库”。
2. closed + won 项目可一键沉淀为知识库案例文档，并立即参与知识库检索。
3. 回流链路必须经过真实 file_assets、knowledge_documents、knowledge_chunks 和项目活动日志，并保持租户隔离。

### 代码交付

1. File Service 新增 `CreateGeneratedAsset`，支持后端生成 Markdown 内容并写入 MinIO，按 `knowledge_case/:projectId` 生成稳定 object_key，重复回流会更新同一 file_asset。
2. Project Store 新增 `BuildWonCaseDraft` 和 `ArchiveWonCase`，仅允许 `status=closed` 且 `result=won` 的项目回流，汇总项目概况、关联标书、里程碑和成本复盘。
3. `POST /projects/:id/archive-case` 新增项目 + 知识库双权限校验，生成 ready 文件资产，写入 `doc_type=won_case`、`parse_status=processed` 的知识库文档，确保分类“项目案例”和标签“中标案例”，并写入可搜索 chunk。
4. 项目详情页新增“回流知识库”操作，回流成功后提示并跳转知识库文档页。
5. API 蓝图同步补充 Project 和 Knowledge 中的回流接口说明。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
docker compose build backend frontend
docker compose up -d backend frontend
./infra/scripts/check.sh
curl -X POST /api/v1/projects
curl -X POST /api/v1/projects/:id/milestones
curl -X POST /api/v1/projects/:id/transition
curl -X POST /api/v1/projects/:id/archive-case
curl -X GET /api/v1/knowledge/documents
curl -X POST /api/v1/knowledge/search
curl -X GET /api/v1/files/:fileId/preview-url
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. Docker backend/frontend 构建成功，并启动成功。
4. `./infra/scripts/check.sh` 通过。
5. 运行时创建项目 `387d2def-ceb2-4737-a14a-3d6a0638b65e`，项目名 `自动验收中标案例-20260611144210`，添加里程碑后流转为 `closed/won`。
6. `POST /projects/:id/archive-case` 返回 document `014f91b6-a823-4abc-995b-4cb014952fef`、file `c3a99299-5aae-45bd-b469-8ccfdc23f110`、chunk `9305ddb6-fad3-40d5-bc8e-f8eb4df4ed06`，file.status=`ready`。
7. `GET /knowledge/documents` 可查到该 `doc_type=won_case`、`parse_status=processed` 文档。
8. `POST /knowledge/search` 使用项目名和 `doc_type=won_case` 可召回该 document。
9. `GET /files/:fileId/preview-url` 返回有效预览 URL，长度 850。
10. tenant2/other 调用 tenant1 项目 `POST /projects/:id/archive-case` 返回 404。
11. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询 document/chunk/file/project_log 数量均为 1；设置 tenant2 RLS 上下文查询同一批 ID 数量均为 0。

### 偏离蓝图

1. 本轮回流案例的 chunk 先写入文本检索字段，embedding 为空；关键词/全文检索可立即召回。后续如需语义向量召回，可复用现有知识库处理或新增 generated case embedding 任务。

## Loop-23 / RAG RRF 融合与 RerankProvider - 2026-06-11

### 本轮目标

1. 补齐 x.md RAG 设计中“pgvector Top K + 全文 Top K -> RRF 融合 -> RerankProvider 精排”的排序链路。
2. 知识库搜索不再只做简单加权排序，RerankProvider 必须经过 Python AI 服务 ModelRouter。
3. rerank 调用需写入 `ai_call_logs`，并保持租户隔离。

### 代码交付

1. Go Knowledge Store 的 `POST /knowledge/search` 改为先取向量候选和关键词候选，再用 RRF 合并候选；query 为空时保留 recent fallback。
2. Go 后端新增 `/rerank/knowledge` 调用、rerank 请求/响应结构、candidate fanout、候选截断和 rerank 结果应用逻辑；AI 服务不可用时回退 RRF 结果。
3. Python AI 服务新增 `KnowledgeRerankRequest/Response` schema 和 `/rerank/knowledge` endpoint，通过 `knowledge_rerank` 路由获取 RerankProvider。
4. ModelRouter 新增 `get_rerank`，MockProvider 的 rerank 从原始顺序占位改为基于 query/document token overlap 的确定性排序。
5. Go 单元测试覆盖 candidate fanout、rerank 应用和 Unicode 截断；Python 测试文件补充 Mock rerank 排序覆盖。
6. RAG 设计和模型网关文档同步更新当前状态。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/knowledge/store.go internal/platform/knowledge/store_test.go && GOTOOLCHAIN=local go test ./...
cd ai-service && python3 -m compileall app
cd ai-service && python3 -m pytest app/tests
docker compose build backend ai-service
docker compose up -d backend ai-service
curl -X POST http://127.0.0.1:8000/rerank/knowledge
./infra/scripts/check.sh
curl -X POST /api/v1/knowledge/search
curl -X GET /api/v1/ai-call-logs
```

结果：

1. backend Go 测试通过，`knowledge` 包新增测试通过。
2. ai-service `python3 -m compileall app` 通过。
3. 本机 Python 缺少 pytest 依赖，`python3 -m pytest app/tests` 返回 `No module named pytest`；容器镜像依赖安装成功，后续由 Docker 与 check 脚本覆盖。
4. Docker backend/ai-service 构建成功，并启动成功。
5. 直接调用 `POST http://127.0.0.1:8000/rerank/knowledge` 返回 provider=`mock`、model=`configurable-rerank-model`，相关文档排第一。
6. `./infra/scripts/check.sh` 通过。
7. 运行时 tenant1/admin 调用 `/knowledge/search`，query=`智慧交通 项目实施`，返回 3 条结果，首条 score=`1`，证明已应用 rerank 分数。
8. `GET /ai-call-logs` 可查到 `knowledge_rerank` 日志，provider=`mock`、model=`configurable-rerank-model`、status=`done`。
9. tenant2/other 通过 API 查询 `ai-call-logs` 时看不到 tenant1 的 rerank 日志。
10. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询 `knowledge_rerank` 日志数量为 1；设置 tenant2 RLS 上下文查询数量为 0。

### 偏离蓝图

1. RerankProvider 当前仍为 MockProvider；调用入口、ModelRouter 路由、Go 端 rerank 调用、ai_call_logs 和租户隔离已落地，后续可替换 Cohere/Jina/Local BGE reranker。

## Loop-24 / README 启动说明与 AI pytest 验证 - 2026-06-11

### 本轮目标

1. 补齐 x.md 最终验收项“模型可通过 model_routing.yaml 切换”和“README 能指导新开发者启动”。
2. 消除此前多轮记录的 AI 服务 pytest 环境缺口，让 Docker 环境可以直接运行 Python 测试。
3. 让 `./infra/scripts/check.sh` 在服务已启动时自动执行容器内 pytest。

### 代码交付

1. Python AI 服务启动时优先读取 `MODEL_ROUTING_FILE`，未设置时回退内置 `app/config/model_routing.yaml`。
2. AI Dockerfile 新增 `INSTALL_DEV_DEPS` 构建参数，默认安装 `.[dev]`，因此容器内包含 pytest、ruff 和 mypy；需要瘦身时可设置 `INSTALL_DEV_DEPS=false`。
3. `infra/scripts/check.sh` 保留本地 `compileall`，本机有 pytest 时运行本机测试；本机缺 pytest 时跳过并提示；若 `ai-service` 容器正在运行，则执行 `docker compose exec -T ai-service python -m pytest app/tests`。
4. 根 README 重写为当前真实启动、账号、验证、模型路由、文件上传和验收定位说明；`frontend/README.md` 从 Vite 模板改为项目内前端说明。
5. `.env.example` 的 `MODEL_ROUTING_FILE` 改为 Docker 内真实路径 `./app/config/model_routing.yaml`。
6. API 和数据库蓝图同步更新 RAG rerank、ai_call_logs 和知识库检索说明。

### 检查结果

已运行：

```bash
cd ai-service && python3 -m compileall app
docker compose config
docker compose build ai-service
docker compose up -d ai-service
docker compose exec -T ai-service python -m pytest app/tests
curl http://127.0.0.1:8000/healthz
curl http://127.0.0.1:8000/models/health
docker compose exec -T ai-service python -c 'from app.main import CONFIG_PATH; print(CONFIG_PATH)'
./infra/scripts/check.sh
```

结果：

1. ai-service `python3 -m compileall app` 通过。
2. `docker compose config` 通过。
3. Docker ai-service 镜像构建成功，日志显示安装了 `pytest-9.0.3`。
4. ai-service 容器重启成功。
5. `docker compose exec -T ai-service python -m pytest app/tests` 通过，5 个 Python 测试全部通过。
6. AI `/healthz` 和 `/models/health` 返回 ok，mock provider 健康。
7. 容器内 `CONFIG_PATH` 输出 `app/config/model_routing.yaml`，证明 `MODEL_ROUTING_FILE` 生效。
8. `./infra/scripts/check.sh` 通过；其中本机 pytest 不可用时按预期跳过本机 pytest，随后在运行中的 ai-service 容器内执行 pytest，5 个测试通过。

### 偏离蓝图

1. 本机 Python 仍未安装项目依赖，因此本机 pytest 会被检查脚本跳过；Docker 开发环境已能直接运行 pytest，符合新开发者一键启动和验证路径。

## Loop-25 / 合规问题跳转编辑器定位 - 2026-06-11

### 本轮目标

1. 补齐 x.md 最终验收项“可跳转编辑器定位问题”。
2. 合规 issue 需要携带关联标书、章节和编辑器路径，前端可从问题清单直接打开编辑器复核。
3. 保持旧合规检查兼容，缺少新 location 字段时仍可回退到检查关联标书。

### 代码交付

1. `platform/compliance` 生成 issue 时新增 `buildIssueLocation`，关联标书检查会写入 `module=bid_editor`、`bid_document_id`、`chapter_id`、`part_code` 和内部编辑器 `path`。
2. issue 定位章节取关联标书的首个章节，按 `bid_parts.sort_order`、`bid_chapters.sort_order`、章节创建时间排序，保证可跳到真实存在的编辑器章节。
3. 合规问题表新增“定位”按钮，优先使用后端 `location.path`，缺字段时用 `check.bid_document_id`、`chapter_id`、`part_code` 拼出 `/bids/:bidId/editor`。
4. autofix 审计消息从“后续接入编辑器自动定位”更新为当前可定位复核语义。
5. API 文档同步说明 `compliance_issues.location` 的编辑器定位字段。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./...
cd frontend && pnpm build
./infra/scripts/check.sh
docker compose build backend frontend
docker compose up -d backend frontend
curl -X POST /api/v1/bids ...
curl -X POST /api/v1/compliance/checks ...
curl -X GET /api/v1/compliance/checks/:id/issues ...
curl -X GET /api/v1/bids/:bidId/chapters ...
docker compose exec -T postgres psql -U zbt_app -d zbt ...
```

结果：

1. backend Go 测试通过。
2. frontend build 通过；仍有既有大 chunk warning，无失败。
3. `./infra/scripts/check.sh` 通过，包含前端构建、Go 测试和运行中 ai-service 容器内 pytest。
4. Docker backend/frontend 构建成功，并启动成功。
5. 运行时创建标书 `c2298b64-02ea-49ed-ac2d-1b44ead891cd`，创建合规检查 `c4625b0c-5d42-45f2-b976-657d23537836`。
6. `GET /compliance/checks/:id/issues` 返回 issue `dae1526d-5931-4ad9-b197-e5aade2f438f`，location 为 `/bids/c2298b64-02ea-49ed-ac2d-1b44ead891cd/editor?chapter=770f80e7-7c03-4f78-b321-0d9b4ad4c8d5&part=combined_body`。
7. `GET /bids/:bidId/chapters` 验证该 `chapter_id` 属于同一标书。
8. tenant2 使用明确 tenant_id 登录后访问 tenant1 的 check 返回 404。
9. 使用 `zbt_app` 设置 tenant1 RLS 上下文查询该 issue 数量为 1；设置 tenant2 RLS 上下文查询数量为 0。

### 偏离蓝图

1. 目前定位到章节级，尚未在 Tiptap 文档内部滚动到具体段落或高亮 rule anchor；后续可把 `anchor` 接入编辑器内文档节点定位。

## Loop-26 / x.md 尾部验收脚本 - 2026-06-11

### 本轮目标

1. 为 x.md 第 39-50 项补充可重复运行的验收入口，避免只靠历史日志证明尾部闭环。
2. 脚本必须走真实 HTTP API，覆盖审批、成本、中标案例、知识库搜索、AI 日志、模型路由和文档记录。
3. 常规检查脚本只做语法检查，避免无意中在每次 `check.sh` 写入运行时验收数据。

### 代码交付

1. 新增 `infra/scripts/acceptance_tail_check.py`，使用 Python 标准库登录本地 API，创建独立验收数据并检查 x.md 39-50。
2. 验收脚本覆盖：提交审批、审批通过、审批驳回、驳回后标书回到 `editing`、中标项目创建成本项目、录入成本项、成本分析预算/实际/利润率、中标案例回流知识库、知识库搜索触发 `ai_call_logs`、AI `/models/health` MockProvider、`model_routing.yaml` 路由、README 和 DEV_LOOP_LOG。
3. `infra/scripts/check.sh` 新增 `python3 -m py_compile infra/scripts/acceptance_tail_check.py`，确保常规检查能发现脚本语法错误。
4. README 新增尾部验收脚本命令和运行条件说明。

### 检查结果

已运行：

```bash
python3 infra/scripts/acceptance_tail_check.py
./infra/scripts/check.sh
```

结果：

1. 验收脚本登录 `admin@zbt.local` 成功。
2. 第 39 项通过：创建标书 `c1b5c214-8a2d-43e2-968a-93eb46497048` 并提交审批，审批实例 `20d36415-cd35-4d0d-a897-b8e5736aa35e`，标书进入 `in_review`。
3. 第 40 项审批通过路径通过：标书 `385d010c-5716-4ff1-bf49-6bc1820d7661`，审批实例 `a16e2072-862d-42ba-b533-fb06b9a7dd7c`，最终 `approved`。
4. 第 40-41 项驳回路径通过：标书 `8eab5180-522a-467a-9df7-29d79988a722`，审批实例 `28c5b411-90b5-42e3-b21e-0d62e5b78e56`，审批 `rejected` 后标书回到 `editing`。
5. 第 42 项通过：中标项目 `97dc90e5-26ab-46f0-8eb5-b83c911878fb` 创建成本项目 `3e896284-c1ae-452e-b03d-c31651263fa5`。
6. 第 43-44 项通过：成本项 `d0b208ca-7fe7-4b2d-9020-73528596bdb3` 写入后，分析返回 total_budget=10000、total_actual=8500、margin_rate=15。
7. 第 45 项通过：中标案例回流知识库 document `30b71ebb-9fd2-49b7-9304-06687e1f0415`、chunk `8e2bc839-e385-482d-afed-0c2e55598b34`、file `dc5f5ae7-e355-488d-9f13-678fb04a285c`。
8. 第 46 项通过：知识库搜索召回该中标案例，并产生 1 条新的 `knowledge_embedding` AI 调用日志。
9. 第 47-48 项通过：AI `/models/health` 返回 `{"mock": true}`，本地 `model_routing.yaml` 包含 knowledge_embedding、knowledge_rerank、chapter_generate、cost_advice、document_export 等路由。
10. 第 49-50 项通过：README 包含启动、检查、模型路由、MockProvider 和默认账号说明；DEV_LOOP_LOG 包含最新 Loop 记录和检查结果段落。
11. `./infra/scripts/check.sh` 通过，包含新增验收脚本语法检查、前端构建、Go 测试、AI compileall 和运行中 ai-service 容器内 pytest。

### 偏离蓝图

1. 该脚本是尾部验收脚本，当前只覆盖 x.md 第 39-50 项；完整 1-50 一键黄金链路仍可继续沉淀为独立验收脚本。

## Loop-27 / x.md 核心验收脚本 - 2026-06-11

### 本轮目标

1. 为 x.md 第 1-38 项补充可重复运行的核心验收入口，和 Loop-26 的 39-50 尾段脚本形成完整验收证据。
2. 核心脚本必须走真实 Docker 运行栈和公开 HTTP API，覆盖 SaaS 底座、标讯、项目、标书、知识库、章节生成、编辑器、导出和合规定位。
3. `check.sh` 只编译验收脚本，避免普通检查命令反复写入大量运行时验收数据。

### 代码交付

1. 新增 `infra/scripts/acceptance_core_check.py`，使用 Python 标准库检查服务连通并创建独立时间戳验收数据。
2. 核心验收覆盖 x.md 1-38：前端/后端/AI/Postgres/Redis/MinIO 运行、注册登录、企业租户、成员邀请、角色权限、菜单权限依据、API 权限、多租户隔离、仪表盘、标讯检索/收藏/来源验证、从标讯创建项目、项目状态流转、里程碑/成员/关联标书、合并标/分离标、招标文件上传/解析/确认/大纲生成和编辑、知识文档上传/解析/检索/素材选择、章节生成、采纳/重新生成/手工编辑/diff/版本、source_refs、needs_human_input、三栏编辑器、DOCX/ZIP 导出和合规检查/证据/建议/编辑器定位。
3. `infra/scripts/check.sh` 同时编译 `acceptance_core_check.py` 和 `acceptance_tail_check.py`。
4. README 新增核心验收脚本命令、运行条件和覆盖范围说明。

### 检查结果

已运行：

```bash
python3 infra/scripts/acceptance_core_check.py
./infra/scripts/check.sh
python3 infra/scripts/acceptance_tail_check.py
```

结果：

1. `acceptance_core_check.py` 通过，确认 x.md 1-38 项验收完成。
2. 第 1 项通过：Docker 运行服务包含 ai-service、backend、frontend、minio、postgres、redis，后端和 AI health 均 ok，前端返回应用 HTML。
3. 第 2-8 项通过：默认管理员登录成功；注册创建新企业租户成功；邀请成员并绑定自定义角色成功；viewer 访问 `/cost-projects` 返回 403；tenant2 访问 tenant1 标书返回 404。
4. 第 9-16 项通过：仪表盘返回 stats、pending_approvals、notifications；标讯来源验证、检索、收藏通过；从标讯创建项目并流转到 submitted；项目详情包含里程碑、成员和关联标书；合并标和分离标创建成功，分离标包含 tech/business。
5. 第 17-25 项通过：招标文件上传、解析、确认、大纲生成和大纲编辑通过；知识文档上传、处理、切片检索和标书素材选择通过。
6. 第 26-31 项通过：章节生成完成，生成章节包含 5 条 source_refs 和 2 条 needs_human_input；采纳、重新生成、手工编辑、版本记录和 diff 均通过；编辑器路由返回前端应用，源码包含三栏编辑器、source_refs 和 needs_human_input 展示。
7. 第 32-34 项通过：综合 DOCX、技术标 DOCX、商务标 DOCX 和 ZIP 打包导出均完成并返回下载 URL。
8. 第 35-38 项通过：合规检查完成，生成 5 条 issues，issue 均包含 evidence/suggestion；location 指向 `/bids/f84d83b2-2b27-484e-94e8-e2d0fd54e0f6/editor?chapter=bc319cce-340e-4b7c-bb82-43bfb02f772d&part=combined_body`。
9. `./infra/scripts/check.sh` 通过：验收脚本语法检查、前端生产构建、Go 测试、AI compileall、Docker compose config、运行中 ai-service 容器内 pytest 均成功；本机 pytest 不可用时按预期跳过。
10. `acceptance_tail_check.py` 复验通过，确认 x.md 39-50 项在最新代码上仍通过；本次尾段复验创建审批、成本、中标案例、知识库和 AI 日志验收数据均成功。

### 偏离蓝图

1. 当前形成的是两个脚本组合覆盖 1-50，而不是单个黄金链路脚本；这样可以保持每个脚本职责清晰，后续如需 CI 夜间验收可再增加一个聚合入口。

## Loop-28 / 审查报告安全收口 - 2026-06-11

### 本轮目标

1. 处理外部代码实现审查报告中可立即落地的高优先级问题。
2. 移除 RBAC 中容易误导维护者的 demo 全权限残留，确保缺少权限上下文时默认拒绝。
3. 将伪造 `X-Tenant-ID` 不能越权的判断固化到核心验收脚本，而不是只保留在历史日志和人工审查结论中。

### 代码交付

1. 删除 `backend/internal/platform/rbac/middleware.go` 中未被使用的 `demoPermissions` 和 `DemoPermissions()`，避免后续误用为生产 fallback。
2. 新增 `backend/internal/platform/rbac/middleware_test.go`，覆盖：缺少权限上下文返回 403、read 权限只能读、full 权限可写。
3. `infra/scripts/acceptance_core_check.py` 支持请求级附加 headers，并在第 8 项租户隔离验证中新增 `X-Tenant-ID` 伪造头断言：tenant1 token 即使携带 tenant2 header 仍读取 tenant1 数据，tenant2 token 即使携带 tenant1 header 仍无法读取 tenant1 数据。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_core_check.py
cd backend && GOTOOLCHAIN=local go test ./internal/platform/rbac
python3 infra/scripts/acceptance_core_check.py
./infra/scripts/check.sh
```

结果：

1. `rg demoPermissions|DemoPermissions` 未发现残留定义或引用。
2. RBAC 包测试通过，确认缺失权限上下文默认 403，read/full 级别判断符合预期。
3. 核心验收脚本通过；第 8 项输出 `spoofed_header=session tenant wins`，证明认证 session 的 tenant 优先于请求头。
4. 重跑核心验收前发现运行中的 backend 容器未发布 8080 端口，和当前 compose 配置不一致；使用 `docker compose up -d --force-recreate --no-build backend frontend` 重新创建容器后，`127.0.0.1:8080/healthz` 返回 ok。
5. `./infra/scripts/check.sh` 通过：验收脚本语法检查、前端生产构建、Go 全量测试、AI compileall、Docker compose config、运行中 ai-service 容器内 pytest 均成功；本机 pytest 不可用时按预期跳过。

### 偏离蓝图

1. 本轮只处理审查报告中能快速降低误用风险的安全收口项；真实模型 Provider、OCR/表格解析、Word 排版保真和成本计费仍需要独立设计与分阶段实现。

## Loop-29 / 真实模型兼容 Provider 与解析增量 - 2026-06-11

### 本轮目标

1. 处理审查报告中优先级最高的核心能力缺口，先拆掉 “AI 只能跑 MockProvider” 和 “estimated_cost 恒为 0” 两个底层阻塞。
2. 在不写入任何 API Key 的前提下，落地 OpenAI-compatible Provider，可通过环境变量接 OpenAI、DeepSeek、DashScope 兼容网关。
3. 扩展文档解析的低风险格式覆盖：docx 表格、xlsx/xlsm 工作表文本、pptx/pptm 幻灯片文本。

### 代码交付

1. 新增 `ai-service/app/gateway/openai_compatible_provider.py`，实现 OpenAI-compatible chat completions、JSON 输出、embedding、LLM rerank、章节生成、章节改写和成本建议的统一 Provider。
2. `ModelRouter` 改为按 `model_routing.yaml` 构建 provider；未知或未配置 provider 不再静默回退 mock，只有 route 显式配置 fallback 时才会降级，并在 route 中写入 `fallback_from`。
3. `model_routing.yaml` 中的 `openai_compatible_primary` 现在可真实注册；DeepSeek、DashScope 等兼容 provider 通过 `*_API_KEY` 和 `*_BASE_URL` 环境变量启用。
4. 后端 `aicall.Store` 增加 `AI_MODEL_PRICING_JSON` 计价支持，按 `provider/model`、`model`、`provider/*` 或 `*` 匹配 input/output token 单价，写入 `ai_call_logs.estimated_cost`。
5. 文档解析增加 docx 表格、xlsx/xlsm、pptx/pptm 抽取，README 明确当前仍未覆盖 OCR、复杂表格结构识别和版面坐标抽取。
6. `.env.example`、`docker-compose.yml` 和 README 补充真实 Provider 与模型计价配置说明。

### 检查结果

已运行：

```bash
python3 -m compileall app
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests
cd backend && GOTOOLCHAIN=local go test ./internal/platform/aicall
docker compose config >/dev/null
./infra/scripts/check.sh
tmp_docker_config="$(mktemp -d)" && DOCKER_CONFIG="$tmp_docker_config" docker compose build backend ai-service && DOCKER_CONFIG="$tmp_docker_config" docker compose up -d backend ai-service frontend
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8000/models/health
docker compose exec -T ai-service python -m pytest app/tests
```

结果：

1. AI 服务 `compileall` 通过。
2. 挂载当前源码运行 AI 容器 pytest 通过，11 个测试全部通过；新增覆盖 OpenAI-compatible provider 注册、缺 key 不健康、显式 fallback、禁止静默 fallback、docx 表格、xlsx 和 pptx 解析。
3. 后端 aicall 测试通过，覆盖 provider/model 精确计价、provider 通配计价、未配置价格保持 0。
4. `docker compose config` 通过，确认新增环境变量插值合法。
5. `./infra/scripts/check.sh` 通过：验收脚本语法检查、前端生产构建、Go 全量测试、AI compileall、Docker compose config、运行中 ai-service 容器内旧镜像 pytest 均成功。
6. 使用临时 `DOCKER_CONFIG` 绕过本机缺失 `docker-credential-desktop.exe` 后，backend 和 ai-service 镜像构建成功，并已重启本地运行栈。
7. 后端 `/healthz` 返回 ok；AI `/models/health` 返回 `mock=true`，`openai_compatible_primary=false`、`dashscope=false`、`deepseek=false`，符合未配置真实 key/base_url 的当前环境。
8. 新 ai-service 容器内 `python -m pytest app/tests` 通过，11 个测试全部通过。

### 偏离蓝图

1. 本轮完成真实模型兼容接入底座，但没有在仓库中配置真实 API Key，也没有把默认 route 从 mock 切到真实 provider；生产切换仍需在部署环境设置 key/base_url 并修改 `model_routing.yaml` route provider。
2. 文档解析只补了 Office 文本抽取增量；扫描件 OCR、PDF 表格结构化、版面坐标、页眉页脚/目录/水印级 Word 母版保真仍未完成，需要后续独立实现。

## Loop-30 / PDF 版面表格与 OCR 接入点 - 2026-06-11

### 本轮目标

1. 继续处理审查报告中的文档解析核心缺口，把 PDF 从纯文本抽取推进到可审计的版面 metadata 和表格候选。
2. 为扫描件 OCR 增加可配置接入点；未配置 OCR 时明确标记需要 OCR，不伪装解析成功。
3. 保持当前知识库处理链路兼容，不改变现有 chunk/embedding/search API。

### 代码交付

1. `document_parser.py` 的 PDF 解析新增 `layout_blocks`、`layout_block_count`、`tables`、`table_count`、`ocr_required` metadata。
2. PDF 表格提取优先尝试 PyMuPDF `page.find_tables()`，不可用或未识别时回退到文本行分隔启发式候选。
3. 新增 `OCR_HTTP_ENDPOINT` / `OCR_HTTP_TIMEOUT_S` 接入点：空文本 PDF 会尝试调用 HTTP OCR；配置 `OCR_API_KEY` 时附带 Bearer header；未配置时 metadata 写入 `ocr.status=provider_not_configured`。
4. `.env.example`、`docker-compose.yml` 和 README 增加 OCR HTTP 配置与能力边界说明。
5. `test_document_parser.py` 新增 PDF layout/table 候选和空白 PDF OCR required 测试。

### 检查结果

已运行：

```bash
python3 -m compileall app
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests/test_document_parser.py
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests
docker compose config >/dev/null
./infra/scripts/check.sh
tmp_docker_config="$(mktemp -d)" && DOCKER_CONFIG="$tmp_docker_config" docker compose build ai-service && DOCKER_CONFIG="$tmp_docker_config" docker compose up -d ai-service backend frontend
docker compose exec -T ai-service python -m pytest app/tests
curl http://127.0.0.1:8000/models/health
docker compose ps backend ai-service frontend
```

结果：

1. AI 服务 `compileall` 通过。
2. 挂载当前源码运行 `test_document_parser.py` 通过，5 个解析测试全部通过，覆盖 PDF layout/table、空白 PDF OCR required、docx 表格、xlsx、pptx。
3. 挂载当前源码运行完整 AI pytest 通过，13 个测试全部通过。
4. `docker compose config` 通过，确认 OCR 相关环境变量插值合法。
5. `./infra/scripts/check.sh` 通过：验收脚本语法检查、前端生产构建、Go 全量测试、AI compileall、Docker compose config、运行中 ai-service 容器内 pytest 均成功。
6. 使用临时 `DOCKER_CONFIG` 绕过本机缺失 `docker-credential-desktop.exe` 后，ai-service 镜像构建成功，并已重启本地运行栈。
7. 重建后的 ai-service 容器内 `python -m pytest app/tests` 通过，13 个测试全部通过。
8. AI `/models/health` 返回 `mock=true`，`openai_compatible_primary=false`、`dashscope=false`、`deepseek=false`，符合当前环境未配置真实 key/base_url 的状态。
9. `docker compose ps` 显示 backend、ai-service、frontend 均处于 Up 状态。

### 偏离蓝图

1. 这仍不是完整 OCR 能力；当前只提供 HTTP OCR 接入点和空文本 PDF 的显式状态标记，真实 OCR 服务、图片预处理、表格结构语义识别和坐标级引用仍需后续实现。
2. Word/PDF 母版级排版保真本轮未处理。

## Loop-31 / Word PDF 默认母版导出 - 2026-06-11

### 本轮目标

1. 继续处理审查报告中的导出保真缺口，把最小 docx 输出推进到可验收的默认母版。
2. 在不改变现有 Go -> AI payload 必填字段的前提下，增加可选 layout 配置和企业 docx 样式模板入口。
3. 保持 docx、pdf、zip 三种导出路径复用同一版式源。

### 代码交付

1. `ExportLayoutOptions` 增加默认 layout 配置，旧请求不传该字段时自动使用 `zbt-standard`。
2. `export_bid_docx` 新增默认母版：封面、可刷新目录域、页眉页脚、页码域、中文字体样式、章节分页和 Word field 自动更新设置。
3. 正文渲染增加 Markdown 表格、无序列表、有序列表和简单 heading 识别。
4. `export_bid_pdf` 和 `export_bid_zip` 复用同一 `export_bid_docx` 母版输出，避免 PDF/ZIP 与单独 docx 版式分叉。
5. 新增 `BID_EXPORT_TEMPLATE_PATH` 和 `BID_EXPORT_WATERMARK_TEXT` 配置；前者可指向企业 docx 样式模板，后者可写入水印文字。
6. 新增 `test_docx_exporter.py`，覆盖 docx 母版 XML、ZIP 内 docx 和 PDF 转换前 docx 源复用。

### 检查结果

已运行：

```bash
python3 -m compileall app
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests/test_docx_exporter.py
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests
docker compose config >/dev/null
./infra/scripts/check.sh
tmp_docker_config="$(mktemp -d)" && DOCKER_CONFIG="$tmp_docker_config" docker compose build ai-service && DOCKER_CONFIG="$tmp_docker_config" docker compose up -d ai-service backend frontend
docker compose exec -T ai-service python -m pytest app/tests
curl http://127.0.0.1:8000/models/health
docker compose ps backend ai-service frontend
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m ruff check app/pipelines/export/docx_exporter.py app/schemas/export.py app/tests/test_docx_exporter.py app/main.py
```

结果：

1. AI 服务 `compileall` 通过。
2. 导出器专项测试通过，覆盖目录域、updateFields、页码域、页眉页脚、水印 XML、Markdown 表格和 ZIP/PDF 复用路径。
3. 挂载当前源码运行完整 AI pytest 通过，16 个测试全部通过。
4. `docker compose config` 通过，确认导出模板环境变量插值合法。
5. `./infra/scripts/check.sh` 通过：验收脚本语法检查、前端生产构建、Go 全量测试、AI compileall、Docker compose config、运行中旧 ai-service 容器内 pytest 均成功；前端仍有既有大 chunk 警告。
6. 使用临时 `DOCKER_CONFIG` 绕过本机缺失 `docker-credential-desktop.exe` 后，ai-service 镜像构建成功，并已重启本地运行栈。
7. 重建后的 ai-service 容器内 `python -m pytest app/tests` 通过，16 个测试全部通过。
8. AI `/models/health` 返回 `mock=true`，`openai_compatible_primary=false`、`dashscope=false`、`deepseek=false`，符合当前环境未配置真实 key/base_url 的状态。
9. `docker compose ps` 显示 backend、ai-service、frontend 均处于 Up 状态。
10. Ruff 针对本轮改动文件检查通过。

### 偏离蓝图

1. 本轮是默认母版和企业模板路径入口，不是复杂企业模板占位符系统；还不能按任意客户 docx 模板自动定位并替换占位内容。
2. 附件清单、电子标特殊目录结构、目录页码在 LibreOffice 转 PDF 后的逐页视觉校验仍需后续补端到端样本文档验证。

## Loop-32 / 企业模板占位符与导出清单 - 2026-06-12

### 本轮目标

1. 继续推进导出保真剩余项，把企业 docx 模板从“样式模板路径”增强为可替换占位符模板。
2. 给 ZIP 导出增加机器可审计 manifest，支撑后续电子标包清单和验收脚本。
3. 让后端导出 API 可透传可选 layout，不破坏现有导出按钮和旧 payload。

### 代码交付

1. `ExportLayoutOptions` 增加 `include_manifest` 和 `context`，支持调用方传入自定义模板变量。
2. `docx_exporter.py` 支持模板标量占位符：`{{ bid_title }}`、`{{ part_title }}`、`{{ generated_at }}`、`{{ template_name }}` 和 layout context 中的自定义 key。
3. `docx_exporter.py` 支持富文本锚点：`{{ZBT_COVER}}`、`{{ZBT_TOC}}`、`{{ZBT_BODY}}`，可把默认封面、目录域和章节正文插入企业模板指定位置。
4. ZIP 导出新增 `manifest.json`，记录 bid title、模板名、生成时间、分册文件、章节数、大小和 sha256。
5. AI 回调结果增加实际 `layout` 和 ZIP `manifest_filename`，Go `CreateExportRequest` 增加可选 `layout` 并透传至 AI 服务。
6. 前端 `createBidExport` API 类型增加可选 `layout` 字段，为后续 UI 导出设置预留入口。
7. `test_docx_exporter.py` 新增模板占位符/正文锚点测试，并把 ZIP manifest 纳入断言。

### 检查结果

已运行：

```bash
python3 -m compileall app
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests
docker compose config >/dev/null
cd backend && GOTOOLCHAIN=local go test ./internal/platform/bid ./internal/api
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m ruff check app/pipelines/export/docx_exporter.py app/schemas/export.py app/tests/test_docx_exporter.py app/main.py
./infra/scripts/check.sh
tmp_docker_config="$(mktemp -d)" && DOCKER_CONFIG="$tmp_docker_config" docker compose build ai-service && DOCKER_CONFIG="$tmp_docker_config" docker compose up -d ai-service backend frontend
docker compose exec -T ai-service python -m pytest app/tests
curl http://127.0.0.1:8000/models/health
docker compose ps backend ai-service frontend
```

结果：

1. AI 服务 `compileall` 通过。
2. 挂载当前源码运行完整 AI pytest 通过，17 个测试全部通过。
3. `docker compose config` 通过。
4. Go bid/api 包测试通过。
5. Ruff 针对本轮改动文件检查通过。
6. `./infra/scripts/check.sh` 通过：验收脚本语法检查、前端生产构建、Go 全量测试、AI compileall、Docker compose config、运行中旧 ai-service 容器内 pytest 均成功；前端仍有既有大 chunk 警告。
7. 使用临时 `DOCKER_CONFIG` 绕过本机缺失 `docker-credential-desktop.exe` 后，ai-service 镜像构建成功，并已重启本地运行栈。
8. 重建后的 ai-service 容器内 `python -m pytest app/tests` 通过，17 个测试全部通过。
9. AI `/models/health` 返回 `mock=true`，`openai_compatible_primary=false`、`dashscope=false`、`deepseek=false`，符合当前环境未配置真实 key/base_url 的状态。
10. `docker compose ps` 显示 backend、ai-service、frontend 均处于 Up 状态。

### 偏离蓝图

1. 本轮支持的是 docx 模板文本占位符和三类富文本锚点，仍不是完整 docxtpl/Jinja 条件循环模板系统。
2. ZIP manifest 已覆盖生成物清单，但尚未把外部附件、工程量清单、投标软件专用目录结构纳入打包。

## Loop-33 / 导出剩余缺口收敛 - 2026-06-12

### 本轮目标

1. 补齐上一轮列出的导出侧剩余缺口：docxtpl/Jinja 条件循环模板、附件/工程量清单打包、电子标目录结构、PDF 输出校验。
2. 保持旧导出 payload 兼容；所有新增能力均为可选字段或默认增强。
3. 用可重复测试覆盖模板循环、附件清单、电子标目录和 PDF 非空校验。

### 代码交付

1. `ExportLayoutOptions` 增加 `render_body`、`validate_pdf`、`e_bidding_structure` 和任意类型 `context`；`ExportAttachment` 支持 `content_base64`、`local_path`、`object_key`、`zip_path`。
2. 企业模板改用 `docxtpl.DocxTemplate` 渲染，支持 `chapters` 循环、自定义 `layout.context` 和 `render_body=false` 的纯模板正文模式。
3. ZIP 导出支持全局 `attachments`、`boq_files` 和分册级 `part.attachments`，默认按 `01_投标文件/`、`02_附件/`、`03_工程量清单/` 结构打包。
4. `manifest.json` 扩展记录电子标结构、全局附件、分册附件、工程量清单、文件大小和 sha256。
5. PDF 导出后使用 PyMuPDF 校验：文件可打开、页数大于 0、有可抽取文本、首屏渲染非空。
6. AI 回调结果增加 `pdf_validation`；Go `CreateExportRequest` 和前端 `createBidExport` API 类型支持 `attachments`、`boq_files` 透传。
7. `test_docx_exporter.py` 增加 Jinja 循环、附件/工程量清单/电子标目录、PDF 校验通过和空白 PDF 拒绝测试。

### 检查结果

已运行：

```bash
python3 -m compileall app
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests/test_docx_exporter.py
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m pytest app/tests
docker run --rm -v /mnt/c/users/wsfwk/downloads/love/ai-service/app:/app/app love-ai-service python -m ruff check app/pipelines/export/docx_exporter.py app/schemas/export.py app/tests/test_docx_exporter.py app/main.py
docker compose config >/dev/null
cd backend && GOTOOLCHAIN=local go test ./internal/platform/bid ./internal/api
cd frontend && pnpm build
./infra/scripts/check.sh
tmp_docker_config="$(mktemp -d)" && DOCKER_CONFIG="$tmp_docker_config" docker compose build ai-service && DOCKER_CONFIG="$tmp_docker_config" docker compose up -d ai-service backend frontend
docker compose exec -T ai-service python -m pytest app/tests
curl http://127.0.0.1:8000/models/health
docker compose ps backend ai-service frontend
```

结果：

1. AI 服务 `compileall` 通过。
2. 导出器专项测试通过，7 个测试全部通过。
3. 挂载当前源码运行完整 AI pytest 通过，20 个测试全部通过。
4. Ruff 针对本轮改动文件检查通过。
5. `docker compose config` 通过。
6. Go bid/api 包测试通过。
7. 前端生产构建通过，仍有既有大 chunk 警告。
8. `./infra/scripts/check.sh` 通过：验收脚本语法检查、前端生产构建、Go 全量测试、AI compileall、Docker compose config、运行中旧 ai-service 容器内 pytest 均成功；前端仍有既有大 chunk 警告。
9. 使用临时 `DOCKER_CONFIG` 绕过本机缺失 `docker-credential-desktop.exe` 后，ai-service 镜像构建成功，并已重启本地运行栈。
10. 重建后的 ai-service 容器内 `python -m pytest app/tests` 通过，20 个测试全部通过。
11. AI `/models/health` 返回 `mock=true`，`openai_compatible_primary=false`、`dashscope=false`、`deepseek=false`，符合当前环境未配置真实 key/base_url 的状态。
12. `docker compose ps` 显示 backend、ai-service、frontend 均处于 Up 状态。

### 偏离蓝图

1. 电子标目录结构已具备标准目录和 manifest，但仍不是对接某个具体省市投标软件的专用二进制包格式。
2. PDF 校验已覆盖文本层和首屏非空，不等同于人工逐页版式审阅；后续若有真实黄金样本，可继续固化像素级对比。

## Loop-34 / 模型网关额度与运行期账本 - 2026-06-17

### 本轮目标

1. 收敛审查报告中 `ModelRouter.log_call()` / `enforce_quota()` 仍为空壳的缺口。
2. 在不改变 Go 侧持久化审计职责的前提下，补齐 Python AI 服务运行期租户预算判断。
3. 让 `quotas.on_exceed=downgrade_then_block` 成为可执行策略，而不是配置占位。

### 代码交付

1. `ModelRouter` 新增运行期 `_tenant_usage` 和 `_call_log`，`log_call()` 会归一化 `estimated_cost`、按租户累计用量，并返回 `quota_status` 快照。
2. `quota_status()` 支持 `default_tenant_monthly_budget` 和 `per_tenant_monthly_budget`，非法、负数、非有限成本统一按 0 处理。
3. Provider-backed 路由解析前执行额度判断；预算耗尽后优先降级到 `mock/local` 零外部模型成本 Provider，没有可降级候选时明确拒绝解析。
4. `MODEL_GATEWAY.md` 更新为当前职责边界：Python 侧维护运行期账本和额度快照，Go 侧继续负责 `ai_call_logs` 持久化审计与计价落库。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_model_router.py -q -s
cd ai-service && .venv/bin/ruff check app/gateway/model_router.py app/tests/test_model_router.py
cd ai-service && .venv/bin/python -m compileall -q app/gateway/model_router.py app/tests/test_model_router.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
git diff --check
```

结果：

1. 模型网关专项测试 45 条全部通过。
2. AI 服务完整测试 206 条全部通过。
3. Ruff、compileall 和 `git diff --check` 均通过。
4. Go 后端全量测试通过。

### 偏离蓝图

1. Python 侧账本仍是进程内存级，用于路由和运行期快照；跨进程、跨重启、月度重置和账单聚合仍以 Go/PostgreSQL `ai_call_logs` 为准。
2. 当前降级策略只区分 `mock/local` 与真实外部 Provider，尚未实现按模型单价自动选择更便宜的真实 Provider。

## Loop-35 / OCR Provider 异步轮询与容器配置补齐 - 2026-06-17

### 本轮目标

1. 补齐 OCR Provider 清单中“异步轮询”仍未落地的缺口。
2. 确保 Docker 运行态能真正拿到通用 OCR、MinerU 和 PaddleOCR 的 endpoint/key/poll 配置。
3. 保持同步 OCR Provider 兼容，并继续避免泄露 OCR 原始响应体或密钥。

### 代码交付

1. `document_parser.py` 支持 OCR 初始响应为 HTTP 202 或 `pending/running/processing` 时进入轮询流程。
2. 轮询地址优先使用响应中的 `status_url` / `poll_url` / `result_url`，其次使用 `OCR_POLL_ENDPOINT`、`MINERU_POLL_ENDPOINT`、`PADDLEOCR_POLL_ENDPOINT`，最后回退到 `endpoint/{task_id}`。
3. 轮询结果继续归一为 `markdown`、`pages`、`blocks`、`layout_blocks`、`table_blocks`，同时把 `async_task_id` 和 `async_attempts` 写入安全的 `provider_metadata`。
4. 修复 OCR 同时返回 markdown 和 pages 时只保留 markdown 的问题，现在会合并页级识别正文，避免丢失正文。
5. `.env.example` 和 `docker-compose.yml` 补齐 `OCR_API_KEY`、通用轮询参数、MinerU 专属配置、PaddleOCR 专属配置和响应大小限制。
6. README、AI_PIPELINE 和 AI_IMPLEMENTATION_CHECKLIST 更新 OCR 异步轮询与配置说明。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py -q -s
cd ai-service && .venv/bin/ruff check app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
cd ai-service && .venv/bin/python -m compileall -q app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
docker compose config >/dev/null
git diff --check
```

结果：

1. 文档解析专项测试 42 条全部通过。
2. AI 服务完整测试 209 条全部通过。
3. Ruff 针对本轮 Python 改动文件检查通过。
4. Python compileall 通过。
5. Go 后端全量测试通过。
6. Docker compose 配置检查通过。
7. `git diff --check` 通过。

### 偏离蓝图

1. 本轮实现 Provider-agnostic HTTP 异步轮询；未内置某个厂商 SDK，也不把 MinerU/PaddleOCR 变成硬依赖。
2. 轮询状态仍由 Python 进程内完成；若后续需要处理超长 OCR 任务，应把 OCR 任务拆成独立 ai_task 子任务并持久化进度。

## Loop-36 / 生成覆盖与来源引用离线评测 - 2026-06-17

### 本轮目标

1. 补齐清单中“生成评测：每个 requirement_item 是否被章节覆盖、source_ref 是否可解析”的可执行入口。
2. 复用现有 AutoRFP 式 `Requirement -> Coverage -> Source` 数据结构，不引入数据库依赖。
3. 为后续真实样本生成验收提供可放入 CI 的 JSON 检查器。

### 代码交付

1. 新增 `ai-service/app/evaluation/generation_coverage_eval.py`。
2. 输入 JSON 支持 `requirements` / `requirement_items`、`chapters[].requirement_coverage`、`model_metadata.self_check.requirement_coverage`、章节 `source_refs` 和 `knowledge_chunks`。
3. 输出 mandatory requirement 覆盖率、已覆盖 mandatory 数、source_ref 总数、可解析 source_ref 数、source_ref 解析率和逐项 checks。
4. 评测规则要求强制项覆盖率满足阈值、已覆盖项必须携带来源、source_refs 必须能通过 `chunk_id/document_id` 或 `resolved=true` 解析。
5. 新增 `test_generation_coverage_eval.py` 覆盖完整通过样例，以及强制项未覆盖和来源未解析的失败样例。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 与 `SAMPLE_DOCS_EVALUATION.md` 更新生成覆盖评测入口说明。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_coverage_eval.py -q -s
cd ai-service && .venv/bin/ruff check app/evaluation/generation_coverage_eval.py app/tests/test_generation_coverage_eval.py
cd ai-service && .venv/bin/python -m compileall -q app/evaluation/generation_coverage_eval.py app/tests/test_generation_coverage_eval.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
git diff --check
```

结果：

1. 新增生成覆盖评测测试 2 条全部通过。
2. AI 服务完整测试 211 条全部通过。
3. Ruff 针对新增评测器和测试文件检查通过。
4. Python compileall 通过。
5. Go 后端全量测试通过。
6. `git diff --check` 通过。

### 偏离蓝图

1. 本轮先提供离线 JSON 评测器；尚未把运行态章节生成结果自动导出为 `<生成覆盖样本>.json`。
2. 评测器验证来源是否能解析到给定 `knowledge_chunks`，不直接查询 PostgreSQL；运行态解析仍由 Go 回调链路负责。

## Loop-37 / 运行态生成覆盖样本导出 - 2026-06-17

### 本轮目标

1. 将 Loop-36 的离线评测输入从手工 JSON 推进到运行态后端导出。
2. 从真实租户数据拼装 AutoRFP 式 `Requirement -> Coverage -> Source` 契约。
3. 给前端共享 API client 暴露类型化读取函数，避免后续页面手写路径。

### 代码交付

1. `backend/internal/platform/bid/store.go` 新增 `GenerationCoverageSpec`、`GenerationCoverageRequirement`、`GenerationCoverageChapter`、`GenerationCoverageKnowledgeChunk` DTO。
2. 新增 `Store.GenerationCoverageSpec()`，在 RLS 事务内读取 `bid_requirement_items`、最新 `bid_chapter_versions.model_metadata`、章节 `source_refs` 和已解析 `knowledge_references -> knowledge_chunks`。
3. 新增 `GET /bids/:id/generation-coverage`，按 bid read 权限返回可直接交给 `generation_coverage_eval.py` 的 JSON。
4. `frontend/src/shared/api/client.ts` 新增 `BidGenerationCoverageSpecDTO` 与 `fetchBidGenerationCoverage()`。
5. 补充后端单元测试，验证 requirement 使用 `external_id` 作为评测 id、章节覆盖矩阵顶层暴露、阈值和来源要求默认值。
6. 补充路由测试，确认新接口为同步只读 bid 路由。
7. `API_SPEC.md`、`AI_IMPLEMENTATION_CHECKLIST.md`、`SAMPLE_DOCS_EVALUATION.md` 更新“运行态导出 -> 离线评测”链路说明。

### 检查结果

已运行：

```bash
cd backend && go test ./...
cd frontend && pnpm build
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_coverage_eval.py -q -s
git diff --check
```

结果：

1. Go 后端全量测试通过，新增 bid store 与 route 测试覆盖本轮接口契约。
2. 前端 TypeScript + Vite 生产构建通过。
3. 生成覆盖离线评测器专项测试 2 条全部通过。
4. `git diff --check` 通过。

### 偏离蓝图

1. 本轮只提供导出 JSON，不在 Go 服务内直接执行 Python 评测器；CI 或验收脚本仍需显式调用 `generation_coverage_eval.py`。
2. 导出 `knowledge_chunks` 只包含已通过 `knowledge_references.chunk_id` 解析的引用；AI 返回但未解析的 source_ref 会留在章节 `source_refs` 中，由离线评测器按未解析项判失败或进入复核。

## Loop-38 / 样本表格块结构验收加固 - 2026-06-17

### 本轮目标

1. 加固 xparse 清单中的“复杂表格保留”验收，避免只用 `min_table_blocks` 数量判断。
2. 对 `docs/ex/工程1` 的 PDF、DOCX、XLSX 样本增加表格来源、行结构和关键单元格检查。
3. 保持评测器离线可运行，不引入外部 OCR 或数据库依赖。

### 代码交付

1. `ai-service/app/evaluation/tender_parse_eval.py` 新增 `documents[].table_blocks` 评测配置。
2. 支持 `required_sources`、`min_total_rows`、`min_blocks_with_rows` 和 `must_contain` 四类表格块检查。
3. `test_tender_parse_eval.py` 新增 xlsx 表格块结构通过样例，以及纯文本样本缺少表格块的失败样例。
4. `docs/sample_docs/golden/工程1.parse.json` 对采购 PDF、响应 DOCX、盖章投标 PDF、固化清单 XLSX 增加表格块结构断言。
5. `AI_IMPLEMENTATION_CHECKLIST.md` 和 `SAMPLE_DOCS_EVALUATION.md` 更新样本评测从 63/63 提升到 87/87，并说明表格块结构验收。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_tender_parse_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd ai-service && .venv/bin/ruff check app/evaluation/tender_parse_eval.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m compileall -q app/evaluation/tender_parse_eval.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
git diff --check
```

结果：

1. `test_tender_parse_eval.py` 4 条测试全部通过。
2. 工程1 真实样本解析评测 87/87 通过。
3. Ruff 针对本轮 Python 文件检查通过。
4. Python compileall 通过。
5. AI 服务完整测试 214 条全部通过。
6. Go 后端全量测试通过。
7. `git diff --check` 通过。

### 偏离蓝图

1. 本轮强化的是离线验收门槛，不改变解析算法本身。
2. 表格块仍以 `rows` 和来源 metadata 为主，尚未要求 PDF 单元格级 bbox 全覆盖；bbox 覆盖应跟 OCR/版面 Provider 样本一起继续推进。

## Loop-39 / table_blocks md_table 保真落地 - 2026-06-17

### 本轮目标

1. 将 xparse 清单中的“复杂表格保留 `md_table`”从文档要求推进到实际解析产物。
2. 让 PDF、DOCX、XLSX、PPTX 和 OCR 归一化表格块都具备可直接给模型使用的 Markdown 表格文本。
3. 把 `md_table` 纳入工程1真实样本离线验收。

### 代码交付

1. `document_parser.py` 的 `_table_block()` 对所有带 `rows` 的表格块生成 `md_table`，并保留 Provider 原始 `md_table`。
2. 表格块新增 `column_count`，方便后续评估表头和列宽覆盖。
3. `tender_parse_eval.py` 新增 `require_md_table` 和 `md_table_must_contain` 检查。
4. `test_document_parser.py` 更新 DOCX/XLSX/PPTX 表格块断言，确认 `md_table` 输出。
5. `test_tender_parse_eval.py` 增加 `md_table` 正反验收。
6. `docs/sample_docs/golden/工程1.parse.json` 对四个样本文档启用 `require_md_table` 和 `md_table_must_contain`。
7. `AI_IMPLEMENTATION_CHECKLIST.md` 与 `SAMPLE_DOCS_EVALUATION.md` 更新当前样本验收为 99/99。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd ai-service && .venv/bin/ruff check app/pipelines/parse/document_parser.py app/evaluation/tender_parse_eval.py app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m compileall -q app/pipelines/parse/document_parser.py app/evaluation/tender_parse_eval.py app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
git diff --check
```

结果：

1. 文档解析与招标解析评测专项测试 47 条全部通过。
2. 工程1 真实样本解析评测 99/99 通过。
3. Ruff 针对本轮 Python 文件检查通过。
4. Python compileall 通过。
5. AI 服务完整测试 214 条全部通过。
6. Go 后端全量测试通过。
7. `git diff --check` 通过。

### 偏离蓝图

1. `md_table` 目前由行数据生成或保留 Provider 原文，不额外推断合并单元格语义。
2. 单元格级 bbox、跨页表格合并和复杂表头层级仍需跟版面/OCR Provider 样本继续推进。

## Loop-40 / PDF 表格 bbox 版面证据验收 - 2026-06-17

### 本轮目标

1. 继续推进 OCR/版面证据清单，将 PDF 表格的 table-level bbox 从底层能力提升为解析产物。
2. 让工程1真实样本能够约束 PDF 表格 bbox 数量，避免表格版面证据回退。
3. 保持 DOCX/XLSX 等无页面坐标来源的表格不被强行要求 bbox。

### 代码交付

1. `document_parser.py` 在 PyMuPDF `page.find_tables()` 路径中读取 `table.bbox`，归一化为四元数并写入 `table_blocks[].bbox`。
2. `_table_block()` 统一校验 bbox 有效性，过滤空面积或非法 bbox。
3. `tender_parse_eval.py` 新增 `table_blocks.min_blocks_with_bbox` 检查。
4. `test_document_parser.py` 新增 fake PyMuPDF table 测试，确认 bbox 保留和四舍五入。
5. `test_tender_parse_eval.py` 新增 bbox 缺失失败检查。
6. `docs/sample_docs/golden/工程1.parse.json` 对采购 PDF 和盖章投标 PDF 增加 `min_blocks_with_bbox`。
7. `AI_IMPLEMENTATION_CHECKLIST.md` 与 `SAMPLE_DOCS_EVALUATION.md` 更新当前样本验收为 101/101。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd ai-service && .venv/bin/ruff check app/pipelines/parse/document_parser.py app/evaluation/tender_parse_eval.py app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m compileall -q app/pipelines/parse/document_parser.py app/evaluation/tender_parse_eval.py app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
git diff --check
```

结果：

1. 文档解析与招标解析评测专项测试 48 条全部通过。
2. 工程1 真实样本解析评测 101/101 通过。
3. Ruff 针对本轮 Python 文件检查通过。
4. Python compileall 通过。
5. AI 服务完整测试 215 条全部通过。
6. Go 后端全量测试通过。
7. `git diff --check` 通过。

### 偏离蓝图

1. 本轮落地 table-level bbox；单元格级 bbox 尚未实现。
2. 启发式 PDF 表格没有可靠版面区域，不强行生成 bbox。

## Loop-41 / PDF 表格 cell_bboxes 单元格级版面证据 - 2026-06-17

### 本轮目标

1. 继续推进 OCR/版面证据清单，将 PyMuPDF 表格的单元格坐标保存到解析产物。
2. 让工程1真实样本能够约束 PDF 表格单元格 bbox 数量，避免来源引用只停留在整表级区域。
3. 保持坐标语义保守：无效、空面积或不可获得的单元格 bbox 不伪造。

### 代码交付

1. `document_parser.py` 在 PyMuPDF `page.find_tables()` 路径中读取 `table.cells` 和 `table.col_count`，按行列还原到 `table_blocks[].cell_bboxes`。
2. `_table_block()` 新增 `cell_bboxes` 归一化和 `cell_bbox_count`，过滤无效 bbox，并保留空单元格的 `None` 占位。
3. `tender_parse_eval.py` 新增 `table_blocks.min_cells_with_bbox` 检查。
4. `test_document_parser.py` 新增单元格 bbox 有效性、空面积过滤和 PDF 表格 cell bbox 保留断言。
5. `test_tender_parse_eval.py` 新增单元格 bbox 缺失失败检查。
6. `docs/sample_docs/golden/工程1.parse.json` 对采购 PDF 和盖章投标 PDF 增加 `min_cells_with_bbox`。
7. `AI_IMPLEMENTATION_CHECKLIST.md` 与 `SAMPLE_DOCS_EVALUATION.md` 更新当前样本验收为 103/103。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd ai-service && .venv/bin/ruff check app/pipelines/parse/document_parser.py app/evaluation/tender_parse_eval.py app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m compileall -q app/pipelines/parse/document_parser.py app/evaluation/tender_parse_eval.py app/tests/test_document_parser.py app/tests/test_tender_parse_eval.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
git diff --check
```

结果：

1. 文档解析与招标解析评测专项测试 48 条全部通过。
2. 工程1 真实样本解析评测 103/103 通过。
3. Ruff 针对本轮 Python 文件检查通过。
4. Python compileall 通过。
5. AI 服务完整测试 215 条全部通过。
6. Go 后端全量测试通过。
7. `git diff --check` 通过。

### 偏离蓝图

1. 本轮只固化 PyMuPDF 可直接提供的单元格 bbox；扫描件 OCR 的单元格 bbox 仍依赖 MinerU/PaddleOCR 等外部 Provider 样本继续验收。
2. 合并单元格、跨页表格延续和复杂表头层级暂不推断，避免把不可靠坐标写成确定事实。

## Loop-42 / OCR 页级表格提升与 cells 归一化 - 2026-06-17

### 本轮目标

1. 修补 OCR Provider 响应归一化缺口：Provider 只返回 `pages[].tables` 时，也要进入文档级 `table_blocks`。
2. 支持 MinerU/PaddleOCR 常见的 `cells` 表格结构，将单元格文本还原为 `rows`，并保留可用 `cell_bboxes`。
3. 只有表格、没有纯文本的 OCR 结果也要把 `md_table` 合并进 chunk 文本，避免扫描清单不可检索。

### 代码交付

1. `document_parser.py` 新增 `_normalize_ocr_table()`，统一处理 OCR 表格的页码、表级 bbox、`rows`、`cell_bboxes` 和 `md_table`。
2. `document_parser.py` 新增 `_ocr_rows_from_cells()`、`_ocr_cell_bboxes_from_cells()` 和单元格坐标/文本提取辅助函数。
3. `_normalize_ocr_result()` 在顶层 `tables/table_blocks` 为空时，从已归一化的 `pages[].tables` 提升文档级表格块。
4. `_normalize_ocr_result()` 将 OCR 表格 `md_table` 合并进 `text`，并把表格型结果视为 `done`。
5. `test_document_parser.py` 覆盖页级 OCR 表格提升、PaddleOCR `cells` 转 `rows/cell_bboxes`、表格文本进入 chunk。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 更新 OCR 表格归一化和扫描清单可检索要求。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py -q -s
cd ai-service && .venv/bin/ruff check app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
cd ai-service && .venv/bin/python -m compileall -q app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd backend && go test ./...
git diff --check
```

结果：

1. 文档解析专项测试 45 条全部通过。
2. Ruff 针对本轮 Python 文件检查通过。
3. Python compileall 通过。
4. AI 服务完整测试 216 条全部通过。
5. 工程1 真实样本解析评测 103/103 通过。
6. Go 后端全量测试通过。
7. `git diff --check` 通过。

### 偏离蓝图

1. 本轮固化的是 OCR 响应归一化和表格可检索性；仍未接入真实外部 MinerU/PaddleOCR 服务跑端到端样本。
2. `cells` 只按常见行列索引或顺序列数还原，不推断合并单元格语义和跨页表格延续。

## Loop-43 / OCR 顶层与页级表格合并去重 - 2026-06-17

### 本轮目标

1. 修补真实 OCR Provider 常见响应形态：顶层 `tables/table_blocks` 与 `pages[].tables` 同时存在时，不能只保留顶层表格。
2. 保留页级新增表格，同时对顶层和页级重复表格做稳定去重。
3. 继续保证 OCR 表格进入 chunk 文本，支撑后续 6 模块解析和 RAG 检索。

### 代码交付

1. `document_parser.py` 将顶层 OCR 表格和页级 OCR 表格统一合并为文档级 `table_blocks`。
2. 新增 `_dedupe_ocr_tables()` 和 `_ocr_table_identity()`，按页码、表序号、行内容、`md_table` 与 bbox 生成稳定身份，避免重复表格污染后续解析。
3. `test_document_parser.py` 新增“顶层表 + 页级表 + 重复表”样例，验证最终只保留两个有效表格并进入 chunk 文本。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录 OCR 顶层/页级表格合并去重要求。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py -q -s
cd ai-service && .venv/bin/ruff check app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
cd ai-service && .venv/bin/python -m compileall -q app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd backend && go test ./...
git diff --check
```

结果：

1. 文档解析专项测试 46 条全部通过。
2. Ruff 针对本轮 Python 文件检查通过。
3. Python compileall 通过。
4. AI 服务完整测试 217 条全部通过。
5. 工程1 真实样本解析评测 103/103 通过。
6. Go 后端全量测试通过。
7. `git diff --check` 通过。

### 偏离蓝图

1. 本轮仍属于 OCR 响应归一化加固，没有连接真实外部 OCR 服务做端到端样本。
2. 去重身份保守使用结构化内容和 bbox，不尝试语义合并相似表格。

## Loop-44 / requirement_coverage 回写响应矩阵 - 2026-06-17

### 本轮目标

1. 补齐 AutoRFP 式 `Requirement -> Coverage -> Source` 运行态闭环。
2. 章节生成、整标逐章生成和章节 AI 自检返回 `self_check.requirement_coverage` 后，要求表不能只在离线导出中看到覆盖结果。
3. 保持招标原文来源和响应侧来源语义分离，避免覆盖解析阶段的 `source_ref`。

### 代码交付

1. `applyChapterGeneration()` 在同一事务内调用 `syncRequirementCoverageStatuses()`。
2. 新增 `syncRequirementCoverageStatuses()`，按 `requirement_id/id/external_id/reference_id/referenceId` 匹配 `bid_requirement_items.external_id` 或数据库 id。
3. 覆盖状态映射：`covered/satisfied/pass` 且不需复核写入 `covered`；`needs_review`、`satisfied=false`、`failed/not_covered/unsatisfied` 写入 `needs_review`。
4. 响应侧证据写入 `bid_requirement_items.metadata.latest_coverage`，包含 `requirement_id`、`chapter_id`、`status`、`needs_review`、`evidence` 和 `source_refs`；不覆盖招标解析阶段的 `source_ref`。
5. `store_test.go` 新增覆盖状态映射和 AutoRFP reference 字段兼容测试。
6. `AI_IMPLEMENTATION_CHECKLIST.md`、`AI_PIPELINE.md`、`API_SPEC.md` 更新运行态回写说明。

### 检查结果

已运行：

```bash
gofmt -w backend/internal/platform/bid/store.go backend/internal/platform/bid/store_test.go
cd backend && go test ./internal/platform/bid
cd backend && go test ./...
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
git diff --check
```

结果：

1. Go bid 包测试通过。
2. Go 后端全量测试通过。
3. AI 服务完整测试 217 条全部通过。
4. 工程1 真实样本解析评测 103/103 通过。
5. `git diff --check` 通过。

### 偏离蓝图

1. 本轮回写要求项覆盖状态，但不新增人工编辑覆盖状态接口；人工确认/调整仍沿用后续产品流程。
2. 一个要求可能被多章覆盖时，当前以最新生成回调为准写入 `metadata.latest_coverage`；完整历史仍保留在各章节版本 `model_metadata.requirement_coverage` 中。

## Loop-45 / 响应证据进入响应要点表 - 2026-06-17

### 本轮目标

1. 补齐 AutoRFP 式 `Requirement -> Coverage -> Source` 在文件解读页的用户可见闭环。
2. `bid_requirement_items.metadata.latest_coverage` 已由后端回写后，前端不能只显示覆盖状态，必须显示响应证据和来源数量。
3. 继续保持业务口径，不在页面中暴露模型、token、schema、provider 等技术字段。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 从要求项 `metadata.latest_coverage` 提取响应证据和 `source_refs` 数量。
2. “响应要点”表新增“响应证据”列，展示证据摘要和“来源 N 处”；已覆盖但无证据的项标记为“待补证据”。
3. `frontend/src/index.css` 新增响应证据单元格样式，使用固定栅格、单行省略和 Tooltip，防止长证据撑开表格。
4. `AI_IMPLEMENTATION_CHECKLIST.md`、`AI_PIPELINE.md`、`API_SPEC.md` 同步更新当前状态。

### 检查结果

已运行：

```bash
pnpm --dir frontend build
git diff --check
cd backend && go test ./...
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
```

结果：

1. 前端 TypeScript 构建和 Vite 打包通过。
2. `git diff --check` 通过。
3. Go 后端全量测试通过。
4. 工程1 真实样本解析评测 103/103 通过。
5. AI 服务完整测试 217 条全部通过。

### 偏离蓝图

1. 本轮只展示最新一次覆盖证据；多章节、多轮生成的覆盖历史仍保留在章节版本 `model_metadata.requirement_coverage`，暂未做历史时间线。
2. 本轮不新增人工编辑覆盖状态接口，也不新增响应矩阵导出。

## Loop-46 / 评审响应矩阵 CSV 导出 - 2026-06-17

### 本轮目标

1. 补齐 AutoRFP 式响应矩阵的可交付输出，不能只停留在页面展示和 JSON API。
2. 矩阵导出必须使用业务字段，包含覆盖状态、响应证据、响应来源和招标原文来源。
3. 导出应走只读权限，不进入 AI 导出队列，也不写文件资产，避免把确定性数据导出变成异步任务。

### 代码交付

1. `backend/internal/api/routes.go` 新增 `GET /bids/:id/requirements/export`，返回带 UTF-8 BOM 的 CSV 文件。
2. CSV 列包含：来源分组、要求、是否必须响应、优先级、分值、覆盖状态、复核状态、期望响应、响应证据、响应来源数量、响应来源、招标原文来源和更新时间。
3. `routes_test.go` 新增路由元数据测试和 CSV 内容测试，覆盖只读权限、非异步标记、中文 BOM、证据、来源数量和页码摘要。
4. `frontend/src/shared/api/client.ts` 新增 Blob 下载方法，并从 `Content-Disposition` 解析中文文件名。
5. `frontend/src/features/bid/index.tsx` 在“响应要点”页签新增“导出矩阵”按钮。
6. `AI_IMPLEMENTATION_CHECKLIST.md`、`AI_PIPELINE.md`、`API_SPEC.md` 同步更新当前状态。

### 检查结果

已运行：

```bash
gofmt -w backend/internal/api/routes.go backend/internal/api/routes_test.go
cd backend && go test ./internal/api
pnpm --dir frontend build
git diff --check
cd backend && go test ./...
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
```

结果：

1. Go API 专项测试通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过。
4. Go 后端全量测试通过。
5. AI 服务完整测试 217 条全部通过。
6. 工程1 真实样本解析评测 103/103 通过。

### 偏离蓝图

1. 本轮导出 CSV，不生成 xlsx；CSV 使用 UTF-8 BOM 保证主流表格软件可直接打开中文。
2. 本轮导出当前快照，不包含多轮覆盖历史；覆盖历史仍保留在章节版本元数据中。

## Loop-47 / 人工调整响应覆盖状态 - 2026-06-17

### 本轮目标

1. 补齐 AutoRFP 式响应矩阵的人工闭环：模型生成后，业务人员必须能修正单条要求的覆盖状态。
2. 人工补充的响应证据必须进入同一套 `latest_coverage` 展示与导出链路，不覆盖招标原文来源。
3. 写操作必须走 `bid` full 权限，读取和导出仍保持 read 权限。

### 代码交付

1. `backend/internal/platform/bid/store.go` 新增 `UpdateRequirementCoverageRequest` 和 `UpdateRequirementCoverage()`，按租户、标书和要求项 id/external_id 定位单条 `bid_requirement_items`。
2. 人工调整会更新 `coverage_status`、`needs_review`、`metadata.latest_coverage` 和 `metadata.manual_coverage`；人工元数据包含状态、证据、来源引用、更新人和更新时间，不覆盖 `source_ref`。
3. `backend/internal/api/routes.go` 新增 `PATCH /bids/:id/requirements/:requirementId`，路由元数据声明为 `bid` full 权限、同步接口，并加入自定义路由白名单。
4. `frontend/src/shared/api/client.ts` 新增 `updateBidRequirementCoverage()`。
5. `frontend/src/features/bid/index.tsx` 在“响应要点”表格为可更新要求项提供覆盖状态下拉和“补证据”入口；保存成功后刷新要求矩阵。
6. `frontend/src/index.css` 固定状态下拉与证据列布局，避免长证据、来源标签和操作按钮挤压错行。
7. `AI_IMPLEMENTATION_CHECKLIST.md`、`AI_PIPELINE.md`、`API_SPEC.md` 同步更新人工调整接口和当前状态。

### 检查结果

已运行：

```bash
gofmt -w internal/platform/bid/store.go internal/platform/bid/store_test.go internal/api/routes.go internal/api/routes_test.go
cd backend && go test ./internal/platform/bid ./internal/api
pnpm --dir frontend build
cd backend && go test ./...
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
git diff --check
```

结果：

1. Go bid/API 专项测试通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. Go 后端全量测试通过。
4. AI 服务完整测试 217 条全部通过。
5. 工程1 真实样本解析评测 103/103 通过。
6. `git diff --check` 通过。

### 偏离蓝图

1. 本轮补齐人工调整当前快照，不新增多轮覆盖历史时间线；覆盖历史仍保留在章节版本 `model_metadata.requirement_coverage` 和人工 `metadata.manual_coverage` 中。
2. 本轮前端只支持补充文本证据；附件级证据上传、证据引用精确选区和批量调整仍属后续增强。

## Loop-48 / 行业 MCP 与 Skills 雷达固化 - 2026-06-17

### 本轮目标

1. 把全网和 GitHub 上招投标、采购、RFP 方向的 MCP / Skills 调研沉淀为工程内可追踪文档。
2. 区分可接入数据源、可借鉴工具边界、可借鉴 Skill 方法论和暂不采用项，避免把普通仓库误当成生产能力。
3. 明确外部 MCP 接入的租户授权、脱敏、审计、成本和只读边界。

### 代码交付

1. 新增 `docs/blueprint/EXTERNAL_MCP_SKILL_RADAR.md`。
2. 雷达文档按“可优先评估接入”“可借鉴工具边界”“可借鉴 Skill 方法论”“全球采购数据 MCP 观察池”分层记录外部项目。
3. 明确智标通接入清单：P0 只读外部 MCP 工具网关、P1 业务入口、P2 Skill 方法论内化。
4. 明确不做事项：不把外部 RFP MCP 作为生产解析/生成主路径，不默认外发文件原文，不绕过 RBAC/RLS/审计/成本核算。
5. `AI_IMPLEMENTATION_CHECKLIST.md` 增加雷达文档指针。
6. `AI_PIPELINE.md` 增加“MCP 负责确定性外部数据，Skill 负责交付物方法论”的边界说明。

### 检查结果

已运行：

```bash
git diff --check
```

结果：

1. `git diff --check` 通过。

### 偏离蓝图

1. 本轮只固化调研和接入清单，不实现 MCP client。
2. 真实外部 MCP 接入必须先补租户配置、工具白名单、脱敏策略和审计表后再进入业务界面。

## Loop-49 / 响应覆盖历史时间线 - 2026-06-17

### 本轮目标

1. 把 AutoRFP 式要求矩阵从“当前覆盖快照”推进到“模型/人工多轮覆盖历史可追溯”。
2. 覆盖历史必须落库、启用 RLS，并保留响应证据、响应来源、章节和操作人信息。
3. 前端只展示业务口径的响应历史，不暴露模型、token、provider、schema 等技术字段。

### 代码交付

1. 新增 `backend/internal/db/migrations/00033_bid_requirement_coverage_events.sql`，创建 `bid_requirement_coverage_events` RLS 表，记录模型回写和人工调整事件。
2. `backend/internal/platform/bid/store.go` 在章节生成覆盖回写和人工调整覆盖状态时追加覆盖历史，并新增 `ListRequirementCoverageEvents()` 查询最近 50 条历史。
3. `backend/internal/api/routes.go` 新增 `GET /bids/:id/requirements/:requirementId/history`，只读权限、同步接口，并加入自定义路由白名单。
4. `frontend/src/shared/api/client.ts` 新增覆盖历史 DTO 和读取接口，路径参数使用 `encodeURIComponent` 防止外部要求编号包含特殊字符。
5. `frontend/src/features/bid/index.tsx` 在“响应要点”表新增“历史”入口，弹窗按时间线展示覆盖状态、人工/自动来源、响应证据和响应来源摘要；打开新要求项时清理旧请求状态，避免历史缓存错显。
6. `frontend/src/index.css` 增加历史证据换行样式。
7. `API_SPEC.md`、`DATABASE_SCHEMA.md`、`AI_PIPELINE.md`、`AI_IMPLEMENTATION_CHECKLIST.md` 同步更新当前链路。

### 检查结果

已运行：

```bash
gofmt -w backend/internal/platform/bid/store.go backend/internal/platform/bid/store_test.go backend/internal/api/routes.go backend/internal/api/routes_test.go
cd backend && go test ./internal/platform/bid ./internal/api
pnpm --dir frontend build
git diff --check
cd backend && go test ./...
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
```

结果：

1. Go bid/API 专项测试通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过。
4. Go 后端全量测试通过。
5. AI 服务完整测试 217 条全部通过。
6. 工程1 真实样本解析评测 103/103 通过。

### 偏离蓝图

1. 本轮补齐单条要求的覆盖历史时间线；暂未提供跨要求批量审阅、按操作者/章节筛选和历史 xlsx 导出。
2. 历史项保存响应来源摘要和原始 `source_refs`，但前端尚未支持点击跳转到原文精确选区或知识库 chunk。

## Loop-50 / 外部 MCP 工具网关后端基础 - 2026-06-17

### 本轮目标

1. 把行业 MCP / Skills 雷达中的 P0 “只读外部 MCP 工具网关”从文档清单推进到后端可运行基础设施。
2. 外部工具必须租户级显式启用、强制工具白名单、记录摘要审计和预算阻断，不能默认外发客户文件原文。
3. 首轮只支持受控 `streamable_http` JSON-RPC `tools/call`，不引入 stdio、本地命令执行或黑盒解析/生成主链路。

### 代码交付

1. 新增 `backend/internal/db/migrations/00034_external_tool_gateway.sql`，创建 `external_tool_configs` 和 `external_tool_audit_logs` 两张 RLS 表。
2. 新增 `backend/internal/platform/externaltool/store.go`，提供外部工具配置、白名单校验、调用预算校验、JSON-RPC `tools/call` 调用、环境变量 token 注入、请求摘要/响应摘要和审计记录。
3. 新增 `backend/internal/platform/externaltool/store_test.go`，覆盖配置归一化、请求摘要不泄露原文、MCP `tools/call` envelope、Authorization header 和超时。
4. `backend/internal/api/routes.go` 新增 `GET /external-tools`、`PUT /external-tools/:providerKey`、`POST /external-tools/:providerKey/invoke`、`GET /external-tools/audit`，配置和调用走 team full，审计读取走 team read。
5. `backend/cmd/server/main.go` 注入 `externaltool.Store`，服务启动后可直接使用新网关。
6. `API_SPEC.md`、`DATABASE_SCHEMA.md`、`AI_PIPELINE.md`、`AI_IMPLEMENTATION_CHECKLIST.md`、`EXTERNAL_MCP_SKILL_RADAR.md` 同步更新当前状态和剩余边界。

### 检查结果

已运行：

```bash
gofmt -w backend/cmd/server/main.go backend/internal/api/routes.go backend/internal/api/routes_test.go backend/internal/platform/externaltool/store.go backend/internal/platform/externaltool/store_test.go
cd backend && go test ./internal/platform/externaltool ./internal/api
cd backend && go test ./...
git diff --check
```

结果：

1. 外部工具 store 单元测试通过。
2. API 路由元数据和既有 API 测试通过。
3. Go 后端全量测试通过。
4. `git diff --check` 通过。

### 偏离蓝图

1. 本轮只完成后端 P0 网关，不提供前端配置页、业务页调用入口和审计页展示。
2. 本轮只支持 `streamable_http`；stdio 工具、本地命令执行和 SSE 长连接暂不开放。
3. 本轮只做通用 JSON-RPC MCP 调用，不内置 handaas 等 Provider 的字段模板和生产凭证验证。

## Loop-51 / 响应矩阵覆盖历史 xlsx 导出 - 2026-06-17

### 本轮目标

1. 补齐 AutoRFP 响应矩阵从 CSV 快照到“当前覆盖 + 覆盖历史”可交付工作簿的缺口。
2. xlsx 导出必须保持业务字段，不暴露 provider、model、token、schema 等技术口径。
3. 继续沿用现有只读导出权限，不引入异步 AI 导出任务。

### 代码交付

1. `backend/internal/platform/bid/store.go` 新增 `ListRequirementCoverageEventsForBid()`，按标书读取最近覆盖历史事件，用于批量导出。
2. `backend/internal/api/routes.go` 扩展 `GET /bids/:id/requirements/export`：默认仍返回 UTF-8 CSV；`?format=xlsx` 返回标准 Office Open XML 工作簿，包含“响应矩阵”和“覆盖历史”两个工作表。
3. xlsx 工作簿由 Go 标准库生成 zip + worksheet XML，不新增后端依赖；矩阵 sheet 复用 CSV 字段，历史 sheet 包含历史来源、覆盖状态、复核状态、响应证据、响应来源和记录时间。
4. `backend/internal/api/routes_test.go` 新增 xlsx zip 内容测试，检查 workbook 双 sheet、矩阵证据和覆盖历史内容。
5. `frontend/src/shared/api/client.ts` 为响应矩阵导出增加 `csv/xlsx` 格式参数。
6. `frontend/src/features/bid/index.tsx` 在“响应要点”工具栏新增“导出历史”按钮，下载含历史工作表的 xlsx。
7. `API_SPEC.md`、`AI_PIPELINE.md`、`AI_IMPLEMENTATION_CHECKLIST.md` 同步更新。

### 检查结果

已运行：

```bash
gofmt -w backend/internal/platform/bid/store.go backend/internal/api/routes.go backend/internal/api/routes_test.go
cd backend && go test ./internal/platform/bid ./internal/api
pnpm --dir frontend build
git diff --check
```

结果：

1. Go bid/API 专项测试通过。
2. xlsx 导出单元测试通过，生成的工作簿包含“响应矩阵”和“覆盖历史”两个工作表。
3. 前端 TypeScript 构建和 Vite 打包通过。
4. `git diff --check` 通过。

### 偏离蓝图

1. 本轮补齐带历史的 xlsx 导出，不提供跨要求批量审阅、批量修改和筛选 UI。
2. xlsx 使用标准库生成基础工作簿，未加样式、冻结窗格、筛选器和列宽优化。
3. 真实外部 MinerU/PaddleOCR 服务端到端样本验证仍需要可用 endpoint/key。

## Loop-52 / OCR Provider 真实 endpoint 验收入口 - 2026-06-17

### 本轮目标

1. 把 MinerU / PaddleOCR 从“代码支持和假 HTTP 单测”推进到可对真实 endpoint/key 重复运行的验收命令。
2. 默认使用工程1真实样本，且必须走 OCR Provider 路径，避免只验证 PDF 文本层。
3. 没有 endpoint 时明确输出 skipped，不伪装为通过。

### 代码交付

1. 新增 `ai-service/app/evaluation/ocr_provider_eval.py`。
2. CLI 默认读取 `docs/ex/工程1/采购文件桥梁检查.pdf`，把第一页渲染为 PNG 后调用现有 `parse_document()`，从而触发 `OCR_PROVIDER=mineru|paddleocr` 的真实 Provider 路径。
3. 验收检查包含：Provider 支持、endpoint 配置、样本存在、OCR status、provider 匹配、识别文本长度、chunk 数、provider_profile.endpoint_env，并支持 `--min-table-blocks` / `--min-layout-blocks`。
4. 新增 `--allow-skip`，用于本地或 CI 未配置真实 endpoint 时不阻断流水线，但结果状态仍为 `skipped`。
5. 新增 `ai-service/app/tests/test_ocr_provider_eval.py`，覆盖 endpoint 缺失 skipped、MinerU 假服务通过、PaddleOCR 缺表格按门槛失败。
6. `SAMPLE_DOCS_EVALUATION.md` 和 `AI_IMPLEMENTATION_CHECKLIST.md` 同步新增 OCR Provider 验收命令和边界。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_ocr_provider_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.ocr_provider_eval --provider mineru --allow-skip
cd ai-service && .venv/bin/python -m app.evaluation.ocr_provider_eval --provider paddleocr --allow-skip
```

结果：

1. OCR Provider 验收 CLI 单元测试 3 条全部通过。
2. 当前本地未配置 `MINERU_HTTP_ENDPOINT`，MinerU 验收输出 `skipped provider=mineru passed=1/2`。
3. 当前本地未配置 `PADDLEOCR_HTTP_ENDPOINT`，PaddleOCR 验收输出 `skipped provider=paddleocr passed=1/2`。

### 偏离蓝图

1. 本轮新增真实 endpoint 验收入口，但当前环境没有 MinerU/PaddleOCR endpoint/key，因此不能宣称真实 Provider 已通过端到端样本。
2. CLI 默认验证第一页 OCR；多页扫描件、复杂表格、多 Provider 对比评分仍需后续扩展。

## Loop-53 / 响应矩阵批量审阅 - 2026-06-17

### 本轮目标

1. 补齐 AutoRFP 式响应矩阵的批量人工审阅入口，避免只能逐条点选覆盖状态。
2. 批量操作必须写入同一套 `latest_coverage`、`manual_coverage` 和覆盖历史，不绕开审计链路。
3. 前端入口只展示业务动作，不暴露 provider、model、schema、token 等技术口径。

### 代码交付

1. `backend/internal/platform/bid/store.go` 新增 `BatchUpdateRequirementCoverage()`，支持最多 100 条要求项批量标记覆盖状态；请求会去重、清理空 ID，任一要求项不存在时整批失败。
2. 单条和批量覆盖更新共用 `updateRequirementCoverageItem()`，统一更新 `bid_requirement_items.coverage_status`、`needs_review`、`metadata.latest_coverage`、`metadata.manual_coverage` 并追加 `bid_requirement_coverage_events`。
3. `backend/internal/api/routes.go` 新增 `PATCH /bids/:id/requirements`，按 bid full 权限执行同步批量审阅，并纳入 routeSpecs 和自定义路由白名单。
4. `frontend/src/shared/api/client.ts` 新增 `batchUpdateBidRequirementCoverage()`。
5. `frontend/src/features/bid/index.tsx` 的“响应要点”表新增多选列、选中数量提示和“批量标记”控件；批量执行期间锁定单条/批量状态控件，避免重复提交。
6. `frontend/src/index.css` 增加批量标记控件固定宽度，减少工具栏换行错位。
7. `API_SPEC.md`、`AI_PIPELINE.md`、`AI_IMPLEMENTATION_CHECKLIST.md` 同步更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
gofmt -w backend/internal/platform/bid/store.go backend/internal/platform/bid/store_test.go backend/internal/api/routes.go backend/internal/api/routes_test.go
cd backend && go test ./internal/platform/bid ./internal/api
pnpm --dir frontend build
cd backend && go test ./...
git diff --check
```

结果：

1. Go bid/API 专项测试通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. Go 后端全量测试通过。
4. `git diff --check` 通过。

### 偏离蓝图

1. 本轮完成批量覆盖状态审阅，不做批量证据选区、批量来源引用编辑和跨条件高级筛选。
2. 批量上限暂定为 100 条，后续如需整标级全选审阅，应改为服务端条件批处理并返回任务进度。

## Loop-54 / 响应矩阵批量补证据 - 2026-06-17

### 本轮目标

1. 补齐 Loop-53 留下的批量证据选区缺口，让评审人员能对多条响应要求一次性补充共同证据。
2. 继续复用 `PATCH /bids/:id/requirements`，确保批量证据进入 `latest_coverage`、`manual_coverage`、覆盖历史和导出链路。
3. 前端仍只展示业务动作，不出现 provider、model、token、schema 等技术口径。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 在“响应要点”工具栏新增“批量补证据”按钮，仅在选中可审阅要求项时可用。
2. 批量证据弹窗支持选择覆盖状态并填写共同响应证据，默认状态按已选项推断；混合状态或未确认项默认进入“已覆盖”。
3. 批量证据保存复用 `batchUpdateBidRequirementCoverage()`，提交选中的要求项、覆盖状态和证据；保存成功后刷新要求矩阵并清空选中项。
4. 批量状态和批量证据共用更新锁，执行期间锁定单条状态、单条补证据和批量控件，避免重复提交。
5. `API_SPEC.md`、`AI_PIPELINE.md`、`AI_IMPLEMENTATION_CHECKLIST.md` 同步更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 TypeScript 构建和 Vite 打包通过。
2. `git diff --check` 通过。

### 偏离蓝图

1. 本轮只支持批量填写共同证据文本，不做批量来源引用结构化编辑。
2. 本轮不新增高级筛选、按条件全选或跨页服务端批处理。

## Loop-55 / 响应矩阵来源编辑 - 2026-06-17

### 本轮目标

1. 把 AutoRFP 的 `Answer -> Source` 思路继续推进到人工审阅入口，补齐单条和批量响应来源编辑。
2. 来源编辑必须复用现有 `source_refs`、覆盖历史和导出链路，不新增旁路字段。
3. 前端只展示业务字段：文件或章节、页码、原文摘录；不暴露 chunk_id、schema、provider、token 等技术口径。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 为响应要点行补充 `coverageSourceRefs`，单条补证据弹窗可预填并编辑已有响应来源。
2. 新增 `RequirementSourceRefsEditor`，支持添加、编辑和删除来源；字段固定为文件或章节、页码、原文摘录。
3. 单条 `updateBidRequirementCoverage()` 和批量 `batchUpdateBidRequirementCoverage()` 调用均可提交 `source_refs`。
4. 批量补证据弹窗同步支持批量编辑共同响应来源；保存成功后仍刷新要求矩阵并清空选中项。
5. `frontend/src/index.css` 为来源编辑器新增响应式网格，桌面紧凑排列，小屏自动换行，避免输入框错位。
6. `API_SPEC.md`、`AI_PIPELINE.md`、`AI_IMPLEMENTATION_CHECKLIST.md` 同步更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 TypeScript 构建和 Vite 打包通过。
2. `git diff --check` 通过。

### 偏离蓝图

1. 本轮支持人工录入结构化响应来源，但不做点击跳转到原文精确选区或知识库 chunk。
2. 本轮不新增高级筛选、按条件全选或跨页服务端批处理。

## Loop-56 / 响应矩阵依据完整性筛选 - 2026-06-17

### 本轮目标

1. 补齐响应矩阵审阅中的高级筛选缺口，让评审人员能快速定位待补证据、待补来源和依据完整项。
2. 筛选只使用当前响应矩阵已有业务字段，不新增后端查询条件和技术字段。
3. 工具栏控件保持紧凑，避免新增筛选造成按钮错行或宽度跳动。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 新增 `RequirementEvidenceFilter`，在覆盖状态筛选之外叠加“全部依据 / 待补证据 / 待补来源 / 依据完整”筛选。
2. `filterRequirementRows()` 改为先按覆盖状态筛选，再按证据/来源完整性筛选。
3. “响应要点”工具栏新增依据筛选下拉，继续保持批量标记、批量补证据和导出操作不变。
4. `frontend/src/index.css` 固定依据筛选下拉宽度，避免工具栏因选项文字变化产生布局跳动。
5. `AI_IMPLEMENTATION_CHECKLIST.md` 同步更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 TypeScript 构建和 Vite 打包通过。
2. `git diff --check` 通过。

### 偏离蓝图

1. 本轮实现当前页前端筛选，不新增跨页服务端条件批处理。
2. 本轮不做来源原文精确跳转或知识库 chunk 定位。

## Loop-57 / 响应来源原文预览入口 - 2026-06-17

### 本轮目标

1. 补齐响应矩阵来源只显示数量、不能打开原文的缺口。
2. 优先复用现有文件预览和知识库文档预览接口，不新增来源解析旁路。
3. 仍保持业务口径：展示文件或章节、页码、原文摘录和“查看原文”，不暴露 chunk_id、schema、provider、token 等技术字段。

### 代码交付

1. `frontend/src/shared/api/client.ts` 新增 `fetchKnowledgeDocumentPreview()`，调用 `GET /knowledge/documents/:id/preview` 获取知识库文档预览链接。
2. `frontend/src/features/bid/index.tsx` 将“来源 N 处”从标签改成可点击入口，打开响应来源列表。
3. 来源列表逐条展示来源标题和摘录；来源携带 `file_id` / `file_asset_id` 时走文件预览，携带 `document_id` / `source_document_id` 时走知识库文档预览。
4. 有页码的来源会给预览 URL 附加 `#page=N` 锚点，PDF 预览器可直接定位对应页。
5. `frontend/src/index.css` 新增来源预览列表的响应式布局，避免标题、摘录和按钮错位。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 同步更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 TypeScript 构建和 Vite 打包通过。
2. `git diff --check` 通过。

### 偏离蓝图

1. 本轮实现文件/文档级预览和页码锚点，不做 chunk 内文本高亮。
2. 人工录入但没有文件或文档 ID 的来源只能展示详情，不能伪装成可跳转来源。

## Loop-58 / 外部 MCP Provider 预设目录 - 2026-06-18

### 本轮目标

1. 将全网/GitHub 检索出的 Handaas、AutoRFP、qlows、BidCraft、Loopio 等候选从文档雷达推进到可治理的后端 Provider 目录。
2. 管理员配置外部 MCP 时不能再完全依赖自由字符串；已知 Provider 应有默认工具白名单、token env、用途和数据边界。
3. 继续保持外部 MCP 只读、租户显式启用、摘要审计和脱敏边界，不让外部工具绕过智标通主链路。

### 代码交付

1. `backend/internal/platform/externaltool/presets.go` 新增 `ProviderPreset` 和 `ProviderPresets()`，内置 Handaas、AutoRFP、qlows、BidCraft Compliance、BidCraft Win Strategy、Loopio 只读预设。
2. `backend/internal/platform/externaltool/store.go` 的配置归一化会为已知 Provider 应用预设名称和默认工具白名单；严格 Provider 会拒绝目录外工具；已知 Provider 不允许关闭脱敏策略；启用配置必须提供 endpoint。
3. `backend/internal/api/routes.go` 新增 `GET /external-tools/catalog`，通过 team read 权限返回 Provider 目录，并同步 routeSpecs/customRouteSet。
4. `frontend/src/shared/api/client.ts` 新增外部工具目录和配置 DTO，以及 `fetchExternalToolCatalog()` / `fetchExternalTools()`。
5. `docs/blueprint/EXTERNAL_MCP_SKILL_RADAR.md`、`AI_IMPLEMENTATION_CHECKLIST.md`、`API_SPEC.md`、`AI_PIPELINE.md` 同步更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
go test ./...
pnpm --dir frontend build
git diff --check
```

结果：

1. 后端全量 Go 测试通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过。

### 偏离蓝图

1. 本轮实现 Provider 目录和后端校验，不接真实第三方凭证。
2. 本轮只加共享前端 API client，不实现外部工具管理页面。
3. AutoRFP、qlows、Loopio 等 SaaS 工具名可能随服务端版本变化，预设默认白名单作为初始建议；生产接入仍需 live smoke test 后确认。

## Loop-59 / 外部数据源前端管理入口 - 2026-06-18

### 本轮目标

1. 补齐外部 MCP Provider 目录只有后端 API、没有租户管理员可操作入口的问题。
2. 前端展示必须围绕业务可理解信息：数据源、用途、数据边界、启用状态、调用记录和费用估算。
3. 配置入口继续复用后端白名单、脱敏策略、预算和审计约束，不提供绕过安全边界的自由调用面板。

### 代码交付

1. `frontend/src/shared/api/client.ts` 新增 `ExternalToolAuditLogDTO`、`updateExternalToolConfig()` 和 `fetchExternalToolAuditLogs()`。
2. `frontend/src/features/team/index.tsx` 新增 `external-tools` tab，展示 Provider 目录、用途、数据边界、启用状态、密钥配置项、启用工具数量和最近调用记录。
3. 外部数据源配置弹窗支持维护名称、启用状态、访问地址、启用工具、超时时间、月度预算、单次估算和脱敏策略；启用时必须填写访问地址。
4. 团队页状态标签补充 `success` / `blocked`，外部工具审计表可展示成功、失败和阻断状态。
5. 修正团队页残留的 `Space orientation="vertical"` 为 Ant Design 支持的 `direction="vertical"`。
6. `frontend/src/index.css` 新增外部数据源描述文本样式，避免表格单元内长文本错行撑裂。
7. `AI_IMPLEMENTATION_CHECKLIST.md`、`EXTERNAL_MCP_SKILL_RADAR.md`、`API_SPEC.md`、`AI_PIPELINE.md` 同步更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
go test ./...
pnpm --dir frontend build
pnpm --dir frontend lint
git diff --check
```

结果：

1. 后端全量 Go 测试通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. 前端 ESLint 通过。
4. `git diff --check` 通过。

### 偏离蓝图

1. 本轮不接真实第三方凭证，不执行生产 smoke test。
2. 本轮只做团队管理入口，不把外部标讯搜索或企业画像接入项目/标书创建业务流。
3. 配置弹窗不提供任意工具调用调试面板，避免用户把客户文件正文直接外发。

## Loop-60 / OCR Provider 路由审计固化 - 2026-06-18

### 本轮目标

1. 修补 MinerU / PaddleOCR OCR Provider 已有环境变量和评测入口，但模型网关配置没有显式 `document_ocr` 路由的问题。
2. 让 `/models/health`、`provider_backed_mock_routes()` 和生产配置检查能够覆盖 OCR Provider 能力边界。
3. 保持 OCR 实际调用仍走本地解析管线，避免把扫描件交给未明确配置的外部 LLM 路由。

### 代码交付

1. `ai-service/app/config/model_routing.yaml` 新增 `document_ocr` 路由，使用 `local` provider、`configurable-ocr-provider-pipeline`、`OCRProviderResult` schema 和 300 秒超时。
2. `ai-service/app/tests/test_model_router.py` 将 `document_ocr` 纳入本地管线路由断言，并新增 shipped config 审计测试，确保 OCR 路由不会被识别为 Mock Provider。
3. `AI_IMPLEMENTATION_CHECKLIST.md`、`MODEL_GATEWAY.md`、`SAMPLE_DOCS_EVALUATION.md` 同步记录 OCR Provider 的路由审计口径。

### 检查结果

已运行：

```bash
PYTHONPATH=. ./.venv/bin/python -m pytest -s app/tests/test_model_router.py
PYTHONPATH=. ./.venv/bin/python -m pytest -s app/tests/test_document_parser.py app/tests/test_ocr_provider_eval.py
git diff --check
```

结果：

1. 模型路由测试 46 项通过，`document_ocr` shipped config 审计通过。
2. 文档解析和 OCR Provider 评测测试 49 项通过。
3. AI 服务全量 pytest 221 项通过。
4. `git diff --check` 通过。

### 偏离蓝图

1. 本轮不配置真实 MinerU / PaddleOCR endpoint，不执行真实 OCR smoke test。
2. `document_ocr` 路由只用于配置审计和运行边界表达；实际 OCR HTTP 调用仍由 `document_parser.py` 根据 `OCR_PROVIDER` 系列环境变量完成。

## Loop-61 / xparse 模块上下文路由审计 - 2026-06-18

### 本轮目标

1. 修补六模块增强 prompt 只拿关键词文本行、缺少 `chunk_id/page/table_block` 来源锚点的问题。
2. 把 xparse 的“标题优先、关键词路由、表格保留 md_table”从文档清单推进到运行态 prompt 和 `parse_metadata`。
3. 为 AutoRFP 式来源引用继续提供稳定的 `source_context`，便于模型返回 `citation_id/reference_id` 并进入人工复核。

### 代码交付

1. `ai-service/app/pipelines/parse/tender_parser.py` 新增 `xparse-context-router-v1`，按模块关键词从标题路径、正文行和 `table_blocks.md_table` 中选择上下文。
2. `build_tender_module_prompt()` 现在输出结构化 `source_context`，携带 `chunk_id`、页码、`table_block_id`、匹配原因和摘录；`source_excerpt` 也保留同样锚点。
3. `build_tender_structured_result()` 的 `parse_metadata.module_context` 记录每个模块的命中 chunk、表格块和路由原因，供回归审计。
4. `ai-service/app/tests/test_main_security.py` 新增模块上下文路由测试，覆盖评分 chunk、PDF 表格块和附件 chunk 进入对应模块。

### 检查结果

已运行：

```bash
PYTHONPATH=. ./.venv/bin/python -m pytest -s app/tests/test_main_security.py::test_tender_module_context_routes_chunks_and_tables_with_source_anchors app/tests/test_main_security.py::test_build_tender_structured_result_extracts_business_fields
PYTHONPATH=. ./.venv/bin/python -m pytest -s app/tests/test_tender_parse_eval.py
PYTHONPATH=. ./.venv/bin/python -m pytest -s
./.venv/bin/python -m ruff check app/pipelines/parse/tender_parser.py app/tests/test_main_security.py
git diff --check
```

结果：

1. 新增上下文路由测试和既有结构化解析测试通过。
2. 招标解析 golden 评测单元测试 4 项通过。
3. AI 服务全量 pytest 222 项通过。
4. Python ruff 检查通过。
5. `git diff --check` 通过。

### 偏离蓝图

1. 本轮不做相邻页/相邻 chunk 扩展和复杂表头层级推断，避免把弱关联上下文误标为确定来源。
2. 本轮只增强 AI 服务解析 prompt 与 metadata，不新增前端 chunk 内文本高亮。

## Loop-62 / 响应来源定位复制与 Space 布局修正 - 2026-06-18

### 本轮目标

1. 补齐响应来源只支持打开文件/页、不能把 chunk/引用定位传递给人工复核的问题。
2. 来源详情必须继续使用业务口径，不展示 provider、model、token 等技术信息。
3. 修复前端残留的 Ant Design `Space orientation="vertical"` 无效属性，避免页面垂直布局被渲染成横向排列。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的响应来源弹窗新增页码、章节、引用号、定位码展示，并支持复制来源定位文本。
2. 来源摘要会包含可用的引用号或定位码；打开原文仍复用现有文件/知识库文档预览和页码锚点。
3. `frontend/src/index.css` 新增来源定位标签布局，长引用号和摘录可换行，不撑裂弹窗。
4. 全局修正前端残留 `Space orientation="vertical"` 为 `direction="vertical"`，覆盖标书、认证、成本、知识库、项目、标讯和 PageFrame。

### 检查结果

已运行：

```bash
rg -n "orientation=\"vertical\"" frontend/src -S
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端无残留 `orientation="vertical"`。
2. 前端 ESLint 通过。
3. 前端 TypeScript 构建和 Vite 打包通过。
4. `git diff --check` 通过。

### 偏离蓝图

1. 本轮不实现 PDF/Word 预览器内文本高亮；先提供可复制定位信息和页码预览。
2. 人工录入且缺少文件或文档 ID 的来源仍只能展示和复制定位，不能伪装成可打开原文。

## Loop-63 / 解析确认字段编辑闭环 - 2026-06-18

### 本轮目标

1. 补齐解析结果确认前只能提交原始 `structured_result`、无法修正核心业务字段的问题。
2. 让人工确认后的项目名称、截止时间、标书类型和关键要求进入后续目录、素材和响应矩阵使用的结构化事实。
3. 保持解析确认页使用业务口径，不暴露 provider、model、schema、token 等技术信息。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 新增确认信息草稿，绑定当前解析结果版本，展示项目名称、投标截止、标书类型、资格要求、评分要点和否决风险。
2. 确认提交前会把人工修正写回 `structured_result.project_name/deadline/bid_type/qualification_requirements/scoring_points/invalid_clause_risks`，并同步到 `modules.basic/qualification/evaluation/invalid_risk.fields`。
3. 资格要求、评分要点和否决风险发生人工修改时，会同步替换对应模块的 `requirement_items`，后端确认后响应矩阵不再沿用编辑前条目。
4. `parse_metadata.confirm_overrides` 记录本次人工调整字段和确认时间，便于后续审计。
5. `frontend/src/index.css` 新增确认信息区响应式网格，移动端降为单列，长标签和输入内容不撑裂布局。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 更新 P1 解析结果页当前落地状态。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。

### 偏离蓝图

1. 本轮先补核心字段确认编辑，不实现所有六模块字段的逐项编辑、字段置信度手动标记和页内原文高亮。
2. 本轮未启动完整 docker 栈做运行态端到端验收。

## Loop-64 / 字段依据复核与来源跳转 - 2026-06-18

### 本轮目标

1. 把 `structured_result.field_evidence` 和模块 `evidence` 从后端 JSON 变成解析确认页可复核的业务信息。
2. 字段级复核必须展示可信度、原文摘录、页码/引用号/定位码，并支持复制和打开原文页。
3. 复核操作继续走确认提交，不新增后端协议，避免引入未验证的状态写接口。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 新增“字段依据”Tab，按字段展示结果、可信度、来源摘录、定位标签和原文操作。
2. 字段依据优先读取 `field_evidence`；旧数据或兜底结果缺少顶层证据时，会从六模块 `evidence` 汇总。
3. 来源操作复用现有预览和复制能力：支持 `file_id/file_asset_id`、`document_id`、`page_start`、`citation_id/reference_id`、`chunk_id`。
4. 字段可标记“确认无误/需要补充/不适用”，确认时写入 `parse_metadata.field_reviews`，并在 `confirm_overrides.reviewed_fields` 留审计记录。
5. `frontend/src/index.css` 新增字段来源单元布局，长摘录、定位标签和操作按钮在窄屏下不撑裂。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 更新 P1 解析结果页当前落地状态。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过。

### 偏离蓝图

1. 本轮不实现 PDF/Word 预览器内文本高亮；当前先支持页码预览、引用号/定位码复制和原文摘录展示。
2. 字段复核状态暂存于 `structured_result.parse_metadata.field_reviews`，尚未拆独立数据库表和历史事件表。

## Loop-65 / xparse 相邻上下文路由 - 2026-06-18

### 本轮目标

1. 修补六模块模块增强只取命中 chunk 和表格块，容易漏掉标题下相邻段落的问题。
2. 保持上下文扩展可审计、可回溯，不把整份招标文件直接塞入模块 prompt。
3. 将上下文路由版本从 `xparse-context-router-v1` 升级到 `xparse-context-router-v2`。

### 代码交付

1. `ai-service/app/pipelines/parse/tender_parser.py` 新增 `MODULE_CONTEXT_NEIGHBOR_WINDOW=1` 和 `_neighbor_module_context_records()`，对每个命中 chunk 扩展前后相邻 chunk。
2. 相邻记录保留 `chunk_id/title_path/page_start/page_end/lines`，并以 `neighbor_chunk`、`adjacent_page` 标记原因，score 降为 1，避免覆盖真正命中上下文。
3. `source_context` 和 `parse_metadata.module_context` 均会包含相邻 chunk 的路由原因和 chunk id。
4. `ai-service/app/tests/test_main_security.py` 增加无关键词相邻人员安排段落，验证 evaluation 模块 prompt 和 metadata 都带入相邻 chunk。
5. `AI_IMPLEMENTATION_CHECKLIST.md` 更新当前 xparse 上下文路由版本和相邻上下文落地状态。

### 检查结果

已运行：

```bash
PYTHONPATH=. ./.venv/bin/python -m pytest -s app/tests/test_main_security.py::test_tender_module_context_routes_chunks_and_tables_with_source_anchors
PYTHONPATH=. ./.venv/bin/python -m pytest -s app/tests/test_tender_parse_eval.py
./.venv/bin/python -m ruff check app/pipelines/parse/tender_parser.py app/tests/test_main_security.py
git diff --check
```

结果：

1. xparse 模块上下文路由测试通过。
2. 招标解析 golden 评测 4 项通过。
3. Python ruff 检查通过。
4. `git diff --check` 通过。
5. 测试确认 `xparse-context-router-v2` 输出命中 chunk、相邻 chunk 和表格块来源锚点。

### 偏离蓝图

1. 本轮只做相邻 chunk / 相邻页一阶扩展，不做跨章节长距离召回，避免把弱相关上下文误标为确定来源。
2. 复杂表头层级推断、单元格级 bbox 和预览器文本高亮仍需后续样本驱动推进。

## Loop-66 / 外部标讯业务入口 - 2026-06-18

### 本轮目标

1. 把行业 MCP / Skills 雷达中的 Handaas 招投标数据源从“可配置”推进到标讯大厅可使用的业务入口。
2. 外部调用仍保持租户显式启用、工具白名单、摘要审计和只读边界，不把客户招标文件或投标文件原文发给第三方。
3. 页面文案使用业务口径，不暴露 JSON-RPC、工具内部字段或模型口径。

### 代码交付

1. `frontend/src/shared/api/client.ts` 新增 `invokeExternalTool()` 和 `ExternalToolInvokeResultDTO`，复用后端 `/external-tools/:providerKey/invoke` 网关。
2. `frontend/src/features/tender/index.tsx` 新增“外部标讯”页签，读取已启用的 Handaas 数据源配置后才允许检索。
3. 检索参数按 Handaas 实际接口传递：`matchKeyword`、`biddingRegion`、`biddingAnncPubStartTime`、`biddingAnncPubEndTime`、`searchMode`、`pageIndex`、`pageSize`。
4. 外部返回结果做宽松归一，只抽取标讯名称、招标单位、地区、预算、发布日期、截止日期、摘要、要求和来源链接。
5. 用户可将选中结果保存为租户内标讯，`metadata.source_type=external_mcp`，并在 `metadata.external_mcp` 记录 provider、工具、导入时间和候选标识。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 和 `EXTERNAL_MCP_SKILL_RADAR.md` 更新当前能力和剩余边界。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。

### 偏离蓝图

1. 本轮未配置真实 Handaas 生产凭证，也未对真实账号做 smoke test；真实响应结构仍需在租户环境验证后继续调优映射。
2. 本轮只接入标讯搜索入口，不接企业画像、采购统计、外部应答库检索和知识库入库。
3. 当前保存的是公开标讯摘要和业务字段，不自动抓取或外发完整招标文件附件。

## Loop-67 / 响应矩阵跨页服务端批处理 - 2026-06-18

### 本轮目标

1. 补齐 AutoRFP 式响应矩阵只能对当前勾选项批量处理，不能按筛选条件跨页批处理的问题。
2. 保持覆盖状态、响应证据和响应来源的人工调整都写入覆盖历史。
3. 不新增路由，扩展现有 `PATCH /bids/:id/requirements` 协议，避免打断已有前端和导出链路。

### 代码交付

1. `backend/internal/platform/bid/store.go` 的 `BatchUpdateRequirementCoverageRequest` 新增 `apply_all`、`filter`、`evidence_filter`。
2. 服务端可按 `all/mandatory/review/covered` 和 `all/missing_evidence/missing_source/complete` 组合筛选当前标书全部要求项，再逐条更新覆盖状态、证据、来源和覆盖历史。
3. 批量历史 metadata 新增 `batch_scope=selected|filtered`、`filter`、`evidence_filter` 和真实 `requirement_count`，便于审计区分勾选批量和筛选批量。
4. `frontend/src/features/bid/index.tsx` 的“响应要点”工具栏新增“筛选全部标记”和“筛选全部补证据”，操作前提示当前筛选命中数量。
5. `frontend/src/shared/api/client.ts` 更新批量接口类型，支持 `apply_all/filter/evidence_filter`。
6. `API_SPEC.md`、`AI_IMPLEMENTATION_CHECKLIST.md` 同步记录当前能力。

### 检查结果

已运行：

```bash
go test ./internal/platform/bid
pnpm --dir frontend lint
pnpm --dir frontend build
```

结果：

1. 后端 bid 包单元测试通过，覆盖筛选批量 metadata、无效筛选拒绝和前后端筛选规则一致性。
2. 前端 ESLint 通过。
3. 前端 TypeScript 构建和 Vite 打包通过。

### 偏离蓝图

1. 本轮不实现预览器文本高亮；只补响应矩阵跨页服务端批处理。
2. 服务端仍保留 `requirementCoverageBatchLimit`，超过限制的筛选批量会拒绝执行，避免误操作一次性覆盖过多要求项。

## Loop-68 / 来源摘录高亮与预览搜索定位 - 2026-06-18

### 本轮目标

1. 补齐 AutoRFP 式响应来源“能看见来源”但缺少醒目摘录定位的问题。
2. 打开原文时同时携带页码和摘录搜索文本，让浏览器/PDF 查看器有机会直接定位命中内容。
3. 保持前端文案为业务口径，不暴露内部 JSON、chunk 或模型字段。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 新增 `requirementSourceExcerpt()`、`requirementSourceSearchText()`、`withPreviewSourceAnchor()` 和 `HighlightedSourceText()`。
2. 响应来源弹窗不再把标题、页码、定位码和摘录挤成一行摘要，而是标题、定位标签、摘录分层展示，并对摘录加高亮。
3. 解析确认表的字段依据列复用同一高亮组件，避免字段来源只是一段普通灰色文本。
4. “查看原文”预览 URL 会携带 `page` 和 `search` hash 参数，保留原有页码跳转能力，同时把摘录作为搜索定位线索。
5. `frontend/src/index.css` 新增 `.source-highlight-mark`，使用轻量背景和 `box-decoration-break` 保证跨行高亮不挤压表格布局。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 更新当前状态：已完成来源摘录高亮和预览搜索定位，内嵌 PDF/Word 选区框仍需后续自建预览器或 viewer 扩展。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过，无空白错误。

### 偏离蓝图

1. 浏览器内置 PDF/Office 预览器是否按 `search` 参数高亮命中，取决于具体 viewer 支持；本轮不自建 PDF canvas 选区层。
2. 本轮只高亮来源摘录本身，不做跨页多片段、多 bbox 或表格单元格级定位。

## Loop-69 / 六模块字段逐项编辑确认 - 2026-06-18

### 本轮目标

1. 补齐文件解读页只能编辑少数核心字段，六模块结构化字段仍主要只读的问题。
2. 不新增后端协议，继续通过现有解析确认接口提交完整 `structured_result`。
3. 前端继续使用业务口径，不把模型、schema、内部字段路径暴露给业务用户。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 新增 `ParseModuleFieldsDraft` 状态，按解析结果版本隔离模块字段草稿。
2. 文件解读页新增“模块字段”页签，将 `modules.*.fields` 展开为分组、字段、确认结果、状态四列。
3. 模块字段编辑会写回 `structured_result.modules[module].fields`；数组按行编辑，数字、布尔和 JSON 结构在确认时尽量按原字段类型还原。
4. 项目名称、投标截止、标书类型、资格要求、评分要点、否决风险这些已有核心字段与模块字段共用同一份确认草稿，避免核心面板和模块字段表保存后互相覆盖。
5. `parse_metadata.confirm_overrides.edited_module_fields` 记录本次调整过的模块字段路径，便于后续审计确认版本。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 同步更新六模块逐项编辑当前状态和剩余边界。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过，无空白错误。

### 偏离蓝图

1. 本轮不做字段置信度手工改写，也不新增字段级 bbox 选区编辑。
2. 复杂对象字段提供 JSON 文本编辑和类型还原，不做专用可视化表单。

## Loop-70 / 字段复核置信度写回 - 2026-06-18

### 本轮目标

1. 修补字段依据复核只写 `parse_metadata.field_reviews`，没有改变字段证据本体的问题。
2. 让“确认无误/需要补充/不适用”真正影响字段依据的可信度标签和解析质量门统计。
3. 继续复用现有解析确认接口，不新增数据库表或后端路由。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 新增 `applyParseFieldReviews()`，确认解析结果时会把复核状态写回顶层 `field_evidence`；旧数据没有顶层证据时，会写回对应模块 `evidence`。
2. 字段状态写回规则：
   - 确认无误：`needs_review=false`，`confidence` 至少提升到 `0.95`。
   - 不适用：`needs_review=false`，`not_applicable=true`，`confidence` 至少提升到 `0.95`。
   - 需要补充：`needs_review=true`，`confidence` 最高收敛到 `0.4`。
3. `refreshParseFieldReviewQualityGate()` 会按写回后的字段证据刷新 `quality_gates.interpret.low_confidence_count`、`missing_source_count`、`human_reviewed_count` 和 `parse_metadata` 计数。
4. 字段依据表可信度标签会优先展示“人工确认 / 需要补充 / 不适用”，避免确认后仍显示旧的低可信百分比。
5. `AI_IMPLEMENTATION_CHECKLIST.md` 同步更新字段置信度手工标记当前状态。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过，无空白错误。

### 偏离蓝图

1. 本轮仍不新增字段复核历史事件表；复核记录随解析确认版本保存在 `structured_result` 中。
2. 缺来源字段即使人工确认，仍会进入缺来源统计；“不适用”字段不参与低置信和缺来源统计。

## Loop-71 / 字段复核模块状态同步 - 2026-06-18

### 本轮目标

1. 修补 Loop-70 后的状态一致性问题：顶层 `field_evidence` 写回复核状态后，模块 `evidence/status` 仍可能保持旧的待确认状态。
2. 让“字段依据”“信息分组”和 `quality_gates.interpret` 对同一次人工复核给出一致结论。
3. 不新增后端协议，继续在解析确认时写回完整 `structured_result`。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的 `applyParseFieldReviews()` 不再在顶层 `field_evidence` 存在时提前返回，而是继续同步模块证据。
2. 新增 `parseFieldReviewByField()`，用字段名把顶层字段复核结果同步到模块 `evidence`，兼容 top-level evidence 与 module evidence 双写结构。
3. 新增 `parseModuleStatusFromEvidence()`，模块证据全部通过或不适用时写 `status=done`，仍有低可信或需要补充时写 `status=needs_review`。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录字段复核会同步模块 `evidence/status`。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过，无空白错误。

### 偏离蓝图

1. 多个模块存在同名字段时，当前按字段名同步相同复核结果；后续若需要区分同名字段不同来源，可把字段依据 row id 扩展为模块级路径。

## Loop-72 / 响应历史来源可追溯操作 - 2026-06-18

### 本轮目标

1. 补齐 Loop-49 留下的历史项来源只能看摘要，不能直接打开原文或复制定位的问题。
2. 保持覆盖历史仍使用业务口径，不暴露模型、token、provider、schema 等技术字段。
3. 复用现有响应来源预览、页码/搜索定位和复制定位能力，不新增后端接口。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的响应历史时间线向 `requirementHistoryTimelineItem()` 注入来源打开和复制定位动作。
2. 历史项中的 `source_refs` 改为逐条来源列表，展示来源标题、页码/章节/引用号/定位码、摘录高亮，以及“查看原文”和“复制来源定位”操作。
3. 删除不再使用的历史来源摘要 helper，避免维护两套来源展示逻辑。
4. `frontend/src/index.css` 新增响应历史来源列表样式和移动端单列布局，避免时间线弹窗窄屏错行。
5. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录覆盖历史来源已可打开原文和复制定位。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. `git diff --check` 通过，无空白错误。

### 偏离蓝图

1. 历史来源继续依赖已有 `source_refs` 中的文件或文档 ID；人工录入且缺少文件/文档 ID 的来源只能复制定位，不能伪装成可打开原文。
2. 预览仍使用浏览器/PDF 查看器的页码和搜索参数，不自建 PDF canvas 选区层。

## Loop-73 / OCR Provider 坐标级质量门控 - 2026-06-18

### 本轮目标

1. 把 MinerU / PaddleOCR 真实 endpoint 验收从“能返回文本”推进到“能返回页级置信度和版面证据”。
2. 让 `docs/ex/工程1` 的 OCR Provider 样本评测可以显式要求 layout bbox、table bbox 和 cell bbox。
3. 不把 MinerU/PaddleOCR 变成硬依赖；未配置 endpoint 时仍输出 skipped，不伪装通过。

### 代码交付

1. `ai-service/app/evaluation/ocr_provider_eval.py` 新增 `--min-page-confidence`、`--min-layout-bbox-count`、`--min-table-bbox-count`、`--min-cell-bbox-count` 门槛。
2. 评测会从归一化 OCR metadata 中统计页级最低置信度、版面块 bbox 数、表格 bbox 数和单元格 bbox 数，未满足时返回 failed。
3. `ai-service/app/tests/test_ocr_provider_eval.py` 补充通过和失败用例，覆盖 MinerU/PaddleOCR 响应中的页级置信度和坐标级版面证据。
4. `SAMPLE_DOCS_EVALUATION.md` 和 `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录真实 OCR Provider 验收命令和剩余生产凭证/持续回归报告缺口。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest -s app/tests/test_ocr_provider_eval.py
cd ai-service && .venv/bin/python -m ruff check app/evaluation/ocr_provider_eval.py app/tests/test_ocr_provider_eval.py
cd ai-service && .venv/bin/python -m compileall app/evaluation/ocr_provider_eval.py app/tests/test_ocr_provider_eval.py
ai-service/.venv/bin/python -m app.evaluation.ocr_provider_eval --provider mineru --sample docs/ex/工程1/采购文件桥梁检查.pdf --allow-skip --json
git diff --check
```

结果：

1. OCR Provider 评测单测 4 项通过。
2. Ruff 通过。
3. 评测脚本和测试文件编译通过。
4. 本地未配置 `MINERU_HTTP_ENDPOINT` 时，真实样本 OCR 验收返回 `skipped`，没有伪装通过。
5. `git diff --check` 通过，无空白错误。

### 偏离蓝图

1. 本轮没有配置真实 MinerU/PaddleOCR endpoint 或 API Key；生产环境仍需提供凭证并保存持续回归报告。
2. 当前门槛只检查 OCR Provider 返回的已归一化证据数量，不评价 OCR 文字准确率、表格语义正确性或 bbox 与原图的几何重合度。

## Loop-74 / 文件预览来源定位条 - 2026-06-18

### 本轮目标

1. 缩小“来源预览依赖浏览器 search 参数，页面内没有稳定定位提示”的缺口。
2. 可解析到文件 ID 的来源应打开前端文件预览页，并在页面内显示页码、摘录关键词和复制定位入口。
3. 保持页面业务口径，不展示模型、token、provider、schema 等技术字段。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的来源打开逻辑改为：文件来源优先打开 `/files/:fileId/preview`，并携带 `page`、`search`、`source_title`、`source_locator` query；知识库文档来源仍复用原预签名预览链路。
2. `frontend/src/features/knowledge/index.tsx` 的文件预览页读取 query，生成页面内“定位来源”条，展示来源标题、页码、摘录关键词，并支持复制定位。
3. 文件预览页会继续把页码和搜索词写入内嵌文件 URL hash，保留浏览器/PDF viewer 原有页码和搜索能力。
4. `frontend/src/index.css` 新增文件预览定位条布局和移动端单列样式，避免长摘录挤压按钮或窄屏错行。
5. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录文件预览页定位条已落地，PDF canvas 选区框仍为后续增强。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。

### 偏离蓝图

1. 本轮没有自建 PDF canvas 选区框或 Word 文本坐标层；定位条提供稳定业务侧定位提示，内嵌 viewer 的实际文本高亮仍取决于浏览器/PDF 预览器支持。
2. 知识库 document ID 来源暂时保留原预签名预览方式；只有携带 file_id/file_asset_id 的来源进入前端文件预览页。

## Loop-75 / 工程1生成覆盖与导出格式回归 - 2026-06-18

### 本轮目标

1. 补齐推荐开发顺序中“工程1 golden 样本加入章节生成覆盖率和导出格式回归”的缺口。
2. 让章节覆盖、来源解析和 DOCX/PDF/ZIP 导出格式都能通过离线 CLI 重复验收。
3. PDF 在无 LibreOffice 的环境中必须显式 skipped，生产验收可关闭 skip 作为硬门槛。

### 代码交付

1. 新增 `ai-service/app/evaluation/export_format_eval.py`，读取导出格式 golden JSON，临时生成 DOCX、ZIP 和 PDF，并检查 DOCX 可打开性、目录域、自动更新域、页码域、页眉页脚、表格、ZIP manifest、ZIP 内 DOCX 可打开性、PDF 可打开性、文本层和首屏非空。
2. 新增 `ai-service/app/tests/test_export_format_eval.py`，覆盖 DOCX/ZIP 通过、PDF 显式 skipped、缺少必需文本失败。
3. 新增 `docs/sample_docs/golden/工程1.generation_coverage.json`，覆盖工程1三条强制/高优先级要求、章节覆盖矩阵和可解析 source_refs。
4. 新增 `docs/sample_docs/golden/工程1.export.json`，覆盖工程1响应场景的 DOCX/ZIP/PDF 导出格式回归。
5. `SAMPLE_DOCS_EVALUATION.md` 和 `AI_IMPLEMENTATION_CHECKLIST.md` 同步补充生成覆盖与导出格式评测命令、门槛和生产 skip 边界。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest -s app/tests/test_export_format_eval.py
cd ai-service && .venv/bin/python -m ruff check app/evaluation/export_format_eval.py app/tests/test_export_format_eval.py
cd ai-service && .venv/bin/python -m compileall app/evaluation/export_format_eval.py app/tests/test_export_format_eval.py
cd ai-service && .venv/bin/python -m app.evaluation.generation_coverage_eval --input ../docs/sample_docs/golden/工程1.generation_coverage.json --json
cd ai-service && .venv/bin/python -m app.evaluation.export_format_eval --input ../docs/sample_docs/golden/工程1.export.json --json
```

结果：

1. 新增导出格式评测单测 2 项通过。
2. Ruff 通过。
3. 新增评测器和测试文件编译通过。
4. 工程1生成覆盖 golden 通过，7/7 检查通过，mandatory coverage 和 source_ref resolution 均为 1.0。
5. 工程1导出格式 golden 通过，23/23 检查通过；本机实际生成 PDF，3 页、文本层 324 字符、首屏非空。

### 偏离蓝图

1. 本轮评测器生成的是离线 golden 场景，不直接拉取运行态真实 bid；运行态可继续用 `GET /bids/:id/generation-coverage` 导出后交给同一评测器。
2. PDF 视觉验收仍是“可打开、文本层、首屏非空”，不是逐页像素级排版对比。

## Loop-76 / 知识库文档来源定位预览 - 2026-06-18

### 本轮目标

1. 补齐 Loop-74 的尾部偏离：知识库 document ID 来源不应绕过前端定位预览页。
2. 文件来源和知识库文档来源都使用同一套定位条、页码、摘录关键词和复制定位体验。
3. 不改变后端权限模型，继续使用已有 `GET /knowledge/documents/:id/preview`。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的来源打开逻辑统一生成前端预览页 URL：文件来源走 `/files/:fileId/preview`，知识库文档来源走 `/knowledge/documents/:documentId/preview`，两者都携带 `page`、`search`、`source_title`、`source_locator`。
2. `frontend/src/routes/router.tsx` 新增 `/knowledge/documents/:documentId/preview` 路由，复用文件预览页组件。
3. `frontend/src/features/knowledge/index.tsx` 的 `FilePreviewPage` 增加 `sourceType`，根据文件 ID 或知识库文档 ID 调用 `fetchFileURL` / `fetchKnowledgeDocumentPreview`，并复用定位条和下载逻辑。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录文件 ID 和知识库文档 ID 来源均已进入前端预览页定位条。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。

### 偏离蓝图

1. 仍未实现 PDF canvas 选区框或 Word 文本坐标层；本轮解决的是两类来源都进入稳定业务定位条。
2. 后端 `GET /knowledge/documents/:id/preview` 仍按 knowledge read 权限校验；若某角色只有 bid 权限而没有 knowledge 权限，前端路由可打开但预览接口会按后端权限返回错误。

## Loop-77 / 解析来源引用门禁 - 2026-06-18

### 本轮目标

1. 把 AutoRFP 式 `Question/Requirement -> Source` 从“有字段”提高到“可验收的来源引用完整性”。
2. 工程1 golden 不只检查 evidence 数量和 source_text，还要检查引用号和定位信息。
3. 避免解析结果退化为只有摘要、没有 `citation_id/reference_id/chunk/file/page` 的来源说明。

### 代码交付

1. `ai-service/app/evaluation/tender_parse_eval.py` 新增来源引用门禁：
   - `requirements.require_traceable_source_refs`
   - `requirements.require_reference_ids`
   - `requirements.require_source_locations`
   - `evidence.require_traceable`
   - `evidence.require_reference_ids`
   - `evidence.require_source_locations`
2. `docs/sample_docs/golden/工程1.parse.json` 开启上述门禁，工程1解析检查数从 103 项提升到 109 项。
3. `ai-service/app/tests/test_tender_parse_eval.py` 增加完整样本通过和无可追溯来源失败两个场景。
4. `SAMPLE_DOCS_EVALUATION.md` 与 `AI_IMPLEMENTATION_CHECKLIST.md` 同步更新当前解析验收口径。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_tender_parse_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
```

结果：

1. `test_tender_parse_eval.py` 5 项通过。
2. 工程1真实样本解析评测 109/109 通过。

### 偏离蓝图

1. 本轮校验的是来源引用结构完整性，不做来源文本与 PDF canvas bbox 的视觉框选验证。
2. 真实 OCR Provider 的持续报告仍依赖外部 MinerU/PaddleOCR endpoint 配置，本轮没有伪造通过。

## Loop-78 / 来源坐标定位贯通 - 2026-06-18

### 本轮目标

1. 把解析和 OCR 产出的坐标信息从后端结构字段继续传到前端来源复核界面。
2. 响应来源、覆盖历史、字段依据和预览页定位条都能展示并复制坐标定位。
3. 不在业务界面展示 bbox 等技术口径。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 新增来源坐标解析：支持 `bbox`、`bounding_box`、`boundingBox`、`rect`、`position` 的四点坐标或 `x/y/width/height` 形态，只接受有限数字和有效区域。
2. 来源定位标签新增“坐标”，并写入复制定位文本；打开文件或知识库文档原文时追加 `source_bbox` query。
3. `frontend/src/features/knowledge/index.tsx` 的预览定位条读取 `source_bbox`，展示“坐标”标签，并在缺少完整定位文本时仍可复制页码、坐标和摘录组合。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录来源坐标标签已进入人工复核链路。

### 检查结果

已运行：

```bash
pnpm --dir frontend lint
pnpm --dir frontend build
git diff --check
```

结果：

1. 前端 ESLint 通过。
2. 前端 TypeScript 构建和 Vite 打包通过。
3. diff 空白检查通过。

### 偏离蓝图

1. 本轮是坐标信息贯通和业务展示，不是自建 PDF canvas/Word 坐标层。
2. 内嵌预览器仍由浏览器或对象存储预览 URL 承载，不能保证按坐标绘制选区框；后续若要视觉框选，需要引入可控 PDF/Office 渲染层。

## Loop-79 / OCR Provider 生效配置审计 - 2026-06-18

### 本轮目标

1. 修正 MinerU/PaddleOCR 使用通用 OCR endpoint/key/poll 兜底时的审计 metadata。
2. `provider_profile` 必须记录实际生效的环境变量，避免生产排查时误以为使用了 Provider 专属配置。
3. 不改变 OCR 请求行为和未配置时的显式失败语义。

### 代码交付

1. `ai-service/app/pipelines/parse/document_parser.py` 新增 `_effective_env()`，在构建 `OCRProviderConfig` 时按“Provider 专属 env 有值优先，否则通用 OCR env，有值才记录”的顺序选择 `endpoint_env`、`api_key_env` 和 `poll_endpoint_env`。
2. `provider_profile.endpoint_env` / `api_key_env` / `poll_endpoint_env` 现在代表实际读取的 env 名；例如 `OCR_PROVIDER=mineru` 且只配置 `OCR_HTTP_ENDPOINT` 时，profile 会记录 `OCR_HTTP_ENDPOINT`。
3. `ai-service/app/tests/test_document_parser.py` 增加 MinerU 使用通用 OCR endpoint/key/poll 兜底的测试，覆盖请求 URL、Authorization 和 provider_profile。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py app/tests/test_ocr_provider_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
git diff --check
```

结果：

1. 文档解析与 OCR Provider 评测相关单测 51 项通过。
2. 工程1真实样本解析评测 109/109 通过。
3. diff 空白检查通过。

### 偏离蓝图

1. 本轮修复配置审计准确性，不提供真实 MinerU/PaddleOCR endpoint 或 API Key。
2. Provider 专属 timeout/mode 仍按 Provider 语义读取；本轮只修正 endpoint、key 和 poll endpoint 的实际来源记录。

## Loop-80 / 路由与外部工具文档防漂移 - 2026-06-18

### 本轮目标

1. 修正蓝图文档与当前前端/后端能力不一致的问题。
2. 把外部 MCP 业务入口、知识库文档预览路由和 OCR 生效 env 审计口径写入静态验收。
3. 保持外部 MCP 只读、租户显式启用和不外发文件原文的边界。

### 代码交付

1. `PAGE_ROUTE_MAP.md` 补齐标讯大厅“外部标讯”页签、团队管理 `external-tools` 页签和 `/knowledge/documents/:documentId/preview` 前端预览路由，并注明后端仍按 knowledge read 权限校验。
2. `AI_PIPELINE.md` 修正“尚未接业务入口”的过期表述，明确 `/tenders` 外部标讯页签已通过 Handaas 预设执行只读检索并保存为 `external_mcp` 标讯来源。
3. `README.md`、`AI_PIPELINE.md`、`SAMPLE_DOCS_EVALUATION.md` 同步 OCR `provider_profile.endpoint_env` / `api_key_env` / `poll_endpoint_env` 的实际生效 env 审计口径。
4. `acceptance_tail_check.py` 增加静态防漂移检查：模型路由必须包含 `document_ocr`，README/AI_PIPELINE/SAMPLE_DOCS_EVALUATION 必须记录 OCR profile env，PAGE_ROUTE_MAP 必须记录知识库预览和外部工具页签。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
git diff --check
rg -n "/knowledge/documents/:documentId/preview|external-tools|外部标讯|provider_profile\.endpoint_env|api_key_env|poll_endpoint_env|尚未接业务入口" README.md docs/blueprint/PAGE_ROUTE_MAP.md docs/blueprint/AI_PIPELINE.md docs/blueprint/SAMPLE_DOCS_EVALUATION.md infra/scripts/acceptance_tail_check.py
```

结果：

1. 尾部验收脚本语法检查通过。
2. diff 空白检查通过。
3. 关键路由、外部工具页签和 OCR profile env 文档锚点均可检索；`AI_PIPELINE.md` 已不再声明外部 MCP 尚未接业务入口。

### 偏离蓝图

1. 本轮是文档和验收防漂移，不新增外部 MCP Provider，也不配置真实 Handaas/MinerU/PaddleOCR 凭证。
2. 未运行完整 Docker 运行态验收；本轮未改业务运行时代码。

## Loop-81 / xparse 模块级质量门禁摘要 - 2026-06-18

### 本轮目标

1. 继续补强 xparse 六模块解析清单，把“是否需要复核”从全局计数细化到模块级。
2. 避免某个模块缺来源、低置信或空结果时，只能从全局 low_confidence_count 反推问题来源。
3. 保持 AutoRFP 式来源引用和人工复核链路，不把模型空结果当作通过。

### 代码交付

1. `ai-service/app/pipelines/parse/tender_parser.py` 新增模块质量摘要：`quality_gates.interpret.module_quality` 按 basic、qualification、evaluation、submission、invalid_risk、annex 记录字段数、证据数、要求项数、缺来源数、低置信数、要求项待复核数、可追溯来源数和 warning 数。
2. `quality_gates.interpret.review_modules` 明确列出需要人工复核的模块；模块状态为 `needs_review` / `empty`、存在缺来源、低置信或要求项待复核时，整体 interpret gate 会进入 `needs_review`。
3. `parse_metadata.module_quality` 和 `parse_metadata.review_modules` 同步保存同一摘要，便于离线评测和人工排查读取。
4. `test_main_security.py` 增加模块质量摘要断言，覆盖 deterministic 解析和模型增强解析两条路径。
5. `AI_IMPLEMENTATION_CHECKLIST.md`、`SAMPLE_DOCS_EVALUATION.md` 同步更新当前能力。

### 检查结果

已运行：

```bash
python3 -m py_compile ai-service/app/pipelines/parse/tender_parser.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py -q -s -k 'tender_parse or tender_module_context'
cd ai-service && .venv/bin/python -m pytest app/tests/test_tender_parse_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
```

结果：

1. `tender_parser.py` 语法检查通过。
2. tender parse 相关安全/主流程单测 9 项通过。
3. 解析评测单测 5 项通过。
4. 工程1真实样本解析评测 109/109 通过。

### 偏离蓝图

1. 本轮是解析质量门禁增强，不接真实 MinerU/PaddleOCR endpoint。
2. 模块质量摘要用于定位和复核，不替代人工对真实招标文件准确率的抽样核对。

## Loop-82 / OCR Provider env 验收实化 - 2026-06-18

### 本轮目标

1. 修正 OCR Provider 真实 endpoint 验收脚本与文档口径不一致的问题。
2. `ocr_provider_eval.py` 不应只检查 `endpoint_env`，还应在配置 key 或 poll endpoint 时检查 `api_key_env` / `poll_endpoint_env`。
3. MinerU/PaddleOCR 使用通用 OCR endpoint/key/poll 兜底时，验收必须接受实际生效 env，而不是误判为必须使用 Provider 专属 env。

### 代码交付

1. `ai-service/app/evaluation/ocr_provider_eval.py` 的 `ocr.provider_profile.endpoint_env` 检查改为匹配实际生效 env：Provider 专属 endpoint 优先，否则接受 `OCR_HTTP_ENDPOINT` 兜底。
2. 新增 `ocr.provider_profile.api_key_env` 检查：仅当 Provider 专属 key 或通用 `OCR_API_KEY` 已配置时启用，避免无 key 环境误失败。
3. 新增 `ocr.provider_profile.poll_endpoint_env` 检查：仅当 Provider 专属 poll endpoint 或通用 `OCR_POLL_ENDPOINT` 已配置时启用。
4. `provider.endpoint_configured` 的 expected 文案改为“Provider 专属 endpoint 或通用 OCR endpoint”，让 skipped/failed 输出更准确。
5. `test_ocr_provider_eval.py` 增加通用 OCR endpoint/key/poll 兜底验收用例，并补 Provider 专属 key env 断言。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录 OCR eval 已检查 endpoint/key/poll 实际生效 env。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_ocr_provider_eval.py -q -s
python3 -m py_compile ai-service/app/evaluation/ocr_provider_eval.py ai-service/app/tests/test_ocr_provider_eval.py
cd ai-service && .venv/bin/python -m app.evaluation.ocr_provider_eval --provider mineru --allow-skip
cd ai-service && .venv/bin/python -m app.evaluation.ocr_provider_eval --provider paddleocr --allow-skip
git diff --check
```

结果：

1. OCR Provider eval 单测 5 项通过。
2. OCR eval 脚本与测试语法检查通过。
3. 当前本地未配置 MinerU endpoint，工程1 OCR Provider 验收输出 `skipped provider=mineru passed=1/2`，没有伪装通过。
4. 当前本地未配置 PaddleOCR endpoint，工程1 OCR Provider 验收输出 `skipped provider=paddleocr passed=1/2`，没有伪装通过。
5. diff 空白检查通过。

### 偏离蓝图

1. 本轮增强真实 Provider 验收逻辑，不提供真实 MinerU/PaddleOCR endpoint 或 API Key。
2. `api_key_env` 和 `poll_endpoint_env` 只有在对应 env 已配置时才作为硬检查；无 key 或无轮询地址的同步 OCR Provider 不因此失败。

## Loop-83 / AutoRFP 生成来源引用细节门禁 - 2026-06-18

### 本轮目标

1. 把 AutoRFP 式 `Requirement -> Coverage -> Source` 生成验收从“有来源、能解析 chunk”推进到“来源可引用、可定位”。
2. 避免响应侧 `source_refs` 只保留 `chunk_id` 或摘要文本，缺少引用号、定位码、页码、文件或文档定位时仍被黄金样本误判通过。
3. 保持离线评测器可用于运行态 `GET /bids/:id/generation-coverage` 导出的 JSON，不引入数据库依赖。

### 代码交付

1. `ai-service/app/evaluation/generation_coverage_eval.py` 新增 `min_source_ref_reference_id_ratio` 和 `min_source_ref_location_ratio` 两个阈值。
2. 生成覆盖评测新增 `generation.source_refs.reference_id_ratio` 和 `generation.source_refs.location_ratio` 检查，分别验证响应来源是否具备 `citation_id/reference_id/locator/source_locator`，以及页码、chunk、文件或文档定位。
3. 来源解析兼容嵌套 `metadata.source_ref`，运行态导出或人工来源编辑把来源放入 metadata 时也能参与解析、引用号和定位完整性检查。
4. `test_generation_coverage_eval.py` 增加缺引用号和缺定位信息的失败样例，确保退化来源不会只因 chunk 可解析而通过。
5. `docs/sample_docs/golden/工程1.generation_coverage.json` 补齐工程1黄金样本的 `citation_id`、`reference_id`、页码和来源摘录，并把生成覆盖门槛提高到 9 项检查全部通过。
6. `AI_IMPLEMENTATION_CHECKLIST.md`、`SAMPLE_DOCS_EVALUATION.md` 同步记录响应侧来源引用结构验收口径。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_coverage_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.generation_coverage_eval --input ../docs/sample_docs/golden/工程1.generation_coverage.json
python3 -m py_compile ai-service/app/evaluation/generation_coverage_eval.py ai-service/app/tests/test_generation_coverage_eval.py
cd ai-service && .venv/bin/python -m ruff check app/evaluation/generation_coverage_eval.py app/tests/test_generation_coverage_eval.py
git diff --check
```

结果：

1. 生成覆盖评测单测 4 项通过。
2. 工程1生成覆盖黄金样本通过，mandatory coverage、source_ref resolution、引用号完整率和定位完整率均为 1.0，9/9 检查通过。
3. 评测脚本与测试语法检查通过。
4. Ruff 检查通过。
5. diff 空白检查通过。

### 偏离蓝图

1. 本轮校验响应来源引用结构完整性，不做来源文本与 PDF canvas 选区的视觉框选验证。
2. 引用号、定位码和页码能证明来源可追溯，但不能单独证明模型生成内容与来源语义完全一致；语义一致性仍需后续人工抽样或更强评测器。

## Loop-84 / 行业 MCP 与 Skills 雷达补充 - 2026-06-18

### 本轮目标

1. 回答“同行业是否有现成 MCP / Skills 可调用”的问题，并把结果固化到工程目录。
2. 区分可直接接入的外部只读数据 MCP、可内化的方法论 Skill、以及只能观察工具粒度的海外公共采购 MCP。
3. 保持边界：外部 MCP / Skill 不接管智标通解析、合规、生成、审计和成本核算主链路。

### 代码交付

1. `docs/blueprint/EXTERNAL_MCP_SKILL_RADAR.md` 新增“当前检索结论”，明确有可调用同类 MCP / Skills，但强项分散，优先作为只读数据源和 checklist。
2. Skill 方法论补充 `Hugin-Z/tender-writer-v4` 和 `JbBom/cn-bid-doc-automation`，用于中国技术标工作流、控制矩阵、废标风险、模板残留和最终 clean export 检查。
3. 全球采购数据 MCP 观察池补充 `Licinexus/licinexus-mcp`、`IDNSIDNS/tenderapi-mcp`、`ab75173/sam-gov-mcp`、`n0edlg/bidsparq-mcp-server` 和 `mayai-it/bandiradar`。

### 检查结果

已运行：

```bash
git diff --check
rg -n "当前检索结论|Hugin-Z/tender-writer-v4|JbBom/cn-bid-doc-automation|Licinexus/licinexus-mcp|IDNSIDNS/tenderapi-mcp|ab75173/sam-gov-mcp|n0edlg/bidsparq-mcp-server|mayai-it/bandiradar" docs/blueprint/EXTERNAL_MCP_SKILL_RADAR.md
```

结果：

1. diff 空白检查通过。
2. 新增候选和检索结论均能在雷达文档中检索到。

### 偏离蓝图

1. 本轮只做全网/GitHub 调研归档，不安装第三方 Skill，也不新增 Provider 运行时代码。
2. GitHub CLI 已安装但尚未完成账号授权；本轮仓库检索使用 GitHub API 和网页搜索结果交叉筛选。

## Loop-85 / 运行态生成覆盖门禁契约同步 - 2026-06-18

### 本轮目标

1. 修正运行态 `GET /bids/:id/generation-coverage` 导出契约滞后于离线评测器的问题。
2. 真实标书导出的 generation coverage JSON 必须默认触发 AutoRFP 响应来源的引用号/定位码和来源位置完整性门禁。
3. CLI 普通输出需要展示新增比率，避免只看到 source_ref resolution 而忽略引用号或定位退化。

### 代码交付

1. `backend/internal/platform/bid/store.go` 的 `buildGenerationCoverageSpec()` 默认阈值新增：
   - `min_source_ref_reference_id_ratio=1`
   - `min_source_ref_location_ratio=1`
2. `backend/internal/platform/bid/store_test.go` 更新导出契约测试，断言运行态 spec 保留章节 `source_refs` 中的 `citation_id` 和 `page_start`，并携带全部四个评测阈值。
3. `ai-service/app/evaluation/generation_coverage_eval.py` 普通 CLI 输出新增 `source_ref_reference_id` 和 `source_ref_location` 摘要。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录运行态导出已携带引用号/定位完整性阈值。

### 检查结果

已运行：

```bash
cd backend && go test ./internal/platform/bid -run 'TestBuildGenerationCoverageSpecUsesEvaluatorContract'
cd backend && go test ./internal/api -run 'TestRouteInfosExposeGenerationCoverageAsReadOnlyBidRoute'
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_coverage_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.generation_coverage_eval --input ../docs/sample_docs/golden/工程1.generation_coverage.json
python3 -m py_compile ai-service/app/evaluation/generation_coverage_eval.py
cd ai-service && .venv/bin/python -m ruff check app/evaluation/generation_coverage_eval.py app/tests/test_generation_coverage_eval.py
git diff --check
```

结果：

1. Go 生成覆盖导出契约单测通过。
2. Go API 路由只读权限元数据单测通过。
3. Python 生成覆盖评测单测 4 项通过。
4. 工程1生成覆盖黄金样本通过，普通输出显示 mandatory coverage、source_ref resolution、引用号完整率、来源位置完整率，9/9 检查通过。
5. Python 语法检查、Ruff 和 diff 空白检查通过。

### 偏离蓝图

1. 本轮同步运行态导出阈值，不改历史已生成章节的旧 source_refs；旧数据若缺引用号会被评测器正确判为待修复。
2. 新阈值证明来源结构完整，不替代来源语义一致性的人工抽样或后续语义评测。

## Loop-86 / 生成来源引用归一化与验收门禁 - 2026-06-18

### 本轮目标

1. 修复生成源头仍可能只保存 `chunk_id/document_id/page_start`，但运行态评测已要求引用号/定位码的断层。
2. 章节顶层 `source_refs`、`self_check.requirement_coverage[].source_refs` 和 `knowledge_references.metadata.source_ref` 必须共享同一套可追溯定位字段。
3. 核心运行态验收脚本不能只检查“source_refs 非空”，还要检查引用号/定位码和来源位置线索。

### 代码交付

1. `backend/internal/platform/bid/store.go` 新增生成来源归一化：
   - `sourceRefsAsAny()` 会从 chunk/document/page 派生 `citation_id`、`reference_id`、`source_locator` 和 `locator`。
   - `chapterVersionModelMetadata()` 会补齐 `self_check.requirement_coverage[].source_refs` 中缺失的引用号和页码定位。
   - `syncRequirementCoverageStatuses()` 通过已归一化 self_check 写入 `metadata.latest_coverage` 和覆盖历史。
   - `replaceKnowledgeReferences()` 的 metadata.source_ref 复用同一归一化结果，并追加 trace、resolved 和章节标题。
2. `backend/internal/platform/bid/store_test.go` 增加和扩展单测，覆盖章节 source_refs 派生稳定 locator，以及覆盖矩阵 source_refs 从章节来源补齐 citation/reference/page。
3. `infra/scripts/acceptance_core_check.py` 增加 `require_traceable_source_refs()`，生成章节验收现在要求 source_refs 具备 citation/reference/locator 和 page/chunk/file/document 定位，并检查 `/generation-coverage` 导出默认携带新阈值。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录生成回调来源引用归一化。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_core_check.py
cd backend && go test ./internal/platform/bid -run 'TestSourceRefsAsAnyAddsTraceableCitationAndLocator|TestChapterVersionModelMetadataCarriesSelfCheckCoverage|TestBuildGenerationCoverageSpecUsesEvaluatorContract'
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_coverage_eval.py -q -s
cd ai-service && .venv/bin/python -m app.evaluation.generation_coverage_eval --input ../docs/sample_docs/golden/工程1.generation_coverage.json
git diff --check
```

结果：

1. 核心验收脚本语法检查通过。
2. Go 生成来源归一化和导出契约相关单测通过。
3. Python 生成覆盖评测单测 4 项通过。
4. 工程1生成覆盖黄金样本 9/9 通过。
5. diff 空白检查通过。

### 偏离蓝图

1. 本轮派生的是稳定定位码，不要求模型自然语言输出自带同名字段。
2. 核心验收脚本仍依赖运行中的服务版本；若服务未重建，旧运行态仍会按新脚本失败。

## Loop-87 / generation coverage 运行态验收加严 - 2026-06-18

### 本轮目标

1. 核心验收不能只检查生成章节顶层 `source_refs`，还要检查 `GET /bids/:id/generation-coverage` 导出的 AutoRFP 响应矩阵契约。
2. 运行态导出必须包含要求项、覆盖矩阵行，以及覆盖矩阵行上的可追溯响应来源。
3. 保持验收脚本轻量，不在 acceptance 中额外依赖 Python venv 或离线评测器执行环境。

### 代码交付

1. `infra/scripts/acceptance_core_check.py` 新增 `coverage_rows_from_generation_spec()`，可从章节顶层、`model_metadata.requirement_coverage` 或 `model_metadata.self_check.requirement_coverage` 提取覆盖矩阵。
2. 新增 `row_source_refs()` 和 `require_generation_coverage_contract()`：
   - 检查 `thresholds.min_source_ref_reference_id_ratio=1`。
   - 检查 `thresholds.min_source_ref_location_ratio=1`。
   - 检查 `requirements` 非空。
   - 检查 `requirement_coverage` 行非空。
   - 检查每行 `source_refs` 具备引用号/定位码和页码、chunk、文件或文档定位。
3. 生成章节验收输出新增 `coverage.requirements` 和 `coverage.coverage_rows`，便于定位 acceptance 失败点。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 同步记录核心验收脚本已覆盖运行态 generation coverage 契约。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_core_check.py
python3 - <<'PY'
import importlib.util
from pathlib import Path
path = Path('infra/scripts/acceptance_core_check.py')
spec = importlib.util.spec_from_file_location('acceptance_core_check', path)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
coverage = mod.require_generation_coverage_contract({
    'thresholds': {
        'min_source_ref_reference_id_ratio': 1,
        'min_source_ref_location_ratio': 1,
    },
    'requirements': [{'id': 'req-1'}],
    'chapters': [{
        'requirement_coverage': [{
            'requirement_id': 'req-1',
            'source_refs': [{
                'chunk_id': 'chunk-1',
                'document_id': 'doc-1',
                'citation_id': 'SRC-1',
                'page_start': 1,
            }],
        }],
    }],
})
print(coverage)
PY
git diff --check
```

结果：

1. 核心验收脚本语法检查通过。
2. generation coverage contract helper 冒烟通过，输出 `{'requirements': 1, 'coverage_rows': 1}`。
3. diff 空白检查通过。

### 偏离蓝图

1. 本轮没有启动完整 Docker 栈跑端到端 acceptance；只加严脚本契约并做静态/函数级验证。
2. 脚本仍不替代 `generation_coverage_eval.py` 的完整离线评测，acceptance 只覆盖运行态关键结构。

## Loop-88 / AI 调用费用回调闭环 - 2026-06-18

### 本轮目标

1. 修补 Python AI 服务只在内存 Router 里支持 `log_call()`，但业务回调不主动携带 `estimated_cost` 的断层。
2. 让 tender_parse 六模块、knowledge embedding、chapter_generate、chapter_action 和 cost_advice 成功调用后都进入同一套费用估算和 quota 记录。
3. 继续保持 Go 后端价格表兜底，避免 AI 服务与后端任一侧缺配置时审计链路直接断开。

### 代码交付

1. `ai-service/app/gateway/model_router.py` 新增 `estimate_cost()` 和价格表匹配逻辑，支持 `AI_MODEL_PRICING_JSON` 或配置内 `pricing`，匹配顺序与 Go 后端一致：`provider/model`、`model`、`provider/*`、`*`，并支持 `input_per_1k/output_per_1k` 与 `input_per_1m/output_per_1m`。
2. `ModelRouter.log_call()` 在调用方未显式传入正数 `estimated_cost` 时，会按 provider/model/token 用量估算费用并写入租户运行期 quota usage；内存账本使用可重入锁保护，适配 tender_parse 六模块并发调用。
3. `ai-service/app/main.py` 新增 `ai_call_accounting()` 和 `enrich_ai_response_accounting()`：
   - 六模块 tender_parse 每个模块单独记录费用，回调的 `model_metadata.module_calls[]`、`model_metadata.estimated_cost` 和顶层 `estimated_cost` 都带出费用。
   - knowledge embedding 回调 `model_metadata` 和顶层结果携带费用与 quota 快照。
   - chapter_generate、chapter_action、cost_advice 在 Pydantic 响应回调前补齐 `model_metadata.estimated_cost` 和 `quota_usage`。
4. `ai-service/app/tests/test_model_router.py` 增加价格表派生费用和配置内 per-million 计价测试。
5. `ai-service/app/tests/test_main_security.py` 增加招标解析回调费用断言，确认六模块费用会汇总到回调结果。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 和 `MODEL_GATEWAY.md` 同步记录 AI 服务回调已主动携带费用估算，Go 后端仍保留落库兜底。

### 检查结果

已运行：

```bash
python3 -m py_compile ai-service/app/gateway/model_router.py ai-service/app/main.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_model_router.py -q -s
cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/gateway/model_router.py app/main.py app/tests/test_model_router.py app/tests/test_main_security.py
cd backend && go test ./internal/platform/aicall
git diff --check
```

结果：

1. Python 语法检查通过。
2. ModelRouter 单测 48 项通过，覆盖价格表估算费用、quota 累计和 fallback 策略。
3. AI 服务安全/任务回调单测 62 项通过，覆盖招标解析六模块回调费用汇总。
4. 后端 AI 调用审计包测试通过，价格表落库兜底仍然有效。
5. Ruff 和 diff 空白检查通过。

### 偏离蓝图

1. 本轮没有接入真实厂商账单 API；费用仍按配置价格表和 token 估算。
2. Python 侧 quota 是进程内运行期账本，重启后不作为持久化限额依据；持久化审计仍以 Go 后端 `ai_call_logs` 为准。

## Loop-89 / 实时检索 Provider 审计字段闭环 - 2026-06-18

### 本轮目标

1. 补齐同步 `/embeddings/knowledge` 和 `/rerank/knowledge` 端点没有返回 token/费用审计字段的缺口。
2. 让 Go 知识库搜索写 `ai_call_logs` 时优先使用 AI 服务返回的 Provider token/费用，保留本地估算兜底。
3. 保持响应扩展兼容旧调用方，不改变 embedding/rerank 主业务返回结构。

### 代码交付

1. `ai-service/app/schemas/knowledge.py` 为 `KnowledgeEmbeddingResponse` 和 `KnowledgeRerankResponse` 增加默认字段：`token_usage`、`estimated_cost`、`quota_usage`。
2. `ai-service/app/main.py` 在同步 embedding/rerank 调用成功后执行 `ai_call_accounting()`：
   - embedding 按输入文本计算 `input_tokens`，`output_tokens=0`。
   - rerank 按 query、候选文档和排序结果 JSON 计算 input/output token。
   - 两个同步响应都返回费用估算和当前 quota 快照。
3. `backend/internal/platform/knowledge/store.go` 的 `embeddingResponse` / `rerankResponse` 接收新增字段；写 `ai_call_logs` 时优先使用响应里的 `token_usage` 和 `estimated_cost`，无效或缺失时退回本地 token 估算和后端价格表。
4. `ai-service/app/tests/test_main_security.py` 增加同步 embedding/rerank 响应审计字段断言。
5. `backend/internal/platform/knowledge/store_test.go` 增加 token/费用归一化单测，防止负数、NaN 或无穷费用进入日志。
6. `AI_IMPLEMENTATION_CHECKLIST.md` 和 `MODEL_GATEWAY.md` 同步记录实时检索端点审计字段闭环。

### 检查结果

已运行：

```bash
python3 -m py_compile ai-service/app/main.py ai-service/app/schemas/knowledge.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/main.py app/schemas/knowledge.py app/tests/test_main_security.py
cd backend && go test ./internal/platform/knowledge
cd backend && go test ./internal/platform/aicall
git diff --check
```

结果：

1. Python 语法检查通过。
2. AI 服务安全/任务回调单测 62 项通过，覆盖同步 embedding/rerank 的费用响应字段。
3. 后端知识库包测试通过，覆盖响应 token/费用优先级和安全归一化。
4. 后端 AI 调用审计包测试通过。
5. Ruff 和 diff 空白检查通过。

### 偏离蓝图

1. 本轮仍未接真实厂商账单 API；费用来源仍为 token 用量乘配置价格表。
2. 同步响应新增字段为兼容扩展，旧前端和旧 Go 客户端可忽略这些字段。

## Loop-90 / OCR 异步轮询 SSRF 边界加固 - 2026-06-18

### 本轮目标

1. 审查 MinerU/PaddleOCR/通用 HTTP OCR 异步轮询路径，避免 Provider 响应中的 `status_url` / `poll_url` 把服务端带到非预期地址。
2. 防止 OCR API Key 被转发给响应体指定的任意跨域轮询地址。
3. 保留真实 Provider 常见的跨域结果服务能力，但必须由部署侧显式配置 `*_POLL_ENDPOINT`。

### 代码交付

1. `ai-service/app/pipelines/parse/document_parser.py` 收紧 `_ocr_poll_endpoint()`：
   - 响应体里的 `status_url` / `poll_url` / `result_url` 只允许提交 endpoint 同源或相对路径。
   - 显式配置的 `OCR_POLL_ENDPOINT` / `MINERU_POLL_ENDPOINT` / `PADDLEOCR_POLL_ENDPOINT` 可跨域，但仍走既有 URL 安全校验。
   - 被拒绝的响应轮询地址返回安全失败 `ocr async poll target rejected`，不暴露恶意 URL。
2. 新增 `_same_origin_endpoint()` / `_url_port()`，按 scheme、hostname 和默认端口判断同源。
3. `ai-service/app/tests/test_document_parser.py` 新增测试：
   - 响应体返回跨域 `status_url` 时只发起提交请求，不继续 GET，也不泄露 Authorization。
   - 显式配置跨域 `OCR_POLL_ENDPOINT` 时仍允许轮询并正常归一化结果。
4. `AI_IMPLEMENTATION_CHECKLIST.md` 和 `AI_PIPELINE.md` 同步记录异步 OCR 轮询边界。

### 检查结果

已运行：

```bash
python3 -m py_compile ai-service/app/pipelines/parse/document_parser.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py -q -s
cd ai-service && .venv/bin/python -m pytest app/tests/test_ocr_provider_eval.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
git diff --check
```

结果：

1. Python 语法检查通过。
2. 文档解析测试 49 项通过，覆盖 OCR 异步轮询、响应过大、错误摘要和 URL/header 安全。
3. OCR Provider 评测测试 5 项通过。
4. Ruff 和 diff 空白检查通过。

### 偏离蓝图

1. 本轮是安全边界加固，不接真实 MinerU/PaddleOCR endpoint。
2. 若某真实 OCR 厂商必须使用跨域响应轮询地址，需要通过部署环境显式配置 `*_POLL_ENDPOINT`，不能信任响应体直接指定跨域目标。

## Loop-91 / OCR endpoint 端口合法性校验 - 2026-06-18

### 本轮目标

1. 继续审查 OCR Provider URL 安全边界，补齐非法端口未在配置校验阶段拦截的问题。
2. 避免 `https://host:bad/path` 或越界端口进入外部请求层，统一返回安全失败。

### 代码交付

1. `ai-service/app/pipelines/parse/document_parser.py` 的 `_url_port()` 现在会捕获 `urllib.parse` 对非法端口抛出的 `ValueError`，并转为 OCR endpoint 校验错误。
2. `_safe_ocr_endpoint()` 会显式调用 `_url_port()`，确保主 OCR endpoint 和显式配置的 poll endpoint 都在发请求前校验端口合法性。
3. `ai-service/app/tests/test_document_parser.py` 在非法 endpoint 参数化样例中加入非数字端口和越界端口，确认不会发出外部请求。

### 检查结果

已运行：

```bash
python3 -m py_compile ai-service/app/pipelines/parse/document_parser.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_document_parser.py app/tests/test_ocr_provider_eval.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/pipelines/parse/document_parser.py app/tests/test_document_parser.py
git diff --check
```

结果：

1. Python 语法检查通过。
2. 文档解析与 OCR Provider 评测测试合计 56 项通过。
3. Ruff 和 diff 空白检查通过。

### 偏离蓝图

1. 本轮只加固 URL 校验，不改变 OCR Provider 请求格式或响应归一化。

## Loop-92 / 模型解析来源引用反查 - 2026-06-18

### 本轮目标

1. 审查招标解析模型增强链路，避免模型返回“看似带 citation/reference 的来源”但原文并不存在时仍被当作可信证据。
2. 保持 AutoRFP 式 `Requirement -> Source` 不只校验字段形状，还要能反查到当前模块解析上下文。
3. 修复 requirement item 已经 `needs_review=true` 时模块状态仍可能停留在 `done` 的缺陷。

### 代码交付

1. `ai-service/app/pipelines/parse/tender_parser.py` 导出 `tender_module_source_context_records()`，复用模块 prompt 的同一份 chunk/table 上下文。
2. `merge_tender_module_result()` 增加可选 `source_context_records` 参数；模型证据归一化时按 `chunk_id`、`table_block_id`、页码或 citation 解析出的锚点筛选上下文，并确认 `source_text` 真实出现在上下文中。
3. 未能反查到原文的模型来源会被降级：`traceable=false`、`needs_review=true`、置信度封顶 0.5，并追加人工复核提示。
4. `_normalized_model_module_result()` 现在同时检查字段证据和 requirement item 的 `needs_review`，避免要求项已降级但模块状态仍显示完成。
5. `ai-service/app/main.py` 在模块并发解析和最终合并时传入对应模块上下文，保证运行态回调结果也执行来源反查。
6. `ai-service/app/tests/test_main_security.py` 新增伪造来源回归测试，覆盖 citation/chunk 看似完整但原文不存在的模型输出。

### 检查结果

已运行：

```bash
python3 -m py_compile ai-service/app/pipelines/parse/tender_parser.py ai-service/app/main.py ai-service/app/tests/test_main_security.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py::test_merge_tender_module_result_downgrades_unverified_model_source_refs app/tests/test_main_security.py::test_process_tender_parse_uses_model_provider_and_callback app/tests/test_tender_parse_eval.py -q -s
cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py::test_build_tender_structured_result_extracts_business_fields app/tests/test_main_security.py::test_tender_module_context_routes_chunks_and_tables_with_source_anchors app/tests/test_main_security.py::test_process_tender_parse_runs_modules_with_configured_concurrency app/tests/test_main_security.py::test_process_tender_parse_falls_back_when_primary_provider_call_fails app/tests/test_main_security.py::test_process_tender_parse_keeps_result_when_one_module_enhancement_fails -q -s
cd ai-service && .venv/bin/python -m ruff check app/pipelines/parse/tender_parser.py app/main.py app/tests/test_main_security.py
```

结果：

1. Python 语法检查通过。
2. 招标解析来源反查、模型 Provider 回调和解析 golden 相关测试 7 项通过。
3. 解析结构、模块上下文、并发、fallback 和单模块失败保留结果测试 5 项通过。
4. Ruff 检查通过。

### 偏离蓝图

1. 本轮验证的是模型返回 `source_text` 与解析上下文的一致性，不做 PDF canvas 可视高亮框选。
2. 引用原文反查可以降低伪造来源风险，但仍不等价于语义正确性评测；复杂表格跨行引用仍需要更强的版面级评测或人工抽样。

## Loop-93 / 总检脚本 AI pytest 覆盖修复 - 2026-06-18

### 本轮目标

1. 审查全工程验证入口，修复 `infra/scripts/check.sh` 在系统 Python 缺少 pytest 时跳过 AI 服务测试但仍输出 `all checks passed` 的盲区。
2. 让本地总检优先使用 `ai-service/.venv`，与日常 AI 服务测试环境一致。
3. 规避当前 WSL/Windows 路径环境下 pytest 默认 capture 偶发 `FileNotFoundError`，确保总检能稳定跑完整 AI 测试集。

### 代码交付

1. `infra/scripts/check.sh` 在 AI 服务阶段新增 `AI_PYTHON` 选择：
   - 若 `ai-service/.venv/bin/python` 可执行，优先使用虚拟环境 Python。
   - 否则回退系统 `python3`。
2. `compileall` 和 pytest 均使用同一个 `AI_PYTHON`，避免编译和测试环境不一致。
3. AI 服务 pytest 改为 `-q -s`，禁用 capture 并保留简洁输出，避免本机捕获临时文件问题导致总检失败。

### 检查结果

已运行：

```bash
./infra/scripts/check.sh
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd frontend && pnpm lint
```

结果：

1. 总检通过：前端 production build 通过，后端 `go test ./...` 通过，AI 服务 compileall 通过。
2. 总检已真实执行 AI 服务测试，输出 `237 passed`，不再跳过本地 pytest。
3. 独立 AI 服务完整测试 237 项通过。
4. 前端 ESLint 通过。

### 偏离蓝图

1. Docker 中的 AI 服务容器未运行，因此总检仍按既有逻辑跳过容器内 pytest；本轮修复的是本地 AI 服务 pytest 盲区。

## Loop-94 / 总检脚本静态检查收口 - 2026-06-18

### 本轮目标

1. 继续审查全工程验证入口，把前端 lint、Go vet 和 AI ruff 从手动补跑收口到统一总检脚本。
2. 避免 `./infra/scripts/check.sh` 只验证构建/测试，而漏掉静态规则、vet 和 Python lint。
3. 保持检查入口仍能在本机 WSL 环境稳定执行。

### 代码交付

1. `infra/scripts/check.sh` 在前端 `pnpm build` 后新增 `pnpm lint`。
2. 后端 `go test ./...` 后新增 `GOTOOLCHAIN=local go vet ./...`。
3. AI 服务在 `compileall` 后新增 ruff 检查；若当前 Python 环境没有 ruff，输出明确跳过提示，避免静默失败。

### 检查结果

已运行：

```bash
cd frontend && pnpm lint
cd backend && GOTOOLCHAIN=local go vet ./...
cd ai-service && .venv/bin/python -m ruff check app
./infra/scripts/check.sh
```

结果：

1. 前端 ESLint 通过。
2. 后端 Go vet 通过。
3. AI 服务 ruff 通过。
4. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest 均通过，AI pytest 输出 `237 passed`。

### 偏离蓝图

1. 本轮不改变业务逻辑；只收紧本地/CI 可复用的质量门禁。
2. Docker 中 AI 服务容器未运行，容器内 pytest 仍按既有逻辑跳过。

## Loop-95 / AI 本地质量门禁强制执行 - 2026-06-18

### 本轮目标

1. 继续审查总检脚本，避免 AI 服务本地 ruff 或 pytest 缺失时仍输出 `all checks passed`。
2. 保证 `./infra/scripts/check.sh` 在本地/CI 环境中对 AI 服务质量门禁给出强失败，而不是静默跳过。
3. 让容器内 pytest 与本地 pytest 使用一致的 `-q -s` 参数，减少环境差异。

### 代码交付

1. `infra/scripts/check.sh` 中 AI 服务 ruff 和 pytest 从“可选跳过”改为“必须可用”：
   - 缺少 ruff 时输出 `ai-service ruff unavailable; install ai-service dev dependencies` 并退出 1。
   - 缺少 pytest 时输出 `ai-service pytest unavailable; install ai-service dev dependencies` 并退出 1。
2. 容器内 AI 服务 pytest 改为 `python -m pytest app/tests -q -s`，与本地执行参数一致。

### 检查结果

已运行：

```bash
./infra/scripts/check.sh
```

结果：

1. 前端 build 和 lint 通过。
2. 后端 `go test ./...` 和 `go vet ./...` 通过。
3. AI 服务 compileall、ruff 和 pytest 均通过，pytest 输出 `237 passed`。
4. Docker 中 AI 服务容器未运行，容器内 pytest 按既有逻辑跳过。

### 偏离蓝图

1. 本轮仍只收紧工程验证入口，不改变业务功能。

## Loop-96 / 工程1黄金样本纳入总检 - 2026-06-18

### 本轮目标

1. 继续按标准招投标文件验证系统功能可用性，避免黄金样本只停留在手动验收。
2. 将工程1的解析、投标文件生成覆盖、导出格式验收纳入 `./infra/scripts/check.sh`。
3. 让后续每次总检都能回归核心业务闭环，而不只验证单元测试和静态检查。

### 代码交付

1. `infra/scripts/check.sh` 在 AI 服务 pytest 后新增工程1黄金样本回归：
   - `app.evaluation.tender_parse_eval --golden docs/sample_docs/golden/工程1.parse.json`
   - `app.evaluation.generation_coverage_eval --input docs/sample_docs/golden/工程1.generation_coverage.json`
   - `app.evaluation.export_format_eval --input docs/sample_docs/golden/工程1.export.json`
2. 复用本地 `ai-service/.venv/bin/python` 优先策略，保证总检与手动验收使用同一 Python 环境。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m app.evaluation.tender_parse_eval --golden ../docs/sample_docs/golden/工程1.parse.json
cd ai-service && .venv/bin/python -m app.evaluation.generation_coverage_eval --input ../docs/sample_docs/golden/工程1.generation_coverage.json
cd ai-service && .venv/bin/python -m app.evaluation.export_format_eval --input ../docs/sample_docs/golden/工程1.export.json
./infra/scripts/check.sh
```

结果：

1. 工程1解析黄金样本通过：`score=1.0 passed=109/109`。
2. 工程1生成覆盖验收通过：`mandatory_coverage=1.0 source_ref_resolution=1.0 source_ref_reference_id=1.0 source_ref_location=1.0 passed=9/9`。
3. 工程1导出格式验收通过：`passed=23/23 name=工程1.export`。
4. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest + 工程1三项黄金样本回归全部通过。

### 偏离蓝图

1. OCR Provider 的真实外部端点验收仍未纳入总检，因为它依赖部署环境中的 provider 地址和凭据；本轮先固化可离线重复执行的业务黄金样本。
2. Docker 中 AI 服务容器未运行，容器内 pytest 仍按既有逻辑跳过。

## Loop-97 / 尾部验收静态防漂移实化 - 2026-06-18

### 本轮目标

1. 继续审查全工程验收脚本，修正尾部验收脚本对 DEV_LOOP_LOG 的过期断言。
2. 避免脚本声称检查最新 Loop 记录，却只硬编码匹配早期 `Loop-26`。
3. 将不依赖运行态服务的静态防漂移检查纳入 `./infra/scripts/check.sh`。

### 代码交付

1. `infra/scripts/acceptance_tail_check.py` 拆出 `check_model_router_health()` 与 `check_static_docs()`。
2. 新增 `latest_loop_section()`，从 DEV_LOOP_LOG 末尾解析最新 Loop 段落，并要求最新段落同时包含“本轮目标 / 代码交付 / 检查结果 / 偏离蓝图”。
3. `acceptance_tail_check.py --static-docs` 支持离线执行模型路由、README、页面路由、AI_PIPELINE、SAMPLE_DOCS_EVALUATION 和最新 Loop 结构检查。
4. `infra/scripts/check.sh` 在语法检查后执行 `acceptance_tail_check.py --static-docs`。
5. `README.md` 同步更新总检覆盖范围和 `--static-docs` 用法。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
git diff --check
./infra/scripts/check.sh
```

结果：

1. 尾部验收脚本语法检查通过。
2. 静态防漂移检查通过，能识别最新 Loop 段落并检查必需小节。
3. `git diff --check` 通过，无空白错误。
4. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest、工程1三项黄金样本回归、尾部验收静态防漂移检查均通过。

### 偏离蓝图

1. 本轮未运行完整 `acceptance_core_check.py` / `acceptance_tail_check.py` 运行态验收；它们仍需要本地 Docker 栈启动并会创建运行态验收数据。
2. Docker 中 AI 服务容器未运行时，容器内 pytest 仍按既有逻辑跳过。

## Loop-98 / 外部数据源 UI 技术口径收口 - 2026-06-18

### 本轮目标

1. 继续审查前端可见文案，处理团队页外部数据源配置中残留的开发者口径。
2. 避免租户管理员在界面里看到环境变量名、底层工具名或服务商 endpoint 模板。
3. 将该类 UI 防漂移纳入已有静态验收入口。

### 代码交付

1. `frontend/src/features/team/index.tsx` 为外部数据源工具名增加中文业务化显示名，调用记录列从“工具”改为“能力”。
2. 外部数据源列表和配置弹窗不再展示 `token_env`，改为“授权状态 / 授权说明”。
3. 启用项文案从“工具”统一改为“能力”，受控标签从“受控工具”改为“受控调用”。
4. 访问地址输入框不再展示后端 `endpoint_hint` 模板，改为通用服务商访问地址提示。
5. `acceptance_tail_check.py --static-docs` 增加团队页防漂移检查：禁止渲染 `preset.token_env` 和 `endpoint_hint`，并要求保留“授权状态 / 授权说明 / 启用能力”用户侧标签。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
./infra/scripts/check.sh
```

结果：

1. 前端团队页源码检查无 `preset.token_env`、`endpoint_hint`、旧“密钥项 / 密钥配置项 / 启用工具 / 已选工具”残留。
2. 静态防漂移检查通过。
3. 前端 ESLint 通过。
4. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest、工程1三项黄金样本回归均通过。

### 偏离蓝图

1. 本轮只处理团队页外部数据源配置和调用记录的可见技术口径；其它页面仍需继续逐页审查。
2. 未运行完整浏览器截图回归；本轮通过 TypeScript build、ESLint 和总检验证渲染代码可构建。

## Loop-99 / 标书解析确认复杂字段去 JSON 化 - 2026-06-18

### 本轮目标

1. 继续审查标书编制流程的用户可见技术口径，处理解析确认页模块字段直接展示 JSON 的问题。
2. 让复杂数组/对象字段以中文摘要呈现，避免用户在文本框里看到原始结构、括号和技术键名。
3. 保持未编辑复杂字段保存时仍保留原始结构，避免“打开确认页再保存”造成数据降级。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的模块字段展示从 `JSON.stringify(value, null, 2)` 改为 `parseModuleObjectSummary()` / `parseModuleFieldItemSummary()` 生成中文摘要。
2. 新增复杂字段摘要标签映射，过滤 `id`、`metadata`、`source_refs`、`confidence` 等内部字段。
3. `parseModuleFieldDraftValue()` 在文本未变化时返回原始值；人工编辑复杂字段时再按可读文本写回。
4. 删除模块字段编辑链路中的 JSON 解析要求，用户不需要编辑 JSON 才能保存。
5. `acceptance_tail_check.py --static-docs` 增加标书页防漂移检查，禁止重新使用 `JSON.stringify(value, null, 2)` 作为字段展示。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. 静态防漂移检查通过。
2. 前端 ESLint 通过。
3. 前端生产构建通过。
4. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest、工程1三项黄金样本回归均通过。

### 偏离蓝图

1. 本轮修复的是标书解析确认页的复杂字段展示，不等同于完成全站所有页面的逐像素 UI 验收。
2. 未运行浏览器截图检查；本轮使用源码静态防漂移、lint、build 和总检验证。

## Loop-100 / 合规规则 UI 编号字段收口 - 2026-06-18

### 本轮目标

1. 继续审查合规页面用户可见字段，移除规则编码和标书编号等内部标识录入。
2. 让租户管理员通过业务名称选择标书、创建规则，避免暴露后端唯一键或对象 ID。
3. 将合规页面技术字段防漂移纳入静态验收脚本。

### 代码交付

1. `frontend/src/features/compliance/index.tsx` 创建规则弹窗移除“规则编号”字段，提交时基于层级、分类、规则名称自动生成内部 `code`。
2. 规则列表不再展示“编码”列，改为展示用户可理解的“说明”列。
3. 发起合规检查时，关联标书从手填编号改为通过 `fetchBids()` 获取标书并以下拉选择呈现。
4. `acceptance_tail_check.py --static-docs` 新增合规页面检查，禁止“规则编号”“填写标书编号”和编码列回流，并要求保留自动编码与标书选择入口。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. 静态防漂移检查通过，最新交付日志识别为 Loop-100。
2. 前端 ESLint 和生产构建通过。
3. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest、工程1三项黄金样本回归均通过。
4. Docker 中 AI 服务容器未运行时，容器内 pytest 仍按既有逻辑跳过。

### 偏离蓝图

1. 后端仍保留 `code` 作为合规规则唯一键；本轮只把它从用户录入界面收口为前端自动生成。
2. 未新增浏览器截图验收；本轮先用静态防漂移、lint、build 和总检验证。

## Loop-101 / 外部数据源调用记录名称收口 - 2026-06-18

### 本轮目标

1. 继续审查团队页外部数据源模块，处理调用记录里残留的 provider key 展示问题。
2. 让租户管理员在调用记录中看到业务数据源名称，而不是 `tool_provider` 对应的内部标识。
3. 把这类调用日志字段展示约束纳入静态验收。

### 代码交付

1. `frontend/src/features/team/index.tsx` 新增 `externalToolDisplayNameMap`，优先使用外部数据源目录名称，目录缺失时使用已配置名称。
2. 调用记录“数据源”列不再直接展示 `tool_provider`，统一通过 `formatExternalToolProviderName()` 渲染为业务名称；未知来源显示“外部数据源”。
3. `acceptance_tail_check.py --static-docs` 新增团队页调用记录守卫，禁止回到直接绑定 `tool_provider` 的展示方式，并要求保留名称映射函数。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. 静态防漂移检查通过，最新交付日志识别为 Loop-101。
2. 前端 ESLint 和生产构建通过。
3. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest、工程1三项黄金样本回归均通过。
4. Docker 中 AI 服务容器未运行时，容器内 pytest 仍按既有逻辑跳过。

### 偏离蓝图

1. 后端审计日志仍保存 `tool_provider` 作为内部追踪键；本轮只收口用户侧展示。
2. 未新增浏览器截图验收；本轮先用静态防漂移、lint、build 和总检验证。

## Loop-102 / 响应矩阵导出文件名业务化 - 2026-06-18

### 本轮目标

1. 继续审查标书向导导出链路，处理响应矩阵下载文件名暴露标书 UUID 或缺少业务语义的问题。
2. 让 CSV/XLSX 响应矩阵文件名包含标书标题和时间戳，便于用户在本地文件夹识别归档。
3. 将导出文件名防退化约束纳入静态验收。

### 代码交付

1. `backend/internal/api/routes.go` 的响应矩阵导出接口先读取标书信息，并通过 `bidRequirementExportFilename()` 生成 `响应矩阵-<标书标题>-<时间>.csv` 或 `响应矩阵-覆盖历史-<标书标题>-<时间>.xlsx`。
2. 新增 `downloadSafeFilenamePart()` 清洗 Windows/macOS/Linux 文件名不安全字符，标题为空时回退“标书”。
3. `frontend/src/shared/api/client.ts` 的导出兜底文件名不再使用 `bidId`，改为 `bidRequirementFallbackFilename()` + 当前标书标题。
4. `frontend/src/features/bid/index.tsx` 调用导出接口时传入当前标书标题。
5. `backend/internal/api/routes_test.go` 增加业务标题文件名和非法字符清洗单测。
6. `acceptance_tail_check.py --static-docs` 增加防漂移检查，禁止 `响应矩阵-${bidId}` 回流，并要求保留前后端文件名清洗入口。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && go test ./internal/api
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. 静态防漂移检查通过，最新交付日志识别为 Loop-102。
2. 后端 API 单测通过，覆盖业务标题文件名和非法字符清洗。
3. 前端 ESLint 和生产构建通过。
4. 总检通过：前端 build + lint、后端 test + vet、AI compileall + ruff + pytest、工程1三项黄金样本回归均通过。
5. Docker 中 AI 服务容器未运行时，容器内 pytest 仍按既有逻辑跳过。

### 偏离蓝图

1. 本轮只处理响应矩阵 CSV/XLSX 的下载文件名，不改动 DOCX/PDF/ZIP 正式标书导出物的命名策略。
2. 未新增浏览器下载行为截图验收；本轮以单测、静态防漂移、lint、build 和总检验证。

## Loop-103 / 标书编辑器副标题 ID 收口 - 2026-06-18

### 本轮目标

1. 继续审查标书编辑器页面标题区，处理副标题兜底展示 URL 中 `bidId` 的问题。
2. 避免用户在主工作区看到 UUID 类内部标识。
3. 将该副标题防退化约束纳入静态验收。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的标书编辑器 `PageFrame` 副标题从 `bid.data?.title ?? bidId` 改为 `bid.data?.title || '正在编辑标书'`。
2. `acceptance_tail_check.py --static-docs` 新增检查，禁止编辑器副标题回退到 `bidId`，并要求保留业务兜底文案。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. `acceptance_tail_check.py --static-docs` 通过，且识别最新交付循环为 Loop-103。
2. 前端 `pnpm lint` 与 `pnpm build` 通过。
3. 总检 `./infra/scripts/check.sh` 通过：前端构建/lint、后端 Go test/vet、AI 服务 compileall/ruff/pytest 均通过。
4. 工程1 三段黄金样例回归通过：解析评分 `passed=109/109`、来源引用 `passed=9/9`、导出回归 `passed=23/23`。
5. 本地未运行中的 `ai-service` 容器测试按脚本规则跳过：`ai-service container is not running; skipping container pytest`。

### 偏离蓝图

1. 本轮只处理标书编辑器标题区的 ID 兜底，不等同于全站所有详情页的可见 ID 审计完成。
2. 未新增浏览器截图验收；本轮先用静态防漂移、lint、build 和总检验证。

## Loop-104 / 来源定位技术字段收口 - 2026-06-18

### 本轮目标

1. 继续审查响应来源与文件预览页，收口仍会露出的 OCR/引用定位技术字段。
2. 用户侧只展示页码、章节、原文位置和摘录，不展示引用号、chunk 定位码或 bbox 坐标值。
3. 将该 UI 口径加入静态验收，防止后续回退。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 的响应来源定位 Tag 移除引用号、定位码和坐标值，仅保留业务定位信息；存在 bbox 时展示“原文位置：已定位”。
2. `frontend/src/features/knowledge/index.tsx` 的文件预览定位条不再展示原始坐标；复制定位时也不再输出坐标值，并清洗历史链接中的旧定位行。
3. `infra/scripts/acceptance_tail_check.py --static-docs` 新增来源定位防退化检查，禁止响应来源回到技术定位字段展示。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. 静态验收通过，最新交付循环识别为 Loop-104。
2. 前端 `pnpm lint` 与 `pnpm build` 通过。
3. 总检 `./infra/scripts/check.sh` 通过：前端构建/lint、后端 Go test/vet、AI 服务 compileall/ruff/pytest 均通过。
4. AI 服务 pytest 结果为 `237 passed`。
5. 工程1三段黄金样例回归通过：解析评分 `passed=109/109`、来源引用 `passed=9/9`、导出回归 `passed=23/23`。
6. 本地未运行中的 `ai-service` 容器测试按脚本规则跳过：`ai-service container is not running; skipping container pytest`。

### 偏离蓝图

1. 本轮不改变 source_refs、source_bbox、chunk_id 等后端数据结构，也不改变预览 URL 的定位能力，只处理用户可见文案和复制文本。
2. 未新增浏览器截图验收；本轮先使用静态防漂移、lint、build 和总检验证。

## Loop-105 / 历史来源定位清洗修正 - 2026-06-18

### 本轮目标

1. 复查上一轮来源定位清洗逻辑，修正历史 `source_locator` 多行文本被通用归一化压平后无法过滤坐标行的问题。
2. 保证旧链接中的“引用号/定位码/坐标”行在复制定位前仍会被清洗。
3. 将“定位文本必须保留行边界再清洗”的实现约束加入静态验收。

### 代码交付

1. `frontend/src/features/knowledge/index.tsx` 新增 `normalizePreviewLocatorParam()`，对 `source_locator` 保留换行边界，仅清理行内空白。
2. 文件预览页改为 `sanitizePreviewLocatorText(normalizePreviewLocatorParam(...))`，避免技术定位行在进入 sanitizer 前被压平成单行。
3. `infra/scripts/acceptance_tail_check.py --static-docs` 新增防退化检查，禁止回到 `sanitizePreviewLocatorText(normalizePreviewParam(...))`。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. 静态验收通过，最新交付循环识别为 Loop-105。
2. 前端 `pnpm lint` 与 `pnpm build` 通过。
3. 总检 `./infra/scripts/check.sh` 通过：前端构建/lint、后端 Go test/vet、AI 服务 compileall/ruff/pytest 均通过。
4. AI 服务 pytest 结果为 `237 passed`。
5. 工程1三段黄金样例回归通过：解析评分 `passed=109/109`、来源引用 `passed=9/9`、导出回归 `passed=23/23`。
6. 本地未运行中的 `ai-service` 容器测试按脚本规则跳过：`ai-service container is not running; skipping container pytest`。

### 偏离蓝图

1. 本轮只修复文件预览页历史链接清洗链路，不改变新生成来源定位 URL 的参数结构。
2. 未新增浏览器截图验收；本轮继续用静态防漂移、lint、build 和总检验证。

## Loop-106 / 外部数据源错误展示收口 - 2026-06-18

### 本轮目标

1. 继续审查团队页外部数据源调用记录，处理结果列直接展示后端 `error_message` 的问题。
2. 防止 HTTP、allowlist、provider、connection 等外部调用技术错误出现在用户界面。
3. 将调用记录结果列的业务化展示加入静态验收。

### 代码交付

1. `frontend/src/features/team/index.tsx` 新增 `formatExternalToolAuditResult()`，成功时展示响应摘要或“调用成功”。
2. 新增 `externalToolBusinessErrorMessage()`，将常见英文技术错误映射为“数据源未启用”“该能力未启用”“本月调用预算已用完”“数据源暂时不可用”等业务文案。
3. 调用记录“结果”列不再直接渲染 `row.error_message || row.response_summary`。
4. `infra/scripts/acceptance_tail_check.py --static-docs` 新增防退化检查，禁止调用记录回退到原始错误展示。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过，最新记录识别为 Loop-106。
2. `cd frontend && pnpm lint` 通过。
3. `cd frontend && pnpm build` 通过。
4. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
5. AI pytest 通过：`237 passed`。
6. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
7. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 后端审计仍保留原始 `error_message` 供排查，本轮只调整前端展示口径。
2. 未新增浏览器截图验收；本轮继续用静态防漂移、lint、build 和总检验证。

## Loop-107 / AI 任务失败文案收口 - 2026-06-18

### 本轮目标

1. 继续审查标书、知识库、成本页的 AI 任务失败展示，处理 `error_message` 直接透出的问题。
2. 防止 provider、HTTP、callback、timeout、connection 等技术错误进入用户界面。
3. 将 AI 任务失败文案的共享过滤和防回退检查纳入静态验收。

### 代码交付

1. `frontend/src/shared/api/client.ts` 导出 `getUserFacingErrorMessage()`，复用既有“中文业务文案可展示，非中文技术错误回退默认文案”的策略。
2. `frontend/src/features/bid/index.tsx` 的文件解读、正文生成、导出、章节重生成失败展示改走共享过滤；取消状态固定展示“任务已取消”。
3. `frontend/src/features/knowledge/index.tsx` 的文档整理失败展示改走共享过滤。
4. `frontend/src/features/cost/index.tsx` 的智能建议失败展示改走共享过滤；取消状态固定展示“建议生成已取消”。
5. `infra/scripts/acceptance_tail_check.py --static-docs` 新增防退化检查，禁止上述页面回退到 `.error_message.trim()` 直出。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过，最新记录识别为 Loop-107。
2. `cd frontend && pnpm lint` 通过。
3. `cd frontend && pnpm build` 通过。
4. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
5. AI pytest 通过：`237 passed`。
6. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
7. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 后端和 AI 服务仍保留原始 `error_message` 供排查，本轮只调整前端展示口径。
2. 未新增浏览器截图验收；本轮继续用静态防漂移、lint、build 和总检验证。

## Loop-108 / 响应证据校验文案收口 - 2026-06-18

### 本轮目标

1. 继续审查标书响应矩阵证据补充弹窗，处理 rejected Promise 中残留英文内部错误的问题。
2. 保证用户未填写证据或来源时，弹窗、运行时异常和防回退检查都使用业务文案。
3. 将响应证据弹窗内部校验文案纳入静态验收。

### 代码交付

1. `frontend/src/features/bid/index.tsx` 将单条补证和批量补证弹窗中的 `new Error('empty evidence or source')` 改为 `new Error('请填写响应证据或来源')`。
2. `infra/scripts/acceptance_tail_check.py --static-docs` 新增防退化检查，禁止标书页回退到英文内部校验错误。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过，最新记录识别为 Loop-108。
2. `cd frontend && pnpm lint` 通过。
3. `cd frontend && pnpm build` 通过。
4. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
5. AI pytest 通过：`237 passed`。
6. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
7. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 本轮只修复响应证据补充弹窗的内部校验错误文案，不调整响应矩阵数据结构。
2. 未新增浏览器截图验收；本轮继续用静态防漂移、lint、build 和总检验证。

## Loop-109 / 合规规则层级展示收口 - 2026-06-18

### 本轮目标

1. 继续审查合规规则页，处理规则层级直接展示 `L1/L2/L3/L4` 编码的问题。
2. 用业务语义展示检查层级，避免用户理解技术枚举。
3. 将合规层级展示加入静态验收防回退。

### 代码交付

1. `frontend/src/features/compliance/index.tsx` 新增 `complianceLevelLabels` 和 `complianceLevelLabel()`，把 L1-L4 显示为“基础完整性 / 响应一致性 / 废标条款 / 评分优化”。
2. 检查层级选择器和规则表层级列统一显示业务标签，提交给后端的枚举值保持不变。
3. `infra/scripts/acceptance_tail_check.py --static-docs` 新增防回退检查，禁止回到 raw L1-L4 label 和 `<Tag>{value}</Tag>`。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
./infra/scripts/check.sh
```

结果：

1. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过，最新记录识别为 Loop-109。
2. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
3. AI pytest 通过：`237 passed`。
4. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
5. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 后端仍保留 L1-L4 枚举作为规则配置和接口契约，本轮只调整前端展示语义。
2. 未新增浏览器截图验收；继续用静态防漂移、lint、build 和总检验证。

## Loop-110 / 外部数据源调用记录展示收口 - 2026-06-18

### 本轮目标

1. 继续审查团队页外部数据源调用记录，处理结构化请求摘要和 JSON 结果摘要直接展示的问题。
2. 将 `keyword=string(len=...)`、`{"items":[...]}` 这类工程口径改为业务化摘要。
3. 将调用记录摘要展示加入静态验收防回退。

### 代码交付

1. `frontend/src/features/team/index.tsx` 新增 `formatExternalToolAuditRequest()`，把外部调用请求摘要显示为“提交 N 项条件”。
2. 新增 `formatExternalToolSuccessSummary()`、`parseExternalToolSummary()` 和 `countExternalToolResultItems()`，成功结果不再直出 JSON，而是显示“返回 N 条结果”或“已返回结果”。
3. 调用记录表头从“请求摘要 / 结果”调整为“提交内容 / 结果摘要”，避免暴露内部结构字段。
4. `infra/scripts/acceptance_tail_check.py --static-docs` 新增防回退检查，禁止回到 `request_summary` 直出和 `row.response_summary || '调用成功'`。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过，最新记录识别为 Loop-110。
2. `cd frontend && pnpm lint` 通过。
3. `cd frontend && pnpm build` 通过。
4. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
5. AI pytest 通过：`237 passed`。
6. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
7. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 后端审计仍保留结构化摘要与原始 JSON 摘要用于排查，本轮只调整前端展示口径。
2. 未新增浏览器截图验收；继续用静态防漂移、lint、build 和总检验证。

## Loop-111 / 外部数据源公网边界收口 - 2026-06-18

### 本轮目标

1. 继续审查外部 MCP 数据源配置和调用链路，处理 endpoint 仅校验 http/https 的 SSRF 风险。
2. 禁止外部数据源指向 localhost、私网 IP、特殊用途 IP、`.local` 域名或携带 userinfo 的 URL。
3. 让运行时 HTTP client 也执行 DNS 解析后的公网 IP 校验，防止历史配置或重定向绕过。

### 代码交付

1. `backend/internal/platform/externaltool/store.go` 新增 `validExternalToolEndpoint()`、`publicExternalToolNetIP()` 和 `externalToolSpecialUseIPPrefixes`，配置保存时使用公网 endpoint 校验。
2. 新增 `newExternalToolHTTPClient()`，`NewStore()` 默认使用受限 HTTP client；运行时 `DialContext` 对解析出的所有 IP 执行公网校验，`CheckRedirect` 对跳转目标再次校验。
3. `backend/internal/platform/externaltool/store_test.go` 新增 unsafe endpoint 回归测试和 localhost runtime guard 测试。
4. `infra/scripts/acceptance_tail_check.py --static-docs` 新增外部数据源公网边界静态防回退检查。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && go test ./...
./infra/scripts/check.sh
```

结果：

1. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过，最新记录识别为 Loop-111。
2. `cd backend && go test ./...` 通过。
3. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
4. AI pytest 通过：`237 passed`。
5. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
6. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 本轮收紧外部 MCP endpoint 为公网 HTTP/HTTPS 地址；如需本地调试外部 MCP，应通过显式测试 client 或安全隧道处理，不再允许生产配置直连内网地址。
2. 未新增端到端真实第三方 MCP 调用；本轮通过单元测试、静态验收和总检验证边界行为。

## Loop-112 / 外部工具审计摘要脱敏 - 2026-06-18

### 本轮目标

1. 继续审查外部 MCP/工具调用记录，处理 `response_summary` 仍按第三方返回 JSON 原文截断入库的问题。
2. 避免第三方响应里的客户名称、手机号、邮箱、token、金额等原始值进入审计表或团队页。
3. 禁止外部工具 endpoint 使用常见敏感 query 参数，并清理预设里的 token-in-URL 示例。

### 代码交付

1. `backend/internal/platform/externaltool/store.go` 将 `summarizeValue()` 改为 `structuralSummary()`：只保留对象 key、数组数量、字段类型和字符串长度，不保存原始响应值。
2. `safeError()` 增加 `redactExternalToolError()`，把外部 URL 和常见 token/api key/password/signature 片段归一为脱敏标记。
3. `validExternalToolEndpoint()` 增加敏感 query 拒绝逻辑，拦截 `token`、`api_key`、`access_token` 等参数。
4. `backend/internal/platform/externaltool/presets.go` 移除 Handaas 和 qlows 预设里的 token-in-URL 示例。
5. `frontend/src/features/team/index.tsx` 兼容结构化响应摘要，继续把调用记录显示为“返回 N 条结果 / 已返回结果”。
6. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查：禁止回到原始响应 JSON 摘要、要求错误脱敏和结构化摘要测试存在。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && go test ./internal/platform/externaltool
cd backend && go test ./...
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
2. `cd backend && go test ./internal/platform/externaltool` 通过，覆盖 unsafe endpoint、结构化响应摘要和错误脱敏。
3. `cd backend && go test ./...` 通过。
4. `cd frontend && pnpm lint` 通过。
5. `cd frontend && pnpm build` 通过。
6. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
7. AI pytest 通过：`237 passed`。
8. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
9. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 审计表仍保留字段名、数组数量和类型摘要，用于排查调用质量；不再保留第三方响应原始值。
2. 本轮未新增真实第三方 MCP 端到端调用，继续用单元测试、静态验收和构建验证数据最小化行为。

## Loop-113 / 外部工具成本元数据治理 - 2026-06-18

### 本轮目标

1. 继续审查外部 MCP/工具配置，处理 `monthly_budget` 和 `metadata.cost_per_call` 缺少后端金额边界的问题。
2. 避免 API 客户端写入非有限值、超大金额或任意敏感 metadata，影响预算判断、审计写入和后续展示。
3. 将成本字段治理固化进单元测试和尾部静态验收。

### 代码交付

1. `backend/internal/platform/externaltool/store.go` 新增 `normalizeExternalToolMetadata()`、`parseExternalToolMoney()`、`validExternalToolMoney()` 和金额上限常量。
2. `normalizeConfig()` 保存配置前校验并四舍五入 `monthly_budget`，只保留白名单内的 `cost_per_call` metadata，丢弃其他原始 metadata 字段。
3. `costPerCall()` 复用统一金额解析逻辑，旧数据里如果出现 `Infinity`、超大值、非数字字符串或复杂对象，会按 0 处理，不再进入预算/审计金额。
4. `backend/internal/platform/externaltool/store_test.go` 新增成本 metadata 规范化、非法金额拒绝、旧脏 metadata 忽略三组回归测试。
5. `frontend/src/features/team/index.tsx` 为“月度预算”和“单次估算”输入框补充与后端一致的最大值，减少误填。
6. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求金额治理函数和回归测试存在。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && go test ./internal/platform/externaltool
cd backend && go test ./...
cd frontend && pnpm lint
cd frontend && pnpm build
./infra/scripts/check.sh
```

结果：

1. `cd backend && go test ./internal/platform/externaltool` 通过，覆盖 endpoint guard、结构化摘要、错误脱敏和成本 metadata 治理。
2. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
3. `cd backend && go test ./...` 通过。
4. `cd frontend && pnpm lint` 通过。
5. `cd frontend && pnpm build` 通过。
6. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
7. AI pytest 通过：`237 passed`。
8. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
9. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 本轮只治理外部工具预算和单次成本的配置边界，没有新增真实第三方 MCP 调用。
2. 成本金额目前按配置值估算，仍不是第三方真实 token/调用账单回传。

## Loop-114 / AI 成本审计边界与认证文档收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 调用审计和模型路由成本口径，处理 `estimated_cost` 可被超大有限值污染的问题。
2. 避免异常 provider 回调或错误定价配置导致 `ai_call_logs.estimated_cost numeric(12,4)` 写入失败，或 ModelRouter 内存 quota 被异常成本值拖垮。
3. 同步修正 API 文档中 logout 和 AI 服务验签的陈旧描述。

### 代码交付

1. `backend/internal/platform/aicall/store.go` 新增 `maxAIEstimatedCost`，`sanitizeCost()` 同时过滤负数、NaN、Inf 和超大有限值，并按 4 位小数对齐数据库审计精度。
2. `backend/internal/platform/aicall/pricing_test.go` 新增超大显式成本回退定价、成本四位小数规范化、超大定价结果忽略三组回归测试。
3. `ai-service/app/gateway/model_router.py` 新增 `MAX_AI_ESTIMATED_COST` 和 `_estimated_cost_or_zero()`，在显式成本、定价估算和内存 quota 累计前统一过滤超大值，并按 4 位小数记账。
4. `ai-service/app/tests/test_model_router.py` 新增超大成本忽略、审计精度四舍五入、超大定价结果忽略三组回归测试。
5. `docs/blueprint/API_SPEC.md` 更新认证说明：logout 会写 `session_revoked_at`，refresh 和受保护 API 会拒绝撤销前签发的 token；AI 服务除健康检查外均强制 HMAC 验签，生产必须使用非开发密钥。
6. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，覆盖 API 文档口径、Go AI 审计成本边界、Python ModelRouter quota 成本边界和对应回归测试。

### 检查结果

已运行：

```bash
cd backend && go test ./internal/platform/aicall
cd backend && go test ./...
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
./infra/scripts/check.sh
```

结果：

1. `cd backend && go test ./internal/platform/aicall` 通过。
2. `cd backend && go test ./...` 通过。
3. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
4. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`240 passed`。
5. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
6. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
7. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
8. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 本轮治理的是 AI 成本审计边界和文档口径，没有新增真实模型、OCR 或导出版式能力。
2. `estimated_cost` 仍依赖 provider 回调或 `AI_MODEL_PRICING_JSON` 配置估算；未接入第三方真实账单回传。

## Loop-115 / 前端下载文件名边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查前后端文件下载链路，处理响应头文件名在浏览器下载层的二次防护缺口。
2. 避免异常 `Content-Disposition` 文件名携带路径片段、控制字符、Windows 保留名或超长名称，影响用户下载体验。
3. 将该客户端边界固化进尾部静态验收，防止后续导出功能回退为直接透传响应头文件名。

### 代码交付

1. `frontend/src/shared/api/client.ts` 在标书需求导出接口中新增 `safeDownloadFilename()` 调用，响应头文件名必须先清洗，再进入 `link.download`。
2. 同文件新增下载文件名规范化逻辑：取路径最后一段、复用非法字符清洗、处理 Windows 保留名，并限制最大长度为 120 字符且尽量保留扩展名。
3. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求导出响应头文件名经过 `safeDownloadFilename`，并要求保留 Windows 保留名与长度上限守卫。

### 检查结果

已运行：

```bash
cd frontend && pnpm lint
cd frontend && pnpm build
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
git diff --check
./infra/scripts/check.sh
```

结果：

1. `cd frontend && pnpm lint` 通过。
2. `cd frontend && pnpm build` 通过。
3. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `git diff --check` 通过。
5. `./infra/scripts/check.sh` 通过：前端 build/lint、Go test/vet、AI compileall/ruff/pytest 均通过。
6. AI pytest 通过：`240 passed`。
7. 工程1 样例回归通过：解析 `109/109`，来源引用 `9/9`，导出 `23/23`。
8. 容器内 pytest 因 `ai-service container is not running` 按脚本逻辑跳过。

### 偏离蓝图

1. 本轮治理的是文件导出体验和浏览器下载边界，没有新增新的导出格式或 Word/PDF 母版能力。
2. 前端目前仍没有独立单元测试框架；本轮以 lint/build、全量验收和静态防回退脚本覆盖。

## Loop-116 / 知识库检索 AI 输入体积边界 - 2026-06-18

### 本轮目标

1. 继续审查知识库检索链路，处理 embedding/rerank 同步接口缺少单条文本长度限制的问题。
2. 避免签名请求内的超长 query、embedding text 或 rerank document 被直接送入模型网关，造成模型调用成本、内存和数据库全文检索压力失控。
3. 将 AI schema、Go 搜索入口和尾部验收脚本的边界保持一致，防止后续回退。

### 代码交付

1. `ai-service/app/schemas/knowledge.py` 为知识库 embedding/rerank 请求新增 Pydantic 字符串约束：tenant、embedding text、rerank query、document id/title/content/section_path 均设置最大长度，并使用 `StringConstraints(strip_whitespace=True)` 拒绝纯空白关键字段。
2. `ai-service/app/tests/test_knowledge_schema.py` 新增知识库 schema 回归测试，覆盖超长文本、超长 query、超长 rerank 字段、空白必填字段和首尾空白清理。
3. `backend/internal/platform/knowledge/store.go` 在搜索入口新增 `normalizeKnowledgeSearchQuery()`，将 query 去首尾空白并按 2000 个 rune 截断，保证进入 embedding、rerank 和数据库全文检索的 query 体积可控。
4. `backend/internal/platform/knowledge/store_test.go` 新增中文 rune 边界测试，确认搜索 query 截断不破坏多字节字符。
5. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求知识库 AI schema、schema 测试、Go 搜索 query 上限和对应测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_knowledge_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/schemas/knowledge.py app/tests/test_knowledge_schema.py
cd backend && go test ./internal/platform/knowledge
cd backend && go test ./...
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_knowledge_schema.py -q -s` 通过：`5 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/schemas/knowledge.py app/tests/test_knowledge_schema.py` 通过。
3. `cd backend && go test ./internal/platform/knowledge` 通过。
4. `cd backend && go test ./...` 通过。
5. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
6. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`245 passed`。
7. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。

### 偏离蓝图

1. 本轮治理的是检索输入边界和成本保护，没有新增新的检索算法、embedding 模型或 rerank 模型。
2. 搜索 query 过长时采取截断策略而非业务错误提示；这是为了保持搜索入口容错和现有 UI 交互稳定。

## Loop-117 / 成本建议 AI 输入边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 服务 signed 任务入口，处理成本建议请求中无界字符串、无界列表和异常金额的问题。
2. 避免超长成本项目、建议列表或非有限金额进入 provider prompt，造成模型成本、内存和审计口径失控。
3. 将成本建议 schema 的字段边界与后端数据库金额约束、成本项状态/类型约束保持一致。

### 代码交付

1. `ai-service/app/schemas/cost.py` 新增成本建议请求边界常量和 Pydantic 约束，覆盖 tenant/task/cost project id、项目名称、分类、成本项名称、供应商、备注、建议、callback_url 和 model_hint。
2. 同文件将金额字段统一为有限非负金额，设置最大金额；利润额允许正负但有限，利润率限制在合理范围；成本类型和状态改为枚举字面量。
3. `CostAdviceRequest` 为 `category_totals`、`overrun_items`、`recommendations` 增加列表数量上限，避免超大任务体进入模型网关。
4. `ai-service/app/tests/test_cost_schema.py` 新增成本建议 schema 回归测试，覆盖超长列表、非法金额、非法枚举、超长文本、空白必填字段、首尾空白清理和超长建议。
5. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求成本建议 schema 的列表上限、金额上限、有限金额约束、字符串 trim 和回归测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_cost_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/schemas/cost.py app/tests/test_cost_schema.py
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && go test ./...
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_cost_schema.py -q -s` 通过：`6 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/schemas/cost.py app/tests/test_cost_schema.py` 通过。
3. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `cd backend && go test ./...` 通过。
5. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
6. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`251 passed`。

### 偏离蓝图

1. 本轮治理的是成本建议输入边界，没有新增真实成本优化模型或新的成本分析算法。
2. 对异常成本请求采取 schema 拒绝策略；后端正常生成的成本建议 payload 已与这些约束对齐。

## Loop-118 / 章节生成 AI 输入边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 服务 signed 任务入口，处理章节生成/改写请求中无界需求、知识引用、正文和 Tiptap JSON 的问题。
2. 避免超长章节正文、超大引用列表或异常 action 直接进入 provider prompt，造成模型成本、内存和输出契约风险。
3. 将章节生成 schema 的字段边界与后端当前生成 payload 的实际规模对齐，并保留足够业务余量。

### 代码交付

1. `ai-service/app/schemas/generation.py` 新增章节生成请求边界常量和 Pydantic 约束，覆盖 tenant/task/bid/chapter id、章节标题、招标需求文本、需求引用、知识引用、callback_url 和 model_hint。
2. 同文件为 `tender_requirements`、`requirement_refs`、`selected_knowledge_refs`、`retrieved_knowledge_refs` 增加列表数量上限，并限制知识引用正文、需求来源原文和期望响应长度。
3. `ChapterActionRequest` 将 action 收敛为 `optimize/expand/shorten/add_detail/self_check`，限制 instruction、current_plain_text，并对 `current_tiptap_json` 增加 256KB 序列化体积上限。
4. `ai-service/app/tests/test_generation_schema.py` 新增章节生成 schema 回归测试，覆盖超长列表、超长文本、非法 score/priority/action、超长正文、超大 Tiptap JSON 和首尾空白清理。
5. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求章节生成 schema 的列表上限、正文/Tiptap JSON 上限、action 枚举和对应回归测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/schemas/generation.py app/tests/test_generation_schema.py
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && go test ./...
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_schema.py -q -s` 通过：`6 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/schemas/generation.py app/tests/test_generation_schema.py` 通过。
3. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `cd backend && go test ./...` 通过。
5. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
6. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`257 passed`。

### 偏离蓝图

1. 本轮治理的是章节生成/改写输入边界，没有新增真实章节生成模型或新的生成策略。
2. 对异常章节生成请求采取 schema 拒绝策略；后端正常章节生成 payload 已与这些约束对齐。

## Loop-119 / 文档解析任务请求边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 服务 signed 任务入口，处理招标解析与知识入库任务中无界文件标识、对象 key、文件名、content_type 和 callback_url 的问题。
2. 避免超长文件元数据在解析入口进入 MinIO 读取、回调、审计和后续 provider prompt 链路，造成内存、日志和错误面失控。
3. 将解析入口 schema 与运行态 tenant object_key 校验、callback_url 白名单校验形成前置边界和运行态边界的双层保护。

### 代码交付

1. `ai-service/app/schemas/tender.py` 新增招标解析请求边界常量和 Pydantic `StringConstraints`，覆盖 task/tenant/bid/file id、bid_title、object_key、filename、content_type 和 callback_url。
2. `TenderParseRequest` 将必填字段收敛为非空、自动 trim、最大长度受控；可选字段同样限制最大长度，避免超长标题或回调地址进入任务链路。
3. `ai-service/app/schemas/knowledge.py` 为 `KnowledgeProcessRequest` 增加文档入库任务字段边界，覆盖 task/document/file id、object_key、filename、content_type 和 callback_url。
4. `ai-service/app/tests/test_tender_schema.py` 新增招标解析 schema 回归测试，覆盖超长文档字段、空白必填字段和首尾空白清理。
5. `ai-service/app/tests/test_knowledge_schema.py` 补充知识入库任务 schema 回归测试，覆盖超长对象 key/文件名、空白必填字段和首尾空白清理。
6. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求招标解析与知识入库文档字段边界、callback_url 边界和回归测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_knowledge_schema.py app/tests/test_tender_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/schemas/knowledge.py app/schemas/tender.py app/tests/test_knowledge_schema.py app/tests/test_tender_schema.py
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && go test ./...
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_knowledge_schema.py app/tests/test_tender_schema.py -q -s` 通过：`11 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/schemas/knowledge.py app/schemas/tender.py app/tests/test_knowledge_schema.py app/tests/test_tender_schema.py` 通过。
3. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `cd backend && go test ./...` 通过。
5. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
6. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`263 passed`。

### 偏离蓝图

1. 本轮治理的是文档解析/入库任务请求边界，没有新增 OCR、表格解析、xlsx/pptx 解析或真实语义抽取能力。
2. 对异常文档任务请求采取 schema 拒绝策略；运行态仍继续依赖 tenant object_key 校验和 callback_url 白名单作为第二层保护。

## Loop-120 / 文档导出任务请求边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 服务 signed 任务入口，处理文档导出请求中无界章节正文、布局上下文、文件名、对象 key、callback_url 和导出类型的问题。
2. 避免超大导出 payload 在 DOCX/PDF/ZIP 生成阶段才触发内存、CPU、模板渲染或日志风险。
3. 将导出入口从“请求体总大小保护”推进到“字段级 + 总章节预算 + 模板上下文预算”的业务边界保护。

### 代码交付

1. `ai-service/app/schemas/export.py` 新增导出请求边界常量和 Pydantic `StringConstraints`，覆盖 task/tenant/export/bid id、分册 code、标题、文件名、object_key、content_type、zip_path 和 callback_url。
2. `ExportChapter` 限制章节标题和单章正文长度；`DocumentExportRequest` 增加总章节数量与总章节正文预算，避免 ZIP 多分册绕过单列表上限。
3. `ExportLayoutOptions` 收敛 `e_bidding_structure` 为 `standard/flat`，限制模板名、页眉页脚、水印、生成时间，并为 `layout.context` 增加 key、项数、字符串值、嵌套深度和序列化字节上限。
4. `ExportAttachment` 对文件名、分类、content_type、object_key、zip_path 和 local_path 使用统一 trim/长度约束，继续保留 local_path 禁用与单一内容来源校验。
5. `ai-service/app/tests/test_export_schema.py` 新增导出 schema 回归测试，覆盖超长章节、超长字段、非法导出类型、空白必填字段、总章节预算、总正文预算、layout.context 约束和首尾空白清理。
6. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求导出 schema 的章节预算、layout.context 预算、导出类型枚举、trim 约束和回归测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_export_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/schemas/export.py app/tests/test_export_schema.py
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd ai-service && .venv/bin/python -m pytest app/tests/test_export_schema.py app/tests/test_docx_exporter.py app/tests/test_main_security.py -q -s
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
./infra/scripts/check.sh
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_export_schema.py -q -s` 通过：`10 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/schemas/export.py app/tests/test_export_schema.py` 通过。
3. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `cd ai-service && .venv/bin/python -m pytest app/tests/test_export_schema.py app/tests/test_docx_exporter.py app/tests/test_main_security.py -q -s` 通过：`91 passed`。
5. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
6. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`269 passed`。
7. `cd backend && go test ./...` 通过。
8. `./infra/scripts/check.sh` 通过；其中工程1导出验收通过：`passed=23/23 name=工程1.export`；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是导出任务请求边界和模板上下文安全预算，没有新增新的 Word/PDF 排版能力、母版模板或 LibreOffice 转换能力。
2. `export_type` 已收敛为枚举值，但 endpoint 路径类型与 payload 类型的一致性仍由后端正常调用约束，后续可在 AI 服务入口增加显式一致性校验。

## Loop-121 / AI 回调响应契约边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 服务 provider 输出到后端 callback 的路径，处理章节生成/改写与成本建议响应中无界 `source_refs`、`needs_human_input`、`self_check`、`model_metadata`、`token_usage` 和建议列表的问题。
2. 避免真实模型或兼容 provider 返回超大 JSON 后直接写入回调 payload、审计字段或后端任务结果，造成内存、日志和数据库字段风险。
3. 将前几轮“请求入口边界”补齐为“请求 + 响应”双向契约边界。

### 代码交付

1. `ai-service/app/schemas/common.py` 为 `TaskAccepted` 与 `SourceRef` 增加 trim/长度/page 范围约束，并新增 `bounded_json_object`、`bounded_token_usage` 作为响应 metadata 和 token_usage 的通用预算校验。
2. `ai-service/app/schemas/generation.py` 为 `ChapterGenerateResponse` 增加响应级上限，覆盖 source refs 数量、needs_human_input 数量/长度、Tiptap JSON 字节数、self_check 字节数、model_metadata 字节数和 token_usage 项数/取值范围。
3. `ai-service/app/schemas/cost.py` 为 `CostAdviceResponse` 增加 trace/summary/recommendations/risk_flags/focus_items 的长度与数量约束，并复用 metadata/token_usage 通用校验。
4. `ai-service/app/tests/test_generation_schema.py` 新增章节响应 schema 回归测试，覆盖超长列表、超大 JSON 输出、非法 source_ref/token_usage 和首尾空白清理。
5. `ai-service/app/tests/test_cost_schema.py` 新增成本建议响应 schema 回归测试，覆盖超长 summary/列表项、超大 metadata、非法 token_usage 和首尾空白清理。
6. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求通用响应预算、章节响应预算、成本响应预算和对应回归测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_schema.py app/tests/test_cost_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/schemas/common.py app/schemas/generation.py app/schemas/cost.py app/tests/test_generation_schema.py app/tests/test_cost_schema.py
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_schema.py app/tests/test_cost_schema.py app/tests/test_model_router.py app/tests/test_openai_compatible_provider.py app/tests/test_main_security.py -q -s
cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_coverage_eval.py -q -s
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
./infra/scripts/check.sh
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_schema.py app/tests/test_cost_schema.py -q -s` 通过：`19 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/schemas/common.py app/schemas/generation.py app/schemas/cost.py app/tests/test_generation_schema.py app/tests/test_cost_schema.py` 通过。
3. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_schema.py app/tests/test_cost_schema.py app/tests/test_model_router.py app/tests/test_openai_compatible_provider.py app/tests/test_main_security.py -q -s` 通过：`170 passed`。
5. `cd ai-service && .venv/bin/python -m pytest app/tests/test_generation_coverage_eval.py -q -s` 通过：`4 passed`。
6. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
7. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`276 passed`。
8. `cd backend && go test ./...` 通过。
9. `./infra/scripts/check.sh` 通过；其中工程1导出验收通过：`passed=23/23 name=工程1.export`；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是 AI 回调响应契约边界，没有新增新的生成模型、成本分析算法或语义质量提升。
2. 若 provider 返回超出响应预算的内容，当前策略是让任务进入失败回调，而不是自动截断模型输出；这样可以避免把不完整响应误当成可用业务结果。

## Loop-122 / 文档导出 endpoint 类型一致性收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 服务导出入口，处理 `/tasks/export/{docx,pdf,zip}` 路径类型与 payload `export_type` 可能不一致的问题。
2. 避免直接调用 AI 服务时带入矛盾的导出类型，造成请求语义、后台任务和回调结果之间的契约不清。
3. 保持 endpoint 路径作为导出类型的权威来源，同时兼容未显式传 `export_type` 的请求。

### 代码交付

1. `ai-service/app/main.py` 新增 `document_export_payload_for_endpoint`，在入队前统一校验并归一化导出类型。
2. 如果请求 payload 显式传入 `export_type` 且与 endpoint 路径不一致，AI 服务返回 `400`，错误为 `export_type must match export endpoint`。
3. 如果请求 payload 未显式传入 `export_type`，入队前按 endpoint 路径补正，例如 `/tasks/export/pdf` 会把后台 payload 归一化为 `pdf`。
4. `ai-service/app/tests/test_main_security.py` 新增两条回归测试，分别覆盖缺省 `export_type` 的路径归一化和显式 mismatch 的 400 拒绝。
5. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求导出 endpoint 类型一致性 helper、400 拒绝、payload 归一化和回归测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/main.py app/tests/test_main_security.py
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd ai-service && .venv/bin/python -m ruff check app/main.py app/tests/test_main_security.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py app/tests/test_export_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
cd backend && go test ./...
./infra/scripts/check.sh
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py -q -s` 通过：`65 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/main.py app/tests/test_main_security.py` 通过。
3. `python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `cd ai-service && .venv/bin/python -m pytest app/tests/test_main_security.py app/tests/test_export_schema.py -q -s` 通过：`75 passed`。
5. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
6. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`278 passed`。
7. `cd backend && go test ./...` 通过。
8. `./infra/scripts/check.sh` 通过；其中工程1导出验收通过：`passed=23/23 name=工程1.export`；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是文档导出 endpoint 与 payload 的契约一致性，没有新增导出格式能力或排版能力。
2. 对显式类型冲突采取 400 拒绝策略；对未显式传类型的请求采取按路径归一化策略，以保持现有路径语义兼容。

## Loop-123 / 文件名输入边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查后端文件服务的上传与生成资产入口，处理文件名只做路径清理、未限制长度的问题。
2. 避免超长文件名进入 `file_assets.filename`、预签名下载响应和 `Content-Disposition`，造成数据库字段膨胀、响应头过大或前端下载体验异常。
3. 保持中文业务文件名可用，同时给上传文件名设置明确、可测试、可回归的业务上限。

### 代码交付

1. `backend/internal/platform/file/service.go` 新增 `maxFilenameRunes = 255`，并在 `sanitizeFilename` 中使用 `utf8.RuneCountInString(base)` 拒绝超长文件名。
2. 该校验位于共享清理函数中，覆盖预签名上传 `PresignUpload` 和后端生成资产 `CreateGeneratedAsset`。
3. `backend/internal/platform/file/service_test.go` 新增超长文件名拒绝测试，以及 255 字符内中文文件名仍可接受的回归测试。
4. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求文件名长度常量、rune 级长度校验和对应回归测试同时存在。

### 检查结果

已运行：

```bash
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && GOTOOLCHAIN=local go test ./internal/platform/file
cd backend && GOTOOLCHAIN=local go test ./...
cd backend && GOTOOLCHAIN=local go vet ./...
./infra/scripts/check.sh
```

结果：

1. `python3 -m py_compile infra/scripts/acceptance_tail_check.py && python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
2. `cd backend && GOTOOLCHAIN=local go test ./internal/platform/file` 通过。
3. `cd backend && GOTOOLCHAIN=local go test ./...` 通过。
4. `cd backend && GOTOOLCHAIN=local go vet ./...` 通过。
5. `./infra/scripts/check.sh` 通过；其中前端 build/lint、后端 go test/vet、AI ruff、AI pytest `278 passed`、工程1解析评估 `passed=109/109`、生成覆盖评估 `passed=9/9`、工程1导出评估 `passed=23/23 name=工程1.export` 均通过；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是文件名输入边界和下载响应稳定性，没有新增文件存储后端、对象扫描或病毒检测能力。
2. 文件名上限按字符数处理，保留中文文件名兼容性；若未来需要兼容特定网关响应头字节上限，可进一步增加 `Content-Disposition` 级字节预算。

## Loop-124 / 成本业务输入边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查成本模块写入入口，处理项目名称、明细名称、供应商、备注等业务文本缺少应用层长度边界的问题。
2. 避免超长成本文本或异常金额进入数据库、报表与 AI 上下文，造成字段膨胀、导出异常或后续生成质量下降。
3. 将 Go 成本模块的金额边界与数据库 `numeric(14,2)` 和 AI schema 的输入边界对齐，形成可测试、可回归的业务约束。

### 代码交付

1. `backend/internal/platform/cost/store.go` 新增成本模块输入边界常量：项目名称 255 字符、短文本 128 字符、备注 1000 字符、金额上限 `999_999_999_999.99`。
2. 新增 `validateCostTextLength` 与 `boundedCostText`，使用 `utf8.RuneCountInString` 按字符数处理中文输入，避免按字节误伤中文业务文本。
3. `CreateProject` 与 `UpdateProject` 在访问数据库前校验显式项目名称；从项目名生成的默认成本项目名称会被截断到安全长度。
4. `normalizeItemRequest` 对分类、明细名称、供应商和备注做统一 trim 与长度校验，并继续拒绝负数、非有限数和超过金额上限的预算/实际金额。
5. `backend/internal/platform/cost/store_test.go` 新增超长项目名拒绝、默认名称截断、超长明细字段拒绝、中文边界文本允许等回归测试。
6. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求成本输入边界常量、rune 级长度校验、默认名称截断和对应回归测试同时存在。

### 检查结果

已运行：

```bash
cd backend && GOTOOLCHAIN=local go test ./internal/platform/cost
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
git diff --check
cd backend && GOTOOLCHAIN=local go test ./...
cd backend && GOTOOLCHAIN=local go vet ./...
./infra/scripts/check.sh
```

结果：

1. `cd backend && GOTOOLCHAIN=local go test ./internal/platform/cost` 通过。
2. `python3 -m py_compile infra/scripts/acceptance_tail_check.py && python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
3. `git diff --check` 通过。
4. `cd backend && GOTOOLCHAIN=local go test ./...` 通过。
5. `cd backend && GOTOOLCHAIN=local go vet ./...` 通过。
6. `./infra/scripts/check.sh` 通过；其中前端 build/lint、后端 go test/vet、AI ruff、AI pytest `278 passed`、工程1解析评估 `passed=109/109`、生成覆盖评估 `passed=9/9`、工程1导出评估 `passed=23/23 name=工程1.export` 均通过；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是成本模块业务输入边界，没有新增成本预测算法、报价策略或模型能力。
2. 金额上限按当前数据库 `numeric(14,2)` 能力收敛；租户级预算规则、币种规则和动态阈值仍属于后续业务策略层工作。

## Loop-125 / 标书导出文件名边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查标书导出链路，处理导出文件名只替换少量路径字符、未清理控制字符、未限制长度的问题。
2. 避免异常标书标题生成带控制字符或超长的导出文件名，进入 `bid_exports.filename`、`file_assets.filename`、AI 导出 payload 和下载响应链路。
3. 保持中文业务标题、导出类型后缀和业务标识可读，同时为导出文件名建立可测试、可回归的边界。

### 代码交付

1. `backend/internal/platform/bid/store.go` 新增 `maxExportFilenameRunes = 255` 和 `maxExportFilenameLabelRunes = 64`。
2. `exportFilename` 先清理并限制业务标签，再按最终后缀长度计算标题段预算，确保 `.docx`、`.pdf`、`.zip` 等后缀和业务标签不会被超长标题挤掉。
3. `sanitizeFilename` 新增控制字符清理、首尾空白与点号清理，并用 `utf8.RuneCountInString` 按字符数限制导出文件名长度。
4. `backend/internal/platform/bid/store_test.go` 新增导出文件名清理控制字符/路径字符、超长标题截断且保留业务后缀的回归测试。
5. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求导出文件名边界常量、控制字符清理、后缀预算和对应回归测试同时存在。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/bid/store.go internal/platform/bid/store_test.go
cd backend && GOTOOLCHAIN=local go test ./internal/platform/bid
git diff --check
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd backend && GOTOOLCHAIN=local go test ./...
cd backend && GOTOOLCHAIN=local go vet ./...
./infra/scripts/check.sh
```

结果：

1. `cd backend && GOTOOLCHAIN=local go test ./internal/platform/bid` 通过。
2. `git diff --check` 通过。
3. `python3 -m py_compile infra/scripts/acceptance_tail_check.py && python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
4. `cd backend && GOTOOLCHAIN=local go test ./...` 通过。
5. `cd backend && GOTOOLCHAIN=local go vet ./...` 通过。
6. `./infra/scripts/check.sh` 通过；其中前端 build/lint、后端 go test/vet、AI ruff、AI pytest `278 passed`、工程1解析评估 `passed=109/109`、生成覆盖评估 `passed=9/9`、工程1导出评估 `passed=23/23 name=工程1.export` 均通过；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是导出文件名稳定性和下载链路输入边界，没有新增导出格式、排版母版或文件扫描能力。
2. 文件名上限按字符数处理，优先保证中文业务标题和后缀可读；若未来需要兼容特定网关响应头字节上限，可在下载响应层继续增加字节预算。

## Loop-126 / 招标解析响应结构边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查 AI 招标解析链路，处理请求字段已有边界但响应结构仍可承载过大证据、需求项、warning、metadata 和模型字段的问题。
2. 避免模型增强或解析器异常输出过大的 `structured_result`，撑爆 HMAC 回调、任务日志、Go 落库字段或前端解析展示。
3. 在不改变六模块解析算法的前提下，为解析响应结果建立 schema 级数量、长度、页码、bbox 和 JSON 尺寸约束。

### 代码交付

1. `ai-service/app/schemas/tender.py` 新增响应侧边界常量：证据项、需求项、warning、列表项、模块数、bbox 点数、页码和 JSON 字节预算。
2. `TenderParseFieldEvidence` 收敛字段名、来源文本、引用 ID、文件名、页码和 bbox 范围，禁止非有限 bbox 数值与越界页码。
3. `TenderRequirementItem` 收敛需求文本、类型、预期响应和评分范围，避免模型返回异常大需求或异常分数。
4. `TenderParseModuleResult` 与 `TenderParseStructuredResult` 对 evidence、requirement_items、warnings、modules、material_suggestions、metadata、quality_gates、enhancement_error 等结构增加数量和 JSON 字节边界。
5. `ai-service/app/main.py` 新增 `tender_structured_result_for_callback`，在构造成功回调前用 `TenderParseStructuredResult` 对最终模型增强后的 `structured_result` 重新校验和归一化，避免模型输出绕过 schema。
6. `ai-service/app/tests/test_tender_schema.py` 新增响应侧超长 evidence、过多集合、过大 metadata、非法 bbox/页码的回归测试。
7. `ai-service/app/tests/test_main_security.py` 新增超大模型增强结果走失败回调的主流程回归测试。
8. `infra/scripts/acceptance_tail_check.py --static-docs` 增加防回退检查，要求 tender 响应边界常量、字段校验器、最终回调校验函数和对应回归测试同时存在。

### 检查结果

已运行：

```bash
cd ai-service && .venv/bin/python -m pytest app/tests/test_tender_schema.py -q -s
cd ai-service && .venv/bin/python -m ruff check app/main.py app/schemas/tender.py app/tests/test_tender_schema.py app/tests/test_main_security.py
cd ai-service && .venv/bin/python -m pytest app/tests/test_tender_schema.py app/tests/test_main_security.py app/tests/test_tender_parse_eval.py -q -s
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
cd ai-service && .venv/bin/python -m ruff check app
cd ai-service && .venv/bin/python -m pytest app/tests -q -s
git diff --check
cd backend && GOTOOLCHAIN=local go test ./...
cd backend && GOTOOLCHAIN=local go vet ./...
./infra/scripts/check.sh
```

结果：

1. `cd ai-service && .venv/bin/python -m pytest app/tests/test_tender_schema.py -q -s` 通过：`6 passed`。
2. `cd ai-service && .venv/bin/python -m ruff check app/main.py app/schemas/tender.py app/tests/test_tender_schema.py app/tests/test_main_security.py` 通过。
3. `cd ai-service && .venv/bin/python -m pytest app/tests/test_tender_schema.py app/tests/test_main_security.py app/tests/test_tender_parse_eval.py -q -s` 通过：`77 passed`。
4. `python3 -m py_compile infra/scripts/acceptance_tail_check.py && python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
5. `cd ai-service && .venv/bin/python -m ruff check app` 通过。
6. `cd ai-service && .venv/bin/python -m pytest app/tests -q -s` 通过：`282 passed`。
7. `git diff --check` 通过。
8. `cd backend && GOTOOLCHAIN=local go test ./...` 通过。
9. `cd backend && GOTOOLCHAIN=local go vet ./...` 通过。
10. `./infra/scripts/check.sh` 通过；其中前端 build/lint、后端 go test/vet、AI ruff、AI pytest `282 passed`、工程1解析评估 `passed=109/109`、生成覆盖评估 `passed=9/9`、工程1导出评估 `passed=23/23 name=工程1.export` 均通过；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是招标解析响应结构边界，没有新增解析算法、OCR 能力或模型提示策略。
2. JSON 字节预算按 AI 服务与回调链路稳定性收敛；若后续真实模型需要返回更丰富证据，应优先分页/引用化存储，而不是放宽单次回调 payload。

## Loop-127 / 项目业务输入边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查后端业务写入链路，处理项目名称、里程碑标题/备注、成本项目派生名称和中标案例派生文件名缺少长度边界的问题。
2. 避免异常长中文标题在项目、成本、知识库案例和文件资产链路间放大，造成数据库字段膨胀、响应过大或后续导出/下载体验异常。
3. 保持现有项目状态机、里程碑逻辑和中标案例回流语义不变，只补写库前校验与系统派生文本截断。

### 代码交付

1. `backend/internal/platform/project/store.go` 新增项目模块输入边界常量：项目名称 255 字符、里程碑标题 255 字符、里程碑备注 1000 字符、派生案例标题/文件名 255 字符。
2. 新增 `normalizeProjectName()`、`normalizeMilestoneWriteRequest()`、`validateProjectTextLength()` 和 `boundedProjectText()`，统一按 `utf8.RuneCountInString` 做中文友好的长度校验。
3. `Create` / `Update` 在进入事务前校验项目名称，超长或空白名称会直接返回 `ErrInvalidRequest`，避免无效请求触达数据库。
4. `CreateMilestone` / `UpdateMilestone` 统一归一化标题和备注，保留原状态、日期和排序逻辑。
5. `CreateCostProject` 对系统派生的成本项目名做 255 字符截断并保留业务兜底名。
6. `BuildWonCaseDraft` 对中标案例标题和 Markdown 文件名做边界截断，避免超长项目名扩散到文件资产和知识库回流。
7. `backend/internal/platform/project/store_test.go` 新增项目名、里程碑标题/备注、中文边界和派生文本截断回归测试。
8. `infra/scripts/acceptance_tail_check.py --static-docs` 增加项目模块业务输入边界防回退检查。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/project/store.go internal/platform/project/store_test.go
cd backend && GOTOOLCHAIN=local go test ./internal/platform/project
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
git diff --check
cd backend && GOTOOLCHAIN=local go test ./...
cd backend && GOTOOLCHAIN=local go vet ./...
./infra/scripts/check.sh
```

结果：

1. `cd backend && GOTOOLCHAIN=local go test ./internal/platform/project` 通过。
2. `python3 -m py_compile infra/scripts/acceptance_tail_check.py && python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
3. `git diff --check` 通过。
4. `cd backend && GOTOOLCHAIN=local go test ./...` 通过。
5. `cd backend && GOTOOLCHAIN=local go vet ./...` 通过。
6. `./infra/scripts/check.sh` 通过；其中前端 build/lint、后端 go test/vet、AI ruff、AI pytest `282 passed`、工程1解析评估 `passed=109/109`、生成覆盖评估 `passed=9/9`、工程1导出评估 `passed=23/23 name=工程1.export` 均通过；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是项目业务输入与派生文本边界，没有新增项目协同、成本分析或知识库检索能力。
2. 派生标题/文件名采用字符数截断，优先保证中文业务名称可读；若未来需要适配特定网关响应头字节预算，可在下载响应层继续增加字节级限制。

## Loop-128 / 合规规则输入边界收敛 - 2026-06-18

### 本轮目标

1. 继续审查合规模块写入链路，处理检查名称、规则字段、规则 metadata 和 levels 配置缺少边界的问题。
2. 避免超长检查名、重复层级数组或异常 metadata 放大 `compliance_checks.config`、`compliance_rules.metadata`、问题列表和报告链路。
3. 对旧规则生成 issue 的标题、证据和建议补派生文本边界，避免历史异常数据继续扩散到前端和报告。

### 代码交付

1. `backend/internal/platform/compliance/store.go` 新增合规模块输入边界常量：检查名称 255 字符、层级选择最多 4 项、规则 code 128 字符、规则名 255 字符、分类 128 字符、描述 2000 字符、metadata 最多 50 项且 JSON 不超过 16KB。
2. 新增 `normalizeCheckName()`、`normalizeRuleMetadata()`、`validateComplianceTextLength()` 和 `boundedComplianceText()`，统一按 `utf8.RuneCountInString` 做中文友好的长度校验。
3. `normalizeLevels()` 增加层级数量上限和去重逻辑，保留空层级默认 L1-L3 的既有行为。
4. `CreateCheck()` 在进入事务前校验检查名称和 levels，超长请求直接返回 `ErrInvalidRequest`。
5. `CreateRule()` / `UpdateRule()` 对规则文本和 metadata 做写库前校验，并不再忽略 metadata marshal 错误。
6. `generateIssues()` 对规则派生的 issue 标题、证据和建议做截断保护，并为空标题保留业务兜底。
7. `backend/internal/platform/compliance/store_test.go` 新增 levels 去重/上限、检查名写库前拒绝、规则文本/metadata 边界、metadata key trim 和派生文本截断回归测试。
8. `infra/scripts/acceptance_tail_check.py --static-docs` 增加合规模块业务输入边界防回退检查。

### 检查结果

已运行：

```bash
cd backend && gofmt -w internal/platform/compliance/store.go internal/platform/compliance/store_test.go
cd backend && GOTOOLCHAIN=local go test ./internal/platform/compliance
python3 -m py_compile infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
git diff --check
cd backend && GOTOOLCHAIN=local go test ./...
cd backend && GOTOOLCHAIN=local go vet ./...
./infra/scripts/check.sh
```

结果：

1. `cd backend && GOTOOLCHAIN=local go test ./internal/platform/compliance` 通过。
2. `python3 -m py_compile infra/scripts/acceptance_tail_check.py && python3 infra/scripts/acceptance_tail_check.py --static-docs` 通过。
3. `git diff --check` 通过。
4. `cd backend && GOTOOLCHAIN=local go test ./...` 通过。
5. `cd backend && GOTOOLCHAIN=local go vet ./...` 通过。
6. `./infra/scripts/check.sh` 通过；其中前端 build/lint、后端 go test/vet、AI ruff、AI pytest `282 passed`、工程1解析评估 `passed=109/109`、生成覆盖评估 `passed=9/9`、工程1导出评估 `passed=23/23 name=工程1.export` 均通过；容器内 pytest 因 `ai-service` 容器未运行被脚本跳过。

### 偏离蓝图

1. 本轮治理的是合规规则和检查输入边界，没有新增语义合规模型、OCR 或人工复核工作流。
2. metadata 当前按条目数和 JSON 字节预算收敛；若后续需要复杂规则参数，应拆成明确字段或版本化 schema，而不是继续扩大自由 JSON。

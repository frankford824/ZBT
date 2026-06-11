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

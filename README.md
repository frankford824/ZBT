# 智标通 ZBT

智标通是面向企业投标团队的 B2B 智能标书生成 SaaS 平台雏形，按 `x.md` 的验收清单持续推进。当前仓库包含 React 前端、Go 主业务后端、Python AI/RAG/文档服务、PostgreSQL + RLS + pgvector、Redis 和 MinIO。

## 模块现状

- `frontend`：React + TypeScript + Vite + Ant Design，覆盖工作台、标讯、标书向导/编辑器、合规、项目、成本、知识库、团队协作等页面。
- `backend`：Go + Gin 模块化单体，包含认证、租户、RBAC、RLS、文件预签名、项目/标书/知识库/合规/成本/审批接口和 AI 回调验签。
- `ai-service`：Python FastAPI，包含 ModelRouter、MockProvider、embedding、rerank、知识库解析、章节生成/改写、成本建议和 Word/PDF/ZIP 导出。
- `docs/blueprint`：API、数据库、权限、RAG、模型网关和每轮开发日志。

## 一键启动

```bash
cp .env.example .env
docker compose up -d --build
```

启动后访问：

- 前端：`http://localhost:5173`
- Go 后端健康检查：`http://localhost:8080/healthz`
- Python AI 健康检查：`http://localhost:8000/healthz`
- MinIO Console：`http://localhost:9001`

常用验证：

```bash
curl http://localhost:8080/healthz
curl http://localhost:8000/models/health
curl http://localhost:5173/api/v1/meta/routes
docker compose exec -T ai-service python -m pytest app/tests
./infra/scripts/check.sh
python3 infra/scripts/acceptance_core_check.py
python3 infra/scripts/acceptance_tail_check.py
python3 infra/scripts/acceptance_tail_check.py --static-docs
```

`./infra/scripts/check.sh` 会执行验收脚本语法检查、尾部验收静态文档防漂移检查、前端生产构建和 lint、Go 测试和 vet、AI `compileall` / ruff / pytest、工程1黄金样本回归、`docker compose config`，并在 `ai-service` 容器运行时追加容器内 pytest。

`python3 infra/scripts/acceptance_core_check.py` 需要在本地 Docker 服务启动后运行，会通过真实 API 和前端路由创建验收数据，覆盖 `x.md` 第 1-38 项：服务连通、注册登录、企业租户、成员邀请、角色权限、菜单权限依据、API 权限、多租户隔离、仪表盘、标讯、项目、标书、招标文件解析、知识库、素材选择、章节生成、版本/diff、三栏编辑器、DOCX/ZIP 导出和合规定位。

`python3 infra/scripts/acceptance_tail_check.py` 需要在本地 Docker 服务启动后运行，会通过真实 API 创建验收数据，覆盖 `x.md` 第 39-50 项：审批提交/通过/驳回、驳回回到 editing、成本项目和成本项、成本分析、中标案例回流知识库、AI 调用日志、模型路由、MockProvider、README 和开发日志。

`python3 infra/scripts/acceptance_tail_check.py --static-docs` 不需要启动服务，只检查模型路由、README、页面路由文档、AI_PIPELINE、SAMPLE_DOCS_EVALUATION 和 DEV_LOOP_LOG 最新 Loop 段落的防漂移约束；该检查已纳入 `./infra/scripts/check.sh`。

## 默认账号

默认密码均为 `demo-password`。

- `admin@zbt.local`：企业管理员，全部模块 full。
- `pm@zbt.local`：项目经理，项目 full，成本和团队无权限。
- `bidder@zbt.local`：投标专员，标书/合规 full。
- `viewer@zbt.local`：查看者，多数模块 read，成本和团队无权限。
- `other@zbt.local`：第二租户管理员，用于多租户隔离验证。

也可以通过 `/register` 创建新企业租户。注册会创建 tenant、管理员用户、默认角色矩阵、成员关系和欢迎通知，并直接返回登录态。

## 典型开发命令

```bash
cd frontend && pnpm install && pnpm build
cd backend && GOTOOLCHAIN=local go test ./...
cd ai-service && python3 -m compileall app
docker compose build backend frontend ai-service
docker compose up -d backend frontend ai-service
```

前端也可以单独部署到 Cloudflare Pages：

- Git 集成部署时，项目根目录选择 `frontend`，构建命令使用 `pnpm install --frozen-lockfile && pnpm build`，输出目录使用 `dist`。
- 直接上传部署时，执行 `cd frontend && pnpm install --frozen-lockfile && pnpm build && pnpm dlx wrangler pages deploy`。`frontend/wrangler.jsonc` 已声明 Pages 输出目录。
- 在 Pages 构建环境变量中设置 `VITE_API_BASE_URL` 指向 Go 后端的 `/api/v1`。
- `frontend/public/_redirects` 提供前端路由刷新回退，`frontend/public/_headers` 提供安全响应头和静态资源长期缓存策略。

本机 Python 环境如果没有安装项目依赖，`python3 -m pytest app/tests` 会失败；推荐使用已安装 dev extra 的容器执行：

```bash
docker compose exec -T ai-service python -m pytest app/tests
```

## 环境和模型路由

后端启动会自动执行嵌入式 goose 迁移。迁移连接使用 `MIGRATION_DATABASE_URL`，业务连接使用非超级账号 `DATABASE_URL=postgres://zbt_app:zbt_app@postgres:5432/zbt?sslmode=disable`，用于确保 RLS 在应用查询中真实生效。

AI 模型名称和 Provider 路由通过 `MODEL_ROUTING_FILE` 指向的 YAML 配置，不写死在代码中。Router 当前支持 `mock`、本地管线和 OpenAI-compatible Provider；DeepSeek、DashScope、OpenAI 兼容网关可通过 YAML provider 配置和对应 `*_API_KEY` / `*_BASE_URL` 环境变量启用。随仓路由默认以 OpenAI-compatible Provider 为主路径，MockProvider 只作为无 Key 本地验收的显式降级。Docker 默认路径为：

```bash
MODEL_ROUTING_FILE=./app/config/model_routing.yaml
USE_MOCK_PROVIDERS=false
ALLOW_MOCK_FALLBACK=true
```

没有真实 API Key 时，MockProvider 可以作为显式 fallback 跑通 embedding、rerank、章节生成、章节改写、成本建议和导出链路；配置 Key 后会优先走真实 Provider。真实 Key 只允许放在 `.env` 或密钥管理中，不要写入代码、prompt、日志或数据库。切换真实 Provider 时，可直接编辑 `model_routing.yaml`；也可以配置 `AI_LLM_PROVIDER` / `AI_LLM_MODEL`、`AI_EMBEDDING_PROVIDER` / `AI_EMBEDDING_MODEL`、`AI_RERANK_PROVIDER` / `AI_RERANK_MODEL` 覆盖每类任务的 provider/model。生产环境必须设置 `USE_MOCK_PROVIDERS=false` 和 `ALLOW_MOCK_FALLBACK=false`；真实 Provider 不可用时只会走显式 fallback，不会静默回退 mock。

可选接入 Cloudflare AI Gateway 时，不需要改业务代码，直接使用内置的 `cloudflare_ai_gateway` Provider：

```bash
CLOUDFLARE_ACCOUNT_ID=<account_id>
CLOUDFLARE_API_TOKEN=<api_token_with_ai_gateway_permission>
CLOUDFLARE_AI_GATEWAY_ID=<optional_gateway_id>
AI_LLM_PROVIDER=cloudflare_ai_gateway
AI_LLM_MODEL=openai/gpt-4.1
AI_EMBEDDING_PROVIDER=cloudflare_ai_gateway
AI_EMBEDDING_MODEL=@cf/baai/bge-large-en-v1.5
AI_RERANK_PROVIDER=cloudflare_ai_gateway
AI_RERANK_MODEL=@cf/baai/bge-reranker-base
USE_MOCK_PROVIDERS=false
ALLOW_MOCK_FALLBACK=false
```

`cloudflare_ai_gateway` 默认使用 Cloudflare 当前 REST API 基址 `https://api.cloudflare.com/client/v4/accounts/<account_id>/ai/v1`，LLM 走 `/chat/completions`，Workers AI embedding/rerank 模型必须使用 `@cf/...` 并走 `/ai/run/<model>`。所有请求发送 `Authorization: Bearer <CLOUDFLARE_API_TOKEN>`；需要指定某个 AI Gateway 时设置 `CLOUDFLARE_AI_GATEWAY_ID`，服务会发送 `cf-aig-gateway-id` 请求头；若部署环境必须使用自定义 OpenAI-compatible 基址，可设置 `CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL` 覆盖默认值。需要传递 metadata、cache 等附加头时，可用 JSON 对象配置 `CLOUDFLARE_AI_GATEWAY_HEADERS`，例如 `{"cf-aig-metadata":"{\"tenant\":\"prod\"}"}`。

OpenAI-compatible Provider 成功响应默认最多读取 8 MB，可用 `OPENAI_COMPATIBLE_MAX_RESPONSE_BYTES` 调整；超限响应会被拒绝且不会把模型返回内容写入错误信息。

AI 调用成本通过后端环境变量 `AI_MODEL_PRICING_JSON` 配置，例如：

```bash
AI_MODEL_PRICING_JSON='{"deepseek/deepseek-chat":{"input_per_1m":1,"output_per_1m":2},"openai_compatible_primary/*":{"input_per_1m":2,"output_per_1m":8}}'
```

价格可按 `provider/model`、`model`、`provider/*` 或 `*` 匹配；未配置价格时 `estimated_cost` 保持 0。

真实 Provider 配置完成后，可在 AI 服务目录运行生产 canary。无真实密钥的本地检查允许跳过；上线前应开启严格模式，要求路由不落回 Mock、Provider 健康检查通过、最小真实调用成功，并能按价格表得到正向费用和 quota 快照：

```bash
cd ai-service
python -m app.evaluation.provider_canary_eval --allow-skip
python -m app.evaluation.provider_canary_eval --strict --call-provider --require-cost \
  --route chapter_generate --route knowledge_embedding --route knowledge_rerank
```

AI 服务向后端投递任务回调时，成功响应默认最多读取 64 KB，可用 `AI_CALLBACK_MAX_RESPONSE_BYTES` 在 1 MB 内调整；超限响应会触发重试且不会把响应体写入错误信息。

Go 后端 API JSON 请求体默认限制为 96 MB，可通过 `API_MAX_BODY_BYTES` 调整，最大允许配置到 256 MB。AI 服务入口请求体同样默认限制为 96 MB，可通过 `AI_SERVICE_MAX_BODY_BYTES` 调整，最大允许配置到 256 MB。

## 文件和对象存储

文件上传通过 Go 后端获取 MinIO 预签名 URL。开发环境中：

- `MINIO_ENDPOINT=minio:9000` 用于容器内访问。
- `MINIO_PUBLIC_ENDPOINT=127.0.0.1:9000` 用于浏览器直连预签名 URL。
- `MINIO_USE_SSL=false`。
- `MINIO_ENSURE_BUCKET=true`，启动时自动创建本地 bucket。
- bucket 保持私有，下载/预览必须经 Go 鉴权后返回预签名 URL。

生产环境可以直接切换到 Cloudflare R2 的 S3 兼容接口：

- `MINIO_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com`
- `MINIO_PUBLIC_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com`
- `MINIO_USE_SSL=true`
- `MINIO_REGION=auto`
- `MINIO_ACCESS_KEY`、`MINIO_SECRET_KEY` 使用 R2 API Token 生成的 S3 凭据。
- `MINIO_BUCKET` 使用已创建的私有 bucket，`MINIO_ENSURE_BUCKET=false` 可避免应用启动凭据必须具备建桶权限。
- 浏览器上传需要在 R2 bucket 上允许业务域名的 `PUT` / `GET` / `HEAD` CORS。

预签名 URL 必须使用 R2 的 S3 API 域名，不要把自定义公开域名或 bucket path 写进 `MINIO_ENDPOINT` / `MINIO_PUBLIC_ENDPOINT`。
R2 CORS 示例见 `infra/cloudflare/r2-cors.example.json`，上线前把 `AllowedOrigins` 替换为真实业务域名后应用到 bucket：

```bash
aws s3api put-bucket-cors \
  --bucket <bucket> \
  --cors-configuration file://infra/cloudflare/r2-cors.example.json \
  --endpoint-url https://<account_id>.r2.cloudflarestorage.com
```

知识库解析支持纯文本、PDF 文本层、PDF layout blocks、PDF 表格候选、docx 段落和表格、xlsx/xlsm 工作表文本、pptx/pptm 幻灯片文本。AI 服务从 MinIO 读取解析源文件时默认限制 128 MB，可通过 `AI_TASK_OBJECT_MAX_BYTES` 调整，最高 256 MB；扫描件可通过 `OCR_PROVIDER=http_ocr|mineru|paddleocr` 接入外部 OCR 服务，通用 HTTP 使用 `OCR_HTTP_ENDPOINT` / `OCR_API_KEY`，MinerU 使用 `MINERU_HTTP_ENDPOINT` / `MINERU_API_KEY`，PaddleOCR 使用 `PADDLEOCR_HTTP_ENDPOINT` / `PADDLEOCR_API_KEY`；Provider 专属 endpoint/key/poll 未配置但通用 OCR 配置存在时会回退通用配置，`provider_profile.endpoint_env` / `api_key_env` / `poll_endpoint_env` 记录实际生效的 env 名。OCR 请求默认限制单文件 20 MB，可通过 `OCR_MAX_BYTES` 调整；同步响应和异步轮询响应都会归一为 `pages`、`blocks`、`layout_blocks`、`table_blocks` 和安全 metadata。异步 OCR 返回 202、pending、running 等状态时，会按响应中的 `status_url` / `poll_url`、`*_POLL_ENDPOINT`、`OCR_POLL_ENDPOINT`，或默认 `endpoint/{task_id}` 轮询；`*_POLL_INTERVAL_S` 和 `*_POLL_MAX_ATTEMPTS` 控制间隔与次数。解析结果默认最多生成 300 个知识片段，可通过 `KNOWLEDGE_PARSE_MAX_CHUNKS` 调整，超限时 metadata 会标记截断。未配置 OCR 时，空文本 PDF 会在解析 metadata 中标记 `ocr_required=true` 和 `provider_not_configured`，不会伪装为解析成功。复杂表格语义识别、版面还原和坐标级引用仍需继续增强。

`./infra/scripts/check.sh` 会运行工程1 OCR canary：无 OCR endpoint 时以 skipped 记录，不伪装通过；配置 `OCR_PROVIDER` 和对应 endpoint 后，会要求 OCR 返回文本、表格块、版面 bbox、表格 bbox 和单元格 bbox。生产验收可直接运行：

```bash
cd ai-service
.venv/bin/python -m app.evaluation.ocr_provider_eval \
  --provider mineru \
  --sample ../docs/ex/工程1/采购文件桥梁检查.pdf \
  --repo-root .. \
  --min-text-chars 20 \
  --min-table-blocks 1 \
  --min-page-confidence 0.80 \
  --min-layout-bbox-count 1 \
  --min-table-bbox-count 1 \
  --min-cell-bbox-count 1
```

标书导出支持默认 Word 母版：封面、可刷新目录域、页眉页脚、页码、中文字体样式、章节分页、Markdown 表格/列表渲染，并由同一 docx 源转换 PDF。可通过 `BID_EXPORT_TEMPLATE_PATH` 指向企业 docx/Jinja 模板，支持 `{{ bid_title }}`、`{{ part_title }}`、`{{ generated_at }}`、`chapters` 循环、自定义 `layout.context`，以及 `{{ZBT_COVER}}`、`{{ZBT_TOC}}`、`{{ZBT_BODY}}` 富文本锚点；通过 `BID_EXPORT_WATERMARK_TEXT` 增加水印文字。ZIP 导出支持 `attachments`、`boq_files` 和 `part.attachments`，对象键附件和 inline 附件默认都限制为单文件 20 MB，可通过 `AI_EXPORT_ATTACHMENT_MAX_BYTES` 下调；默认按 `01_投标文件/`、`02_附件/`、`03_工程量清单/` 电子标目录结构打包，并写入 `manifest.json` 记录分册、附件、工程量清单、大小和 sha256；PDF 导出会做可打开、文本层和首屏渲染非空校验。`app.evaluation.export_format_eval --input docs/sample_docs/golden/工程1.export.json --json` 会回归检查封面、水印、目录域、页眉页脚文本、页码域、表格、ZIP manifest 哈希/大小一致性、安全路径、PDF 文本层和首屏非空；最终 production report 会额外传入 `--require-pdf`，不接受 PDF skipped。

## 验收定位

最终验收清单见 `x.md` 第 21 节。每轮交付和验证证据记录在 `docs/blueprint/DEV_LOOP_LOG.md`，当前 API 状态见 `docs/blueprint/API_SPEC.md`。
工程1样本解析验收会检查六模块、来源引用、表格结构和响应问题清单质量；`requirement_items` 必须具备类型、强制性、优先级、`expected_response` 和可追溯来源，才能进入响应矩阵与章节生成链路。
运行态工程1验收可在本地服务栈启动后执行，使用 `docs/ex/工程1` 的真实 PDF/DOCX/XLSX 文件走上传、解析确认、响应矩阵、知识处理、全量章节生成、生成覆盖、合规检查和 DOCX 导出：

```bash
docker compose up -d --build
python3 infra/scripts/acceptance_project1_check.py --json-output tmp/project1_runtime_acceptance.json
```

第一可用版收口检查可使用：

```bash
python3 infra/scripts/first_usable_release_check.py --run-canaries
cp .env.production.example .env.production
python3 infra/scripts/first_usable_release_check.py --audit-production-env --env-file .env.production
python3 infra/scripts/first_usable_release_check.py --audit-production-env-json --env-file .env.production > tmp/production_env_audit.json
python3 infra/scripts/first_usable_release_check.py --profile production --env-file .env.production
python3 infra/scripts/first_usable_release_report.py --profile production --env-file .env.production --include-repo-check --include-project1-runtime --output tmp/first_usable_release_report.json
```

本地 profile 允许 Provider/OCR 在未配置 endpoint 或密钥时显式 skipped；production profile 会先审计生产环境，再要求 Provider/OCR canary 非 skipped、非 Mock、真实调用成功并具备正向 `estimated_cost`。`.env.production` 已加入 `.gitignore`，不要提交真实密钥；可以从 `.env.production.example` 复制后替换所有占位值，脚本会拒绝 `<replace-with-...>`、`changeme`、`todo` 和 `placeholder`。`--audit-production-env` 只做生产环境聚合审计，不调用外部模型/OCR，会一次列出生产模式、数据库、Redis、对象存储、Provider、OCR 和成本表的全部阻断项；`--audit-production-env-json` 会输出不含密钥值的 route、Provider 凭据 env、OCR endpoint env 和价格表匹配矩阵。`acceptance_project1_check.py --json-output` 会生成 `tmp/project1_runtime_acceptance.json`，记录工程1样本文件 hash、解析矩阵、知识处理、生成覆盖、合规和导出证据。`first_usable_release_report.py` 会生成脱敏 JSON 证据包，production profile 下会自动生成 `tmp/export_format_eval.json`，并为 readiness 命令传入 `--provider-canary-json-output tmp/provider_canary.json` 和 `--ocr-canary-json-output tmp/ocr_provider_canary.json`，在 `artifacts` 中登记 `tmp/export_format_eval.json`、`tmp/production_env_audit.json`、`tmp/provider_canary.json`、`tmp/ocr_provider_canary.json` 与 `tmp/project1_runtime_acceptance.json` 的大小、sha256、JSON 状态和语义校验状态，同时登记 `git_release_state`；报告会同时用 `.env.production` 和当前进程环境变量中的敏感值对命令输出与 JSON artifact 做脱敏。Git 发布状态要求 `origin` 保持 `git@github.com:frankford824/ZBT.git`；远端 HEAD 先用非交互 SSH 读取，SSH 受阻时可用已登录的 `gh` 通过 `https://github.com/frankford824/ZBT.git` 做只读兜底，报告会记录 `remote_check_method` 与 `remote_check_errors`，但不会放宽 SSH origin 约束。只有 production profile、production readiness、`./infra/scripts/check.sh`、工程1运行态验收、导出保真 artifact、关键 production artifact 的 `json_status=passed`、`semantic_status=passed`、工作区干净、`origin` 等于 `git@github.com:frankford824/ZBT.git` 且当前 HEAD 已同步到 `origin/main` 时 `loop_can_end=true`。生产矩阵、运行态证据、导出保真、Git 发布状态、脱敏与阻断条件分别由 `infra/scripts/test_first_usable_release_check.py`、`infra/scripts/test_acceptance_project1_check.py` 和 `infra/scripts/test_first_usable_release_report.py` 回归保护并纳入 `./infra/scripts/check.sh`。生产配置至少需要包含 `APP_ENV=production`、`DATABASE_URL`、`MIGRATION_DATABASE_URL`、`REDIS_URL`、`USE_MOCK_PROVIDERS=false`、`ALLOW_MOCK_FALLBACK=false`、`AI_MODEL_PRICING_JSON`、`JWT_SECRET`、`AI_SERVICE_HMAC_SECRET`、生产对象存储凭据、所选 LLM/embedding/rerank Provider 的 API key/base URL，以及 `OCR_HTTP_ENDPOINT`、`MINERU_HTTP_ENDPOINT` 或 `PADDLEOCR_HTTP_ENDPOINT`。

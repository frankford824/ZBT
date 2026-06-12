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
```

`./infra/scripts/check.sh` 会执行验收脚本语法检查、前端生产构建、Go 测试、AI `compileall`、可用时的 Python pytest、`docker compose config`，并在 `ai-service` 容器运行时追加容器内 pytest。

`python3 infra/scripts/acceptance_core_check.py` 需要在本地 Docker 服务启动后运行，会通过真实 API 和前端路由创建验收数据，覆盖 `x.md` 第 1-38 项：服务连通、注册登录、企业租户、成员邀请、角色权限、菜单权限依据、API 权限、多租户隔离、仪表盘、标讯、项目、标书、招标文件解析、知识库、素材选择、章节生成、版本/diff、三栏编辑器、DOCX/ZIP 导出和合规定位。

`python3 infra/scripts/acceptance_tail_check.py` 需要在本地 Docker 服务启动后运行，会通过真实 API 创建验收数据，覆盖 `x.md` 第 39-50 项：审批提交/通过/驳回、驳回回到 editing、成本项目和成本项、成本分析、中标案例回流知识库、AI 调用日志、模型路由、MockProvider、README 和开发日志。

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

本机 Python 环境如果没有安装项目依赖，`python3 -m pytest app/tests` 会失败；推荐使用已安装 dev extra 的容器执行：

```bash
docker compose exec -T ai-service python -m pytest app/tests
```

## 环境和模型路由

后端启动会自动执行嵌入式 goose 迁移。迁移连接使用 `MIGRATION_DATABASE_URL`，业务连接使用非超级账号 `DATABASE_URL=postgres://zbt_app:zbt_app@postgres:5432/zbt?sslmode=disable`，用于确保 RLS 在应用查询中真实生效。

AI 模型名称和 Provider 路由通过 `MODEL_ROUTING_FILE` 指向的 YAML 配置，不写死在代码中。Router 当前支持 `mock` 和 OpenAI-compatible Provider；DeepSeek、DashScope、OpenAI 兼容网关可通过 YAML provider 配置和对应 `*_API_KEY` / `*_BASE_URL` 环境变量启用。Docker 默认路径为：

```bash
MODEL_ROUTING_FILE=./app/config/model_routing.yaml
USE_MOCK_PROVIDERS=true
```

没有真实 API Key 时，MockProvider 可以跑通 embedding、rerank、章节生成、章节改写、成本建议和导出链路。真实 Key 只允许放在 `.env` 或密钥管理中，不要写入代码、prompt、日志或数据库。切换真实 Provider 时，把对应 route 的 `provider` 改为已配置的 OpenAI-compatible provider；如果未配置 key/base_url 且未显式设置 fallback，AI 服务会报错而不会静默回退 mock。

AI 调用成本通过后端环境变量 `AI_MODEL_PRICING_JSON` 配置，例如：

```bash
AI_MODEL_PRICING_JSON='{"deepseek/deepseek-chat":{"input_per_1m":1,"output_per_1m":2},"openai_compatible_primary/*":{"input_per_1m":2,"output_per_1m":8}}'
```

价格可按 `provider/model`、`model`、`provider/*` 或 `*` 匹配；未配置价格时 `estimated_cost` 保持 0。

## 文件和对象存储

文件上传通过 Go 后端获取 MinIO 预签名 URL。开发环境中：

- `MINIO_ENDPOINT=minio:9000` 用于容器内访问。
- `MINIO_PUBLIC_ENDPOINT=127.0.0.1:9000` 用于浏览器直连预签名 URL。
- bucket 保持私有，下载/预览必须经 Go 鉴权后返回预签名 URL。

知识库解析支持纯文本、PDF 文本层、PDF layout blocks、PDF 表格候选、docx 段落和表格、xlsx/xlsm 工作表文本、pptx/pptm 幻灯片文本。扫描件可通过 `OCR_HTTP_ENDPOINT` 接入外部 OCR 服务，配置 `OCR_API_KEY` 时会以 Bearer header 传递；未配置 OCR 时，空文本 PDF 会在解析 metadata 中标记 `ocr_required=true` 和 `provider_not_configured`，不会伪装为解析成功。复杂表格语义识别、版面还原和坐标级引用仍需继续增强。

标书导出支持默认 Word 母版：封面、可刷新目录域、页眉页脚、页码、中文字体样式、章节分页、Markdown 表格/列表渲染，并由同一 docx 源转换 PDF。可通过 `BID_EXPORT_TEMPLATE_PATH` 指向企业 docx 样式模板，通过 `BID_EXPORT_WATERMARK_TEXT` 增加水印文字；复杂企业模板占位符、附件清单和电子标专用格式仍需继续增强。

## 验收定位

最终验收清单见 `x.md` 第 21 节。每轮交付和验证证据记录在 `docs/blueprint/DEV_LOOP_LOG.md`，当前 API 状态见 `docs/blueprint/API_SPEC.md`。

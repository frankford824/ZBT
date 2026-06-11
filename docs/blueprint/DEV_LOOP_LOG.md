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

# 技术架构

## 运行拓扑

React SPA -> Nginx / API Gateway -> Go 主业务后端 -> PostgreSQL + RLS + pgvector / Redis / MinIO -> Python AI 服务 -> 外部模型或 MockProvider。

## 服务职责

前端负责企业后台 UI、路由守卫、服务端状态缓存、SSE 订阅和编辑器交互。

SSE 订阅使用 fetch stream 实现，携带现有 JWT Authorization 和 `X-Tenant-ID` 请求头；原生 EventSource 仅在无需自定义鉴权头的场景使用。

Go 后端负责认证、租户、RBAC、审计、文件权限、业务状态机、主业务落库、异步任务编排和 AI 服务回调验签。

Python AI 服务负责模型网关、文档解析、OCR、切片、embedding、RAG、章节生成、语义合规检查、成本建议、docx/PDF/ZIP 导出。

## 安全边界

前端不直连 AI 服务和对象存储。文件上传下载通过 Go 获取预签名 URL。所有数据库业务访问同时使用应用层 tenant_id 过滤和 PostgreSQL RLS 兜底。

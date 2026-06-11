# API 规格

统一前缀：/api/v1。

## 基础

GET /healthz，GET /api/v1/meta/routes。

## Auth

POST /auth/register、POST /auth/login、POST /auth/refresh、POST /auth/logout、GET /me。

## Tenant / Team

GET /tenant、PATCH /tenant、GET /tenant/members、POST /tenant/members/invite、PATCH /tenant/members/:id、DELETE /tenant/members/:id、GET /roles、POST /roles、PATCH /roles/:id、DELETE /roles/:id。

## Tender

GET /tenders、POST /tenders、GET /tenders/:id、PATCH /tenders/:id、POST /tenders/:id/favorite、DELETE /tenders/:id/favorite、POST /tenders/:id/create-project、POST /tenders/:id/create-bid、GET /tender-sources、POST /tender-sources、PATCH /tender-sources/:id、DELETE /tender-sources/:id、POST /tender-sources/:id/verify。

## Project

GET /projects、POST /projects、GET /projects/:id、PATCH /projects/:id、DELETE /projects/:id、POST /projects/:id/transition、GET /projects/:id/milestones、POST /projects/:id/milestones、PATCH /projects/:id/milestones/:milestoneId、DELETE /projects/:id/milestones/:milestoneId、POST /projects/:id/members、DELETE /projects/:id/members/:memberId、POST /projects/:id/create-cost-project、GET /projects/:id/activities。

## Bid

GET /bids、POST /bids、GET /bids/:id、PATCH /bids/:id、DELETE /bids/:id、POST /bids/:id/upload-tender-file、POST /bids/:id/parse-tender、GET /bids/:id/parse-result、PUT /bids/:id/parse-result、POST /bids/:id/outline/generate、GET /bids/:id/parts、GET /bids/:id/parts/:partId/outline、PUT /bids/:id/parts/:partId/outline、GET /bids/:id/material-selection、PUT /bids/:id/material-selection、POST /bids/:id/generate、GET /bids/:id/generation-jobs、GET /generation-jobs/:jobId、POST /generation-jobs/:jobId/pause、POST /generation-jobs/:jobId/resume、POST /generation-jobs/:jobId/cancel、GET /bids/:id/generation/stream、GET /bids/:id/chapters、PATCH /chapters/:chapterId、POST /chapters/:chapterId/accept、POST /chapters/:chapterId/regenerate、GET /chapters/:chapterId/versions、GET /chapters/:chapterId/diff、PUT /chapters/:chapterId/content、POST /chapters/:chapterId/ai-action、POST /bids/:id/exports、GET /bids/:id/exports、GET /bid-exports/:exportId、GET /bid-templates、POST /bid-templates/:templateId/use。

标书一期已落地最小 bid_documents、bid_parts、bid_chapters、bid_exports 数据闭环。`POST /bids/:id/exports` 支持 `export_type=docx` 或 `zip`。docx 导出时 Go 写入 ai_tasks 和待确认 file_asset 后调用 Python `/tasks/export/docx`；ZIP 打包时 Go 汇总技术标/商务标内容后调用 Python `/tasks/export/zip`。Python 生成文件并上传 MinIO，再通过 HMAC 回调 Go，Go 将 bid_exports 和 file_assets 标记为 ready/done。`GET /bid-exports/:exportId` 在导出完成后返回下载预签名 URL。

章节一期已落地 `PUT /chapters/:chapterId/content`、`POST /chapters/:chapterId/accept`、`POST /chapters/:chapterId/regenerate`、`GET /chapters/:chapterId/versions`、`GET /chapters/:chapterId/diff` 和 `GET /bids/:id/generation/stream`。保存和采纳会同步写入 `bid_chapter_versions`；重新生成由 Go 先检索当前租户 `knowledge_chunks`，创建 `ai_tasks` 并把章节置为 `generating`，再调用 Python `/tasks/chapter-generate`。Python 返回 202 + task_id 后在后台走 ModelRouter，生成完成后通过 HMAC 回调 Go，Go 回写 Tiptap JSON、source_refs、needs_human_input、版本快照和 `knowledge_references`；真实 chunk 引用会解析为 `source_document_id` + `chunk_id`。前端通过带 Authorization 头的 fetch SSE 订阅 `/bids/:id/generation/stream` 获取章节与 task 快照，并保留 `GET /ai-tasks/:taskId` 轮询兜底。

## Knowledge

GET /knowledge、GET /knowledge/categories、POST /knowledge/categories、PATCH /knowledge/categories/:id、DELETE /knowledge/categories/:id、GET /knowledge/tags、POST /knowledge/tags、PATCH /knowledge/tags/:id、DELETE /knowledge/tags/:id、GET /knowledge/documents、POST /knowledge/documents、GET /knowledge/documents/:id、PATCH /knowledge/documents/:id、DELETE /knowledge/documents/:id、POST /knowledge/documents/:id/process、GET /knowledge/documents/:id/preview、GET /knowledge/documents/:id/references、POST /knowledge/search、GET /knowledge/templates、POST /knowledge/templates、GET /knowledge/stats。

知识库一期已落地分类、标签、文档列表/详情/更新、处理任务创建、统计和文档引用追踪接口。`POST /knowledge/documents/:id/process` 由 Go 记录 `ai_tasks` 后调用 Python `/tasks/knowledge-process`，Python 返回外部 task_id；后续结果通过 `POST /ai/callbacks/tasks` 回调 Go，并将 chunks 与 1024 维 embedding 写入 `knowledge_chunks`。`POST /knowledge/search` 已实现租户内 pgvector 语义召回 + PostgreSQL 全文/关键词融合排序，返回 `items` 和 `source_refs`；query embedding 由 Go 调用 Python `/embeddings/knowledge` 获取。`GET /knowledge/documents/:id/references` 已从 `knowledge_references` 反查当前文档被哪些标书章节引用，返回 bid/chapter/chunk 和解析 metadata。RRF 融合、RerankProvider 精排和真实 embedding Provider 仍按 RAG 设计继续推进。

## Compliance

POST /compliance/checks、GET /compliance/checks、GET /compliance/checks/:id、GET /compliance/checks/:id/issues、GET /compliance/checks/:id/stream、POST /compliance/issues/:id/autofix、POST /compliance/issues/:id/ignore、POST /compliance/issues/:id/confirm-fail、POST /compliance/checks/:id/report、GET /compliance/rules、POST /compliance/rules、PATCH /compliance/rules/:id、DELETE /compliance/rules/:id。

## Cost / Approval / Notification / File

覆盖 x.md 第 14 节列出的成本、审批、通知、文件和 AI task 接口。所有 AI、OCR、解析、向量化、导出、合规和逐章生成接口返回 202 + task_id，并通过 SSE 加轮询兜底。

文件接口一期已落地：

- POST /files/presign-upload：创建待确认 file_asset 并返回 MinIO PUT 预签名 URL。
- POST /files/:id/confirm：校验 MinIO 对象存在后把文件状态置为 ready。
- GET /files/:id/download-url：鉴权和租户校验后返回 attachment 预签名 URL。
- GET /files/:id/preview-url：鉴权和租户校验后返回 inline 预签名 URL。
- GET /knowledge/documents：返回当前租户知识库文件资产列表。

AI 回调接口：

- POST /ai/callbacks/tasks：公开入口，但必须携带 `X-ZBT-Timestamp` 和 `X-ZBT-Signature`。签名内容为 `timestamp.body` 的 HMAC-SHA256 hex，密钥来自 `AI_SERVICE_HMAC_SECRET`。
- GET /ai-tasks/:taskId：当前租户内查询 Go 记录的 AI 任务状态。

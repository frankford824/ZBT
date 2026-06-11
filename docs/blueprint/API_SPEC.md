# API 规格

统一前缀：/api/v1。

## 基础

GET /healthz，GET /api/v1/meta/routes，GET /api/v1/dashboard/summary。

工作台一期已落地 `GET /dashboard/summary`，从当前租户实时聚合项目、标书、合规、成本、知识库、审批和通知数据，返回核心统计、6 个月趋势、推荐标讯、最近项目、待审批和通知快照。前端 `/dashboard` 已从静态数字切换到真实 summary API。

## Auth

POST /auth/register、POST /auth/login、POST /auth/refresh、POST /auth/logout、GET /me。

注册与租户创建已落地 `POST /auth/register`。该接口创建新 tenant、管理员 user、默认角色矩阵、tenant_member、tenant_member_roles 和欢迎通知，并直接返回与 login 相同的 JWT session。默认角色包括 company_admin、department_admin、project_manager、bid_specialist 和 viewer，模块权限通过 `module_permissions` 落库；`POST /auth/refresh` 基于当前 Bearer token 重新签发 session，`POST /auth/logout` 在 stateless JWT 模式下返回成功并由前端清理本地 session。

## Tenant / Team

GET /tenant、PATCH /tenant、GET /tenant/members、POST /tenant/members/invite、PATCH /tenant/members/:id、DELETE /tenant/members/:id、GET /roles、POST /roles、PATCH /roles/:id、DELETE /roles/:id。

团队成员管理已补齐 `PATCH /tenant/members/:id` 和 `DELETE /tenant/members/:id`。PATCH 支持更新成员姓名、状态 active/invited/disabled 和角色集合；DELETE 采用软禁用，将 `tenant_members.status` 置为 disabled，保留历史审批、审计和角色引用。前端 `/team?tab=members` 已接入成员编辑和禁用操作。

AI 调用审计已落地 `GET /ai-call-logs`，返回当前租户内的模型调用记录，包含 task_type、provider、model、token_usage、latency_ms、status、error_message 和业务资源引用。`/team?tab=logs` 已从审批时间线切换为真实 AI 调用日志表。

## Tender

GET /tenders、POST /tenders、GET /tenders/:id、PATCH /tenders/:id、POST /tenders/:id/favorite、DELETE /tenders/:id/favorite、POST /tenders/:id/create-project、POST /tenders/:id/create-bid、GET /tender-sources、POST /tender-sources、PATCH /tender-sources/:id、DELETE /tender-sources/:id、POST /tender-sources/:id/verify。

标讯一期已落地 `tender_sources`、`tenders`、`tender_user_states` 和 `tender_parse_results` RLS 表。`GET /tenders` 支持关键词、地区、状态、数据源、推荐和收藏筛选；`POST /tenders/:id/favorite` / `DELETE /tenders/:id/favorite` 保存用户级收藏状态；`POST /tenders/:id/create-project` 可从标讯创建 opportunity 项目；`POST /tenders/:id/create-bid` 可从标讯创建 draft 标书。`GET /tender-sources` / `POST /tender-sources` / `POST /tender-sources/:id/verify` 已接入前端“监控设置”，URL 检测先 HEAD，失败时回退 GET，并记录最后检测状态和消息。

## Project

GET /projects、POST /projects、GET /projects/:id、PATCH /projects/:id、DELETE /projects/:id、POST /projects/:id/transition、GET /projects/:id/milestones、POST /projects/:id/milestones、PATCH /projects/:id/milestones/:milestoneId、DELETE /projects/:id/milestones/:milestoneId、POST /projects/:id/members、DELETE /projects/:id/members/:memberId、POST /projects/:id/create-cost-project、POST /projects/:id/archive-case、GET /projects/:id/activities。

项目一期已落地 `project_milestones`、`project_members`、`project_logs` 和最小 `cost_projects` RLS 表。`GET /projects` 返回项目、负责人、关联标书数和里程碑数；`POST /projects` 会创建 owner 成员和活动日志；`POST /projects/:id/transition` 按 opportunity -> bidding -> compliance_review -> submitted -> closed 状态链路流转，closed.result 支持 won/lost/pending；里程碑新增/更新/删除会写活动日志；`POST /projects/:id/members` 支持添加租户成员到项目；`POST /projects/:id/create-cost-project` 仅允许 closed + won 项目创建成本项目；`POST /projects/:id/archive-case` 仅允许 closed + won 项目生成 Markdown 案例文件，并回流为 `doc_type=won_case` 的知识库文档、标签和可检索 chunk。前端 `/projects` 和 `/projects/:projectId` 已接入真实看板、列表、状态推进、里程碑、活动、创建成本项目和回流知识库操作。

## Bid

GET /bids、POST /bids、GET /bids/:id、PATCH /bids/:id、DELETE /bids/:id、POST /bids/:id/upload-tender-file、POST /bids/:id/parse-tender、GET /bids/:id/parse-result、PUT /bids/:id/parse-result、POST /bids/:id/outline/generate、GET /bids/:id/parts、GET /bids/:id/parts/:partId/outline、PUT /bids/:id/parts/:partId/outline、GET /bids/:id/material-selection、PUT /bids/:id/material-selection、POST /bids/:id/generate、GET /bids/:id/generation-jobs、GET /generation-jobs/:jobId、POST /generation-jobs/:jobId/pause、POST /generation-jobs/:jobId/resume、POST /generation-jobs/:jobId/cancel、GET /bids/:id/generation/stream、GET /bids/:id/chapters、PATCH /chapters/:chapterId、POST /chapters/:chapterId/accept、POST /chapters/:chapterId/regenerate、GET /chapters/:chapterId/versions、GET /chapters/:chapterId/diff、PUT /chapters/:chapterId/content、POST /chapters/:chapterId/ai-action、POST /bids/:id/exports、GET /bids/:id/exports、GET /bid-exports/:exportId、GET /bid-templates、POST /bid-templates/:templateId/use。

标书一期已落地最小 bid_documents、bid_parts、bid_chapters、bid_exports 数据闭环。`PATCH /bids/:id` 可更新标题、状态和关联项目；`DELETE /bids/:id` 采用 archived 软归档，`GET /bids` 默认不返回 archived 标书。`GET /bid-templates` / `POST /bid-templates/:templateId/use` 已接入 `bid_templates` RLS 表，前端 `/bids/templates` 从真实模板库读取分类、版本、章节结构和使用次数，并可一键按模板创建草稿标书。`POST /bids/:id/exports` 支持 `export_type=docx`、`pdf` 或 `zip`。docx 导出时 Go 写入 ai_tasks 和待确认 file_asset 后调用 Python `/tasks/export/docx`；PDF 导出时 Python 先生成 docx 中间文件，再通过 LibreOffice headless 转换为 PDF；ZIP 打包时 Go 汇总技术标/商务标内容后调用 Python `/tasks/export/zip`。Python 生成文件并上传 MinIO，再通过 HMAC 回调 Go，Go 将 bid_exports 和 file_assets 标记为 ready/done。`GET /bid-exports/:exportId` 在导出完成后返回下载预签名 URL。

招标文件解析和目录大纲已落地 `POST /bids/:id/upload-tender-file`、`POST /bids/:id/parse-tender`、`GET/PUT /bids/:id/parse-result`、`POST /bids/:id/outline/generate`、`GET/PUT /bids/:id/parts/:partId/outline` 和 `GET/PUT /bids/:id/material-selection`。上传链路复用 `file_assets` 私有 MinIO 预签名上传，绑定后写入 `bid_tender_files` 和 `bid_parse_results`；解析与目录生成当前以确定性 bootstrap 方式同步生成结构化结果和章节大纲，同时写入状态为 done 的 `ai_tasks` 并通过 202 响应保持后续异步 AI/OCR 编排契约。前端 7 步向导的 1-4 步已接入真实上传、解析确认、目录生成/编辑和素材选择接口。

章节一期已落地 `PUT /chapters/:chapterId/content`、`POST /chapters/:chapterId/accept`、`POST /chapters/:chapterId/regenerate`、`GET /chapters/:chapterId/versions`、`GET /chapters/:chapterId/diff` 和 `GET /bids/:id/generation/stream`。保存和采纳会同步写入 `bid_chapter_versions`；重新生成由 Go 先检索当前租户 `knowledge_chunks`，创建 `ai_tasks` 并把章节置为 `generating`，再调用 Python `/tasks/chapter-generate`。Python 返回 202 + task_id 后在后台走 ModelRouter，生成完成后通过 HMAC 回调 Go，Go 回写 Tiptap JSON、source_refs、needs_human_input、版本快照和 `knowledge_references`；真实 chunk 引用会解析为 `source_document_id` + `chunk_id`。前端通过带 Authorization 头的 fetch SSE 订阅 `/bids/:id/generation/stream` 获取章节与 task 快照，并保留 `GET /ai-tasks/:taskId` 轮询兜底。

章节 AI 动作已落地 `POST /chapters/:chapterId/ai-action`，支持 `optimize`、`expand`、`shorten`、`add_detail`、`self_check`。Go 记录 `chapter_ai_action` 任务并调用 Python `/tasks/chapter-action`；Python 根据 action 走 `rewrite_assistant` 或 `chapter_self_check` ModelRouter 路由，返回 Tiptap JSON、source_refs、self_check、needs_human_input、model_metadata 和 token_usage 后 HMAC 回调 Go。Go 回写章节、版本快照和知识库引用，前端三栏编辑器 AI 助手已提供优化、扩写、缩写、加细节和自检按钮。

逐章生成 job 已落地 `POST /bids/:id/generate`、`GET /bids/:id/generation-jobs`、`GET /generation-jobs/:jobId`、`POST /generation-jobs/:jobId/pause|resume|cancel`。Go 创建 `bid_generation_jobs` 和 `bid_generation_steps` 后只派发一个章节任务；章节 HMAC 回调完成后刷新 step/job 进度，并在 job 仍为 running 时自动派发下一章。pause 在章节边界生效，resume 会继续派发下一 queued step，cancel 会取消未开始 step。前端 7 步向导第 5 步已接入整标/分册生成、任务进度和暂停/继续/取消操作。

## Knowledge

GET /knowledge、GET /knowledge/categories、POST /knowledge/categories、PATCH /knowledge/categories/:id、DELETE /knowledge/categories/:id、GET /knowledge/tags、POST /knowledge/tags、PATCH /knowledge/tags/:id、DELETE /knowledge/tags/:id、GET /knowledge/documents、POST /knowledge/documents、GET /knowledge/documents/:id、PATCH /knowledge/documents/:id、DELETE /knowledge/documents/:id、POST /knowledge/documents/:id/process、GET /knowledge/documents/:id/preview、GET /knowledge/documents/:id/references、POST /knowledge/search、GET /knowledge/templates、POST /knowledge/templates、GET /knowledge/stats。

知识库一期已落地分类、标签、文档列表/详情/更新、处理任务创建、统计、文档引用追踪和文档模板接口。`GET /knowledge` 返回 stats、categories、tags、recent_documents 和 templates 总览，不再落入通用 stub。分类和标签 CRUD 已接入前端管理交互，文档库页已支持编辑标题、类型、分类、标签和摘要。`POST /projects/:id/archive-case` 会把中标项目摘要、关联标书、里程碑和成本复盘写成 Markdown file_asset，并同步生成 `doc_type=won_case`、`parse_status=processed` 的知识文档及可搜索 `knowledge_chunks`。`POST /knowledge/documents/:id/process` 由 Go 记录 `ai_tasks` 后调用 Python `/tasks/knowledge-process`，Python 返回外部 task_id；后续结果通过 `POST /ai/callbacks/tasks` 回调 Go，并将 chunks 与 1024 维 embedding 写入 `knowledge_chunks`。`POST /knowledge/search` 已实现租户内 pgvector Top K + PostgreSQL 全文关键词 Top K 召回、RRF 融合和 Python `/rerank/knowledge` RerankProvider 精排，返回 `items` 和 `source_refs`；query embedding 由 Go 调用 Python `/embeddings/knowledge` 获取，rerank 调用会写入 `ai_call_logs`。`GET /knowledge/documents/:id/references` 已从 `knowledge_references` 反查当前文档被哪些标书章节引用，返回 bid/chapter/chunk 和解析 metadata。`GET /knowledge/templates` / `POST /knowledge/templates` 已接入 `document_templates` RLS 表，前端文档模板页可列表和新建模板。真实 embedding Provider 和真实 rerank Provider 仍可按模型网关配置替换。

## Compliance

POST /compliance/checks、GET /compliance/checks、GET /compliance/checks/:id、GET /compliance/checks/:id/issues、GET /compliance/checks/:id/stream、POST /compliance/issues/:id/autofix、POST /compliance/issues/:id/ignore、POST /compliance/issues/:id/confirm-fail、POST /compliance/checks/:id/report、GET /compliance/rules、POST /compliance/rules、PATCH /compliance/rules/:id、DELETE /compliance/rules/:id。

合规一期已落地 `compliance_rules`、`compliance_checks`、`compliance_issues`、`compliance_reports`、`compliance_fix_logs` RLS 表。`POST /compliance/checks` 按 L1-L4 选中层级创建检查，返回检查快照和问题列表；规则严重度支持 pass、warn、fail_candidate、fail，其中 LLM/语义类规则默认只能产出 fail_candidate，确定 fail 由规则或人工确认产生。`POST /compliance/issues/:id/autofix` / `ignore` / `confirm-fail` 会写修复日志并回算检查结果；`GET /compliance/checks/:id/stream` 返回 compliance SSE 快照；`POST /compliance/checks/:id/report` 生成报告摘要。前端 `/compliance` 和 `/compliance/:checkId` 已接入真实检查历史、规则库、问题处理和报告动作。

## Cost

GET /cost-projects、POST /cost-projects、GET /cost-projects/:id、PATCH /cost-projects/:id、GET /cost-projects/:id/items、POST /cost-projects/:id/items、PATCH /cost-items/:id、DELETE /cost-items/:id、GET /cost-projects/:id/analysis、POST /cost-projects/:id/ai-advice、POST /cost-projects/:id/report。

成本一期已落地 `cost_projects`、`cost_items`、`cost_reports` RLS 表。`GET /cost-projects` 返回关联项目、预算、实际、利润率和成本项数量；`POST /cost-projects` 可为项目创建或更新成本项目；成本项支持新增、更新和删除；`GET /cost-projects/:id/analysis` 返回分类汇总、超预算项和规则化建议；`POST /cost-projects/:id/ai-advice` 已改为异步 AI 任务，Go 写入 `ai_tasks` 后调用 Python `/tasks/cost-advice`，Python 通过 `cost_advice` ModelRouter 路由生成 summary、recommendations、risk_flags、focus_items、model_metadata 和 token_usage，再 HMAC 回调 Go 更新任务并写入 `ai_call_logs`。前端 `/costs/:costProjectId` 会轮询 `/ai-tasks/:taskId` 展示 AI 建议结果。`POST /cost-projects/:id/report` 生成 `cost_reports` 记录。前端 `/costs` 和 `/costs/:costProjectId` 已接入真实列表、成本构成图、成本明细、建议和报告动作。

## Approval / Notification / File

覆盖 x.md 第 14 节列出的其余审批、通知、文件和 AI task 接口。所有 AI、OCR、解析、向量化、导出和逐章生成接口返回 202 + task_id，并通过 SSE 加轮询兜底；合规检查当前返回 202 + 检查快照，后续异步化时复用同一 SSE 事件结构。

审批一期已落地 `approval_chains`、`approval_instances`、`approval_actions` 和 `comments` RLS 表。`GET /approval-chains` / `POST /approval-chains` / `PATCH /approval-chains/:id` / `DELETE /approval-chains/:id` 管理标书审批链，steps 保存角色、级次、是否必选和条件说明。`POST /bids/:id/submit-for-approval` 创建审批实例、保存审批链快照、将标书置为 `in_review` 并通知当前审批角色；`GET /approvals` / `GET /approvals/:id` 返回审批列表和动作流水；`POST /approvals/:id/approve` 会推进下一级或完成审批并将标书置为 `approved`；`POST /approvals/:id/reject` 会完成驳回并将标书退回 `editing`。`POST /notifications/read` 支持批量或全部标记已读，`GET /notifications/stream` 返回 notifications SSE 快照。前端 `/team` 已接入真实成员、审批链、审批实例、审批动作、通知和通知已读。

文件接口一期已落地：

- POST /files/presign-upload：创建待确认 file_asset 并返回 MinIO PUT 预签名 URL。
- POST /files/:id/confirm：校验 MinIO 对象存在后把文件状态置为 ready。
- GET /files/:id/download-url：鉴权和租户校验后返回 attachment 预签名 URL。
- GET /files/:id/preview-url：鉴权和租户校验后返回 inline 预签名 URL。
- GET /knowledge/documents：返回当前租户知识库文件资产列表。

AI 回调接口：

- POST /ai/callbacks/tasks：公开入口，但必须携带 `X-ZBT-Timestamp` 和 `X-ZBT-Signature`。签名内容为 `timestamp.body` 的 HMAC-SHA256 hex，密钥来自 `AI_SERVICE_HMAC_SECRET`。
- GET /ai-tasks/:taskId：当前租户内查询 Go 记录的 AI 任务状态。
- GET /ai-call-logs：当前租户内查询 AI 调用审计日志；知识库 embedding/rerank 搜索和 Python HMAC 回调完成的章节生成、文档处理、成本建议、导出任务都会由 Go 写入 `ai_call_logs`。

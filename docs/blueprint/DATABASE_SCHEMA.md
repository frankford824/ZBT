# 数据库模型

所有业务表默认包含 id uuid、tenant_id uuid、created_at timestamptz、updated_at timestamptz。除全局字典外，业务表必须启用 RLS，策略基于 current_setting('app.tenant_id', true)。

## SaaS 底座

tenants、users、tenant_members、tenant_member_roles、roles、permissions、role_permissions、module_permissions、audit_logs、notifications、file_assets、ai_call_logs。

迁移连接使用 owner/superuser，业务连接使用非超级 `zbt_app`，否则 PostgreSQL superuser 会绕过 RLS。

`file_assets` 记录 MinIO 对象元数据，`object_key` 固定为 `tenant_id/biz_type/uuid`，`status` 为 pending / ready / failed / deleted。上传链路先由 Go 生成预签名 URL，浏览器 PUT 到私有 bucket 后再 confirm 为 ready。

## 标讯

tender_sources、tenders、tender_user_states、tender_parse_results。

`tender_sources` 保存租户内标讯平台配置，包含平台类型、URL、状态、最后检测时间、检测结果和 JSON 配置。`tenders` 保存标讯主体信息，包含采购单位、地区、预算、发布日期、截止日期、匹配度、摘要、关键要求、风险条款和来源 URL。`tender_user_states` 保存用户级收藏状态；`tender_parse_results` 预留解析结果结构。当前前端 `/tenders` 已接入真实列表、推荐/收藏筛选、详情、收藏、数据源新增和 URL 可达性检测。

## 项目

projects、project_milestones、project_members、project_logs。

项目状态：opportunity -> bidding -> compliance_review -> submitted -> closed。closed.result 为 won / lost / pending。

`project_milestones` 保存项目计划节点、状态、计划日期、完成时间、排序和备注。`project_members` 保存项目成员及 owner/member 等角色。`project_logs` 保存项目创建、状态流转、里程碑和成员变更活动。`cost_projects` 当前保存从中标项目创建的最小成本项目，后续成本模块会继续扩展成本项、分析和报告。

## 标书

bid_documents、bid_parts、bid_chapters、bid_chapter_versions、bid_generation_jobs、bid_generation_steps、bid_exports、bid_templates。

标书类型：combined、separated、custom。bid_parts 支持 combined_body、tech、business、boq、attachment。

`bid_exports` 关联 `file_assets` 保存导出产物，status 为 queued / running / done / failed / cancelled。当前最小实现支持 docx 导出，导出文件仍通过私有 MinIO 和 Go 鉴权后的预签名 URL 下载。

`bid_templates` 保存租户内标书模板库，包含 name、bid_type、category、description、version、content、usage_count 和 status。当前前端 `/bids/templates` 已接入列表和使用动作；使用模板会创建 draft 标书、初始化默认分册章节，并递增模板 usage_count。

`bid_chapter_versions` 保存章节保存、采纳和 AI 重新生成后的版本快照，包含 content、plain_text、source_refs、needs_human_input、model_metadata 和 token_usage。章节生成引用真实 `knowledge_chunks` 时，`knowledge_references` 同时写入 `source_document_id` 和 `chunk_id`，metadata 中标记 `resolved=true`；`knowledge_references.source_document_id` 仍允许为空，用于记录 AI 返回但暂未解析到真实知识库文档的 source_ref。

## 知识库

knowledge_documents、knowledge_categories、knowledge_tags、knowledge_document_tags、knowledge_chunks、knowledge_references、document_templates。

`knowledge_documents` 关联 `file_assets`，记录文档标题、类型、分类、解析状态、摘要和元数据。`knowledge_categories` / `knowledge_tags` 支撑文档库分类树和标签管理，`knowledge_document_tags` 保存文档与标签关系。`ai_tasks` 记录 Go 编排的 AI/文档处理任务，Python AI 服务只返回任务状态或回调结果，最终由 Go 验签后更新业务状态。

`knowledge_chunks` 保存解析切片正文、页码、section_path、metadata 和 `embedding vector(1024)`。当前最小实现使用 Python MockProvider 生成 embedding，Go 回调写入 pgvector，并通过 `idx_knowledge_chunks_embedding_hnsw` 使用 HNSW cosine 索引支撑租户内语义搜索。

`knowledge_references` 是 AI 生成内容引用知识库的反向索引。章节生成引用真实 chunk 时写入 `source_document_id`、`chapter_id`、`chunk_id` 和解析 metadata，`GET /knowledge/documents/:id/references` 据此展示文档被哪些标书章节引用。

`document_templates` 保存租户内文档模板库，包含 name、category、description、version、content、usage_count 和 status。当前前端 `/knowledge/templates` 已接入列表和创建，模板正文先以 JSON section 结构保存，后续可关联 file_assets 或 docx 模板文件。

## 合规

compliance_checks、compliance_issues、compliance_rules、compliance_reports、compliance_fix_logs。

`compliance_rules` 保存租户内规则库，包含 code、name、category、level(L1-L4)、severity(pass/warn/fail_candidate/fail)、description、enabled 和 metadata。`compliance_checks` 保存检查任务、关联标书、状态、结果、得分、层级配置和 task_id。`compliance_issues` 保存规则命中的问题、证据、建议、位置和处理状态(open/fixed/ignored/confirmed_fail)。`compliance_fix_logs` 记录 autofix、ignore、confirm_fail 等动作审计。`compliance_reports` 保存报告生成摘要和检查元数据。当前迁移为每个租户种子化签章完整性、投标有效期、服务响应承诺、目录页码和评分优化规则，并启用 FORCE RLS。

## 成本

cost_projects、cost_items、cost_reports。

`cost_projects` 保存项目级成本测算，关联 `projects`，包含状态、预算金额和元数据。`cost_items` 保存分类、名称、类型、预算金额、实际金额、状态、供应商和备注，用于预算实际对比。`cost_reports` 保存成本报告生成结果和摘要元数据。当前前端 `/costs` 已接入真实列表、成本构成、明细、分析建议和报告生成动作。

## 团队协作

approval_chains、approval_instances、approval_actions、comments、notifications。

`approval_chains` 保存租户内审批流程配置，当前 resource_type 为 bid，steps JSON 保存级次、审批角色、指定用户、是否必选和条件说明。`approval_instances` 保存每次标书提交审批的实例、当前级次、状态、提交人和审批链快照，避免链配置变更影响在途审批。`approval_actions` 保存 submit、approve、reject、cancel 动作和审批意见。`comments` 预留资源级评论与 @ 提醒入口。审批动作会复用 `notifications` 向提交人或当前审批角色写站内通知；当前前端 `/team` 已接入真实审批链、审批实例和通知已读。

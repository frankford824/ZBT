# 数据库模型

所有业务表默认包含 id uuid、tenant_id uuid、created_at timestamptz、updated_at timestamptz。除全局字典外，业务表必须启用 RLS，策略基于 current_setting('app.tenant_id', true)。

## SaaS 底座

tenants、users、tenant_members、tenant_member_roles、roles、permissions、role_permissions、module_permissions、audit_logs、notifications、file_assets、ai_call_logs。

迁移连接使用 owner/superuser，业务连接使用非超级 `zbt_app`，否则 PostgreSQL superuser 会绕过 RLS。

`file_assets` 记录 MinIO 对象元数据，`object_key` 固定为 `tenant_id/biz_type/uuid`，`status` 为 pending / ready / failed / deleted。上传链路先由 Go 生成预签名 URL，浏览器 PUT 到私有 bucket 后再 confirm 为 ready。

## 标讯

tender_sources、tenders、tender_user_states、tender_parse_results。

## 项目

projects、project_milestones、project_members、project_logs。

项目状态：opportunity -> bidding -> compliance_review -> submitted -> closed。closed.result 为 won / lost / pending。

## 标书

bid_documents、bid_parts、bid_chapters、bid_chapter_versions、bid_generation_jobs、bid_generation_steps、bid_exports、bid_templates。

标书类型：combined、separated、custom。bid_parts 支持 combined_body、tech、business、boq、attachment。

`bid_exports` 关联 `file_assets` 保存导出产物，status 为 queued / running / done / failed / cancelled。当前最小实现支持 docx 导出，导出文件仍通过私有 MinIO 和 Go 鉴权后的预签名 URL 下载。

`bid_chapter_versions` 保存章节保存、采纳和 AI 重新生成后的版本快照，包含 content、plain_text、source_refs、needs_human_input、model_metadata 和 token_usage。`knowledge_references.source_document_id` 允许为空，用于记录 AI 返回但暂未解析到真实知识库文档的 source_ref，metadata 中保留原始引用和 resolved 标记。

## 知识库

knowledge_documents、knowledge_categories、knowledge_tags、knowledge_document_tags、knowledge_chunks、knowledge_references、document_templates。

`knowledge_documents` 关联 `file_assets`，记录文档标题、类型、分类、解析状态、摘要和元数据。`knowledge_categories` / `knowledge_tags` 支撑文档库分类树和标签管理，`knowledge_document_tags` 保存文档与标签关系。`ai_tasks` 记录 Go 编排的 AI/文档处理任务，Python AI 服务只返回任务状态或回调结果，最终由 Go 验签后更新业务状态。

## 合规

compliance_checks、compliance_issues、compliance_rules、compliance_reports、compliance_fix_logs。

## 成本

cost_projects、cost_items、cost_reports。

## 团队协作

approval_chains、approval_instances、approval_actions、comments、notifications。

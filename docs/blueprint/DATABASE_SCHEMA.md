# 数据库模型

所有业务表默认包含 id uuid、tenant_id uuid、created_at timestamptz、updated_at timestamptz。除全局字典外，业务表必须启用 RLS，策略基于 current_setting('app.tenant_id', true)。

## SaaS 底座

tenants、users、tenant_members、roles、permissions、role_permissions、module_permissions、audit_logs、notifications、file_assets、ai_call_logs。

## 标讯

tender_sources、tenders、tender_user_states、tender_parse_results。

## 项目

projects、project_milestones、project_members、project_logs。

项目状态：opportunity -> bidding -> compliance_review -> submitted -> closed。closed.result 为 won / lost / pending。

## 标书

bid_documents、bid_parts、bid_chapters、bid_chapter_versions、bid_generation_jobs、bid_generation_steps、bid_exports、bid_templates。

标书类型：combined、separated、custom。bid_parts 支持 combined_body、tech、business、boq、attachment。

## 知识库

knowledge_documents、knowledge_categories、knowledge_tags、knowledge_document_tags、knowledge_chunks、knowledge_references、document_templates。

## 合规

compliance_checks、compliance_issues、compliance_rules、compliance_reports、compliance_fix_logs。

## 成本

cost_projects、cost_items、cost_reports。

## 团队协作

approval_chains、approval_instances、approval_actions、comments、notifications。

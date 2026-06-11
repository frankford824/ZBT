# 页面与路由

| 页面 ID | 页面 | 路由 | 模块 |
| --- | --- | --- | --- |
| page-dashboard | 工作台 | /dashboard | 工作台 |
| page-tender | 标讯大厅 | /tenders | 标讯大厅 |
| page-generate | 标书生成 7 步向导 | /bids/:bidId/wizard?step=1..7 | 标书生成 |
| page-compliance | 合规检查 | /compliance | 合规检查 |
| page-project | 项目管理 | /projects?view=board\|list | 项目管理 |
| page-cost | 成本管理 | /costs | 成本管理 |
| page-knowledge | 知识库主页 | /knowledge | 知识库 |
| page-team | 团队协作 | /team?tab=members\|approvals\|logs\|notifications | 团队协作 |
| page-generate-new | 新建标书 | /bids/new | 标书生成 |
| page-generate-list | 我的标书 | /bids | 标书生成 |
| page-generate-templates | 标书模板 | /bids/templates | 标书生成 |
| page-knowledge-docs | 文档库 | /knowledge/docs | 知识库 |
| page-knowledge-templates | 文档模板 | /knowledge/templates | 知识库 |
| page-knowledge-tags | 标签管理 | /knowledge/tags | 知识库 |

## 详情级路由

/login、/register、/onboarding、/tenders/:tenderId、/projects/:projectId、/bids/:bidId/editor、/compliance/:checkId、/costs/:costProjectId、/files/:fileId/preview。

所有业务路由必须经过权限守卫。无权限 URL 直接访问显示 403。

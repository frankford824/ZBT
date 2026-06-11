# 权限矩阵

模块权限等级：none、read、full。菜单隐藏不是安全边界，后端 API 必须强校验。

| 角色 | 工作台 | 标讯 | 标书 | 合规 | 项目 | 成本 | 知识库 | 团队 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 超级管理员 | full | full | full | full | full | full | full | full |
| 企业管理员 | full | full | full | full | full | full | full | full |
| 部门管理员 | read | full | full | full | full | read | full | read |
| 项目经理 | read | full | full | full | full | none | read | none |
| 投标专员 | read | read | full | full | read | none | read | none |
| 查看者 | read | read | read | read | read | none | read | none |

基础 permission code 至少包括：dashboard.view、tender.view、tender.manage、bid.view、bid.create、bid.edit、bid.delete、bid.export、compliance.view、compliance.run、project.view、project.manage、cost.view、cost.manage、knowledge.view、knowledge.manage、team.view、team.manage、approval.manage、file.upload、file.download、ai.run。

投标专员访问项目必须通过 project_members 做资源级过滤。

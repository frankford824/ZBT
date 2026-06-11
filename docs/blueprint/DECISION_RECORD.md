# 决策记录

| 编号 | 决策 | 依据 |
| --- | --- | --- |
| ADR-001 | 页面结构以 V2.4 的 14 页面为准 | V2.4 是最终页面结构基准 |
| ADR-002 | HTML 原型仅作视觉参考 | 原型存在缺页和错误导航行为 |
| ADR-003 | 项目状态统一为 opportunity / bidding / compliance_review / submitted / closed | x.md 固定状态机 |
| ADR-004 | 角色体系使用超级管理员、企业管理员、部门管理员、项目经理、投标专员、查看者 | x.md SaaS 底座要求 |
| ADR-005 | 两个模板库导航分别命名为标书模板和文档模板 | 避免 V2.4 子菜单歧义 |
| ADR-006 | 合规检查按双文件入口和标书模块自动带入实现 | V2.1 重构要求 |
| ADR-007 | 成本管理按项目成本列表和单项目分析实现 | V2.1 / V2.2 成本架构调整 |
| ADR-008 | Go + Python 双服务固定，不降级全 Python | x.md 硬约束 |
| ADR-009 | 一期向量库使用 PostgreSQL + pgvector，保留 VectorStore 抽象 | 技术方案评估与 x.md 一致 |
| ADR-010 | 分离标书必须一期可用 | V2.2 将分离标书从预留升级为完整可用 |
| ADR-011 | Word 最小导出在 Loop 4 提前验证 | x.md 核心风险前置 |
| ADR-012 | 禁用 GORM、Drools、Rust 主后端和重 LangChain | x.md 禁止事项 |
| ADR-013 | 业务状态只能通过显式 transition 函数推进 | x.md 状态机要求 |
| ADR-014 | 所有 AI 调用统一经过 ModelRouter | x.md 模型网关要求 |

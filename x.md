你是我的 CTO、SaaS 架构师、AI/RAG 工程架构师、文档处理专家、全栈开发 Agent。你现在要把「智标通」从“PRD 补丁集合 + 静态 HTML 高保真原型”升级成一个完整可运行的 SaaS 平台雏形。

你必须使用 Claude Code / AI Agent 循环开发模式执行。不要只分析，不要只写文档，不要只做静态页面，不要把项目缩小成单功能 MVP。

本项目目标是：完整 SaaS 平台雏形，而不是轻量 Demo。

# 0. 最重要的总原则

必须严格遵守：

1. 这是完整 SaaS 平台雏形，不是单功能 AI 写标书工具。
2. 8 大业务模块必须保留。
3. V2.4 的 14 个页面必须保留。
4. 可以降低部分模块深度，不能删除模块骨架和业务链路。
5. 主业务后端固定使用 Golang，不得自行降级为全 Python 单体。
6. AI / RAG / 文档解析 / Word 导出固定使用 Python 服务。
7. 一期向量库使用 PostgreSQL + pgvector，但必须保留 VectorStore 抽象，后期可迁移 Qdrant。
8. Go 后端统一使用 pgx + sqlc，不使用 GORM。
9. 不使用 Drools。
10. 不重度依赖 LangChain；可以用 LiteLLM 或 OpenAI-compatible client 做 Provider 适配，但 RAG 主流程必须自研可控。
11. Rust 不作为一期主后端。Rust 只作为后期高性能文档处理、转换、检索增强 sidecar 的可选技术。
12. SaaS 底座、RBAC、RLS、审计日志必须先做，不能后补。
13. 分离标书必须一期支持技术标/商务标生成、编辑、分别导出和 ZIP 打包。
14. Word docx 最小导出链路必须提前验证，不能等到最后。
15. 所有 AI 调用必须经过 ModelRouter，不得在业务代码中直连具体模型 API。
16. 所有 API Key 必须走 .env 或密钥管理，不得写入代码、prompt、日志或数据库。
17. 所有关键 AI 输出必须 JSON Schema / Pydantic 校验。
18. 所有事实性生成内容必须带 source_refs；无来源的企业资质、人员、证书、金额、业绩必须标记 needs_human_input。
19. 每轮 Loop 必须交付代码、测试或文档更新，并运行可用检查命令。
20. 不要停在 Loop-0 文档阶段，必须持续进入工程实现。

# 1. 输入资料

请先读取并理解以下文件：

* docs/input/智标通-高保真原型.html
* docs/input/智标通PRD_V2.0_20260523.md
* docs/input/智标通PRD_V2.1_20260526.md
* docs/input/智标通PRD_V2.2_20260530.md
* docs/input/智标通PRD_V2.3_20260530.md
* docs/input/智标通PRD_V2.4_20260530.md
* docs/input/智标通 SaaS 平台技术方案评估.md

如果文件不在该目录，请在当前仓库内搜索这些文件名。

如果输入文件缺失，不要停止。先基于已存在资料工作，并在 DEV_LOOP_LOG.md 中记录缺失文件。

# 2. 需求事实源优先级

发生冲突时，按以下优先级裁决：

1. 本 Prompt 中的硬约束最高。
2. docs/input/智标通 SaaS 平台技术方案评估.md 作为技术蓝图参考，但需要按本 Prompt 的修正执行。
3. V2.4：最终页面结构和路由结构基准。
4. V2.3：侧边栏、分组导航、权限控制、项目管理交互基准。
5. V2.2：分离标书、成本分析、审批链、知识库统计的最终功能基准。
6. V2.1：合规检查重构、数据源管理、逐章生成交互、知识库增强补充。
7. V2.0：完整产品底稿、8 大模块、业务流程、技术方案和模块间流转基准。
8. HTML 高保真原型：只作为视觉、布局、交互参考，不作为最终数据模型和后端逻辑事实源。

禁止照抄 HTML 的错误导航行为。比如 HTML 原型中可能没有完整 V2.4 的 14 个页面，最终必须以 V2.4 页面结构为准。

# 3. 产品目标

智标通是一个面向企业投标团队的 B2B 智能标书生成 SaaS 平台。

目标用户：

* 企业投标团队
* 投标专员
* 项目经理
* 部门管理员
* 企业管理员
* 查看者

核心价值：

1. 提效：缩短标书编制周期。
2. 降险：通过合规检查降低废标风险。
3. 沉淀：企业知识库、历史标书、中标案例持续复用。
4. 协作：多人审批、项目管理、成本管理、权限控制。
5. 可追溯：AI 生成内容必须有引用来源和审计记录。

完整业务链路：

标讯大厅
→ 招标文件解析
→ 创建项目
→ 新建标书
→ AI 分步生成标书
→ 知识库 RAG 引用
→ 逐章生成与人工确认
→ 标书编辑器
→ 合规检查
→ 人工审核 / 审批
→ 导出 Word / PDF / ZIP
→ 提交投标
→ 开标结果
→ 中标后创建成本项目
→ 成本分析
→ 中标案例回流知识库

知识库和团队协作必须贯穿全流程。

# 4. 必须保留的 8 大模块

1. 工作台
2. 标讯大厅
3. 标书生成
4. 合规检查
5. 项目管理
6. 成本管理
7. 知识库
8. 团队协作

不能删除任何模块。可以控制深度，但必须有真实数据模型、路由、页面、API 或明确的接口占位。

# 5. 必须实现的 14 个页面

以 V2.4 为准，必须落地 8 个主页面 + 6 个子页面。

## 5.1 主页面 8 个

1. page-dashboard：工作台

   * 路由：/dashboard

2. page-tender：标讯大厅

   * 路由：/tenders

3. page-generate：标书生成 7 步向导

   * 路由：/bids/:bidId/wizard?step=1..7

4. page-compliance：合规检查

   * 路由：/compliance

5. page-project：项目管理

   * 路由：/projects?view=board|list

6. page-cost：成本管理

   * 路由：/costs

7. page-knowledge：知识库主页面

   * 路由：/knowledge

8. page-team：团队协作

   * 路由：/team?tab=members|approvals|logs|notifications

## 5.2 子页面 6 个

1. page-generate-new：新建标书

   * 路由：/bids/new

2. page-generate-list：我的标书

   * 路由：/bids

3. page-generate-templates：标书模板库

   * 路由：/bids/templates
   * 导航名称建议：标书模板

4. page-knowledge-docs：文档库

   * 路由：/knowledge/docs

5. page-knowledge-templates：文档模板库

   * 路由：/knowledge/templates
   * 导航名称建议：文档模板

6. page-knowledge-tags：标签管理

   * 路由：/knowledge/tags

## 5.3 必须补充的详情级路由

1. /login
2. /register
3. /onboarding
4. /tenders/:tenderId
5. /projects/:projectId
6. /bids/:bidId/editor
7. /compliance/:checkId
8. /costs/:costProjectId
9. /files/:fileId/preview 或通过弹窗预览

# 6. 最终技术栈裁决

## 6.1 前端

固定使用：

* React
* TypeScript
* Vite
* React Router
* TanStack Query
* Zustand
* Ant Design
* Tiptap
* ECharts
* React Hook Form
* Zod
* Axios 或 fetch wrapper
* SSE 客户端封装

不要使用 Next.js 作为 SaaS 后台主框架。

前端原则：

1. 这是登录后的企业后台，不需要 SSR。
2. 所有服务端状态交给 TanStack Query。
3. Zustand 只保存客户端状态，例如当前用户、当前租户、权限、侧边栏折叠、编辑器 UI 状态。
4. Ant Design 作为主 UI 框架。
5. Tiptap 负责内容结构编辑，不负责 Word 分页排版。
6. Word 排版保真交给 Python 导出服务。
7. 功能按 feature 切分，不按 components/pages/types 随意堆叠。
8. 所有页面必须有 loading、empty、error、permission denied 状态。

推荐目录：

frontend/
├── src/
│   ├── app/
│   ├── routes/
│   ├── layouts/
│   ├── shared/
│   │   ├── api/
│   │   ├── components/
│   │   ├── hooks/
│   │   ├── permissions/
│   │   ├── sse/
│   │   └── utils/
│   ├── features/
│   │   ├── auth/
│   │   ├── dashboard/
│   │   ├── tender/
│   │   ├── project/
│   │   ├── bid/
│   │   ├── editor/
│   │   ├── compliance/
│   │   ├── cost/
│   │   ├── knowledge/
│   │   ├── team/
│   │   └── notification/
│   └── main.tsx

禁止 feature 之间直接 import 内部实现，只能通过 index.ts 公共出口暴露。

## 6.2 主业务后端

固定使用：

* Golang
* Gin
* pgx
* sqlc
* goose
* Asynq
* PostgreSQL 16
* Redis
* MinIO
* JWT
* RBAC
* PostgreSQL RLS
* OpenAPI 文档生成

禁止使用：

* GORM
* Drools
* Rust 作为一期主后端
* 全 Python 单体降级
* 业务代码直接写 SQL 且不带 tenant_id
* 绕过状态机直接 UPDATE status

推荐目录：

backend/
├── cmd/
│   └── server/
├── internal/
│   ├── platform/
│   │   ├── auth/
│   │   ├── tenant/
│   │   ├── rbac/
│   │   ├── audit/
│   │   ├── notification/
│   │   └── file/
│   ├── modules/
│   │   ├── dashboard/
│   │   ├── tender/
│   │   ├── project/
│   │   ├── bid/
│   │   ├── knowledge/
│   │   ├── compliance/
│   │   ├── cost/
│   │   └── approval/
│   ├── jobs/
│   ├── aiclient/
│   ├── db/
│   │   ├── migrations/
│   │   ├── queries/
│   │   └── sqlc/
│   └── api/
├── sqlc.yaml
├── goose/
└── go.mod

Go 后端是唯一的业务状态机持有者和主业务落库方。

## 6.3 AI / RAG / 文档处理服务

固定使用：

* Python
* FastAPI
* Pydantic v2
* arq 或 Celery
* PyMuPDF
* python-docx
* docxtpl
* openpyxl
* python-pptx
* LibreOffice Headless
* PaddleOCR 或 OCRProvider 抽象
* pgvector 客户端
* 模型网关 ModelRouter
* LLMProvider
* EmbeddingProvider
* RerankProvider
* OCRProvider

推荐目录：

ai-service/
├── app/
│   ├── main.py
│   ├── config/
│   │   └── model_routing.yaml
│   ├── gateway/
│   │   ├── llm_provider.py
│   │   ├── embedding_provider.py
│   │   ├── rerank_provider.py
│   │   ├── ocr_provider.py
│   │   ├── model_router.py
│   │   └── mock_provider.py
│   ├── pipelines/
│   │   ├── parse/
│   │   ├── tender_extract/
│   │   ├── chunking/
│   │   ├── indexing/
│   │   ├── retrieval/
│   │   ├── generation/
│   │   ├── compliance/
│   │   ├── cost_advice/
│   │   └── export/
│   ├── workers/
│   ├── callbacks/
│   ├── schemas/
│   └── tests/
└── pyproject.toml

AI 服务无主业务状态。长任务由 Python 执行，完成后通过 HMAC 签名回调 Go，由 Go 统一落库和推进状态机。

# 7. 总体架构

采用：

React SPA
→ Nginx / API Gateway
→ Go 主业务后端
→ PostgreSQL + RLS + pgvector
→ Redis
→ MinIO
→ Python AI 服务
→ 外部模型 API / 本地 embedding / OCR Provider

架构原则：

1. 前端永远不直连 AI 服务。
2. 前端永远不直连对象存储，上传下载必须通过 Go 获取预签名 URL。
3. Go 是业务状态机持有者。
4. Python AI 服务只负责 AI、文档解析、RAG、导出、OCR 等重任务。
5. Python 完成任务后回调 Go，Go 验签后落库。
6. PostgreSQL 是主业务事实源。
7. pgvector 是一期向量库。
8. Redis 用于缓存、任务队列、SSE pub/sub。
9. MinIO 按 tenant_id 前缀隔离文件。
10. 所有模型调用写 ai_call_logs。

# 8. SaaS 底座必须先做

必须先实现：

1. tenant
2. user
3. tenant_member
4. role
5. permission
6. role_permission
7. module_permission
8. audit_log
9. notification
10. file_asset
11. JWT
12. RBAC
13. RLS
14. 当前租户上下文
15. 前端路由守卫
16. 后端权限中间件
17. MinIO 文件隔离
18. ai_call_logs

## 8.1 多租户隔离

必须使用三层防御：

### 应用层

1. JWT claims 携带 tenant_id。
2. Gin middleware 将 tenant_id 注入 request context。
3. 所有 sqlc 查询必须包含 tenant_id。
4. repository 查询不允许缺 tenant_id。
5. 增加测试：跨租户 ID 枚举必须 403 或 404。

### 数据库层

1. 所有业务表启用 PostgreSQL Row Level Security。
2. RLS 策略使用 current_setting('app.tenant_id')。
3. Go 在事务开始时 SET LOCAL app.tenant_id。
4. 即使 SQL 漏了 tenant_id，RLS 也必须兜底。

### 资源层

1. MinIO object key 必须为 tenant_id/biz_type/uuid。
2. 下载必须经 Go 权限校验后返回预签名 URL。
3. bucket 不公开。
4. ai_call_logs 按 tenant_id 记账。
5. pgvector 检索也必须受 RLS 与 tenant_id 过滤约束。

# 9. 角色与权限

预置角色：

1. 超级管理员
2. 企业管理员
3. 部门管理员
4. 项目经理
5. 投标专员
6. 查看者

权限模型：

user
→ tenant_member
→ role
→ role_permissions
→ module_permissions

模块权限 level：

* none
* read
* full

基础权限 code 至少包括：

* dashboard.view
* tender.view
* tender.manage
* bid.view
* bid.create
* bid.edit
* bid.delete
* bid.export
* compliance.view
* compliance.run
* project.view
* project.manage
* cost.view
* cost.manage
* knowledge.view
* knowledge.manage
* team.view
* team.manage
* approval.manage
* file.upload
* file.download
* ai.run

权限规则：

1. 前端隐藏菜单只是体验。
2. 后端 API 必须强校验。
3. 投标专员只能访问自己参与的项目，必须通过 project_members 做资源级过滤。
4. URL 直接访问无权限页面时显示 403。
5. 文件下载必须校验资源归属与权限。

# 10. 必须实现的业务数据模型

所有业务表必须含：

* id UUID
* tenant_id UUID
* created_at
* updated_at

除全局表外，业务表必须启用 RLS。

## 10.1 SaaS 底座

* tenants
* users
* tenant_members
* roles
* permissions
* role_permissions
* module_permissions
* audit_logs
* notifications
* file_assets
* ai_call_logs

## 10.2 标讯 Tender

* tender_sources
* tenders
* tender_user_states
* tender_parse_results

能力：

1. 标讯列表
2. 搜索筛选
3. 收藏
4. 监控设置
5. 数据源配置
6. 数据源 URL 可达性检测
7. 标讯详情
8. 从标讯创建项目
9. 从标讯生成标书

一期不做复杂自动爬虫。标讯自动抓取、Cookie 登录、自定义 CSS 选择器抓取为二期能力；一期必须保留数据源配置结构和可达性检测。

## 10.3 项目 Project

* projects
* project_milestones
* project_members
* project_logs

项目状态统一为：

1. opportunity：商机评估
2. bidding：标书制作
3. compliance_review：合规审核
4. submitted：投标中
5. closed：已结果

V2.3 的“进行中”只作为 UI 文案兼容，不作为数据库状态。

closed 时 result 字段：

* won
* lost
* pending

中标后必须能创建成本项目。

## 10.4 标书 Bid

* bid_documents
* bid_parts
* bid_chapters
* bid_chapter_versions
* bid_generation_jobs
* bid_generation_steps
* bid_exports
* bid_templates

标书类型：

1. combined：综合标书
2. separated：分离标书
3. custom：自定义组合

bid_parts 必须支持：

1. combined_body：综合标书主体
2. tech：技术标
3. business：商务标
4. boq：工程量清单 / 报价表
5. attachment：附件管理

一期必须支持：

1. 综合标书生成、编辑、导出。
2. 分离标书技术标/商务标分别生成。
3. 分离标书技术标/商务标分别编辑。
4. 分离标书技术标/商务标分别导出 docx。
5. 分离标书打包 ZIP。

可以后置：

1. 工程量清单智能生成。
2. 附件与章节精细关联。
3. 自定义组合复杂交互。
4. 电子标书特殊格式。

标书状态：

draft
→ generating
→ editing
→ in_review
→ approved
→ submitted
→ archived

审批驳回：

in_review → editing

章节状态：

pending
→ generating
→ generated
→ accepted
→ edited
→ needs_fix

生成任务状态：

queued
→ running
⇄ paused
→ done / failed / cancelled

## 10.5 知识库 Knowledge

* knowledge_documents
* knowledge_categories
* knowledge_tags
* knowledge_document_tags
* knowledge_chunks
* knowledge_references
* document_templates

能力：

1. 文档库
2. 文档模板库
3. 标签管理
4. 分类管理
5. 文档上传
6. 文档解析
7. 文档预览
8. AI 自动分类
9. AI 自动标签
10. 有效期识别
11. 资质到期提醒
12. 切片
13. embedding
14. pgvector 入库
15. 语义搜索
16. 混合检索
17. rerank
18. 引用追踪
19. 被哪些标书引用
20. 中标案例回流
21. 知识库统计

## 10.6 合规检查 Compliance

* compliance_checks
* compliance_issues
* compliance_rules
* compliance_reports
* compliance_fix_logs

四层检查：

1. L1：结构化规则校验
2. L2：语义一致性校验
3. L3：废标风险校验
4. L4：评分优化建议

检查项 severity：

* pass
* warn
* fail_candidate
* fail

规则：

1. 规则引擎确定命中可出 fail。
2. LLM 不得直接出 fail。
3. LLM 可出 fail_candidate。
4. fail_candidate 必须经过规则确认或人工确认才能变成 fail。
5. 第 4 层评分优化默认可运行但标注 beta，可默认关闭。

合规检查必须支持：

1. 上传招标文件 + 投标文件。
2. 从标书生成模块跳转时自动带入文件。
3. 检查配置。
4. 检查进度。
5. 结果分组。
6. 问题详情。
7. evidence 原文依据。
8. 修复建议。
9. 跳转编辑器定位。
10. 一键修复接口预留。
11. 检查报告导出。

## 10.7 成本 Cost

* cost_projects
* cost_items
* cost_reports

能力：

1. 项目成本列表。
2. 单项目成本分析。
3. 人力成本。
4. 材料成本。
5. 设备成本。
6. 其他成本。
7. 预算 vs 实际。
8. 利润率。
9. 成本优化建议。
10. 成本报告导出。

成本知识库为二期，但成本管理模块本身不能删除。

## 10.8 团队协作 Team / Approval

* approval_chains
* approval_instances
* approval_actions
* comments
* notifications

能力：

1. 成员管理。
2. 角色分配。
3. 模块权限。
4. 审批链配置。
5. 标书提交审批。
6. 审批通过 / 驳回。
7. 审批实例保存链快照。
8. 评论与 @ 提醒接口预留。
9. 通知中心。
10. 操作日志。

# 11. AI / RAG / 文档处理设计

## 11.1 文档处理流水线

文件上传后：

1. 保存 file_asset。
2. 保存 MinIO object。
3. 创建处理任务。
4. 按文件类型分流。
5. PDF 使用 PyMuPDF 提取文本、页码、表格。
6. Word 使用 python-docx 提取段落、标题、表格。
7. Excel 使用 openpyxl。
8. PPT 使用 python-pptx。
9. doc / ppt / 旧格式通过 LibreOffice 转换后处理。
10. 图片和扫描 PDF 进入 OCRProvider。
11. 输出统一中间文档模型。
12. 清洗文本。
13. 结构感知切片。
14. embedding。
15. 写入 knowledge_chunks + pgvector。
16. 返回处理状态。

## 11.2 OCR 决策

PDF 文本层覆盖率低于阈值时进入 OCR。

默认规则：

* 文本层覆盖率 < 60%：判定扫描件，走 OCR。
* 表格解析失败页：可截图后进入多模态 / OCR Provider。
* OCR 输出必须带 confidence。
* 低置信结果必须在前端标记“需人工确认”。

## 11.3 切片策略

必须结构感知切片：

1. 按标题层级切分。
2. 每个 chunk 目标 300-500 tokens。
3. 超长段落滑窗，overlap 约 15%。
4. 表格尽量整表作为 chunk。
5. 表格 chunk 必须带表头上下文。
6. 每个 chunk 必须带 metadata。

chunk metadata 至少包含：

* tenant_id
* document_id
* chunk_id
* title
* content
* page_start
* page_end
* section_path
* tags
* category
* doc_type
* source_file_id
* embedding_model
* embedding_dimensions
* created_at

## 11.4 RAG 检索流程

完整流程：

1. query 构造：章节标题 + 招标要求 + 当前任务上下文。
2. metadata filter：tenant_id、doc_type、tag、category、expire_at。
3. pgvector 向量召回 Top 30。
4. PostgreSQL tsvector / BM25 关键词召回 Top 30。
5. RRF 融合。
6. RerankProvider 精排。
7. 取 Top 5-8。
8. 构造上下文。
9. LLM 生成。
10. 返回 source_refs。
11. 落库 knowledge_references。

生成内容中的引用建议使用类似 {{ref:chunk_id}} 的中间标记，落库时解析为 source_refs。

## 11.5 逐章生成

输入：

* bid_document
* bid_part
* bid_chapter
* tender_parse_result
* requirement_refs
* selected_knowledge_refs
* retrieved_chunks
* previous_chapter_summaries
* template_style

输出：

* Tiptap JSON 内容
* source_refs
* self_check
* needs_human_input
* model metadata
* token usage
* trace_id

要求：

1. 每章独立任务。
2. 支持暂停 / 继续。
3. 每章生成后支持采纳、重新生成、手动编辑、查看差异。
4. 每次重新生成必须创建新 version。
5. 版本对比基于 bid_chapter_versions。
6. 事实性内容无引用时必须标记。
7. 不得编造企业资质、证书、人员、业绩、金额、日期。

## 11.6 合规检查

规则引擎放 Go：

* 资质完整性。
* 签章。
* 格式规范。
* 报价一致性。
* 日期。
* 保证金。
* 投标有效期。
* 附件清单。
* 结构化废标条款。

LLM 放 Python：

* 语义一致性。
* 条款响应匹配。
* 评分优化建议。
* 模糊废标风险判断。
* 文案修复建议。

禁止用 LLM 替代确定性规则。

# 12. 模型网关设计

所有模型调用必须经过 Python AI 服务中的 ModelRouter。

## 12.1 Provider 接口

必须实现：

1. LLMProvider

   * complete()
   * generate_json()
   * stream()
   * count_tokens()
   * health_check()

2. EmbeddingProvider

   * embed_text()
   * embed_batch()
   * get_dimensions()
   * health_check()

3. RerankProvider

   * rerank(query, documents)
   * health_check()

4. OCRProvider

   * parse_pdf()
   * parse_image()
   * extract_layout()
   * extract_tables()
   * health_check()

5. ModelRouter

   * resolve(task_type, tenant_id)
   * fallback()
   * log_call()
   * enforce_quota()

## 12.2 支持的 Provider 类型

必须预留：

LLM:

* OpenAI-compatible Provider
* Anthropic Provider
* DashScope Provider
* Gemini Provider
* DeepSeek Provider
* Local LLM Provider
* Mock LLM Provider

Embedding:

* OpenAI-compatible Embedding Provider
* DashScope Embedding Provider
* Local BGE Provider
* Mock Embedding Provider

Rerank:

* Cohere Rerank Provider
* Jina Rerank Provider
* Local BGE Reranker
* Mock Rerank Provider

OCR:

* PaddleOCR Provider
* Cloud OCR Provider
* Azure Document Intelligence Provider
* Mistral OCR Provider
* Mock OCR Provider

不要在代码里写死具体商用模型版本。模型名称必须通过 model_routing.yaml 和 .env 配置。

## 12.3 .env.example

创建 .env.example，至少包含：

DATABASE_URL=
REDIS_URL=
JWT_SECRET=
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h

MINIO_ENDPOINT=
MINIO_ACCESS_KEY=
MINIO_SECRET_KEY=
MINIO_BUCKET=zbt-files

AI_SERVICE_URL=[http://ai-service:8000](http://ai-service:8000)
AI_SERVICE_HMAC_SECRET=

MODEL_ROUTING_FILE=./config/model_routing.yaml
USE_MOCK_PROVIDERS=true

OPENAI_API_KEY=
OPENAI_BASE_URL=

ANTHROPIC_API_KEY=
ANTHROPIC_BASE_URL=

DASHSCOPE_API_KEY=
DASHSCOPE_BASE_URL=

DEEPSEEK_API_KEY=
DEEPSEEK_BASE_URL=

GEMINI_API_KEY=

COHERE_API_KEY=
JINA_API_KEY=

OCR_PROVIDER=
OCR_API_KEY=

LIBREOFFICE_PATH=/usr/bin/soffice

VITE_API_BASE_URL=/api/v1

禁止把真实 API Key 写入 .env.example。

## 12.4 model_routing.yaml

创建 ai-service/app/config/model_routing.yaml。

结构示例：

providers:
mock:
type: mock
fixtures_dir: ./tests/fixtures/ai

openai_compatible_primary:
type: openai_compatible
base_url_env: OPENAI_BASE_URL
api_key_env: OPENAI_API_KEY

dashscope:
type: openai_compatible
base_url_env: DASHSCOPE_BASE_URL
api_key_env: DASHSCOPE_API_KEY

anthropic:
type: anthropic
base_url_env: ANTHROPIC_BASE_URL
api_key_env: ANTHROPIC_API_KEY

local_embedding:
type: local
endpoint_env: LOCAL_EMBEDDING_ENDPOINT

routes:
tender_parse:
primary:
provider: mock
model: configurable-high-quality-json-model
temperature: 0.1
output: json
schema: TenderParseResult
fallback:
- provider: mock
model: fallback-json-model
timeout_s: 180

outline_generate:
primary:
provider: mock
model: configurable-json-model
temperature: 0.2
output: json
schema: BidOutline

knowledge_embedding:
primary:
provider: mock
model: configurable-embedding-model
dimensions: 1024

knowledge_rerank:
primary:
provider: mock
model: configurable-rerank-model
top_k_input: 30
top_k_output: 8

chapter_generate:
primary:
provider: mock
model: configurable-high-quality-generation-model
temperature: 0.3
stream: true
require_source_refs: true

chapter_self_check:
primary:
provider: mock
model: configurable-low-cost-json-model
temperature: 0.0
output: json
schema: ChapterSelfCheckResult

compliance_check:
primary:
provider: mock
model: configurable-high-quality-json-model
temperature: 0.0
output: json
schema: ComplianceIssueList

rewrite_assistant:
primary:
provider: mock
model: configurable-fast-generation-model
temperature: 0.4
stream: true

cost_advice:
primary:
provider: mock
model: configurable-low-cost-json-model
temperature: 0.2
output: json

document_ocr:
primary:
provider: mock
model: configurable-ocr-model

quotas:
default_tenant_monthly_budget: 500
currency: CNY
on_exceed: downgrade_then_block

## 12.5 ai_call_logs

每次模型调用必须记录：

* tenant_id
* user_id
* trace_id
* task_type
* provider
* model
* input_tokens
* output_tokens
* estimated_cost
* latency_ms
* status
* error_message
* fallback_from
* biz_ref
* created_at

开发阶段如果没有真实 API Key，必须使用 MockProvider，不能阻塞整体平台开发。

# 13. Word / PDF / ZIP 导出要求

这是核心风险，必须提前验证。

## 13.1 最小 docx 导出链路

Loop 4 必须完成最小 docx 导出，不得等到最后。

最小链路：

Tiptap JSON
→ 中间文档结构
→ docxtpl / python-docx
→ 生成 docx
→ MinIO 保存
→ bid_exports 记录
→ 前端下载

Loop 8 再增强：

1. 排版模板。
2. 页眉页脚。
3. 封面。
4. 水印。
5. 目录。
6. PDF 转换。
7. 分离标书分别导出。
8. ZIP 打包。

## 13.2 导出原则

1. 前端编辑器只负责内容结构。
2. 不做网页端 Word 级分页所见即所得。
3. Word 排版交给模板化导出服务。
4. 容器必须预装中文字体。
5. docx 导出后必须能在 Microsoft Word 正常打开。
6. 目录可刷新。
7. 中文字体不乱码。
8. 分离标书必须可分别导出技术标.docx、商务标.docx。
9. 分离标书必须可打包为投标文件全套.zip。

# 14. REST API 设计要求

统一前缀：

/api/v1

必须覆盖：

## Auth

* POST /auth/register
* POST /auth/login
* POST /auth/refresh
* POST /auth/logout
* GET /me

## Tenant / Team

* GET /tenant
* PATCH /tenant
* GET /tenant/members
* POST /tenant/members/invite
* PATCH /tenant/members/:id
* DELETE /tenant/members/:id
* GET /roles
* POST /roles
* PATCH /roles/:id
* DELETE /roles/:id

## Tender

* GET /tenders
* POST /tenders
* GET /tenders/:id
* PATCH /tenders/:id
* POST /tenders/:id/favorite
* DELETE /tenders/:id/favorite
* POST /tenders/:id/create-project
* POST /tenders/:id/create-bid
* GET /tender-sources
* POST /tender-sources
* PATCH /tender-sources/:id
* DELETE /tender-sources/:id
* POST /tender-sources/:id/verify

## Project

* GET /projects
* POST /projects
* GET /projects/:id
* PATCH /projects/:id
* DELETE /projects/:id
* POST /projects/:id/transition
* GET /projects/:id/milestones
* POST /projects/:id/milestones
* PATCH /projects/:id/milestones/:milestoneId
* DELETE /projects/:id/milestones/:milestoneId
* POST /projects/:id/members
* DELETE /projects/:id/members/:memberId
* POST /projects/:id/create-cost-project
* GET /projects/:id/activities

## Bid

* GET /bids
* POST /bids
* GET /bids/:id
* PATCH /bids/:id
* DELETE /bids/:id
* POST /bids/:id/upload-tender-file
* POST /bids/:id/parse-tender
* GET /bids/:id/parse-result
* PUT /bids/:id/parse-result
* POST /bids/:id/outline/generate
* GET /bids/:id/parts
* GET /bids/:id/parts/:partId/outline
* PUT /bids/:id/parts/:partId/outline
* GET /bids/:id/material-selection
* PUT /bids/:id/material-selection
* POST /bids/:id/generate
* GET /bids/:id/generation-jobs
* GET /generation-jobs/:jobId
* POST /generation-jobs/:jobId/pause
* POST /generation-jobs/:jobId/resume
* POST /generation-jobs/:jobId/cancel
* GET /bids/:id/generation/stream
* GET /bids/:id/chapters
* PATCH /chapters/:chapterId
* POST /chapters/:chapterId/accept
* POST /chapters/:chapterId/regenerate
* GET /chapters/:chapterId/versions
* GET /chapters/:chapterId/diff
* PUT /chapters/:chapterId/content
* POST /chapters/:chapterId/ai-action
* POST /bids/:id/exports
* GET /bid-exports/:exportId
* GET /bid-templates
* POST /bid-templates/:templateId/use

## Knowledge

* GET /knowledge
* GET /knowledge/categories
* POST /knowledge/categories
* PATCH /knowledge/categories/:id
* DELETE /knowledge/categories/:id
* GET /knowledge/tags
* POST /knowledge/tags
* PATCH /knowledge/tags/:id
* DELETE /knowledge/tags/:id
* GET /knowledge/documents
* POST /knowledge/documents
* GET /knowledge/documents/:id
* PATCH /knowledge/documents/:id
* DELETE /knowledge/documents/:id
* POST /knowledge/documents/:id/process
* GET /knowledge/documents/:id/preview
* GET /knowledge/documents/:id/references
* POST /knowledge/search
* GET /knowledge/templates
* POST /knowledge/templates
* GET /knowledge/stats

## Compliance

* POST /compliance/checks
* GET /compliance/checks
* GET /compliance/checks/:id
* GET /compliance/checks/:id/issues
* GET /compliance/checks/:id/stream
* POST /compliance/issues/:id/autofix
* POST /compliance/issues/:id/ignore
* POST /compliance/issues/:id/confirm-fail
* POST /compliance/checks/:id/report
* GET /compliance/rules
* POST /compliance/rules
* PATCH /compliance/rules/:id
* DELETE /compliance/rules/:id

## Cost

* GET /cost-projects
* POST /cost-projects
* GET /cost-projects/:id
* PATCH /cost-projects/:id
* GET /cost-projects/:id/items
* POST /cost-projects/:id/items
* PATCH /cost-items/:id
* DELETE /cost-items/:id
* GET /cost-projects/:id/analysis
* POST /cost-projects/:id/ai-advice
* POST /cost-projects/:id/report

## Approval

* GET /approval-chains
* POST /approval-chains
* PATCH /approval-chains/:id
* DELETE /approval-chains/:id
* POST /bids/:id/submit-for-approval
* GET /approvals
* GET /approvals/:id
* POST /approvals/:id/approve
* POST /approvals/:id/reject

## Notification

* GET /notifications
* POST /notifications/read
* GET /notifications/stream

## File

* POST /files/presign-upload
* POST /files/:id/confirm
* GET /files/:id/download-url
* GET /files/:id/preview-url

## AI Task

* GET /ai-tasks/:taskId

所有涉及 LLM、OCR、文档解析、向量化、导出、合规检查、逐章生成、AI 改写的接口必须异步任务化，返回 202 + task_id。前端使用 SSE + TanStack Query 轮询兜底。

# 15. 异步任务与状态机

## 15.1 任务队列

Go：

* Asynq
* 编排任务
* 状态机推进
* 调用 AI 服务
* 定时扫描
* 通知分发

Python：

* arq 或 Celery
* 文档解析
* OCR
* embedding
* RAG
* 章节生成
* 合规语义检查
* 导出

## 15.2 通用任务字段

任务必须记录：

* task_id
* tenant_id
* user_id
* trace_id
* task_type
* biz_type
* biz_id
* status
* progress
* retries
* error_message
* input_hash
* created_at
* updated_at
* completed_at

## 15.3 状态机禁止散落

所有状态流转必须经过显式 transition 函数。禁止散落 UPDATE status。

需要状态机的对象：

1. projects
2. bid_documents
3. bid_chapters
4. bid_generation_jobs
5. compliance_checks
6. compliance_issues
7. approval_instances
8. bid_exports
9. knowledge_documents

# 16. 工程目录要求

目标结构：

zhibiaotong/
├── docs/
│   ├── input/
│   ├── blueprint/
│   │   ├── PRD_V3_SAAS_BLUEPRINT.md
│   │   ├── DECISION_RECORD.md
│   │   ├── TECH_ARCHITECTURE.md
│   │   ├── PAGE_ROUTE_MAP.md
│   │   ├── DATABASE_SCHEMA.md
│   │   ├── API_SPEC.md
│   │   ├── OPENAPI_DRAFT.yaml
│   │   ├── STATE_MACHINES.md
│   │   ├── PERMISSION_MATRIX.md
│   │   ├── AI_PIPELINE.md
│   │   ├── MODEL_GATEWAY.md
│   │   ├── RAG_DESIGN.md
│   │   ├── DOCUMENT_EXPORT_DESIGN.md
│   │   ├── ACCEPTANCE_CRITERIA.md
│   │   ├── RISK_REGISTER.md
│   │   ├── DEV_LOOP_LOG.md
│   │   └── SAMPLE_DOCS_EVALUATION.md
│   └── sample_docs/
│       ├── README.md
│       ├── tender_samples/
│       ├── bid_samples/
│       └── expected_outputs/
├── frontend/
├── backend/
├── ai-service/
├── infra/
│   ├── nginx/
│   ├── docker/
│   └── scripts/
├── docker-compose.yml
├── .env.example
├── README.md
└── CLAUDE_LOOP.md

如果仓库已有结构，在不破坏现有结构的前提下调整。

# 17. Loop 开发协议

你必须按 Loop 开发，不得跳步。

每轮流程：

1. 阅读 docs/blueprint/。
2. 阅读 DEV_LOOP_LOG.md。
3. 判断当前完成状态。
4. 明确本轮目标。
5. 写入 DEV_LOOP_LOG.md。
6. 实现代码 / 文档 / 测试。
7. 运行检查命令。
8. 修复错误。
9. 更新文档。
10. 输出本轮总结。
11. 继续下一轮，除非上下文即将耗尽。

每轮结束输出格式：

## 本轮完成

* ...

## 关键文件

* ...

## 已运行检查

* ...

## 发现的问题

* ...

## 是否偏离蓝图

* 是 / 否
* 如是，说明原因和修正

## 下一轮目标

* ...

如果上下文快满，输出“下一轮接力 Prompt”，让我复制到新会话继续。

# 18. Loop 计划

## LOOP-0：蓝图与裁决表

目标：

1. 读取全部 PRD、HTML、技术方案评估文件。
2. 合并 V2.0-V2.4。
3. 识别 PRD 与 HTML、PRD 各版本之间的不一致。
4. 创建 DECISION_RECORD.md，逐条裁决。
5. 固化本 Prompt 的技术栈和硬约束。
6. 创建完整 docs/blueprint 文档。

必须生成：

* PRD_V3_SAAS_BLUEPRINT.md
* DECISION_RECORD.md
* TECH_ARCHITECTURE.md
* PAGE_ROUTE_MAP.md
* DATABASE_SCHEMA.md
* API_SPEC.md
* OPENAPI_DRAFT.yaml
* STATE_MACHINES.md
* PERMISSION_MATRIX.md
* AI_PIPELINE.md
* MODEL_GATEWAY.md
* RAG_DESIGN.md
* DOCUMENT_EXPORT_DESIGN.md
* ACCEPTANCE_CRITERIA.md
* RISK_REGISTER.md
* SAMPLE_DOCS_EVALUATION.md
* DEV_LOOP_LOG.md
* CLAUDE_LOOP.md

DECISION_RECORD.md 必须至少裁决：

1. 以 V2.4 14 页面为准。
2. HTML 原型仅作视觉参考。
3. 项目状态统一为 opportunity/bidding/compliance_review/submitted/closed。
4. 角色体系以 6 个预置角色为准。
5. 两个模板库导航名称区分为“标书模板”和“文档模板”。
6. 合规检查按 V2.1 双文件入口 + 标书模块自动带入实现。
7. 成本管理按项目成本列表 + 单项目分析实现。
8. Go + Python 双服务固定，不得降级全 Python。
9. pgvector 一期固定，预留 Qdrant。
10. 分离标书必须一期可用。
11. Word 最小导出 Loop 4 提前验证。
12. 不用 GORM、Drools、Rust 主后端、重 LangChain。

完成后不要停止，继续 LOOP-1。

## LOOP-1：工程脚手架与 Docker

目标：

1. 创建 monorepo。
2. 创建 frontend React + TypeScript + Vite。
3. 创建 backend Go + Gin + pgx + sqlc + goose。
4. 创建 ai-service Python FastAPI。
5. 创建 docker-compose。
6. 配置 PostgreSQL、Redis、MinIO。
7. PostgreSQL 启用 pgvector 扩展。
8. 建立 health check。
9. 建立 .env.example。
10. 建立 README 启动说明。
11. 建立基础 CI / 检查脚本。
12. 创建 sample_docs 目录与评测标准占位。

检查命令：

* docker compose config
* docker compose up -d
* curl backend /healthz
* curl ai-service /healthz
* pnpm install
* pnpm build
* go test ./...
* python -m compileall ai-service/app
* pytest 或说明尚无测试

验收：

1. 一条命令能启动基础服务。
2. 前端能打开。
3. Go healthz 正常。
4. Python healthz 正常。
5. PostgreSQL、Redis、MinIO 正常。
6. docs/blueprint 存在。

## LOOP-2：SaaS 底座

目标：

1. tenants
2. users
3. tenant_members
4. roles
5. permissions
6. role_permissions
7. module_permissions
8. JWT 登录
9. RBAC 中间件
10. RLS
11. audit_logs
12. notifications
13. file_assets
14. MinIO 预签名上传下载
15. 前端登录页
16. 前端路由守卫
17. 当前用户 / 当前租户 / 权限 store

必须测试：

1. 不同租户数据隔离。
2. 无权限访问 API 返回 403。
3. URL 访问无权限页面显示 403。
4. 文件下载不能越权。
5. RLS 生效。

## LOOP-3：前端后台框架与 14 页面路由壳

目标：

1. 主布局。
2. 侧边栏分组。
3. 可折叠侧边栏。
4. 面包屑。
5. 用户菜单。
6. 通知入口。
7. 权限菜单。
8. 14 个页面路由壳。
9. 登录后跳转 dashboard。
10. 工作台基础页面。
11. 所有页面 loading/empty/error/403 状态组件。

必须按 V2.4 实现 14 个页面。

不要照抄 HTML 中缺页的行为。

## LOOP-4：业务主数据 + 标书最小导出验证

目标：

1. Tender 基础模型和 API。
2. Project 基础模型和 API。
3. Bid 基础模型和 API。
4. Knowledge 基础模型和 API。
5. Compliance 基础模型和 API。
6. Cost 基础模型和 API。
7. Approval 基础模型和 API。
8. 标书 7 步向导 UI 骨架。
9. bid_documents / bid_parts / bid_chapters / bid_chapter_versions 模型。
10. Mock AI 解析 / 大纲 / 章节生成。
11. Tiptap 编辑器基础。
12. 最小 docx 导出链路。

重要：Loop 4 必须完成最小 docx 导出验证。

验收链路：

新建标书
→ 创建 bid_document
→ 创建 bid_part
→ 创建章节
→ 编辑 Tiptap 内容
→ 导出 docx
→ MinIO 保存
→ 前端下载

此阶段可使用 MockProvider 和基础模板。

## LOOP-5：标书生成主链路完整化

目标：

1. 新建标书页面。
2. 我的标书页面。
3. 标书模板库页面。
4. 标书类型选择：combined / separated / custom。
5. 分离标书 tech / business bid_parts。
6. 上传招标文件。
7. Mock 解析结果确认。
8. 目录大纲生成与编辑。
9. 知识库素材选择。
10. AI 逐章生成进度。
11. SSE 或模拟 SSE。
12. 每章采纳。
13. 每章重新生成。
14. 手动编辑。
15. 查看差异。
16. 章节版本。
17. 分离标书技术标/商务标分别编辑。
18. 分离标书技术标/商务标分别导出。
19. ZIP 打包接口占位或基础实现。

验收：

1. 综合标书完整走完 7 步。
2. 分离标书技术标/商务标分别生成。
3. 分离标书可分别进入编辑器。
4. 分离标书可分别导出 docx。
5. ZIP 打包至少能打包两个 docx。

## LOOP-6：知识库与文档处理

目标：

1. 文档库页面。
2. 文档模板页面。
3. 标签管理页面。
4. 分类树。
5. 上传文档。
6. 文档处理任务。
7. 文档解析流水线。
8. 文档预览。
9. 自动分类。
10. 自动标签。
11. 有效期识别。
12. 文档切片。
13. embedding MockProvider。
14. pgvector 入库。
15. 语义搜索。
16. 引用追踪。
17. 知识库统计。
18. 资质到期提醒。

Loop 6 开始使用 sample_docs 做解析评测。

验收：

1. PDF / Word 至少可解析。
2. 文档可切片。
3. chunk 可入库。
4. 搜索可返回结果。
5. 文档可预览。
6. 知识库三子页面可用。

## LOOP-7：模型网关真实化与 RAG

目标：

1. LLMProvider。
2. EmbeddingProvider。
3. RerankProvider。
4. OCRProvider。
5. ModelRouter。
6. model_routing.yaml。
7. ai_call_logs。
8. MockProvider。
9. OpenAI-compatible Provider。
10. DashScope / DeepSeek / Anthropic 等 Provider 预留。
11. fallback。
12. JSON Schema / Pydantic 校验。
13. token usage 记录。
14. cost estimate。
15. 招标解析真实接口。
16. 大纲生成真实接口。
17. RAG 检索真实接口。
18. 逐章生成真实接口。
19. 章节自检真实接口。
20. 改写助手真实接口。

没有 API Key 时必须继续使用 MockProvider，不能阻塞工程。

验收：

1. USE_MOCK_PROVIDERS=true 时全链路可跑。
2. 配置真实 Key 后可切真实 Provider。
3. 失败自动 fallback。
4. ai_call_logs 有记录。
5. RAG 返回 source_refs。
6. 生成内容有 source_refs。

## LOOP-8：合规检查闭环

目标：

1. 合规检查三步流程。
2. 上传招标文件 + 投标文件。
3. 从标书模块自动带入文件。
4. 检查配置。
5. L1 规则检查。
6. L3 废标风险结构化检查。
7. L2 语义一致性检查。
8. L4 评分优化建议 beta。
9. 检查进度。
10. 检查结果五类 Tab。
11. compliance_issues。
12. severity：pass/warn/fail_candidate/fail。
13. evidence。
14. 修复建议。
15. 跳转编辑器定位。
16. 一键修复接口。
17. 问题忽略。
18. fail_candidate 人工确认。
19. 检查报告导出。

验收：

1. 规则命中的确定性问题可 fail。
2. LLM 风险只能 fail_candidate 或 warn。
3. 问题可跳转编辑器。
4. 修复后可重新检查。
5. 报告可导出。

## LOOP-9：项目 / 成本 / 团队协作打通

目标：

1. 标讯创建项目。
2. 项目看板。
3. 项目列表。
4. 项目详情。
5. 里程碑弹窗。
6. 项目成员。
7. 项目关联标书。
8. 标书提交审批。
9. 审批链配置。
10. 审批实例。
11. 审批通过 / 驳回。
12. 通知中心。
13. 项目状态流转。
14. 中标后创建成本项目。
15. 成本录入。
16. 预算 vs 实际。
17. 利润率。
18. 成本 AI 建议。
19. 中标案例回流知识库。

验收：

标讯
→ 项目
→ 标书
→ 合规
→ 审批
→ 中标
→ 成本
→ 回流知识库

整条链路可演示。

## LOOP-10：导出增强与平台验收

目标：

1. Word 模板增强。
2. PDF 导出。
3. ZIP 打包。
4. 分离标书完整导出。
5. 目录、页眉页脚、水印、封面基础支持。
6. 预览。
7. 错误处理。
8. 大文件处理。
9. SSE 断线重连。
10. 权限越权扫描。
11. 多租户测试。
12. seed 演示数据。
13. README 完整启动说明。
14. 最终验收清单。

验收：

1. docx 可在 Microsoft Word 打开。
2. 中文字体正常。
3. 技术标/商务标可分别导出。
4. ZIP 包含多份文件。
5. PDF 可生成。
6. Docker 一键启动。
7. 14 页面可访问。
8. 8 模块均有真实骨架和业务流。
9. 多租户隔离通过。
10. 无权限 API 返回 403。

# 19. 检查命令要求

每轮尽可能运行：

前端：

* pnpm install
* pnpm lint
* pnpm typecheck
* pnpm build
* pnpm test

后端：

* go mod tidy
* go test ./...
* go vet ./...
* sqlc generate
* goose status
* goose up

AI 服务：

* python -m compileall app
* pytest
* ruff check .
* mypy app 或说明暂未配置

Docker：

* docker compose config
* docker compose up -d
* docker compose ps
* curl /healthz

如果因为环境缺失无法运行，必须说明原因，并给出替代验证方式。

# 20. 禁止事项

1. 禁止只写文档不写代码。
2. 禁止只写前端静态页面。
3. 禁止删除 8 大模块。
4. 禁止少于 V2.4 的 14 页面。
5. 禁止把完整 SaaS 平台缩成 AI 写标书 Demo。
6. 禁止把主后端降级为全 Python。
7. 禁止使用 GORM。
8. 禁止使用 Drools。
9. 禁止使用 Rust 作为一期主后端。
10. 禁止重度引入 LangChain 造成黑盒化。
11. 禁止业务代码直接调用具体模型 SDK。
12. 禁止前端直接调用模型 API。
13. 禁止在代码、prompt、日志、数据库中硬编码 API Key。
14. 禁止 AI 输出不校验直接入库。
15. 禁止生成没有 source_refs 的事实性企业内容。
16. 禁止无 tenant_id 查询业务表。
17. 禁止绕过 RLS。
18. 禁止直接公开 MinIO bucket。
19. 禁止绕过状态机直接 UPDATE status。
20. 禁止遇到错误就停止，必须尝试修复并记录。

# 21. 最终验收标准

完整 SaaS 平台雏形必须满足：

1. 可以启动前端、Go 后端、Python AI 服务、PostgreSQL、Redis、MinIO。
2. 可以注册 / 登录。
3. 可以创建企业租户。
4. 可以邀请成员。
5. 可以配置角色和权限。
6. 不同角色看到不同菜单。
7. API 权限校验有效。
8. 多租户数据隔离有效。
9. 工作台可展示统计、待办、通知。
10. 标讯大厅可展示、搜索、收藏、配置数据源。
11. 可从标讯创建项目。
12. 项目可在看板流转。
13. 项目详情可维护里程碑、成员、关联标书。
14. 可新建综合标书。
15. 可新建分离标书。
16. 分离标书包含技术标和商务标。
17. 可上传招标文件。
18. 可触发招标文件解析任务。
19. 可人工确认解析结果。
20. 可生成目录大纲。
21. 可编辑目录。
22. 可上传知识库文档。
23. 可解析知识库文档。
24. 可切片、embedding、搜索。
25. 可选择知识库素材。
26. 可逐章生成标书。
27. 每章可采纳、重新生成、手动编辑、查看差异。
28. 每章有版本。
29. 生成内容有 source_refs。
30. 无来源事实性内容标记 needs_human_input。
31. 可进入三栏编辑器。
32. 可导出综合标书 docx。
33. 可分别导出技术标 docx、商务标 docx。
34. 可打包 ZIP。
35. 可发起合规检查。
36. 合规检查有 pass/warn/fail_candidate/fail。
37. 合规问题有 evidence 和修复建议。
38. 可跳转编辑器定位问题。
39. 可提交审批。
40. 可审批通过或驳回。
41. 驳回后标书回到 editing。
42. 中标后可创建成本项目。
43. 可录入成本。
44. 可查看预算 vs 实际和利润率。
45. 中标案例可回流知识库。
46. AI 调用写 ai_call_logs。
47. 模型可通过 model_routing.yaml 切换。
48. 无真实 API Key 时 MockProvider 能跑通。
49. README 能指导新开发者启动。
50. DEV_LOOP_LOG.md 记录每轮完成状态。
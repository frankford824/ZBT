智标通 SaaS 平台技术方案评估
1. 总体判断
这个项目是"标准 B2B SaaS 底座 + 重 AI 文档流水线"的复合体，两部分的工程属性完全不同，必须分开设计、分开选型。

核心判断如下：

难度分布与多数人直觉相反。 真正的难点不在"AI 生成标书"（这是 prompt + RAG 工程，可控），而在三个脏活：招标文件解析准确率（PDF 扫描件、复杂表格、电子标书格式）、Word 导出的排版保真（投标文件对格式有硬性要求，格式错误可直接废标）、合规检查的误报/漏报治理（误报多用户弃用，漏报一次导致废标用户流失）。技术方案必须围绕这三点倾斜资源。
你的技术栈方向基本正确，但有 4 处我不同意：一期不该同时引入 Qdrant（pgvector 足够且隔离更简单）；Gin/GORM vs Chi/sqlc 需要明确取舍（我推荐 Gin + pgx + sqlc 混合）；标讯爬虫不应进一期核心路径（PRD V2.1 自己也降级为"已配置"模式）；"行业公共模板库 10万+ 中标方案"是内容运营问题不是技术问题，一期只做表结构预留。详见第 17 节。
一期范围必须裁剪。 V2.0–V2.4 累积的功能全集做完至少需要 6 个月以上。我在第 14 节给出的 Loop 计划以"全业务链路走通、每个模块达到可演示可用"为标准，部分增强功能（成本知识库、电子标书格式、自定义数据源自动抓取、甘特图）明确推迟。
多租户隔离必须从第一行代码开始做，采用共享库 + tenant_id + PostgreSQL RLS 双保险。事后补租户隔离是 SaaS 最昂贵的返工。
AI 能力必须经过统一模型网关，所有调用记录 token、成本、trace_id，否则 API 成本会在内测阶段就失控（逐章生成 + 四重合规检查 + 自检，单份标书的 token 消耗是数十万级别）。
2. 推荐技术栈
层	推荐选型	备选/说明
前端
React 18 + TypeScript + Vite + React Router v7 + TanStack Query v5 + Zustand + Ant Design 5 + Tiptap v2 + ECharts + React Hook Form + Zod
不用 Next.js（纯后台无 SEO 需求）；MUI 不选（中文 B2B 表格/表单密度场景 AntD 完胜）
主业务后端
Golang 1.23+，Gin + pgx + sqlc（复杂查询）/ 少量 GORM（简单 CRUD 可不用）+ goose 迁移 + Asynq（Redis 任务队列）
一期不用 Rust；模块化单体，不拆微服务
AI 服务
Python 3.12 + FastAPI + arq（异步任务）+ Pydantic v2
独立服务，承担解析/RAG/生成/合规语义层/导出
文档处理
PyMuPDF（PDF）+ python-docx / docxtpl（Word 读写）+ openpyxl + python-pptx + LibreOffice Headless（格式转换/PDF导出） + PaddleOCR 或云 OCR（抽象层）
导出链路放 Python 服务，不放 Go
数据库
PostgreSQL 16（含 RLS 多租户隔离）
单库多 schema 不采用，共享表 + tenant_id
缓存/队列
Redis 7（缓存 + Asynq/arq 队列 + SSE pub/sub）
一期不引入 Kafka
对象存储
MinIO（S3 协议），生产可切阿里云 OSS/腾讯 COS
按 tenant_id/ 前缀隔离
向量库
pgvector（一期），预留 Qdrant 迁移接口
见第 3 节取舍
模型网关
自研轻量网关（Python，内嵌于 AI 服务）：LLM/Embedding/Rerank/OCR Provider 抽象 + ModelRouter + model_routing.yaml
不直接依赖 LangChain 全家桶，可借用 LiteLLM 做 Provider 适配底层
部署
Docker Compose（开发/一期生产）→ K8s（后期）
前端静态托管 + Nginx 反代；Go/Python/Postgres/Redis/MinIO 五容器
3. 技术栈取舍说明
React vs Vue3
选 React。理由：① Tiptap、TanStack Query、AntD 5、各类 diff/虚拟滚动库在 React 生态最成熟，标书编辑器（三栏 + 内联 AI 菜单 + 版本对比）是本项目前端最重的部分，生态决定速度；② TypeScript 类型推导在 React（纯函数组件）下比 Vue SFC 更顺畅，对 AI Agent 写代码的可校验性更好；③ Vue3 并非不行，但没有任何一项本项目的需求是 Vue 显著更优的，没有切换理由。

Vite vs Next.js
选 Vite。这是登录后才能用的 B2B 后台，无 SEO、无首屏 SSR 需求。Next.js 引入 RSC/服务端运行时只会增加部署复杂度和 AI Agent 出错面。纯 SPA + Nginx 静态托管最简单。

Ant Design vs MUI
选 AntD 5。本项目页面密度极高：ProTable 级别的列表（我的标书、项目列表、成本明细）、复杂表单（监控设置、审批链配置）、Tree（章节大纲、文档分类）、Drawer（项目详情）、Steps（7 步向导）。AntD 对这类中文企业后台是量身定做；MUI 的表格和表单需要大量二次封装。

Zustand vs Redux Toolkit
选 Zustand + TanStack Query 组合。关键认知：本项目 90% 的"状态"其实是服务端状态（列表、详情、任务进度），应全部交给 TanStack Query 管理（缓存、失效、轮询 SSE 补偿）。剩下的真正客户端状态只有：当前用户/租户/权限、侧边栏折叠、生成向导步骤、编辑器 UI 状态——Zustand 几个小 store 足够。Redux Toolkit 的模板量在此场景是纯负担。TanStack Query 不是可选项，是必需品：合规检查/生成任务的轮询、乐观更新（看板拖拽）、按租户的缓存 key 设计都依赖它。

Tiptap 是否适合标书编辑器
适合，且是目前最优解，但要清醒认识边界：

优势：ProseMirror 内核，JSON 文档模型（天然适合按章节存储、版本 diff、AI 局部替换）、自定义 Node（引用来源标注块、合规问题锚点）、协同编辑可后期接 Yjs。
边界：Tiptap 不是 Word。页码、分页预览、页眉页脚、目录域这些"纸面排版"概念它没有。结论：编辑器只负责内容结构（章节树 + 富文本 JSON），排版保真由导出服务（docxtpl 模板 + LibreOffice）负责。前端不要试图做"所见即所得的 Word 排版"，这是无底洞。
Go vs Rust（主业务后端，重点比较）
结合本项目场景逐项对比：

维度	Golang	Rust	本项目权重
多租户 CRUD/RBAC/审批流/通知 等"业务面条代码"开发速度
快，生态模板多
慢，所有权系统在频繁改动的业务逻辑上摩擦大
★★★★★（本项目 70% 代码是这类）
Web 框架/ORM/迁移工具成熟度
Gin/pgx/sqlc/goose 全部生产级
axum/sqlx 可用但周边（admin、审计、RBAC 库）薄
★★★★
异步任务/队列
Asynq 开箱即用
需自己拼
★★★★
AI Agent 生成代码的正确率
高（语料多、编译错误信息直白）
明显更低（生命周期报错对 Agent 不友好，返工率高）
★★★★（你计划用 Claude Code 循环开发）
极致性能/内存安全
够用
更优
★★（本项目瓶颈在 LLM 延迟和文档解析，不在主后端 CPU）
高性能文档转换/检索加速
一般
优
★★（后期可作为独立 sidecar 引入）
招聘/维护成本
低
高
★★★
结论：一期主后端选 Golang，明确否决 Rust 作为一期主后端。 本项目主后端的本质是"带工作流的多租户 CRUD + 任务编排"，Rust 的优势（极致性能、内存安全）在此场景兑现不了，劣势（业务迭代摩擦、Agent 写码返工）每天都在付费。你对 Rust 的定位（后期高性能文档处理/转换 sidecar）是对的，保留。 也顺带否决另外三个候选：Java/Spring 适合大团队强规范场景，单人/小团队 + Agent 开发太重；NestJS 可行但 Node 在 CPU 密集的导出/校验上吃亏且类型安全弱于 Go；Python 做主后端是唯一值得认真考虑的替代（见第 17 节——如果团队 ≤2 人，全 Python 单体是合理降级方案）。

Python AI 服务（独立服务的理由）
PyMuPDF、python-docx、PaddleOCR、各家 LLM SDK、LibreOffice 自动化——文档/AI 生态被 Python 垄断，没有第二选项。独立成服务而非塞进主后端的理由：① 依赖重（OCR 模型、LibreOffice）、镜像大、需要独立伸缩；② 长任务（解析 2 分钟、生成 20 分钟）与短请求的资源模型不同；③ 故障隔离——AI 服务 OOM 不能拖死登录和项目管理。但只拆这一刀，Go 侧保持模块化单体，不要再拆。

Qdrant vs pgvector
一期选 pgvector，理由：① 数据规模：5–20 人企业租户，私有知识库每租户几百几千文档、几万几十万 chunk，pgvector + HNSW 索引完全够；② 多租户隔离免费获得——WHERE tenant_id = ? + RLS，Qdrant 需要额外设计 collection/payload 隔离策略；③ chunk 的 metadata（文档类型、标签、有效期）过滤就是普通 SQL，metadata filter + 向量检索 + 关键词检索（pg 的 tsvector/bm25）可以一条 SQL 混合完成；④ 少运维一个有状态组件。 预留迁移路径：检索走 VectorStore 接口抽象，当单租户 chunk 超过百万级或 QPS 上来后再迁 Qdrant。

4. SaaS 平台总体架构
┌──────────────────────────────────────────────────────────────────────┐
│  前端 SPA (React + Vite)                                              │
│  14 页面 / 路由守卫(权限) / TanStack Query / SSE 订阅任务进度          │
└───────────────┬──────────────────────────────────────────────────────┘
                │ HTTPS (REST + SSE)
┌───────────────▼──────────────────────────────────────────────────────┐
│  Nginx / API Gateway（TLS、限流、静态资源）                            │
└───────┬──────────────────────────────────────────────┬───────────────┘
        │                                              │ (内网，仅 Go 可调)
┌───────▼───────────────────────────┐   ┌──────────────▼───────────────┐
│ 主业务后端 (Go, 模块化单体)         │   │ AI 服务 (Python FastAPI)      │
│ ├─ auth / tenant / rbac           │◄──┤ ├─ 模型网关                   │
│ ├─ tender (标讯/数据源/监控)        │HTTP│ │  LLM/Embedding/Rerank/OCR  │
│ ├─ project (项目/里程碑/成员)       │+回调│ │  Provider + ModelRouter    │
│ ├─ bid (标书/章节/版本/导出元数据)  │   │ ├─ 文档解析流水线              │
│ ├─ knowledge (文档/标签/引用)       │   │ │  PDF/Word/Excel/PPT/OCR    │
│ ├─ compliance (检查/问题项)         │   │ ├─ 切片/Embedding/索引        │
│ ├─ cost / approval / notification  │   │ ├─ RAG 检索 (混合检索+rerank) │
│ ├─ file (上传/预签名URL)           │   │ ├─ 招标解析/逐章生成/章节自检   │
│ ├─ audit_log                      │   │ ├─ 合规语义检查(第2/4层)       │
│ └─ 任务编排 (Asynq) ──────────────►│   │ ├─ 导出引擎 docxtpl+LibreOffice│
│         状态机持有者                │   │ └─ 任务执行 (arq worker)      │
└──┬────────┬────────┬──────────────┘   └──┬────────────┬──────────────┘
   │        │        │                     │            │
┌──▼─────┐┌─▼─────┐┌─▼──────────┐      ┌──▼─────┐  ┌───▼──────────────┐
│Postgres││ Redis ││  MinIO     │      │ Redis  │  │ 外部模型 API      │
│ +RLS   ││缓存/队 ││ tenant 前缀 │      │arq队列  │  │ DeepSeek/Qwen/   │
│+pgvector││列/SSE ││ 原文件/导出 │      │        │  │ Claude/OCR云服务  │
└────────┘└───────┘└────────────┘      └────────┘  └──────────────────┘
原则：
· Go 是唯一的状态机持有者和数据写入方（AI 服务通过回调把结果交还 Go 落库）
· AI 服务无业务状态，可水平扩展；ai_call_logs 由网关写入（经 Go 或直写专表）
· 前端永不直连 AI 服务；所有文件经预签名 URL 直传 MinIO
5. 14 个页面与前端路由设计
以 V2.4 页面清单为基准（8 主页面 + 6 子页面），并补充 PRD 中实际需要但页面清单未列出的详情级路由：

#	V2.4 页面	路由	说明
1
工作台 page-dashboard
/dashboard
6 统计卡 + AI 推荐 + 待办 + 趋势图
2
标讯大厅 page-tender
/tenders
Tab: 全部/AI推荐/监控/收藏/监控设置（Tab 用 searchParams）
—
标讯详情（V2.0 §4.2.1）
/tenders/:tenderId
解析摘要、匹配度、一键创建项目/生成标书
3
标书生成向导 page-generate
/bids/:bidId/wizard?step=1..7
7 步向导，step 持久化在 URL，可恢复
4
新建标书 page-generate-new
/bids/new
创建 bid_document 草稿后跳 wizard
5
我的标书 page-generate-list
/bids
统计卡 + 筛选 + 批量操作
6
标书模板库 page-generate-templates
/bids/templates
行业分类 + 我的模板
—
标书编辑器（Step 6 独立化）
/bids/:bidId/editor
三栏布局；多标书 Tab（技术标/商务标）用 ?part=
7
合规检查 page-compliance
/compliance
三步流程入口 + 历史检查列表
—
检查结果
/compliance/:checkId
评分概览 + 5 个结果 Tab + 跳转修复
8
项目管理 page-project
/projects?view=board|list
看板/列表切换
—
项目详情
/projects/:projectId
原型用 Drawer，建议路由化以便分享链接
9
成本管理 page-cost
/costs
项目成本列表
—
单项目成本分析
/costs/:costProjectId
构成图表 + 录入 + 利润分析
10
文档库 page-knowledge-docs
/knowledge/docs
左分类树 + 文档列表
11
知识库模板库 page-knowledge-templates
/knowledge/templates
企业文档模板
12
标签管理 page-knowledge-tags
/knowledge/tags
左标签列表 + 右详情
13
知识库主页 page-knowledge
/knowledge
分类卡 + 语义搜索 + 使用统计（V2.2 §7.2）
14
团队协作 page-team
/team?tab=members|approvals|logs|notifications
成员/审批/日志/通知
—
登录/注册/租户初始化
/login /register /onboarding
PRD 未画但 SaaS 必需
路由守卫设计：<RequirePermission module="cost"> 包装路由元素；权限来自登录后 /me 返回的 permissions: { cost: "none|read|full", ... }；无权限菜单按 V2.3 矩阵隐藏，URL 直接访问则渲染 403 页。侧边栏分组（概览/投标准备/投标管控/企业管理）为纯前端配置，与 V2.3 §3.1 一致。

组件拆分策略（大型 SaaS 后台）：按 feature 切分而非按类型切分——src/features/{tender,bid,compliance,project,cost,knowledge,team}/ 各自包含 api/(TanStack Query hooks)、components/、stores/、types/(Zod schema 即类型源)；跨 feature 复用沉到 src/shared/（ProTable 封装、SSE hook、权限组件、文件上传组件）。编辑器单独 src/features/editor/ 并懒加载（Tiptap + diff 库体积大）。禁止 feature 之间直接 import 内部组件，只能走各自的 index.ts 公共出口——这条规则对 AI Agent 循环开发尤其重要，防止耦合熵增。

6. 后端服务模块设计
Go 主业务后端（模块化单体）
/cmd/server          入口
/internal
  /platform          ── SaaS 底座，先于一切业务
    auth             JWT 签发/刷新、密码、登录限流
    tenant           租户、成员、邀请
    rbac             角色、权限、模块权限矩阵、中间件
    audit            审计日志（中间件自动采集 + 业务手动埋点）
    notification     通知中心（站内 + 邮件适配器）
    file             文件资产、MinIO 预签名、防越权下载
  /modules
    tender           标讯、数据源配置、关键词监控、收藏
    project          项目、里程碑、项目成员、看板状态机
    bid              标书、bid_part(技术标/商务标)、章节、版本、生成任务编排、导出任务
    knowledge        文档、分类、标签、引用关系、回流
    compliance       检查任务编排、规则引擎(第1/3层结构化部分)、问题项
    cost             成本项目、成本明细、利润分析
    approval         审批链定义、审批实例、条件路由(金额阈值)
    dashboard        统计聚合、待办、AI 推荐读取
  /jobs              Asynq handlers：调 AI 服务、超时治理、资质到期扫描、案例回流
  /aiclient          调 AI 服务的 typed client（含 trace_id 透传）
要点：

规则引擎放 Go：合规第一层（资质清单核对、签章项、格式规范、报价一致性）和第三层的结构化部分是确定性规则，用 Go 实现"规则表 + 检查器接口"，不要用 Drools（V2.0 提到的 Drools 是 Java 生态，否决），也不要用 LLM 做能用规则做的事。
状态机集中管理：项目状态、标书状态、生成任务、检查任务、审批实例的状态流转全部经过显式的 transition 函数（校验合法迁移 + 写审计日志 + 发通知），禁止散落的 UPDATE status。
Python AI 服务
/app
  /gateway           模型网关（见第 10 节）
  /pipelines
    parse            文档解析流水线（分格式 extractor + OCR 决策 + 表格抽取）
    tender_extract   招标文件结构化抽取（项目信息/资质/评分/废标条款）
    chunking         切片器（结构感知）
    indexing         embedding + 入库 pgvector
    retrieval        混合检索 + rerank + metadata filter
    generation       大纲生成 / 逐章生成(SSE流式) / 章节自检 / 改写助手
    compliance       第2层语义一致性 / 第4层评分优化（LLM 推理）
    cost_advice      成本优化建议
    export           Tiptap JSON → docx(docxtpl) → PDF(LibreOffice headless)
  /workers           arq 任务消费者
  /callbacks         任务完成回调 Go（带 HMAC 签名）
服务间契约：Go → Python 全部为"提交任务"语义（立即返回 task_id），Python 完成后回调 Go 落库；唯一例外是逐章生成的 SSE 流，由 Go 反向代理转发给前端（保持前端只面对一个域）。

7. 数据库核心表设计
所有业务表均含 id (uuid)、tenant_id、created_at、updated_at，启用 RLS。以下只列关键字段。

SaaS 底座
表	关键字段	说明
tenants
name, plan, status, storage_quota_bytes, settings(jsonb)
users
email(uniq), phone, password_hash, name, status
全局用户，可属多租户
tenant_members
tenant_id, user_id, role_id, status(invited/active/disabled), invited_by
uniq(tenant_id,user_id)
roles
tenant_id(null=系统预置), name, is_system
预置：超管/部门管理员/项目经理/投标专员/查看者（V2.1）
permissions
code, module, action
如 cost.read bid.export
role_permissions
role_id, permission_id
module_permissions
tenant_id, role_id, module, level(none/read/full)
V2.1 模块级管控，驱动侧边栏
audit_logs
tenant_id, user_id, action, resource_type, resource_id, detail(jsonb), ip
仅插入，按月分区
notifications
tenant_id, user_id, type, title, payload(jsonb), read_at, channel
file_assets
tenant_id, bucket, object_key, filename, size, mime, sha256, uploaded_by, biz_type, biz_id
所有文件引用的唯一事实源
标讯
表	关键字段	说明
tender_sources
tenant_id(null=平台预置), name, url, type, region, status(connected/configured/unavailable), fetch_freq, credentials(加密)
V2.1 数据源管理
tenders
source_id, title, region, industry, budget, publish_at, deadline_at, raw_url, attachment_file_id, ai_match_score, summary(jsonb)
平台级数据 + 租户视角表
tender_user_states
tenant_id, tender_id, is_favorite, is_monitored
收藏/监控是租户态，与标讯本体分离
tender_parse_results
tender_id 或 file_asset_id, status, project_info(jsonb), qualification_reqs(jsonb), scoring_criteria(jsonb), rejection_clauses(jsonb), tech_summary, confirmed_by, confirmed_at
用户确认后的版本才是下游事实源；jsonb 内每条目带 source_ref(页码/原文坐标)
项目
表	关键字段	状态流转
projects
name, tender_id, status, priority, budget, deadline_at, owner_id, win_amount, result(won/lost/pending)
opportunity → bidding(标书制作) → compliance_review → submitted(投标中) → closed(已结果)，closed 时写 result
project_milestones
project_id, name, status(pending/active/done), due_at, note, sort
V2.3 弹窗编辑
project_members
project_id, user_id, role(owner/editor/reviewer)
标书（核心域）
表	关键字段	说明
bid_documents
project_id, tender_id, name, type(combined/separated/custom), status, wizard_step(1-7), parse_result_id, deadline_at
状态机：draft → generating → editing → in_review → approved → submitted → archived；驳回回 editing
bid_parts
bid_document_id, part_type(tech/business/boq/attachment), name, sort, outline_locked_at
V2.2 分离标书：综合标书也建 1 个 part，统一模型
bid_chapters
bid_part_id, parent_id, title, level, sort, status(pending/generating/generated/accepted/edited/needs_fix), score_weight, requirement_refs(jsonb), current_version_id
树结构；score_weight 用于评分点高亮
bid_chapter_versions
chapter_id, version_no, content(jsonb=Tiptap doc), source(ai/human/ai_rewrite), source_refs(jsonb), self_check(jsonb), generation_job_id, created_by
引用与自检结果跟版本走，重新生成=新版本，差异对比=两版本 diff
bid_generation_jobs
bid_document_id, scope(outline/chapter/full), chapter_id, status(queued/running/paused/done/failed/cancelled), progress, model_used, prompt_tokens, completion_tokens, error, trace_id
暂停/继续即状态机操作
bid_exports
bid_document_id, format(docx/pdf/zip), template_id, settings(jsonb 页眉页脚水印), status, file_asset_id, error
异步任务
bid_templates
tenant_id(null=平台), category, name, outline(jsonb), usage_count, rating
标书模板库
知识库
表	关键字段	说明
knowledge_documents
category_id, file_asset_id, title, doc_type(qualification/case/solution/personnel/template/contract), ai_meta(jsonb：资质名/有效期/发证机关/合同金额…), expire_at, status(parsing/indexed/failed), source(upload/backflow), backflow_project_id
expire_at 驱动资质到期提醒
knowledge_categories
tenant_id, name, parent_id
左侧分类树
knowledge_chunks
document_id, chunk_index, content, embedding vector(1024), token_count, metadata(jsonb：页码/章节路径/doc_type/tags)
pgvector HNSW 索引 + (tenant_id, document_id) btree；另建 tsvector 列做混合检索
knowledge_tags
tenant_id, name(uniq per tenant), color
knowledge_document_tags
document_id, tag_id, source(ai/human)
knowledge_references
document_id, chunk_id, bid_chapter_version_id
引用追踪："被哪些标书引用"+引用排行均查此表
合规
表	关键字段	说明
compliance_checks
bid_document_id(可空，支持外部文件), bid_file_id, tender_file_id, parse_result_id, layers(jsonb 启用哪几层), scope, status(queued/running/done/failed), score, summary(jsonb 各 tab 统计), trace_id
compliance_issues
check_id, layer(1-4), category(qualification/format/scoring/rejection/semantic), severity(pass/warn/fail), title, detail, suggestion, location(jsonb：chapter_id+锚点), auto_fixable, status(open/fixed/ignored), fixed_version_id
跳转修复用 location；编辑后状态联动
compliance_rules
tenant_id(null=内置), layer, category, rule_type, params(jsonb), enabled
自定义规则（V2.0 §4.4.2）
成本与审批
表	关键字段	说明
cost_projects
project_id(uniq), win_amount, status(active/closed), budget_total
中标后从项目创建，预填中标金额
cost_items
cost_project_id, category(labor/material/equipment/other), phase, name, budget_amount, actual_amount, occurred_at, note
实际 vs 预算对比直接聚合此表
approval_chains
tenant_id, name, biz_type(bid_submit/...), steps(jsonb：[{level, approver_role/user, condition:{amount_gt}}])
V2.2 审批链配置
approval_instances
chain_id, biz_type, biz_id, current_step, status(pending/approved/rejected/cancelled), snapshot(jsonb 链快照)
链快照防止配置变更影响在途审批
approval_actions
instance_id, step, approver_id, action(approve/reject), comment, acted_at
AI 审计
表	关键字段	说明
ai_call_logs
tenant_id, trace_id, task_type(parse/outline/chapter_gen/self_check/compliance/rewrite/embed/rerank/ocr), provider, model, prompt_tokens, completion_tokens, cost_estimate(numeric), latency_ms, status, fallback_from, biz_ref(jsonb)
按月分区；成本审计与租户配额的数据源
关键关系链：tenders → tender_parse_results → projects → bid_documents → bid_parts → bid_chapters → bid_chapter_versions ←(source_refs)→ knowledge_chunks；compliance_issues.location → bid_chapters；projects(won) → cost_projects；projects(won) → knowledge_documents(backflow)。这条链就是产品主流程在数据层的投影。

8. AI / RAG / 文档处理方案
任务-技术映射总表（哪些用规则、LLM、embedding、OCR）
任务	规则引擎	Embedding/检索	LLM	OCR	说明
PDF 文本层提取
—
—
—
—
PyMuPDF 直取
扫描件/图片识别
—
—
—
✅
文本层覆盖率 <60% 即判定扫描件走 OCR
表格抽取（评分表/工程量清单）
—
—
兜底
部分
PyMuPDF/Camelot 规则优先，失败页用多模态 LLM 兜底
招标文件结构化抽取
预筛(正则定位"废标""★""▲"条款)
—
✅ 主力
—
LLM 输出 JSON Schema 严格校验，每字段带 source_ref
文档自动分类/打标签
文件名/关键词规则先行
✅ 辅助
✅ 低成本模型
—
知识库切片
✅ 结构感知
—
—
—
见下文切片策略
向量化
—
✅
—
—
BGE-M3 或商用 embedding
检索
metadata filter(SQL)
✅ 向量+BM25 混合
—
—
Rerank
—
✅ 专用 rerank 模型
—
—
不用 LLM 当 reranker（贵且慢）
大纲生成
模板库骨架优先
✅ 检索同类大纲
✅
—
逐章生成
—
✅ RAG
✅ 高质量模型
—
章节自检
—
—
✅ 低成本模型
—
对照 requirement_refs 逐条核
合规第1层（资质/格式/报价一致性）
✅ 纯规则
—
—
—
禁止用 LLM
合规第2层（语义一致性）
—
✅ 定位响应段落
✅
—
条款↔响应配对后 LLM 判断
合规第3层（废标条款核对）
✅ 清单驱动
✅ 定位
✅ 判断
—
规则定位+LLM 判定+人工最终确认
合规第4层（评分优化建议）
—
✅
✅ 高质量模型
—
默认关闭，按需开（V2.1）
AI 改写助手（优化/扩写/缩写）
—
可选
✅
—
编辑器内联，流式
成本优化建议
✅ 统计先行(占比/同比)
—
✅ 解读
—
数字由 SQL 算，LLM 只负责解读表述
资质到期提醒
✅ 纯定时任务
—
—
—
文档解析流水线
file_asset → 格式分流
  PDF: PyMuPDF 提取文本层 → 覆盖率检测 → (低覆盖) OCRProvider 逐页识别
       表格: 先 PyMuPDF table finder → 失败页截图 → 多模态模型抽取为结构化 JSON
  DOCX: python-docx 抽取段落树/表格/样式 → 保留标题层级
  DOC/XLS/PPT 旧格式: LibreOffice headless 先转 docx/xlsx/pdf 再走对应链路
  图片: 直接 OCR
输出统一为「中间文档模型」: [{type: heading|para|table|figure, text, page, bbox, level}]
切片（chunking）策略
结构感知切片：按标题层级切，目标 300–500 token，超长段落滑窗（overlap 15%）；表格整表为一个 chunk（带表头上下文），不拆行。
每个 chunk 携带 metadata：{doc_id, doc_type, page, heading_path, tags, expire_at} —— 这是 metadata filter 的基础（如生成"资质响应"章节时 filter doc_type=qualification AND expire_at > 投标日）。
RAG 检索链路
query 构造（章节标题 + 该章 requirement_refs 原文）
→ 混合召回：pgvector 余弦 TopK=30  ∪  tsvector/BM25 TopK=30（tenant_id + metadata filter 在 SQL 层）
→ RRF 融合去重 → RerankProvider 精排 → Top 5–8
→ 进入生成 prompt，每段附 [来源: 文档名 p.X]
逐章生成与自检
输入：章节标题 + 对应招标要求（从 parse_result 按章映射）+ rerank 后素材 + 前文摘要（不塞全文，塞已生成章节的 200 字摘要链）。
输出约束：内容中以标记语法内联引用（如 {{ref:chunk_id}}），落库时解析为 source_refs 并写 knowledge_references——这是"生成内容无引用"风险的硬性解法：没有 ref 的关键事实段落在自检中标红。
自检：低成本模型对照该章 requirement_refs 输出逐条 {requirement, satisfied: yes/no/partial, evidence}，结果存 self_check 字段，前端渲染"AI 自检提示"。
SSE 流式推送 token 到前端；暂停/继续操作改 job 状态，worker 在章节边界检查。
合规检查执行模型
第 1 层（Go 规则引擎，秒级）→ 第 3 层结构化部分（Go，废标清单来自 parse_result）→ 第 2 层（Python：条款-响应对齐用 embedding 定位 + LLM 逐对判断）→ 第 4 层（可选，高质量模型推理）。各层结果统一写 compliance_issues，severity 三级。所有 LLM 判定类 issue 默认 severity 不高于 warn，只有规则引擎的确定性命中才能出 fail——这是控制误报伤害的关键设计。

Word/PDF 导出（核心难点，单独强调）
Tiptap JSON(章节树) → 中间表示 → docxtpl/python-docx 按「排版模板」渲染
  排版模板 = 预制 .docx 母版（样式、页眉页脚、封面、目录域、字体字号行距）
  内容只填充，不在代码里拼样式
→ docx 后处理：目录域刷新标记、水印、页码
→ PDF：LibreOffice headless 转换（容器内预装中文字体！）
验收标准必须包含"导出的 docx 在 Microsoft Word 中打开无格式错乱、目录可 F9 刷新、中文字体正确"。

9. 商用模型 API 选型方案
模型版本快速演进，下表为撰写时点的建议档位；落地前用第 10 节的网关跑一周真实标书样本评测再锁版。

方案 A：成本优先
用途	模型	说明
高质量生成（逐章生成、第4层合规）
DeepSeek-V3 / deepseek-chat
中文长文生成性价比第一档
低成本任务（自检、打标、分类、摘要）
Qwen-Flash / GLM-4-Flash
接近免费
Embedding
本地 BGE-M3（GPU 可选 CPU 也能跑）
零边际成本，1024 维
Rerank
本地 bge-reranker-v2-m3
OCR
PaddleOCR 本地 + 百度/腾讯云 OCR 按量兜底
多模态（表格兜底）
Qwen-VL-Plus
按页计费，量小
Fallback
DeepSeek ↔ Qwen 双向互备
预估：单份标书（解析+生成 8 章+自检+合规）¥5–15。

方案 B：质量优先
用途	模型	说明
高质量生成
Claude Sonnet 4.x（生成主力，长文结构与遵循指令最佳）；关键章节可上 Claude Opus / GPT-5 档
招标解析/合规语义
Claude Sonnet（结构化抽取+合规推理强）
低成本任务
Claude Haiku / GPT mini 档
Embedding
OpenAI text-embedding-3-large 或 Cohere embed-v4（多语）
Rerank
Cohere Rerank 3.5
OCR/DocIntel
Azure Document Intelligence（表格还原最强）
解决扫描件表格难题
Fallback
Claude → GPT → DeepSeek 三级
预估单份标书 ¥40–120。注意：海外 API 需评估投标文件数据出境的合规风险——这是 B 方案最大的非技术问题，对政府采购客户可能直接不可接受。

方案 C：国内可控 / 私有化友好
用途	模型	说明
高质量生成
Qwen-Max（公有云）/ Qwen 72B 级开源版（vLLM 私有化）
同系模型云、私两栖，切换成本最低
备选生成
DeepSeek-V3（可私有化部署）、GLM-4
低成本任务
Qwen-Plus / 私有化 Qwen 14B 级
Embedding
BGE-M3（私有化）/ DashScope text-embedding-v3
Rerank
bge-reranker-v2-m3（私有化）/ DashScope gte-rerank
OCR
PaddleOCR（完全私有化）+ 阿里读光/合合 TextIn 云端可选
多模态
Qwen-VL（开源版可私有化）
私有化推理
vLLM + 2×A100/4090 集群起步
我的推荐：一期落地 = 方案 A 为主干 + 方案 C 的本地 embedding/rerank/OCR + 网关里预留 B 档模型用于"质量对照评测"。 私有化部署作为售卖选项（PRD §6.3 已承诺支持），架构上由方案 C 保证可行，但一期不实际部署本地 LLM。

本地模型是否需要：embedding/rerank/OCR 本地化——需要（成本、延迟、数据不出域三赢）；LLM 本地化——一期不需要（运维成本远超 API 费用，质量也打折），仅作为私有化交付选项保留。

成本控制策略：① 租户级月度 token 配额（ai_call_logs 聚合，超额降级到低成本模型并通知）；② 任务级模型路由（自检/打标永远不许用旗舰模型，由网关强制）；③ 招标解析结果缓存（同一文件 sha256 不重复解析）；④ prompt 缓存（各家的 context caching，逐章生成共享的招标要求前缀命中缓存可省 50%+）；⑤ 第 4 层合规默认关闭（PRD V2.1 已如此设计，保持）。

10. 模型网关与配置设计
Provider 接口（Python，示意签名——属于设计文档非实现代码）
LLMProvider:        complete(messages, schema?, stream?, opts) -> LLMResult{text|json, usage, finish_reason}
EmbeddingProvider:  embed(texts: list[str]) -> list[vector]
RerankProvider:     rerank(query, docs, top_n) -> list[{index, score}]
OCRProvider:        recognize(image|pdf_page) -> {blocks: [{text, bbox, conf}], tables?}
ModelRouter.resolve(task_type, tenant_id) -> (provider, model, params)
  · 读取 model_routing.yaml；支持租户级 override（私有化客户指定专属模型）
  · 失败时按 fallback 链重试：每级重试 2 次(指数退避, 仅对 429/5xx/超时)，再降级下一 provider
  · JSON 任务：响应经 Pydantic 模型校验，失败→带错误信息重试 1 次→仍失败走 fallback
  · 每次调用写 ai_call_logs(trace_id 贯穿 Go→Python→Provider)
MockProvider: 录制/回放固定响应，用于 CI 和前端联调，零成本
model_routing.yaml 示例
providers:
  deepseek:
    type: openai_compatible
    base_url: https://api.deepseek.com/v1
    api_key_env: DEEPSEEK_API_KEY          # 只存环境变量名，不存密钥
  dashscope:
    type: openai_compatible
    base_url: https://dashscope.aliyuncs.com/compatible-mode/v1
    api_key_env: DASHSCOPE_API_KEY
  local_bge:
    type: local
    endpoint: http://embedder:8001
  mock:
    type: mock
    fixtures_dir: ./tests/fixtures/ai
pricing:                                    # 用于 cost_estimate 计算
  deepseek-chat: {prompt: 0.001, completion: 0.002, unit: CNY/1k}
  qwen-plus:     {prompt: 0.0008, completion: 0.002, unit: CNY/1k}
routes:
  tender_parse:
    primary:  {provider: deepseek, model: deepseek-chat, temperature: 0.1, json_schema: TenderParseResult}
    fallback: [{provider: dashscope, model: qwen-max}]
    timeout_s: 180
  chapter_generate:
    primary:  {provider: deepseek, model: deepseek-chat, temperature: 0.7, stream: true, max_tokens: 6000}
    fallback: [{provider: dashscope, model: qwen-max}]
  chapter_self_check:
    primary:  {provider: dashscope, model: qwen-flash, temperature: 0.0, json_schema: SelfCheckResult}
  compliance_semantic:
    primary:  {provider: deepseek, model: deepseek-chat, temperature: 0.0, json_schema: SemanticIssueList}
  rewrite_assistant:
    primary:  {provider: dashscope, model: qwen-plus, stream: true}
  embedding:
    primary:  {provider: local_bge, model: bge-m3, dim: 1024, batch: 64}
  rerank:
    primary:  {provider: local_bge, model: bge-reranker-v2-m3}
  ocr:
    primary:  {provider: local_paddle, model: pp-ocrv4}
    fallback: [{provider: baidu_ocr, model: accurate}]
quotas:
  default_tenant_monthly_cny: 500
  on_exceed: downgrade_then_block            # 先降级到低成本路由，再硬限
.env.example
# === 主业务后端 (Go) ===
DATABASE_URL=postgres://zbt:CHANGE_ME@postgres:5432/zhibiaotong?sslmode=disable
REDIS_URL=redis://redis:6379/0
JWT_SECRET=CHANGE_ME_64_BYTES
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=CHANGE_ME
MINIO_SECRET_KEY=CHANGE_ME
MINIO_BUCKET=zbt-files
AI_SERVICE_URL=http://ai-service:8000
AI_SERVICE_HMAC_SECRET=CHANGE_ME            # Go↔Python 回调签名
# === AI 服务 (Python) ===
MODEL_ROUTING_FILE=./config/model_routing.yaml
DEEPSEEK_API_KEY=
DASHSCOPE_API_KEY=
BAIDU_OCR_API_KEY=
BAIDU_OCR_SECRET_KEY=
AI_LOG_DB_URL=${DATABASE_URL}
LIBREOFFICE_PATH=/usr/bin/soffice
USE_MOCK_PROVIDERS=false                    # CI 设 true
# === 前端 ===
VITE_API_BASE_URL=/api/v1
密钥管理纪律：API Key 只存在环境变量/密钥管理服务；model_routing.yaml 只引用 api_key_env；.env 进 .gitignore；ai_call_logs 不记录 prompt 原文中的密钥类字段；CI 中 USE_MOCK_PROVIDERS=true。

11. REST API 设计
统一前缀 /api/v1，JWT Bearer，tenant_id 从 token claims 解析（不从参数传）。[异步] = 返回 202 + task_id，前端经 SSE/轮询取结果。

Auth
  POST /auth/register | /auth/login | /auth/refresh | /auth/logout
  GET  /me                          → 用户 + 当前租户 + 权限矩阵(驱动路由守卫)
Tenant & Team
  GET/PATCH /tenant                 租户信息/设置
  GET/POST  /tenant/members         成员列表/邀请
  PATCH/DELETE /tenant/members/:id  改角色(支持批量 PATCH /members:batch)/移除
  GET/POST/PATCH /roles             角色与模块权限矩阵
Tender
  GET  /tenders?keyword&region&industry&budget&tab=all|recommend|monitored|favorite
  GET  /tenders/:id
  POST /tenders/:id/favorite | DELETE 同路径
  GET/POST/PATCH/DELETE /tender-sources        数据源管理
  POST /tender-sources/:id/verify              [异步] URL 可达性验证
  GET/PUT /tender-monitor-settings             关键词/推送/阈值
  POST /tenders/:id/parse                      [异步] 解析标讯附件
  POST /tenders/:id/create-project             从标讯建项目(同步)
Project
  GET  /projects?view&status&owner&priority    GET /projects/stats
  POST /projects | GET/PATCH/DELETE /projects/:id
  POST /projects/:id/transition {to_status}    看板拖拽/状态标签切换(同步,校验状态机)
  CRUD /projects/:id/milestones
  POST/DELETE /projects/:id/members
  GET  /projects/:id/activities                操作日志
  POST /projects/:id/cost-project              中标后创建成本项目(同步)
Bid
  POST /bids                                   新建标书(草稿,同步)
  GET  /bids?status&type&keyword               GET /bids/stats
  GET/PATCH/DELETE /bids/:id
  POST /bids/:id/parse-tender-file             [异步] Step2 招标文件解析
  GET/PUT /bids/:id/parse-result               解析结果查看/用户确认编辑
  POST /bids/:id/outline/generate              [异步] Step3 大纲生成
  GET/PUT /bids/:id/parts/:partId/outline      大纲树调整(拖拽/增删/改名,同步)
  GET/PUT /bids/:id/material-selection         Step4 素材勾选
  POST /bids/:id/generate                      [异步] Step5 启动逐章生成
  POST /generation-jobs/:jobId/pause|resume|cancel
  GET  /generation-jobs/:jobId                 进度
  GET  /bids/:id/generation/stream             SSE: 流式内容+进度
  POST /chapters/:id/accept                    采纳(同步)
  POST /chapters/:id/regenerate                [异步]
  GET  /chapters/:id/versions                  GET /chapters/:id/diff?from&to
  PUT  /chapters/:id/content                   编辑器保存(同步,自动保存节流)
  POST /chapters/:id/ai-action                 [异步,SSE] 内联改写:优化/扩写/缩写/加细节
  POST /bids/:id/exports {format,template,settings}   [异步] 导出
  GET  /bid-exports/:id                        状态+下载预签名URL
  GET  /bid-templates?category                 POST /bid-templates/:id/use
Knowledge
  CRUD /knowledge/categories /knowledge/tags
  POST /knowledge/documents                    上传登记(文件已直传MinIO)
  POST /knowledge/documents/:id/process        [异步] 解析+提取+打标+切片+向量化
  GET  /knowledge/documents?category&tag&type&q
  GET/PATCH/DELETE /knowledge/documents/:id    PATCH 含标签确认/补充
  GET  /knowledge/documents/:id/references     被引用记录
  POST /knowledge/search {query, filters}      语义搜索(同步,内部走RAG检索)
  GET  /knowledge/stats                        使用统计(V2.2)
Compliance
  POST /compliance/checks {bid_id?|file_ids, layers, scope}   [异步]
  GET  /compliance/checks | /compliance/checks/:id            结果+issues 按 tab 分组
  GET  /compliance/checks/:id/stream                          SSE 各层进度
  POST /compliance/issues/:id/autofix          [异步] AI一键修复
  POST /compliance/issues/:id/ignore
  POST /compliance/checks/:id/report {format}  [异步] 报告导出
  CRUD /compliance/rules                       自定义规则
Cost
  GET  /cost-projects | GET/PATCH /cost-projects/:id
  CRUD /cost-projects/:id/items
  GET  /cost-projects/:id/analysis             构成/对比/利润率(同步聚合)
  POST /cost-projects/:id/ai-advice            [异步] 成本优化建议
  POST /cost-projects/:id/report               [异步] 成本报告导出
Approval
  CRUD /approval-chains
  POST /bids/:id/submit-for-approval           创建审批实例(同步)
  GET  /approvals?status=pending|done          待我审批/我发起的
  POST /approvals/:id/approve|reject {comment}
Notification
  GET  /notifications?type&unread              POST /notifications/read:batch
  GET  /notifications/stream                   SSE 实时推送
Dashboard
  GET /dashboard/stats|recommendations|todos|trends
  POST /todos | PATCH /todos/:id
File
  POST /files/presign-upload {filename,size,mime,biz_type} → 预签名URL+file_id
  POST /files/:id/confirm                      上传完成确认(校验sha256)
  GET  /files/:id/download-url                 预签名下载(校验资源权限)
AI Task(统一任务查询)
  GET /ai-tasks/:taskId                        跨类型任务状态查询
同步/异步划分原则：所有涉及 LLM/OCR/文档转换/向量化的接口一律异步任务化（解析、大纲、逐章生成、自检、合规检查、改写、导出、AI 建议、文档入库处理）；纯 DB 读写一律同步。逐章生成与改写在异步任务之上叠加 SSE 流式通道。

12. 异步任务与状态机设计
任务架构
Go 侧 Asynq（Redis）：轻任务 + 编排者角色（资质到期扫描、案例回流、通知分发、调 AI 服务并跟踪）。
Python 侧 arq（Redis，独立队列）：重任务执行（解析、生成、合规、导出）。队列分级：q_interactive（改写/自检，秒级）、q_generate（逐章生成）、q_heavy(解析/导出/索引)，防止大任务饿死交互任务。
通用任务协议：每个任务记录 task_id, trace_id, tenant_id, type, status, progress, retries, error，任务状态以 Postgres 为准（Redis 只是队列），服务重启不丢状态。
失败治理：自动重试 ≤2 次（仅幂等任务）；超时看门狗（Go 定时扫描 running 超阈值任务标记 failed）；死信记录 + 工作台告警；所有任务幂等键 = (type, biz_id, input_hash)。
核心状态机
项目 projects.status:
  opportunity → bidding → compliance_review → submitted → closed(result: won|lost)
  · 看板拖拽 = transition API；非法迁移(如 opportunity→submitted)后端拒绝
  · → closed(won) 触发: 提示创建成本项目 + 案例回流任务
标书 bid_documents.status:
  draft → generating → editing → in_review → approved → submitted → archived
  · in_review 由"提交审批"触发，审批驳回 → editing(带驳回意见)
  · approved 后才允许"提交投标"；submitted 后内容只读
章节 bid_chapters.status:
  pending → generating → generated → (accepted | edited)；合规命中 → needs_fix → edited

生成任务 bid_generation_jobs.status:
  queued → running ⇄ paused → done | failed | cancelled
  · pause 在章节边界生效；resume 从下一未完成章节继续；失败章节可单独重试
合规检查 compliance_checks.status:
  queued → running(layer1→3→2→4 顺序,逐层推进度) → done | failed
compliance_issues.status: open → fixed(编辑器保存后重跑该 issue 规则联动) | ignored
审批 approval_instances.status:
  pending(step n) → approve→ 下一步或 approved；reject → rejected(终态)
  · 条件步骤(金额>100万)在创建实例时按快照求值决定是否纳入链
前端进度模型
统一 SSE 端点 + TanStack Query 轮询兜底（SSE 断线 5s 降级轮询）。生成进度事件：{job_id, chapter_id, event: chunk|chapter_done|self_check|job_done, payload}。

13. 权限矩阵与多租户隔离
多租户隔离（三层防御）
应用层：JWT claims 携带 tenant_id；中间件将其注入 request context；所有 repository 查询强制带 tenant 条件（sqlc 查询模板统一含 tenant_id = $1，code review/lint 检查无 tenant 条件的查询）。
数据库层（RLS 兜底）：业务表启用 ROW LEVEL SECURITY，策略 tenant_id = current_setting('app.tenant_id')::uuid；Go 在事务开头 SET LOCAL app.tenant_id。即使应用层漏写条件，RLS 兜住。这是防"多租户数据串号"——SaaS 最致命事故——的保险丝。
资源层：MinIO object key 强制 {tenant_id}/{biz_type}/{uuid} 前缀，下载一律走 Go 校验后发预签名 URL，桶不公开；pgvector 检索 SQL 自然受 RLS 约束；ai_call_logs 按 tenant 记账。
跨服务：Go 调 Python 时透传 tenant_id + trace_id，Python 回调带 HMAC 签名，Go 验签后以自身身份落库（Python 永远没有直接写业务表的权限，ai_call_logs 专表除外）。

RBAC 模型
用户 → tenant_member → role → role_permissions(操作级) + module_permissions(模块级 none/read/full)。预置 5 角色（V2.1）：超级管理员、部门管理员、项目经理、投标专员、查看者；支持租户自定义角色。

模块权限矩阵（合并 V2.1 §6.1 与 V2.3 §3.3，以 V2.1 五角色为准）
模块	超管	部门管理员	项目经理	投标专员	查看者
工作台
full
full
full
full
read
标讯大厅
full
full
full
read
none
标书生成
full
full
full
full
read(授权项目)
合规检查
full
full
full
full
read
项目管理
full
full
full
仅参与项目
read(授权项目)
成本管理
full
read
none
none
none
知识库
full
full
read
read
none
团队协作
full
read
none
none
none
数据级补充规则：投标专员的"仅自己参与的项目"通过 project_members join 实现（资源级过滤，叠加在模块级之上）；后端每个 handler 声明所需 permission code，中间件统一校验，前端隐藏菜单只是体验，后端校验才是边界。

14. 开发 Loop 计划（AI Agent 循环开发方案）
通用纪律：每个 Loop 以"检查命令全绿 + 人工验收清单"收口才能进入下一 Loop；Agent 每 Loop 开工前先读 docs/blueprint/（Loop 0 产物）防止偏航；所有 AI 接口在 Loop 1–5 期间用 MockProvider 联调。

Loop	目标	关键交付物	检查命令	验收标准
0 蓝图
把本方案固化为 Agent 可执行的事实源
docs/blueprint/：数据字典、API 契约(OpenAPI 草案)、状态机图、权限矩阵、Loop 任务卡
人工评审
你签字确认蓝图；后续 PRD 冲突以蓝图为准
1 骨架
三端工程可跑通
Monorepo(frontend/ backend/ ai-service/)、Docker Compose(pg+redis+minio)、CI(lint+test)、健康检查、goose 迁移框架、前端路由壳+侧边栏分组
docker compose up 后 curl /healthz×2；pnpm build；go test ./...；pytest
一条命令起全栈；14 个路由壳可点击；CI 绿
2 SaaS 底座
租户/认证/权限/审计/文件
注册登录、JWT 刷新、租户成员邀请、5 预置角色、RBAC 中间件、RLS 迁移、audit_log、MinIO 预签名上传下载、通知表+SSE 通道、前端路由守卫
集成测试：跨租户访问必 403/404；RLS 单测(漏 tenant 条件查询返回空)
两个测试租户数据完全隔离；权限矩阵生效(投标专员看不到成本菜单且 API 403)
3 业务主数据
标讯/项目/团队 CRUD
tenders+seed 数据、收藏/监控设置、projects 看板(拖拽 transition)+列表+详情+里程碑+成员、操作日志、工作台静态统计
API 集成测试；前端 e2e(Playwright)：标讯→创建项目→看板拖拽
状态机非法迁移被拒；看板/列表/详情/里程碑弹窗符合 V2.3 交互
4 标书主链路(Mock AI)
7 步向导端到端走通
bid/part/chapter/version 全模型、向导 7 步 UI、Mock 解析/大纲/逐章生成(SSE)、采纳/重新生成/版本 diff、Tiptap 编辑器三栏、自动保存、Mock 导出
e2e：新建标书→7步→编辑器→导出占位文件
用 Mock 数据完整走完 Step1–7；暂停/继续/采纳/重生成/diff 可用；刷新页面向导状态可恢复
5 知识库
文档管理+引用追踪(检索仍 Mock)
分类树/标签/文档 CRUD、上传处理任务管道(状态机)、文档预览、引用关系表、统计页、资质到期扫描任务
集成测试 + e2e 上传→列表→预览→打标
文档库/模板库/标签管理 3 页面符合 V2.4；到期提醒出现在工作台
6 AI 服务真实化
模型网关+解析+RAG+生成落地
网关(4 Provider+Router+fallback+ai_call_logs)、解析流水线(PDF/Word/OCR/表格)、切片+embedding+pgvector 索引、混合检索+rerank、招标解析、大纲、逐章生成(真实 SSE)、自检、改写助手；替换 Loop4/5 的 Mock
pytest 含 10 份真实招标文件回归集；网关 fallback 注入故障测试；评测脚本输出解析字段准确率
真实招标文件解析关键字段准确率 ≥85%(人工核对)；生成章节带 source_refs；断 primary provider 自动 fallback；ai_call_logs 成本可查
7 合规检查
四层校验+编辑器闭环
Go 规则引擎(L1/L3)+规则表、Python 语义层(L2/L4)、检查三步流程 UI、issues 五 Tab、跳转修复(编辑器锚点定位)、一键修复、报告导出、自定义规则
构造 20 个含已知问题的样本标书，断言查全/查准；e2e 跳转修复
规则层确定性问题 100% 检出；LLM 层 issue 均为 warn 级且带 evidence；跳转修复定位到章节
8 闭环打通
项目/成本/审批/回流/导出
中标→建成本项目、成本录入+分析+AI建议、审批链配置+实例流转、标书提交审批→通知→批准、中标案例回流知识库、真实 docx/PDF 导出(排版模板)、合并 zip 导出
e2e 全链路：标讯→项目→标书→合规→审批→提交→中标→成本→回流
导出 docx 在 Word 中打开格式正确、目录可刷新；审批驳回回到编辑态；回流文档可被下次生成检索到
9 验收加固
性能/安全/体验收口
渗透自查(越权扫描脚本)、大文件(200页PDF)压测、SSE 断线恢复、空态/错误态/加载态补全、配额与成本告警、种子演示数据、部署文档
全量 e2e 套件；k6 基础压测；越权扫描 0 命中
第 15 节验收标准全过
15. 验收标准
功能验收（黄金链路）：用一份真实的政府采购招标文件（含扫描页和评分表），在演示租户完成：标讯入库 → 解析确认 → 创建项目 → 7 步生成（≥8 章，每章有引用来源与自检结果）→ 编辑器修改 → 合规检查（四层，发现并跳转修复至少 1 个问题）→ 提交审批（两级链，含驳回一次）→ 导出 docx+PDF → 标记中标 → 成本录入与利润分析 → 案例回流知识库并在新标书 Step4 被推荐。全程无需开发者介入。

质量门槛：

解析：电子版招标文件关键字段（项目名/预算/截止日/资质/废标条款）准确率 ≥85%，扫描件 ≥70%（带置信度提示）；
生成：每个事实性段落有 source_ref，无引用段落被自检标记率 100%；
合规：规则层零漏报（针对规则覆盖项）；LLM 层 issue 100% 附原文 evidence；
导出：docx 在 Microsoft Word 打开无格式错乱，中文字体/页眉页脚/目录正确；
安全：越权测试套件（跨租户 ID 枚举、无权限模块 API 直调、文件 URL 猜测）0 通过；
性能：列表页 P95 < 500ms；200 页 PDF 解析 < 5 分钟；单章生成首 token < 5s；
工程：CI 全绿；测试覆盖核心状态机与 RBAC；docker compose up 一键起全栈；无任何密钥入库。
16. 风险清单与解决策略（Top 20）
#	风险	等级	解决策略
1
招标文件解析准确率不足，下游全错
🔴
解析结果强制人工确认门控（Step2 已有此设计，坚持）；字段带置信度+原文定位；建立 50+ 真实文件回归评测集，每改 prompt 必跑
2
扫描件 OCR 质量差
🔴
文本层覆盖率检测自动分流；PaddleOCR+云 OCR 双引擎；低置信页 UI 黄标提示人工核对；一期明确告知扫描件为 beta
3
复杂表格（评分表/工程量清单）解析失败
🔴
规则抽取→失败页截图走多模态模型→仍失败降级为"表格图片+人工录入"，不硬解
4
Word 导出格式不保真导致废标
🔴
母版模板填充制（不代码拼样式）；LibreOffice 容器预装中文字体；导出后自动校验清单（页码/目录/字体）；提供导出预览
5
RAG 幻觉（编造业绩/资质）
🔴
强制引用标记语法+无引用段落自检标红；温度分任务管控；资质/业绩类内容只允许从知识库引用生成，prompt 明令禁止补全
6
生成内容无引用、不可溯源
🟠
source_refs 跟版本存储；knowledge_references 反向索引；验收硬指标（见 15 节）
7
合规检查误报多→用户不信任
🔴
LLM 判定项上限 warn 级；规则项才可 fail；issue 必附 evidence；提供 ignore+反馈闭环持续调规则
8
合规检查漏报→废标→客户流失
🔴
产品话术定位"辅助检查"非"担保"；废标条款层采用"规则定位+LLM 判断+强制人工逐条确认"三段式；免责声明
9
多租户数据泄漏
🔴
三层防御（应用层强制条件+RLS+对象存储前缀）；越权扫描进 CI；上线前渗透自查
10
LLM API 成本失控
🟠
网关统一记账+租户配额+任务级模型路由强制+prompt 缓存+解析结果按 sha256 缓存；成本日报
11
大文件处理（500页PDF）超时/OOM
🟠
流式分页处理；文件大小/页数上限（一期 100MB/500页）；重任务独立队列+worker 内存限制+失败分页重试
12
异步任务静默失败，用户卡死等待
🟠
任务状态落 PG；超时看门狗；失败必通知+可重试按钮；死信告警
13
前后端联调复杂（异步+SSE+状态机）
🟠
Loop 0 先定 OpenAPI 契约；前端先对 Mock 开发（Loop4/5）；SSE 统一封装+轮询兜底
14
AI Agent 写代码失控（架构漂移/耦合）
🟠
蓝图文档为唯一事实源；feature 边界 lint 规则；每 Loop 验收门控；禁止 Agent 跨 Loop 自由发挥；小步提交+CI 强制
15
PRD 与原型不一致导致实现歧义
🟠
已确认多处不一致（见 17 节）；以蓝图仲裁，蓝图以 V2.4>V2.3>…优先级合并并经你签字
16
一期功能过大延期
🔴
第 17 节明确推迟清单；Loop 制+每 Loop 可演示；砍功能不砍链路完整性
17
标讯数据源无真实数据（爬虫合规+反爬）
🟠
一期不做自动抓取（V2.1 已降级），用种子数据+手动录入+RSS/公开 API；爬虫的法律与稳定性风险后置专项评估
18
Tiptap 编辑器与 Word 心智差异引发客诉
🟡
明确产品话术"内容在线编辑+模板化导出"；提供导出预览；不承诺在线分页所见即所得
19
海外模型 API 数据出境合规
🟠
默认走国内模型（方案A/C）；B 方案仅在客户书面同意下启用；私有化选项兜底
20
审批/权限等"企业功能"被低估，后期返工
🟡
审批实例存链快照；权限从 Loop2 就位而非后补；模块权限矩阵数据驱动可配置
17. 你不同意或需要修正我当前设想的地方
不建议采用 / 需要修改的技术选择：

Qdrant 一期不要上。 pgvector 在你的数据规模下性能足够，且租户隔离、metadata filter、混合检索全部复用 Postgres 能力，少一个有状态组件。保留 VectorStore 接口即可。
Milvus（V2.0 PRD 写的）同理否决，PRD 技术方案章节整体已过时（它还写着 FastAPI 做主后端 + Celery + Drools），应以本方案取代，避免 Agent 开发时误读 PRD 第六章。
Drools 否决——Java 生态规则引擎，与你的栈完全不搭，自研规则表+检查器接口足够。
GORM 不作为主 ORM。建议 pgx + sqlc：编译期 SQL 校验对 AI Agent 循环开发的纠错效率远高于运行时 ORM 报错，且 RLS 的 SET LOCAL 与事务控制在 pgx 下更直白。
LangChain 不作为架构依赖。 解析/检索/生成流水线自己写（代码量不大、可控可测），最多用 LiteLLM 做 Provider 协议适配底层。
Rust：同意你的判断，一期完全不碰；后期如要做高性能文档转换 sidecar 再评估。
功能边界需要调整（必须推迟）：

标讯自动抓取/自定义数据源爬虫（含 Cookie 登录）→ 一期只做"登记+可达性检测"+种子数据，PRD V2.1 自己也是这么降级的，坚持住；
成本知识库（V2.1 §5.2 标注 MVP 后）、行业公共模板库的"10万+方案"内容运营、电子标书专用格式、甘特图视图、协同编辑（Yjs）、邮件以外的推送渠道 → 全部二期；
V2.2"移除 MVP 限制提示"只是 UI 文案要求，不等于这些功能必须一期实现——分离标书的数据模型一期就位（bid_parts 已设计），但"自定义组合+附件关联章节"的完整交互可放 Loop 8 之后。
可能过度复杂的设计（PRD 侧）：

待办拖拽排序、看板拖拽确认弹窗、收起侧边栏悬浮子菜单等微交互不影响链路，Agent 开发时降为 P2；
合规第 4 层（评分优化）默认关闭是对的，一期甚至可以只做到"可运行但标注 beta"。
必须先做（不可妥协的顺序）：

多租户 + RLS + RBAC（Loop 2）必须先于一切业务模块；
OpenAPI 契约和状态机定义（Loop 0）必须先于前后端并行开发；
模型网关与 ai_call_logs 必须先于任何真实模型调用接入。
PRD 与 HTML 原型已确认的不一致（需要你裁决或按我建议处理）：

HTML 只有 11 个 page div，缺 V2.4 的 3 个页面：page-generate-new 不存在（"新建标书"子菜单直接指向 7 步向导 page-generate），page-knowledge-docs/templates/tags 不存在（知识库 3 个子菜单全部 navigateTo('knowledge')）。→ 建议按 V2.4 PRD 实现 14 页，原型仅作视觉参考（与你的事实源优先级一致，但要明确告知 Agent 不要照抄原型导航行为）。
项目状态命名三个版本不一致：V2.0 看板"商机评估→标书制作→合规审核→投标中→已结果"；V2.1 拖拽描述少了合规审核列；V2.3 状态循环以"进行中"开头。→ 建议统一为 V2.0 五态（opportunity/bidding/compliance_review/submitted/closed），V2.3 的"循环切换"只是交互糖。
角色体系不一致：V2.1 定义 5 角色，V2.3 权限矩阵只有 3 角色（管理员/项目经理/普通成员）。→ 建议数据模型按 5 角色 + 可自定义实现，V2.3 矩阵视为简化展示。
两个"模板库"命名冲突（标书模板库 vs 知识库文档模板库），V2.4 自己也意识到了但 UI 上仍同名。→ 建议导航文案改为"标书模板"与"文档模板"，路由已分开（/bids/templates vs /knowledge/templates）。
合规检查入口矛盾：V2.1 重构后要求"招标文件+投标文件双上传必填"，但从标书模块跳转时自动带入——原型中 startComplianceCheck 并未体现项目关联自动匹配。→ 按 V2.1 PRD 实现，原型忽略。
成本管理统计卡：V2.1 明确否定了 V2.0 的四统计卡（总成本/总收入等），原型仍保留。→ 按 V2.1/V2.2 的"项目成本列表+单项目分析"实现。
一个需要你尽早决策的非技术问题：如果一期开发团队（含你）≤2 人且交付压力大，可以把 Go 主后端降级为"全 Python 单体（FastAPI + 同一进程内模块化）"，省掉双语言运维与跨服务契约成本——架构上第 6、7 节的模块划分照搬即可，后期再把 SaaS 底座剥到 Go。我的默认推荐仍是 Go+Python 双服务（团队 ≥3 人或预期 12 个月内商用），但这个决策应该基于人力而非技术偏好。

方案到此完整。建议你的下一步是：① 对第 17 节的 6 处 PRD 不一致逐条裁决；② 确认团队规模以锁定 Go+Python 或全 Python 路线；③ 批准后我再启动 Loop 0，把本方案展开为 docs/blueprint/ 下的数据字典、OpenAPI 契约与状态机文档。
# 外部 MCP / Skills 雷达

更新时间：2026-06-18

本文件记录招投标、采购、RFP 方向可参考或可接入的外部 MCP / Skills。它们只作为数据源、工具边界和方法论参考；未经明确评估，不复制代码，不把客户招标文件或投标文件默认发送给第三方服务。

## 分层原则

1. MCP 负责确定性外部数据和工具调用：商机搜索、企业画像、资质、政策、公开采购数据、价格和历史中标情报。
2. Skill 负责流程编排和交付物方法论：RFP 审查清单、响应矩阵、合规检查、评审打分、文档生成流程。
3. 智标通核心能力仍由本工程实现：租户隔离、文件解析、响应矩阵、合规审查、投标文件生成、审计和成本核算不能依赖外部黑盒完成。
4. 外部调用必须默认只读、租户级显式开启，并记录调用人、工具名、输入摘要、脱敏策略、耗时、状态、token 或计费估算。
5. 涉及客户招标文件、投标文件、报价、资质证明和合同条款时，必须先经过脱敏策略和管理员授权；禁止默认外发原文。

## 可优先评估接入

| 项目 | 类型 | 价值 | 接入判断 |
| --- | --- | --- | --- |
| [handaas/bidding-mcp-server](https://github.com/handaas/bidding-mcp-server) | 中国招投标数据 MCP | 招投标信息搜索、中标/招标/采购统计、拟建项目查询，支持 streamable-http、stdio、sse | 可作为 P0 商机发现和竞品/业绩分析数据源；需要评估账号、价格、限流和数据授权 |
| [handaas/mcp-server](https://github.com/handaas/mcp-server) | 中国企业大数据 MCP 集合 | 企业工商、风险、资质、经营洞察、招投标、专利、商标等 | 可作为企业画像、供应商/竞争对手尽调和资质辅助校验数据源；应按工具白名单拆分接入 |
| [AutoRFP.ai MCP](https://autorfp.ai/blog/autorfp-mcp-server-launch) | RFP / DDQ 响应库 MCP | 只读查询项目、需求、内容库、标签和复用分析，强调来源引用与 Trust Score | 可借鉴响应库治理、来源引用和答案复用分析；不把客户文件默认同步到第三方平台 |
| [getqlows/qlows-mcp](https://github.com/getqlows/qlows-mcp) | RFP deal / 公共 tender MCP | 读取 RFP/bid deal、合规项、问题路由、竞争对手分析和公开 tender corpus | 可作为外部 deal 快照、公共标讯和问题路由参考；需验证工具名、授权和数据出境边界 |
| [dutchcode/rfp-ai-mcp](https://github.com/dutchcode/rfp-ai-mcp) | 已批准知识库问答 MCP | 从企业已批准知识库回答 RFP、DDQ 和安全问卷，并提供引用 | 可借鉴“只从批准知识库回答”的治理边界；不替代本系统知识库/RAG 主链路 |
| [zhiqianzheng/BidMonitor-AI](https://github.com/zhiqianzheng/BidMonitor-AI) | 招标监控系统 | 中国招标网、政府采购网监控，关键词和排除词过滤，多渠道通知 | 不作为 MCP 直接接入；可借鉴监控源配置、关键词过滤和通知策略 |

## 可借鉴工具边界

| 项目 | 类型 | 可借鉴点 | 不直接采用原因 |
| --- | --- | --- | --- |
| [crawde/mcp-bidcraft](https://github.com/crawde/mcp-bidcraft) | RFP 分析 / 生成 MCP | `analyze_rfp`、`generate_proposal`、`check_compliance` 三类工具边界清晰，适合映射到解析、生成、覆盖检查 | 偏英文 RFP，SaaS 黑盒能力和免费额度限制，不适合作为核心解析与生成依赖 |
| [crawde/mcp-bidcraft-compliance-matrix](https://github.com/crawde/mcp-bidcraft-compliance-matrix) | RFP 合规矩阵 MCP | 要求抽取、章节映射、差距分析、响应提纲的工具拆分与命名边界 | 可作为智标通响应矩阵和差距分析的对照工具；仅允许脱敏摘要或要求项输入 |
| [crawde/mcp-bidcraft-win-strategy](https://github.com/crawde/mcp-bidcraft-win-strategy) | Bid/No-Bid 策略 MCP | 中标概率评分、price-to-win、capture planning 和 bid/no-bid 决策 | 可借鉴商机评分和决策 checklist；不得接触未脱敏报价明细 |
| [fredericboyer/loopio-mcp](https://github.com/fredericboyer/loopio-mcp) | Loopio Data API MCP | 通过企业 RFP 应答库 API 查询历史答案、项目和内容条目 | 可借鉴企业应答库集成和权限继承；不把外部答案库当作本系统事实源 |
| [dbugom/tenderai-mcp-server](https://github.com/dbugom/tenderai-mcp-server) | Tender/RFP 管理 MCP | RFP 解析、合规矩阵、技术章节、完整技术方案 DOCX、BOM 表和伙伴协同 | 文件外发和模型调用链需审计；可借鉴功能拆分，不直接替换本工程 AI 服务 |
| [Acquarts/A2A-MCP-Multiagent-Smart-RFP](https://github.com/Acquarts/A2A-MCP-Multiagent-Smart-RFP) | 多 agent RFP 示例 | 客户研究、RFP 分析、历史案例检索、成本估算、提案生成的 agent 分工 | 示例项目，不具备中国招投标文件和 SaaS 多租户生产边界 |
| [sufyman/auto-rfp](https://github.com/sufyman/auto-rfp) | Hackathon RFP pipeline | 发现 RFP、PDF 抽取、schema 化、知识图谱检索、草拟、自评、发布 microsite 的端到端思路 | Hackathon 项目，适合补充流程视角，不作为生产依赖 |

## 可借鉴 Skill 方法论

| 项目 | 类型 | 可借鉴点 |
| --- | --- | --- |
| [Openclaw-Metis/rfp-architect](https://github.com/Openclaw-Metis/rfp-architect) | RFP 编写 / 审查 Skill | 双模式 write/review、11 必备章节、台湾采购语境、rubric、linter、source register、release gate |
| [1102tools/federal-contracting-skills](https://github.com/1102tools/federal-contracting-skills) | 联邦采购 Skills | “MCPs handle data, Skills handle deliverables”的拆分；SOW/PWS、IGCE 和成本估算的可审计交付物流程 |
| [1102tools/federal-contracting-mcps](https://github.com/1102tools/federal-contracting-mcps) | 采购数据 MCP 组合 | 多个确定性 API MCP 分包、MCPB 打包、每个 server 有测试记录和生产 API 验证 |
| [sarkisova-creator/tender-research-claude-skill](https://github.com/sarkisova-creator/tender-research-claude-skill) | 标案研究 Skill | 门户抓取、公司画像匹配、机会评分、下载高分文件、输出 Excel 的研究流程 |
| [Maxbase91/procurement-skills](https://github.com/Maxbase91/procurement-skills) | 采购 Skills 包 | RFP 生成与评估、供应商校验、合同 redline、Spend 分析和配置化评分权重 |
| [Proposal-Builder/proposal-biz-claude-skill](https://github.com/Proposal-Builder/proposal-biz-claude-skill) | Proposal Skill + MCP | 通过 Skill 询问发现问题、生成结构化 Markdown，再提交到外部 MCP 的交付流 |

## 全球采购数据 MCP 观察池

这些项目对中国招投标文件解析帮助有限，但适合观察公开采购数据模型、OCDS、工具描述和远程 MCP 发布方式。

| 项目 | 覆盖范围 | 可借鉴点 |
| --- | --- | --- |
| [blencorp/capture-mcp-server](https://github.com/blencorp/capture-mcp-server) | 美国 SAM.gov、USASpending.gov、Tango | 多 API 组合、结构化 JSON 输出、工具启用矩阵和限流 |
| [carlosahumada89/govrider-mcp-server](https://github.com/carlosahumada89/govrider-mcp-server) | 多国政府采购机会 | 按公司能力匹配机会、API key 配置、npx 安装体验 |
| [chrisns/uk-tenders-mcp](https://github.com/chrisns/uk-tenders-mcp) | 英国采购 OCDS 数据 | 官方 URL、数据新鲜度、只读 BigQuery、跨源去重和 schema 查询 |
| [h30190/SearchProcurementTenders](https://github.com/h30190/SearchProcurementTenders) | 台湾标案 | 中文标案检索、公告过滤、案号去重、字段精简 |
| [atlasprzetargow/mcp-server](https://github.com/atlasprzetargow/mcp-server) | 波兰公共采购 | 买方/承包商画像、CPV 统计、术语表和 guided workflows |
| [switchr24/mcp-india-tenders](https://github.com/switchr24/mcp-india-tenders) | 印度政府采购 | OCDS 格式、跨门户 tender 搜索 |
| [Digilac/simap-mcp](https://github.com/Digilac/simap-mcp) | 瑞士 simap.ch | 官方公开 API 转 MCP |
| [qune-tech/vergabe-mcp](https://github.com/qune-tech/vergabe-mcp) | 德国采购 | 语义搜索、tender matching、二进制和 npx 发布 |
| [VladyslavMykhailyshyn/prozorro-mcp-server](https://github.com/VladyslavMykhailyshyn/prozorro-mcp-server) | 乌克兰 Prozorro | 公共采购数据库搜索工具 |

## 智标通接入清单

### P0：只读外部 MCP 工具网关

当前后端基础已落地：

1. `external_tool_configs`：租户级外部工具配置，当前只允许 `streamable_http`。
2. `external_tool_audit_logs`：记录工具名、请求哈希、请求摘要、响应摘要、耗时、状态、费用估算和业务资源引用。
3. `GET /external-tools/catalog`：返回 Handaas、AutoRFP、qlows、BidCraft 和 Loopio 等只读 Provider 预设、默认工具白名单、token env、用途和数据边界。
4. `GET /external-tools`、`PUT /external-tools/:providerKey`、`POST /external-tools/:providerKey/invoke`、`GET /external-tools/audit`：通过 team 权限访问。
5. 调用入口使用 JSON-RPC `tools/call`，强制 enabled、allowed_tools、timeout_ms、monthly_budget 和摘要审计。
6. 已知 Provider 会应用预设名称和默认工具白名单；严格 Provider 不允许配置目录外工具，且不允许关闭脱敏策略。
7. `/team?tab=external-tools`：展示 Provider 目录、数据边界、启用状态、预算配置和最近调用记录，支持管理员配置访问地址、启用工具、脱敏策略和费用估算。

继续增强清单：

1. Provider 生产验证：用真实 Handaas、AutoRFP、qlows 或 Loopio 凭证跑只读 smoke test，并记录工具名与响应结构。
2. 业务入口：把已授权的标讯搜索、企业画像、外部应答库检索结果归一为 `external_mcp` 来源。
3. 禁止首批工具接收完整招标文件、投标文件、报价明细和合同正文。

### P1：业务入口

1. 项目 / 标书创建页：根据行业、地区、关键词检索潜在标讯。
2. 客户 / 竞争对手页：查询企业中标趋势、采购统计、资质和风险。
3. 知识库：把公开标讯摘要以外部来源形式入库，来源类型标记为 `external_mcp`。
4. 审计页：按租户展示外部 MCP 调用记录、失败原因和费用估算。

### P2：Skill 方法论内化

1. 把 `rfp-architect` 的章节完整性 linter 思路转成智标通自己的招标文件/投标文件检查规则。
2. 把 `1102tools` 的 “MCP 取数据，Skill 产交付物” 写入 `AI_PIPELINE.md` 和后续开发约束。
3. 把 tender research 的机会评分转为智标通商机评分：匹配度、截止日期、预算规模、地区、资质门槛、历史竞争强度。
4. 把 BidCraft/TenderAI 的 `analyze/generate/check` 工具边界映射为现有解析、生成、合规接口，不引入新的黑盒主链路。

## 不做事项

1. 不把外部 RFP MCP 作为生产解析和生成主路径。
2. 不在没有租户授权、脱敏策略和审计的情况下外发文件原文。
3. 不把海外公共采购 MCP 当作中国招投标数据源。
4. 不把第三方 Skill 文本直接复制为产品规则；只能转成智标通自己的测试、linter 和 checklist。
5. 不允许外部 MCP 调用绕过现有 RBAC、RLS、`ai_call_logs` 和成本审计。

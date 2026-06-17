# AI 能力实现清单

本清单把三个外部项目的方法论转成智标通自己的实现任务。外部仓库只作为方案参考，不复制代码，避免许可证和架构污染。

参考来源与可借鉴点：

- `intsig-textin/xparse-sample-projects/tender-doc-parse`：TextIn `pdf_to_markdown` 解析、按 `#`/`##` 标题切块、关键词路由到 6 模块、模块 prompt 固定 schema、6 模块并发抽取、复杂表格保留 `md_table`。
- `guangshu100/BidMaster-Pro`：`SkillContext`/`SkillResult` 能力单元、`interpret -> outline -> content -> check -> format -> final_check -> export` 流水线、Gate Keeper 人工闸门、强制性要求抽取/检查、MinerU OCR 配置、docx 格式检查与 PDF 书签导出。
- `run-llama/auto_rfp`：`Question -> Answer -> Source` 独立模型、`referenceId`、问题抽取与摘要/资格并行处理、答案来源保存到 `sources`、来源详情展示文件名/页码/片段/相关度。

## 当前差距

当前 ZBT 已有 Go 主后端、Python AI 服务、ModelRouter、OCR HTTP 接入点、RAG 检索、source_refs 落库、Word/PDF/ZIP 导出和 AI 调用成本审计。仍需要继续增强的核心差距：

1. 招标解析已新增 6 模块结构化结果、字段级来源、置信度、要求项矩阵和模块级独立模型增强；6 个模块已支持受控并发执行、固定顺序合并和单模块失败隔离；`docs/ex/工程1` 已建立可执行 golden 回归评测，仍需要继续做前端字段级编辑确认。
2. OCR 已有 Provider 契约、外部 HTTP 接口、成功响应归一化、页级质量指标和统一 `table_blocks`；仍缺少真实 OCR Provider 配置和样本回归评测。
3. 标书生成已有章节 source_refs，但缺少 AutoRFP 式“问题矩阵/响应矩阵”，即从招标文件抽取逐条要求，再逐条匹配回答和引用来源。
4. Skill/Gate 已从隐式状态机收敛为显式阶段闸门：`interpret`、`plan`、`generate`、`check`、`format` 阶段已落库并接入关键写操作。

## 当前落地进展

- `ai-service/app/schemas/tender.py` 已新增 `TenderParseModuleResult`、`TenderParseFieldEvidence`、`TenderRequirementItem` 和 `TenderParseStructuredResult`。
- `ai-service/app/pipelines/parse/tender_parser.py` 已在兼容旧字段的同时输出 `modules`、`field_evidence`、`requirement_items` 和 `quality_gates`。
- `ai-service/app/main.py` 已按 6 个模块逐一调用 `tender_parse` 路由，每个模块独立走 provider 候选和 fallback；单模块失败时保留基础解析并标记待确认。
- `ai-service/app/gateway/contracts.py` 已明确 OCR Provider 的 `recognize_document`、`recognize_page`、`extract_layout`、`extract_tables` 契约。
- HTTP OCR 成功响应已统一归一为 `pages`、`blocks`、`tables`、`table_blocks`、`confidence`、`provider_metadata`。
- `ai-service/app/pipelines/parse/document_parser.py` 已为 PDF 输出 `page_quality`，并把 PDF、docx、xlsx、pptx 表格统一归一为 `table_blocks`；OCR 接入已显式支持 `OCR_PROVIDER=http_ocr|mineru|paddleocr`，Provider 专属 endpoint/token/mode 会写入安全 metadata。MinerU/PaddleOCR 的嵌套响应会归一为 `markdown`、`pages`、`blocks`、`layout_blocks`、`table_blocks`，其中表格和版面块会提升到文档顶层 metadata 参与后续解析。
- `frontend/src/features/bid/index.tsx` 的文件解读步骤已增加“信息分组”和“响应要点”视图，不展示模型、token、schema 等技术口径。
- `backend/internal/db/migrations/00031_bid_requirement_items.sql` 已新增 `bid_requirement_items` 独立表，按租户启用 RLS，承接 AutoRFP 式 referenceId/source attribution 思路。
- `backend/internal/platform/bid/store.go` 已在解析回调和人工确认两条路径同步 `requirement_items`，并提供 `ListRequirementItems`。
- `TenderParseFieldEvidence.source_ref` 已统一携带 `citation_id`、`reference_id`、`source_kind`、`file_id`、`filename`、`chunk_id`、`traceable`，模型增强结果缺少可追溯定位时必须进入人工复核。
- `backend/internal/api/routes.go` 已提供 `GET /bids/:id/requirements` 只读接口。
- `frontend/src/features/bid/index.tsx` 的“响应要点”已优先读取独立要求表，并支持“全部/必须/待确认/已覆盖”筛选。
- `ai-service/app/evaluation/tender_parse_eval.py` 已提供离线解析评测 CLI，`docs/sample_docs/golden/工程1.parse.json` 已覆盖采购 PDF、响应 docx、盖章投标 PDF 和固化清单 xlsx 的 63 项检查。
- `backend/internal/db/migrations/00032_bid_pipeline_gates.sql` 已新增 `bid_pipeline_gates` RLS 表。
- `backend/internal/platform/bid/store.go` 已在上传、解析、解析回调、人工确认、大纲生成、整标生成和导出路径维护阶段闸门；`GenerateOutline` 会检查 `interpret=passed`，`GenerateBid` 会检查 `plan=passed`，`CreateExport` 会检查 `generate=passed` 和 `check=passed`。旧的已确认解析、大纲、已完成章节内容和已完成合规检查会按真实业务状态自动补齐闸门。
- `backend/internal/platform/compliance/store.go` 已在创建检查、问题修复、忽略和人工确认 fail 后同步 `check` 阶段闸门；`pass` 自动通过，`warn/fail_candidate` 进入待复核，`fail` 阻断。
- `backend/internal/api/routes.go` 已提供 `GET /bids/:id/pipeline-gates` 只读接口，前端 API client 已提供对应 DTO 和查询函数。

## P0：6 模块招标解析

目标：把招标文件解析从“单次整体抽取”升级为 6 个独立模块并发抽取，输出可人工确认、可回归评测的结构化结果。

模块定义：

| 模块 | 输出字段 | 主要用途 |
| --- | --- | --- |
| basic | 项目名称、采购人/招标人、预算、地点、公告/文件编号、投标截止日、开标时间 | 创建项目、标书基本信息 |
| qualification | 资质、业绩、人员、证书、联合体、保证金、信用要求 | 资格材料推荐、废标风险检查 |
| evaluation | 评分办法、分值、评分细则、技术/商务/价格权重 | 大纲生成、评分点覆盖检查 |
| submission | 文件组成、格式、签章、份数、密封、递交方式、电子标要求 | 导出模板、合规检查 |
| invalid_risk | 无效标、否决投标、废标、重大偏差、实质性响应条款 | 合规 fail_candidate 和人工确认 |
| annex | 附件、响应文件格式、投标函、报价表、承诺函、清单文件 | 素材清单、ZIP 分册、模板锚点 |

实现任务：

1. 文档切块和模块路由：
   - 保留当前 Python 后端统一编排，不把切块放回前端。
   - 按标题层级、页码、表格块、关键词构造 `TenderChunk`：`chunk_id`、`title_path`、`content`、`page_start`、`page_end`、`table_block_ids`、`quality_flags`。
   - 借鉴 xparse 的“标题优先，正文前 200 字辅助”的路由规则，但增加多模块命中：评分表可同时进入 `evaluation` 和 `submission`，废标条款可同时进入 `invalid_risk` 和 `qualification`。
   - 模块上下文只发送命中 chunk，并扩展相邻页/相邻 chunk，避免全量截断。

2. Python 新增模块契约：`ai-service/app/schemas/tender.py`
   - 新增 `TenderParseModuleResult`、`TenderParseFieldEvidence`、`TenderRequirementItem`、`TenderParseStructuredResult`。
   - 每个字段必须支持 `value`、`confidence`、`source_text`、`page_start`、`page_end`、`bbox`、`chunk_id`、`needs_review`。
   - 顶层保留兼容字段：`project_name`、`bid_type`、`deadline`、`qualification_requirements`、`invalid_clause_risks`、`scoring_points`、`outline`、`material_suggestions`。

3. Python 新增解析编排：`ai-service/app/pipelines/parse/tender_parser.py`
   - 将当前 `build_tender_parse_prompt()` 拆成 6 个 module prompt builder。
   - 根据标题、关键词、页码和表格 metadata 生成模块上下文，不再把全文截断后交给单 prompt。
   - 每个模块独立调用 `run_provider_task()` 或等价 ModelRouter 路由，允许模块级 fallback。
   - 当前已落地：`process_tender_parse` 使用 `TENDER_PARSE_MODULE_CONCURRENCY` 控制 6 模块并发，主线程按 `MODULE_ORDER` 固定顺序合并结果，单模块失败只标记该模块待复核。
   - 聚合时按字段级置信度和来源证据合并，不允许模型覆盖高置信确定性字段为空值。
   - prompt 固定四条硬约束：只引用原文、JSON only、字段缺失显式 `missing_fields`、来源原子性（一个 value 对应一处连续原文）。
   - 评分表、格式表、工程量清单默认保留原始 `md_table`，不得让模型重排表格。

4. Go 落库增强：`backend/internal/platform/bid/store.go`
   - `tender_parse_results.structured_result` 保存完整模块结果。
   - 增加兼容读取层：旧前端字段继续可用，新 UI 读取 `modules` 和 `requirements`。
   - `parse_metadata` 记录 provider、model、fallback_from、module_count、low_confidence_count、ocr_required_pages。

5. 前端确认 UI：`frontend/src/features/bid/index.tsx`
   - Step 2 解析确认页按 6 个模块分组。
   - 低置信字段、OCR 页、无来源字段必须有明确提示。
   - 用户确认后，把“确认版本”作为后续大纲、生成、合规的事实源。

6. 回归评测：
   - 使用 `docs/ex/工程1` 的 PDF、docx、xlsx 样本建立 golden JSON。
   - 新增脚本输出字段命中率：项目名、预算、截止日、资格要求、评分项、无效标条款、附件清单。
   - 当前已落地：`docs/sample_docs/golden/工程1.parse.json` 和 `python -m app.evaluation.tender_parse_eval --golden ...`，当前样本 63/63 通过。
   - P0 验收门槛：电子文本关键字段准确率不低于 85%；扫描/OCR 字段必须带置信度和人工复核标记。

## P0：OCR Provider 和版面证据

目标：把 OCR 从“外部 HTTP 文本补救”升级为可替换、可评测、可追溯的 OCR/Doc Intelligence 层。

实现任务：

1. Provider 契约：`ai-service/app/gateway/contracts.py`
   - 明确 `recognize_document()`、`recognize_page()`、`extract_layout()`、`extract_tables()`。
   - 输出统一为 `blocks`、`tables`、`pages`、`confidence`、`bbox`、`provider_metadata`。

2. OCR Provider 实现：
   - 保留 `OCR_HTTP_ENDPOINT` 通用 Provider。
   - 增加 MinerU/OpenAPI 兼容 Provider 配置：endpoint、token、timeout、poll_interval、max_attempts。
   - 当前配置入口：通用 HTTP 使用 `OCR_HTTP_ENDPOINT` / `OCR_API_KEY`；MinerU 使用 `OCR_PROVIDER=mineru`、`MINERU_HTTP_ENDPOINT`、`MINERU_API_KEY`、`MINERU_PARSE_MODE`；PaddleOCR 使用 `OCR_PROVIDER=paddleocr`、`PADDLEOCR_HTTP_ENDPOINT`、`PADDLEOCR_API_KEY`、`PADDLEOCR_PIPELINE`。
   - Provider 不可用时不伪装成功，必须返回 `provider_not_configured` 或 `failed`。
   - OCR 任务必须支持异步轮询；轮询结果归一到 `pages`、`blocks`、`tables`、`markdown`、`layout_blocks`。
   - OCR Provider 只作为可替换插件，不进入主业务 schema 的厂商字段。

3. 页级质量分流：`ai-service/app/pipelines/parse/document_parser.py`
   - 计算每页文本层覆盖率、字符密度、表格候选数、图片占比。
   - 低覆盖页自动进入 OCR；混合 PDF 只 OCR 低覆盖页。
   - OCR 结果与文本层结果按页合并，保留来源标记：`text_layer`、`ocr`、`table`、`layout_block`。

4. 表格策略：
   - PDF 表格、xlsx 工作表、docx 表格统一转成 `table_blocks`。
   - 评分表和工程量清单不拆成普通段落；作为完整表格块进入模块抽取和 RAG。
   - 表格块保留表头、页码、行列坐标、截断标记。

5. 验收：
   - `docs/ex/工程1/采购文件桥梁检查.pdf` 能输出 page/block/table metadata。
   - 空文本或扫描页不会被标记为普通解析成功。
   - OCR 超时、响应过大、无效 JSON、低置信页均有测试覆盖。

## P0：问题矩阵与来源引用

目标：借鉴 AutoRFP 的 question extraction/source attribution，把“招标要求”变成可跟踪的响应矩阵，支撑生成、合规和验收。

实现任务：

1. 新增问题/要求模型：
   - 从 `evaluation`、`qualification`、`submission`、`invalid_risk` 模块抽取 `requirement_items`。
   - 字段包括：`id`、`type`、`requirement`、`priority`、`mandatory`、`score`、`source_ref`、`expected_response`、`status`。
   - 当前已落地：`TenderRequirementItem` 和 6 模块解析结果会统一汇总到 `structured_result.requirement_items`，每条 `source_ref` 对齐 AutoRFP 的 reference/source 思路并保存可追溯引用。

2. 数据落库：
   - 短期：存入 `tender_parse_results.structured_result.requirement_items`。
   - 中期：新增 `bid_requirement_items` 表，支持章节覆盖状态、人工确认、合规检查引用。
   - 独立表字段对齐 AutoRFP 的 `Question -> Answer -> Source` 思路：`external_id` 对应 `referenceId`，`requirement` 对应问题文本，`source_ref` 对应来源详情，`coverage_status` 对应回答覆盖状态。
   - 当前已落地：短期 JSON 存储路径、独立 `bid_requirement_items` 表、解析回调同步、人工确认同步、只读查询接口和前端筛选。

3. 生成输入：
   - 大纲生成按 requirement_items 生成章节覆盖计划。
   - 章节生成 prompt 必须传入本章负责覆盖的 requirement_items。
   - 输出 `self_check` 必须逐条返回 `requirement_id`、`satisfied`、`evidence`、`source_refs`。
   - 当前已落地：Go 从当前解析结果按章节标题提取 `requirement_refs`，单章重新生成、章节 AI 动作和整标逐章生成都会传给 Python；OpenAI-compatible Provider 提示词要求 `self_check.requirement_coverage`，模型未返回时补需复核覆盖矩阵；Go 回调会把 `self_check` 和 `requirement_coverage` 写入章节版本元数据，前端编辑器显示响应覆盖。

4. 来源引用：
   - 模型输出事实性段落必须带 `{{ref:chunk_id}}` 或结构化 `source_refs`。
   - Go 回调解析后写 `knowledge_references`；无法解析的引用标记 unresolved。
   - 前端显示“来源详情”：文档名、页码、原文摘录、相似度/重排分。

5. 验收：
   - 任一生成章节能追溯到对应 requirement_items。
   - 无引用的事实性段落进入 `needs_human_input`。
   - 前端能按“未覆盖/部分覆盖/已覆盖”过滤招标要求。
   - 当前已落地：后端和 AI 服务测试覆盖 requirement_items 到章节任务 payload、prompt 和 mock/fallback self_check 的追踪链路；前端编辑器已展示最近版本的覆盖状态；文件解读页已支持按“全部/必须/待确认/已覆盖”筛选。

## P1：Skill Pipeline 和阶段闸门

目标：不用引入 LangGraph，也实现 BidMaster 式清晰流水线和 Gate Keeper。

阶段定义：

| 阶段 | Gate | 完成条件 |
| --- | --- | --- |
| interpret | 招标解读 | 6 模块解析完成，低置信字段已人工确认 |
| plan | 大纲/响应矩阵 | requirement_items 已映射到章节 |
| generate | 章节生成 | 每章完成生成、source_refs/self_check 入库 |
| check | 合规检查 | 规则检查和语义检查完成，fail_candidate 已人工处理 |
| format | 文档排版 | docx/PDF/ZIP 导出通过打开、文本层、首屏非空校验 |

实现任务：

1. Go 状态机扩展：
   - 新增 `bid_pipeline_gates` 表，保存 `stage`、`status`、`reviewed_by`、`reviewed_at`、`reason`、`metadata`。
   - 写操作检查前置 gate，未通过时返回业务错误。
   - 不采用 BidMaster 的文件系统 `.reviewed` 闸门；ZBT 必须走租户 RLS 和审计日志。
   - 当前已落地：上传和解析创建 `interpret=pending`，AI 解析成功后进入 `needs_review`，人工确认进入 `passed`，解析失败进入 `blocked`；大纲生成前强制检查 `interpret=passed`，大纲生成成功后写入 `plan=passed`；整标生成前强制检查 `plan=passed`，全部章节生成完成后写入 `generate=passed`，失败或取消写入 `blocked`；合规检查完成和问题处理后写入 `check=passed|needs_review|blocked`；导出前强制检查 `generate=passed` 和 `check=passed`，导出完成写入 `format=passed`。

2. Python skill 规范：
   - 不引入外部框架，新增轻量 `SkillResult` 数据约定：`status`、`data`、`evidence`、`token_usage`、`warnings`。
   - tender_parse、outline_generate、chapter_generate、compliance_check、document_export 均返回统一 metadata。
   - `SkillResult.sources` 必须用于来源追踪，不能只放入自然语言说明。

3. 可观测性：
   - 每个阶段写 `ai_call_logs.biz_ref.stage` 和 `biz_ref.module`。
   - 失败时只记录安全错误摘要，不写 prompt、密钥或模型原始长响应。

4. 强制性要求检查：
   - 将 `mandatory_req_extract` 拆入解析阶段，输出 `requirement_items.mandatory=true`。
   - 将 `mandatory_req_check` 拆入合规阶段，逐项检查章节响应、来源和偏离状态。
   - `risk_level=high` 或强制项未覆盖时，Gate 只能进入人工复核，不能自动通过。

## P1：产品 UI 和人工复核

目标：外部项目都把 AI 结果作为“可编辑草稿”，ZBT 也必须坚持人工确认门控。

实现任务：

1. 解析结果页：
   - 6 模块 tabs。
   - 字段级置信度、来源摘录、页码跳转。
   - 一键标记“确认无误/需要补充/不适用”。

2. 响应矩阵页：
   - requirement_items 表格。
   - 章节覆盖状态、来源状态、合规状态。
   - 支持导出评审响应矩阵。

3. 章节编辑器：
   - 右侧来源详情不展示技术口径，不出现 provider/model/token。
   - 技术元数据仅在团队设置或 AI 调用日志页可见。

## P1：评测和验收脚本

目标：把“解析质量”从主观判断变成可重复回归。

实现任务：

1. 新增 `docs/sample_docs/golden/工程1.parse.json`。
   - 当前已落地：工程1 golden 覆盖 4 个真实样本文档。
2. 新增解析评测脚本：
   - 输入样本文件和 golden JSON。
   - 输出字段准确率、召回率、低置信字段数、无来源字段数、OCR 页数、耗时。
   - 当前已落地：`ai-service/app/evaluation/tender_parse_eval.py` 输出总体 score、逐项 expected/actual 和失败项，并在失败时返回非 0。
3. 新增生成评测：
   - 检查每个 requirement_item 是否被章节覆盖。
   - 检查每个 source_ref 是否可解析到当前租户 knowledge chunk。
4. 验收门槛：
   - 关键字段电子文本准确率不低于 85%。
   - 强制性 requirement 不允许无章节覆盖。
   - source_refs 解析率不低于 95%，未解析项必须进入人工复核。

## 推荐开发顺序

1. 扩展 `docs/ex/工程1` golden 样本，加入章节生成覆盖率和导出格式回归。
2. 接 OCR Provider 质量门控：页级 coverage、confidence、bbox、table_blocks。
3. 做 `bid_pipeline_gates`，把人工确认从 UI 状态变成后端门控。
4. 增强 docx 母版和 PDF 验收：样式、目录、页眉页脚、书签、可打开性。

## 非目标

1. 不把 TextIn、MinerU、LlamaCloud 作为硬依赖；它们只能作为 Provider 选项。
2. 不引入 LangGraph/LangChain 作为核心架构依赖；ZBT 保持 Go 状态机 + Python ModelRouter。
3. 不复制 AGPL 项目的代码、素材或 UI，只参考功能分解和验收思路。
4. 不允许生产环境以 MockProvider 完成真实 AI 能力验收。

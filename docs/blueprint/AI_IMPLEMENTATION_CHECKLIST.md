# AI 能力实现清单

本清单把三个外部项目的方法论转成智标通自己的实现任务。外部仓库只作为方案参考，不复制代码，避免许可证和架构污染。

参考来源与可借鉴点：

- `intsig-textin/xparse-sample-projects/tender-doc-parse`：TextIn `pdf_to_markdown` 解析、按 `#`/`##` 标题切块、关键词路由到 6 模块、模块 prompt 固定 schema、6 模块并发抽取、复杂表格保留 `md_table`。
- `guangshu100/BidMaster-Pro`：`SkillContext`/`SkillResult` 能力单元、`interpret -> outline -> content -> check -> format -> final_check -> export` 流水线、Gate Keeper 人工闸门、强制性要求抽取/检查、MinerU OCR 配置、docx 格式检查与 PDF 书签导出。
- `run-llama/auto_rfp`：`Question -> Answer -> Source` 独立模型、`referenceId`、问题抽取与摘要/资格并行处理、答案来源保存到 `sources`、来源详情展示文件名/页码/片段/相关度。

## 当前差距

当前 ZBT 已有 Go 主后端、Python AI 服务、ModelRouter、OCR HTTP 接入点、RAG 检索、source_refs 落库、Word/PDF/ZIP 导出和 AI 调用成本审计。仍需要继续增强的核心差距：

1. 招标解析已新增 6 模块结构化结果、字段级来源、置信度、要求项矩阵和模块级独立模型增强；6 个模块已支持受控并发执行、固定顺序合并和单模块失败隔离；`docs/ex/工程1` 已建立可执行 golden 回归评测；前端已补核心字段编辑确认、六模块字段逐项编辑、字段依据复核、字段置信度手工标记、来源摘录高亮、预览搜索定位、文件/知识库文档预览页定位条和来源坐标标签，仍需继续补 PDF canvas 选区框。
2. OCR 已有 Provider 契约、外部 HTTP 接口、成功响应归一化、页级质量指标、统一 `table_blocks` 和 `document_ocr` 网关路由；MinerU/PaddleOCR 样本验收已支持页级置信度、版面 bbox、表格 bbox 和单元格 bbox 门槛；仍缺少生产环境真实 OCR Provider 凭证和持续回归报告。
3. AutoRFP 式“问题矩阵/响应矩阵”已形成运行态闭环：招标要求可落入独立表，章节生成可回写覆盖状态、响应证据和来源数量，人工可调整覆盖状态和补充证据，支持单条/批量标记覆盖状态、补充响应证据和编辑响应来源，支持按覆盖、证据、来源完整性筛选，可从响应来源打开原文预览并复制页码/引用号/定位码/摘录，单条要求可查看模型/人工覆盖历史，历史来源也可打开原文和复制定位，并可导出评审响应矩阵 CSV 和带覆盖历史工作表的 xlsx；当前已补当前筛选条件下的跨页服务端批处理，预览入口会携带页码和摘录搜索参数，响应来源弹窗、响应历史和字段依据列已高亮来源摘录。
4. Skill/Gate 已从隐式状态机收敛为显式阶段闸门：`interpret`、`plan`、`generate`、`check`、`format` 阶段已落库并接入关键写操作。
5. 行业 MCP / Skills 调研已固化到 `docs/blueprint/EXTERNAL_MCP_SKILL_RADAR.md`；外部工具只能作为只读数据源、方法论和 checklist 参考。后端 P0 外部工具网关已提供租户级配置、Provider 预设目录、默认工具白名单、摘要审计、预算阻断和 JSON-RPC `tools/call` 入口，前端团队管理页已提供外部数据源配置和审计入口；标讯大厅已提供 Handaas 外部标讯检索和保存入口，仍需继续补生产凭证验证、企业画像和外部应答库业务入口。

## 当前落地进展

- `ai-service/app/schemas/tender.py` 已新增 `TenderParseModuleResult`、`TenderParseFieldEvidence`、`TenderRequirementItem` 和 `TenderParseStructuredResult`。
- `ai-service/app/pipelines/parse/tender_parser.py` 已在兼容旧字段的同时输出 `modules`、`field_evidence`、`requirement_items` 和 `quality_gates`；`quality_gates.interpret.module_quality` 和 `review_modules` 会按 6 模块记录字段数、证据数、要求项数、低置信/缺来源/待复核数量，避免只看全局通过率。模块增强 prompt 已使用 `xparse-context-router-v2`，按标题、关键词行、相邻 chunk 和 `table_blocks.md_table` 生成带 `chunk_id`、页码、`table_block_id`、路由原因的 `source_context`。
- `ai-service/app/main.py` 已按 6 个模块逐一调用 `tender_parse` 路由，每个模块独立走 provider 候选和 fallback；单模块失败时保留基础解析并标记待确认。
- `ai-service/app/gateway/contracts.py` 已明确 OCR Provider 的 `recognize_document`、`recognize_page`、`extract_layout`、`extract_tables` 契约。
- HTTP OCR 成功响应已统一归一为 `pages`、`blocks`、`tables`、`table_blocks`、`confidence`、`provider_metadata`。
- `ai-service/app/pipelines/parse/document_parser.py` 已为 PDF 输出 `page_quality`，并把 PDF、docx、xlsx、pptx 表格统一归一为 `table_blocks`；每个带行结构的表格块会生成或保留 `md_table`，PyMuPDF 表格会保留 table-level `bbox` 和可用的 `cell_bboxes`，用于模块抽取、RAG 上下文和版面追溯。OCR 接入已显式支持 `OCR_PROVIDER=http_ocr|mineru|paddleocr`，Provider 专属 endpoint/token/mode 会写入安全 metadata；若使用通用 `OCR_HTTP_ENDPOINT` / `OCR_API_KEY` / `OCR_POLL_ENDPOINT` 兜底，`provider_profile` 会记录实际生效的 env 名，便于生产排查。MinerU/PaddleOCR 的同步或异步响应会归一为 `markdown`、`pages`、`blocks`、`layout_blocks`、`table_blocks`，其中表格和版面块会提升到文档顶层 metadata 参与后续解析；OCR 顶层 `tables/table_blocks` 与页级 `pages[].tables` 会合并去重，常见 `cells` 输出会归一成 `rows`、`md_table`、`cell_bboxes` 并进入 chunk 文本。
- `ai-service/app/config/model_routing.yaml` 已声明 `document_ocr` 本地路由，便于 `/models/health`、Mock 路由审计和生产配置检查覆盖 OCR Provider 能力边界。
- `ai-service/app/evaluation/ocr_provider_eval.py` 已提供 MinerU / PaddleOCR 真实 endpoint 验收入口，默认把工程1采购 PDF 首页渲染成 PNG 后走 OCR Provider；无 endpoint 时输出 `skipped`，不会伪装为通过；验收会检查 `provider_profile.endpoint_env`、已配置 key 时的 `api_key_env`、已配置轮询地址时的 `poll_endpoint_env` 是否与实际生效 env 一致；可通过 `--min-page-confidence`、`--min-layout-bbox-count`、`--min-table-bbox-count`、`--min-cell-bbox-count` 把页级置信度和坐标级版面证据纳入验收。
- `frontend/src/features/bid/index.tsx` 的文件解读步骤已增加“信息分组”、“模块字段”和“响应要点”视图；“模块字段”会把 6 模块 `modules.*.fields` 展开为逐项编辑表，确认时写回原 `structured_result.modules`，并在 `parse_metadata.confirm_overrides.edited_module_fields` 记录调整路径。可解析到文件 ID 或知识库文档 ID 的来源会打开前端预览页，预览页展示来源标题、页码、摘录关键词、坐标标签和复制定位入口，并继续把页码/搜索词传给内嵌文件预览器。页面不展示模型、token、schema 等技术口径。
- `backend/internal/db/migrations/00031_bid_requirement_items.sql` 已新增 `bid_requirement_items` 独立表，按租户启用 RLS，承接 AutoRFP 式 referenceId/source attribution 思路。
- `backend/internal/platform/bid/store.go` 已在解析回调和人工确认两条路径同步 `requirement_items`，并提供 `ListRequirementItems`。
- `TenderParseFieldEvidence.source_ref` 已统一携带 `citation_id`、`reference_id`、`source_kind`、`file_id`、`filename`、`chunk_id`、`traceable`，模型增强结果缺少可追溯定位时必须进入人工复核。
- `backend/internal/api/routes.go` 已提供 `GET /bids/:id/requirements` 只读接口。
- `frontend/src/features/bid/index.tsx` 的“响应要点”已优先读取独立要求表，并支持“全部/必须/待确认/已覆盖”和“待补证据/待补来源/依据完整”筛选。
- 章节生成、整标逐章生成和章节 AI 自检回调会根据 `self_check.requirement_coverage` 回写 `bid_requirement_items.coverage_status` / `needs_review`，并将响应侧证据保存到 `metadata.latest_coverage`；招标原文 `source_ref` 不被覆盖。生成回调中的章节 `source_refs` 和覆盖矩阵 `source_refs` 会统一派生 `citation_id`、`reference_id` 和 `source_locator`，避免模型只返回 chunk/page 时无法通过 AutoRFP 来源引用门禁。前端“响应要点”表已展示覆盖状态、响应证据摘要和来源数量，不展示模型、token、schema 等技术口径。
- `backend/internal/api/routes.go` 已提供 `PATCH /bids/:id/requirements/:requirementId`，可人工调整单条要求覆盖状态并补充响应证据；人工结果写入 `metadata.latest_coverage` 和 `metadata.manual_coverage`，不覆盖招标原文 `source_ref`。
- `backend/internal/api/routes.go` 已提供 `PATCH /bids/:id/requirements`，前端“响应要点”表支持勾选多条要求后批量标记覆盖状态、批量补充响应证据和批量编辑响应来源；也支持按当前覆盖/证据筛选条件对全部匹配要求做服务端批处理，批量操作会逐条写入覆盖历史，任一要求项不存在时整批失败。
- 前端“响应要点”表的来源数量可打开响应来源列表；若来源携带 `file_id`、`file_asset_id`、`document_id` 或 `source_document_id`，可复用已有文件/知识库文档预览接口打开原文，带页码的来源会附加页码锚点；来源列表会展示并可复制页码、章节、引用号、定位码和摘录，便于人工复核传递精确定位。
- `backend/internal/db/migrations/00033_bid_requirement_coverage_events.sql` 已新增 `bid_requirement_coverage_events` RLS 表；模型回写和人工调整都会追加覆盖历史，`GET /bids/:id/requirements/:requirementId/history` 可按单条要求读取最近历史，前端“响应要点”表提供历史弹窗。
- `backend/internal/api/routes.go` 已提供 `GET /bids/:id/requirements/export`，前端“响应要点”页可导出 UTF-8 CSV 响应矩阵；`?format=xlsx` 可导出含“响应矩阵”和“覆盖历史”两个工作表的 Excel 文件，覆盖状态、响应证据、响应来源、招标原文来源和历史记录均使用业务口径。
- `ai-service/app/evaluation/tender_parse_eval.py` 已提供离线解析评测 CLI，`docs/sample_docs/golden/工程1.parse.json` 已覆盖采购 PDF、响应 docx、盖章投标 PDF 和固化清单 xlsx 的 109 项检查，并强制字段证据和要求项来源具备可追溯原文、引用号和定位信息。
- `backend/internal/db/migrations/00032_bid_pipeline_gates.sql` 已新增 `bid_pipeline_gates` RLS 表。
- `backend/internal/platform/bid/store.go` 已在上传、解析、解析回调、人工确认、大纲生成、整标生成和导出路径维护阶段闸门；`GenerateOutline` 会检查 `interpret=passed`，`GenerateBid` 会检查 `plan=passed`，`CreateExport` 会检查 `generate=passed` 和 `check=passed`。旧的已确认解析、大纲、已完成章节内容和已完成合规检查会按真实业务状态自动补齐闸门。
- `backend/internal/platform/compliance/store.go` 已在创建检查、问题修复、忽略和人工确认 fail 后同步 `check` 阶段闸门；`pass` 自动通过，`warn/fail_candidate` 进入待复核，`fail` 阻断。
- `backend/internal/api/routes.go` 已提供 `GET /bids/:id/pipeline-gates` 只读接口，前端 API client 已提供对应 DTO 和查询函数。
- `backend/internal/db/migrations/00034_external_tool_gateway.sql` 已新增 `external_tool_configs` 和 `external_tool_audit_logs` RLS 表；`backend/internal/platform/externaltool/store.go` 已提供 `streamable_http` 外部工具配置、白名单校验、JSON-RPC `tools/call` 调用、摘要审计和预算阻断；`GET /external-tools`、`PUT /external-tools/:providerKey`、`POST /external-tools/:providerKey/invoke`、`GET /external-tools/audit` 已接入 team 权限。
- `backend/internal/platform/externaltool/presets.go` 已新增 Handaas、AutoRFP、qlows、BidCraft、Loopio 只读 Provider 预设目录；`GET /external-tools/catalog` 可返回用途、默认工具白名单、token env、数据边界和来源链接；已知 Provider 会自动应用预设名称和默认白名单，严格 Provider 会拒绝目录外工具和关闭脱敏策略。
- `frontend/src/features/team/index.tsx` 已新增“外部数据源”tab，可查看 Provider 目录、用途、数据边界、启用状态和调用记录，并通过配置弹窗维护访问地址、启用工具、预算、超时时间、脱敏策略和费用估算。
- `frontend/src/features/tender/index.tsx` 已新增“外部标讯”业务入口，使用已授权 Handaas 只读数据源检索公开标讯，并可保存为 `metadata.source_type=external_mcp` 的租户内标讯。
- `ai-service/app/evaluation/generation_coverage_eval.py` 已提供离线生成覆盖评测：检查 mandatory requirement 覆盖率、已覆盖项是否带来源、`source_refs` 是否能解析到给定 `knowledge_chunks`，并要求响应来源具备引用号/定位码和页码、chunk、文件或文档定位；`docs/sample_docs/golden/工程1.generation_coverage.json` 已固化工程1生成覆盖 golden。
- `backend/internal/platform/bid/store.go` 已提供 `GET /bids/:id/generation-coverage` 运行态导出：从 `bid_requirement_items`、最新章节版本、章节 `source_refs` 与已解析 `knowledge_chunks` 组合出可直接交给离线评测器的 JSON，并默认携带 mandatory 覆盖、source_ref 解析、引用号/定位码和来源位置完整率阈值。
- `ai-service/app/evaluation/export_format_eval.py` 已提供离线导出格式评测：临时生成 DOCX、ZIP 和 PDF，检查 DOCX 可打开性、目录域、自动更新域、页码域、页眉页脚、表格、ZIP manifest、ZIP 内 DOCX 可打开性、PDF 可打开性、文本层和首屏非空；`docs/sample_docs/golden/工程1.export.json` 已固化工程1导出格式 golden。

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
   - 按标题层级、页码、表格块、关键词构造模块上下文：`chunk_id`、`title_path`、`content`、`page_start`、`page_end`、`table_block_ids`、`quality_flags`。
   - 借鉴 xparse 的“标题优先，正文前 200 字辅助”的路由规则，但增加多模块命中：评分表可同时进入 `evaluation` 和 `submission`，废标条款可同时进入 `invalid_risk` 和 `qualification`。
   - 当前已落地：模块上下文只发送命中 chunk、相邻 chunk 和命中表格块，`source_context` 保留 `chunk_id/page/table_block_id/reasons`，避免全量截断；相邻上下文以 `neighbor_chunk/adjacent_page` 标记并降低排序权重。

2. Python 新增模块契约：`ai-service/app/schemas/tender.py`
   - 新增 `TenderParseModuleResult`、`TenderParseFieldEvidence`、`TenderRequirementItem`、`TenderParseStructuredResult`。
   - 每个字段必须支持 `value`、`confidence`、`source_text`、`page_start`、`page_end`、`bbox`、`chunk_id`、`needs_review`。
   - 顶层保留兼容字段：`project_name`、`bid_type`、`deadline`、`qualification_requirements`、`invalid_clause_risks`、`scoring_points`、`outline`、`material_suggestions`。

3. Python 新增解析编排：`ai-service/app/pipelines/parse/tender_parser.py`
   - 将当前 `build_tender_parse_prompt()` 拆成 6 个 module prompt builder。
   - 根据标题、关键词、页码和表格 metadata 生成模块上下文，不再把全文截断后交给单 prompt。
   - 每个模块独立调用 `run_provider_task()` 或等价 ModelRouter 路由，允许模块级 fallback。
   - 当前已落地：`process_tender_parse` 使用 `TENDER_PARSE_MODULE_CONCURRENCY` 控制 6 模块并发，主线程按 `MODULE_ORDER` 固定顺序合并结果，单模块失败只标记该模块待复核。
   - 当前已落地：`parse_metadata.module_context` 记录每个模块命中的 chunk 数、表格块数、`chunk_ids`、`table_block_ids` 和匹配原因，供回归审计和后续前端来源定位使用。
   - 当前已落地：`quality_gates.interpret.module_quality` 和 `parse_metadata.module_quality` 记录每个模块的字段、证据、要求项、低置信、缺来源、待复核和可追溯来源计数；`review_modules` 明确列出需要人工复核的模块。
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
   - 当前已落地：核心字段可直接编辑；6 模块 `fields` 已展开为逐项编辑表，列表字段按行编辑，数字/布尔/JSON 结构确认时尽量按原类型还原。

6. 回归评测：
   - 使用 `docs/ex/工程1` 的 PDF、docx、xlsx 样本建立 golden JSON。
   - 新增脚本输出字段命中率：项目名、预算、截止日、资格要求、评分项、无效标条款、附件清单。
   - 当前已落地：`docs/sample_docs/golden/工程1.parse.json` 和 `python -m app.evaluation.tender_parse_eval --golden ...`，当前样本 109/109 通过；字段证据和要求项 `source_ref` 会检查可追溯原文、引用号和页码/chunk/文件/文档定位。
   - 当前已落地：样本评测会检查 `table_blocks.required_sources`、`min_total_rows`、`min_blocks_with_rows`、`min_blocks_with_bbox`、`min_cells_with_bbox`、`require_md_table`、`md_table_must_contain` 和表格块关键单元格，防止 PDF/DOCX/XLSX 表格退化为普通文本或丢失 PDF 单元格级版面证据。
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
   - 当前已落地：OCR `provider_profile` 记录实际生效的 endpoint/key/poll env；Provider 专属配置为空但通用 OCR 配置生效时，不会误报为使用了专属 env。
   - 当前网关路由：`model_routing.yaml` 的 `document_ocr` 使用 `local` provider 和 `configurable-ocr-provider-pipeline`，实际厂商通过上述环境变量切换，避免把客户文件外发给未显式配置的模型路由。
   - Provider 不可用时不伪装成功，必须返回 `provider_not_configured` 或 `failed`。
   - 当前已落地异步轮询：初始响应为 202 或 `pending/running/processing` 时，使用响应 `status_url` / `poll_url`、`OCR_POLL_ENDPOINT`、`MINERU_POLL_ENDPOINT` 或 `PADDLEOCR_POLL_ENDPOINT` 轮询；未提供轮询地址时默认请求 `endpoint/{task_id}`。轮询结果归一到 `pages`、`blocks`、`tables`、`markdown`、`layout_blocks`，超时和失败只保存安全摘要。
   - 当前已落地真实 endpoint 验收命令：`python -m app.evaluation.ocr_provider_eval --provider mineru|paddleocr`，可设置最小文本、表格块和版面块门槛，并检查 endpoint/key/poll 实际生效 env；无 endpoint 时返回 skipped。
   - OCR Provider 只作为可替换插件，不进入主业务 schema 的厂商字段。

3. 页级质量分流：`ai-service/app/pipelines/parse/document_parser.py`
   - 计算每页文本层覆盖率、字符密度、表格候选数、图片占比。
   - 低覆盖页自动进入 OCR；混合 PDF 只 OCR 低覆盖页。
   - OCR 结果与文本层结果按页合并，保留来源标记：`text_layer`、`ocr`、`table`、`layout_block`。

4. 表格策略：
   - PDF 表格、xlsx 工作表、docx 表格统一转成 `table_blocks`。
   - 评分表和工程量清单不拆成普通段落；作为完整表格块进入模块抽取和 RAG。
   - 表格块保留表头、页码、表级 bbox、可用的单元格 bbox 和截断标记。
   - OCR Provider 返回的顶层表格和页级表格必须合并为文档级 `table_blocks` 并去重；只有表格、没有纯文本的 OCR 结果也要通过 `md_table` 进入 chunk，避免扫描清单不可检索。

5. 验收：
   - `docs/ex/工程1/采购文件桥梁检查.pdf` 能输出 page/block/table metadata。
   - 空文本或扫描页不会被标记为普通解析成功。
   - OCR 超时、响应过大、无效 JSON、低置信页均有测试覆盖。
   - 当前已落地：真实 Provider 评测入口可按页级置信度、版面 bbox、表格 bbox 和单元格 bbox 设置门槛，避免“只有文本、没有版面证据”的结果被当成生产可用 OCR。

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
   - 当前已落地：短期 JSON 存储路径、独立 `bid_requirement_items` 表、解析回调同步、人工确认同步、生成覆盖回写、只读查询接口和前端筛选。

3. 生成输入：
   - 大纲生成按 requirement_items 生成章节覆盖计划。
   - 章节生成 prompt 必须传入本章负责覆盖的 requirement_items。
   - 输出 `self_check` 必须逐条返回 `requirement_id`、`satisfied`、`evidence`、`source_refs`。
   - 当前已落地：Go 从当前解析结果按章节标题提取 `requirement_refs`，单章重新生成、章节 AI 动作和整标逐章生成都会传给 Python；OpenAI-compatible Provider 提示词要求 `self_check.requirement_coverage`，模型未返回时补需复核覆盖矩阵；Go 回调会把 `self_check` 和 `requirement_coverage` 写入章节版本元数据，并同步回写要求项覆盖状态，前端编辑器和“响应要点”显示响应覆盖、证据摘要和来源数量。

4. 来源引用：
   - 模型输出事实性段落必须带 `{{ref:chunk_id}}` 或结构化 `source_refs`。
   - Go 回调解析后写 `knowledge_references`；无法解析的引用标记 unresolved。
   - 前端显示“来源详情”：文档名、页码、原文摘录、相似度/重排分。
   - 当前已落地：响应来源弹窗会展示来源标题、页码、引用号、定位码和高亮摘录，并支持一键复制定位；有可预览文件/文档时可携带页码和摘录搜索参数打开原文，PDF/Word 查看器内搜索命中取决于浏览器或预览器支持。

5. 验收：
   - 任一生成章节能追溯到对应 requirement_items。
   - 无引用的事实性段落进入 `needs_human_input`。
   - 前端能按“未覆盖/部分覆盖/已覆盖”过滤招标要求。
   - 当前已落地：后端和 AI 服务测试覆盖 requirement_items 到章节任务 payload、prompt 和 mock/fallback self_check 的追踪链路；前端编辑器已展示最近版本的覆盖状态；文件解读页已支持按“全部/必须/待确认/已覆盖”和“待补证据/待补来源/依据完整”筛选，并在响应要点表展示响应证据和来源数量；`PATCH /bids/:id/requirements/:requirementId` 可人工调整覆盖状态、证据和响应来源；`PATCH /bids/:id/requirements` 可批量标记覆盖状态、批量补充响应证据和批量编辑响应来源，并支持 `apply_all/filter/evidence_filter` 按当前筛选条件服务端批处理；`GET /bids/:id/requirements/:requirementId/history` 可读取单条要求覆盖历史，前端历史弹窗可从历史来源打开原文和复制定位；`GET /bids/:id/requirements/export` 可下载评审响应矩阵 CSV 和带历史工作表的 xlsx。

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
   - 当前已落地：解析确认页支持项目名称、投标截止、标书类型、资格要求、评分要点、否决风险的确认前编辑；确认提交会同步更新 `structured_result` 顶层字段、对应 6 模块字段和已修改关键要求的响应条目，并记录本次人工调整字段。
   - 当前已落地：解析结果页新增“字段依据”表，展示字段结果、可信度、原文摘录、页码/引用号/定位码/坐标，支持复制来源定位、按页查看原文，并可逐项标记“确认无误/需要补充/不适用”；复核状态随确认写入 `field_evidence.review_status`、`needs_review`、`confidence` 和 `parse_metadata.field_reviews`，同步模块 `evidence/status`，并刷新解析质量门的低置信/缺来源统计。

2. 响应矩阵页：
   - requirement_items 表格。
   - 章节覆盖状态、来源状态、合规状态。
   - 当前已落地：响应要点表展示覆盖状态、证据摘要和来源数量，支持按覆盖、证据和来源完整性筛选，支持人工调整覆盖状态、按选中项或当前筛选全部批量标记覆盖状态、单条或批量补充证据和响应来源，并支持导出评审响应矩阵 CSV 和带历史工作表的 xlsx。

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
   - 当前已落地：`python -m app.evaluation.generation_coverage_eval --input <coverage.json>` 可对 `requirements`、`chapters[].requirement_coverage`、章节 `source_refs` 和 `knowledge_chunks` 做离线验收，输出 mandatory 覆盖率、source_ref 解析率、引用号/定位码完整率和来源位置完整率；`docs/sample_docs/golden/工程1.generation_coverage.json` 作为固定 golden 样本。
   - 当前已落地：`GET /bids/:id/generation-coverage` 可从运行态标书导出 `<coverage.json>` 的完整输入契约。
4. 新增导出格式评测：
   - 当前已落地：`python -m app.evaluation.export_format_eval --input ../docs/sample_docs/golden/工程1.export.json` 会生成并校验 DOCX、ZIP 和 PDF 导出产物，覆盖目录、页码、页眉页脚、表格、manifest、PDF 文本层和首屏非空。
5. 验收门槛：
   - 关键字段电子文本准确率不低于 85%。
   - 强制性 requirement 不允许无章节覆盖。
   - source_refs 解析率不低于 95%，引用号/定位码和来源位置完整率必须满足样本阈值，未解析项或缺定位项必须进入人工复核。
   - DOCX / ZIP / PDF 导出格式检查必须全部通过；本地缺 LibreOffice 时只能在配置允许的评测中显式 skipped，生产验收不得伪装通过。

## 推荐开发顺序

1. 用运行态真实标书导出的 `<coverage.json>` 和真实导出产物持续刷新工程1生成覆盖与导出格式 golden。
2. 用真实 MinerU/PaddleOCR endpoint 固化 OCR Provider 质量门控报告：页级 coverage、confidence、bbox、table_blocks。
3. 做 `bid_pipeline_gates`，把人工确认从 UI 状态变成后端门控。
4. 增强 docx 母版和 PDF 验收：样式、目录、页眉页脚、书签、可打开性。

## 非目标

1. 不把 TextIn、MinerU、LlamaCloud 作为硬依赖；它们只能作为 Provider 选项。
2. 不引入 LangGraph/LangChain 作为核心架构依赖；ZBT 保持 Go 状态机 + Python ModelRouter。
3. 不复制 AGPL 项目的代码、素材或 UI，只参考功能分解和验收思路。
4. 不允许生产环境以 MockProvider 完成真实 AI 能力验收。

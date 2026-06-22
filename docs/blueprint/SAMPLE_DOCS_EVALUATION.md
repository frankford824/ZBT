# 样本文档评测

sample_docs 用于 Loop 6 起验证 PDF / Word 解析、切片、embedding、搜索、引用和导出。

## 目录

- docs/sample_docs/tender_samples：招标文件样本。
- docs/sample_docs/bid_samples：投标文件样本。
- docs/sample_docs/expected_outputs：解析和生成期望输出。
- docs/sample_docs/golden：真实样本的可执行解析验收配置。

当前已建立 `docs/sample_docs/golden/工程1.parse.json`、`docs/sample_docs/golden/工程1.generation_coverage.json` 和 `docs/sample_docs/golden/工程1.export.json`，引用本地 `docs/ex/工程1` 的标准招投标文件和对应响应场景，不复制原始文件内容。

## 基础指标

1. 可提取标题、正文、表格和页码。
2. 扫描件触发 OCRProvider。
3. chunk 带 section_path、page_start、page_end、source_file_id。
4. 搜索结果带 source_refs。
5. 低置信 OCR 内容标记需人工确认。

## 工程1 解析回归

运行命令：

```bash
cd ai-service
.venv/bin/python -m app.evaluation.tender_parse_eval \
  --golden ../docs/sample_docs/golden/工程1.parse.json
```

工程1生成覆盖与来源引用 golden 可使用独立离线评测入口：

```bash
cd ai-service
.venv/bin/python -m app.evaluation.generation_coverage_eval \
  --input ../docs/sample_docs/golden/工程1.generation_coverage.json
```

运行态标书也可以先从后端导出真实标书生成覆盖样本，再交给同一评测器：

```bash
# 先从运行态后端导出真实标书生成覆盖样本
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/bids/<bid_id>/generation-coverage \
  > ../docs/sample_docs/golden/<生成覆盖样本>.json

cd ai-service
.venv/bin/python -m app.evaluation.generation_coverage_eval \
  --input ../docs/sample_docs/golden/<生成覆盖样本>.json
```

输入 JSON 至少包含：

- `requirements` 或 `requirement_items`：解析出的招标要求，强制项使用 `mandatory=true` 或 `priority=high|mandatory|required`。
- `chapters`：章节生成结果，可在章节顶层或 `model_metadata.self_check.requirement_coverage` 放置覆盖矩阵。
- `knowledge_chunks`：可解析来源集合，字段为 `chunk_id` 和可选 `document_id`。

评测会输出 mandatory requirement 覆盖率、source_ref 解析率、引用号/定位码完整率和来源位置完整率，并检查已覆盖项是否携带来源。响应侧 `source_refs` 至少应能解析到已知 chunk/document，且具备 `citation_id` / `reference_id` / `locator` 等引用标识，以及页码、chunk、文件或文档定位信息。

工程1导出格式 golden 可使用独立离线评测入口：

```bash
cd ai-service
.venv/bin/python -m app.evaluation.export_format_eval \
  --input ../docs/sample_docs/golden/工程1.export.json
```

导出评测会临时生成 DOCX、ZIP 和 PDF，并检查 DOCX 可打开性、目录域、自动更新域、页码域、页眉页脚、表格、ZIP manifest、ZIP 内 DOCX 可打开性、PDF 可打开性、文本层和首屏非空。`pdf.allow_skip=true` 时，无 LibreOffice 的本地环境会显式记录 skipped；生产验收可把该值改为 `false` 作为硬门槛。

`./infra/scripts/check.sh` 会运行工程1 OCR canary。无真实 endpoint 的本地环境可 `--allow-skip`，报告会显示 skipped；配置 endpoint 后会检查 OCR 文本、表格块、版面 bbox、表格 bbox 和单元格 bbox。MinerU / PaddleOCR 真实 Provider 也可使用独立 OCR 验收入口。默认会把 `docs/ex/工程1/采购文件桥梁检查.pdf` 第一页渲染为 PNG 后走 OCR Provider，避免只验证 PDF 文本层：

```bash
cd ai-service

# MinerU：需要配置 MINERU_HTTP_ENDPOINT，可选 MINERU_API_KEY / MINERU_PARSE_MODE / MINERU_POLL_ENDPOINT
.venv/bin/python -m app.evaluation.ocr_provider_eval \
  --provider mineru \
  --min-text-chars 20 \
  --min-page-confidence 0.80 \
  --min-layout-bbox-count 1 \
  --min-table-bbox-count 1 \
  --min-cell-bbox-count 1

# PaddleOCR：需要配置 PADDLEOCR_HTTP_ENDPOINT，可选 PADDLEOCR_API_KEY / PADDLEOCR_PIPELINE / PADDLEOCR_POLL_ENDPOINT
.venv/bin/python -m app.evaluation.ocr_provider_eval \
  --provider paddleocr \
  --min-text-chars 20 \
  --min-page-confidence 0.80 \
  --min-layout-bbox-count 1 \
  --min-table-bbox-count 1 \
  --min-cell-bbox-count 1

# 本地或 CI 没有真实 endpoint 时可允许 skipped，但报告不会伪装成 passed
.venv/bin/python -m app.evaluation.ocr_provider_eval \
  --provider mineru \
  --allow-skip
```

OCR 验收会检查 provider 是否配置、样本是否存在、OCR 状态是否为 done、返回 provider 是否匹配、识别文本长度、chunk 数和 `provider_profile.endpoint_env`；`provider_profile.api_key_env` / `poll_endpoint_env` 会记录 key 与轮询地址实际生效的 env 名，便于区分 Provider 专属配置和通用 OCR 兜底。`model_routing.yaml` 同时声明 `document_ocr` 本地路由，用于把 OCR Provider 纳入模型网关配置审计。需要表格、版面块或坐标级版面证据验收时可加 `--min-table-blocks`、`--min-layout-blocks`、`--min-page-confidence`、`--min-layout-bbox-count`、`--min-table-bbox-count`、`--min-cell-bbox-count`。

验收覆盖：

1. `采购文件桥梁检查.pdf`：PDF 文本层、页数、表格块、OCR 标记、项目名、截止日、采购人、预算、地点、资格要求、评分项、否决风险、附件/清单要求。
2. `响应文件格式.docx`：Word 段落、表格块、响应函/承诺书/清单格式。
3. `盖章投标文件.pdf`：已盖章响应文件的 PDF 文本层、页数、表格块和供应商文本。
4. `清单（固化）(1).xlsx`：清单工作表、表格块、最高单价/综合单价字段。
5. 表格块结构验收：`table_blocks` 会检查来源类型、总行数、具备 `rows` 的块数量、PDF 表级 bbox 数量、PDF 单元格 bbox 数量、`md_table` 存在性和关键单元格文本，避免表格被降级成普通正文或丢失版面证据。
6. 来源引用结构验收：字段证据和要求项 `source_ref` 均要求具备可追溯原文、引用号或定位码，并能提供页码、chunk、文件或文档定位，避免 AutoRFP 式来源引用退化成普通摘要。
7. 六模块质量摘要：`quality_gates.interpret.module_quality` 和 `parse_metadata.module_quality` 会记录每个模块字段数、证据数、要求项数、低置信、缺来源和待复核数量；`review_modules` 用于定位需要人工复核的模块。
8. 响应问题清单验收：`requirement_items` 会检查问题类型、强制项数量、高优先级数量、`expected_response` 响应建议和单条问题的来源原文，确保解析结果可直接进入响应矩阵和章节生成。

当前解析门槛为 117 项检查全部通过；生成覆盖门槛为 9 项检查全部通过；导出格式门槛为 23 项检查全部通过。失败时 CLI 返回非 0，并输出失败项的 expected/actual。

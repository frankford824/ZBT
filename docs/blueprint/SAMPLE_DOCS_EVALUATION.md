# 样本文档评测

sample_docs 用于 Loop 6 起验证 PDF / Word 解析、切片、embedding、搜索、引用和导出。

## 目录

- docs/sample_docs/tender_samples：招标文件样本。
- docs/sample_docs/bid_samples：投标文件样本。
- docs/sample_docs/expected_outputs：解析和生成期望输出。
- docs/sample_docs/golden：真实样本的可执行解析验收配置。

当前已建立 `docs/sample_docs/golden/工程1.parse.json`，引用本地 `docs/ex/工程1` 的标准招投标文件，不复制原始文件内容。

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

生成覆盖与来源引用可使用独立离线评测入口：

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

评测会输出 mandatory requirement 覆盖率、source_ref 解析率，并检查已覆盖项是否携带来源。

验收覆盖：

1. `采购文件桥梁检查.pdf`：PDF 文本层、页数、表格块、OCR 标记、项目名、截止日、采购人、预算、地点、资格要求、评分项、否决风险、附件/清单要求。
2. `响应文件格式.docx`：Word 段落、表格块、响应函/承诺书/清单格式。
3. `盖章投标文件.pdf`：已盖章响应文件的 PDF 文本层、页数、表格块和供应商文本。
4. `清单（固化）(1).xlsx`：清单工作表、表格块、最高单价/综合单价字段。
5. 表格块结构验收：`table_blocks` 会检查来源类型、总行数、具备 `rows` 的块数量、PDF 表级 bbox 数量、PDF 单元格 bbox 数量、`md_table` 存在性和关键单元格文本，避免表格被降级成普通正文或丢失版面证据。

当前门槛为 103 项检查全部通过；失败时 CLI 返回非 0，并输出失败项的 expected/actual。

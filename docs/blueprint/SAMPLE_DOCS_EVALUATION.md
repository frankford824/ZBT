# 样本文档评测

sample_docs 用于 Loop 6 起验证 PDF / Word 解析、切片、embedding、搜索、引用和导出。

## 目录

- docs/sample_docs/tender_samples：招标文件样本。
- docs/sample_docs/bid_samples：投标文件样本。
- docs/sample_docs/expected_outputs：解析和生成期望输出。

## 基础指标

1. 可提取标题、正文、表格和页码。
2. 扫描件触发 OCRProvider。
3. chunk 带 section_path、page_start、page_end、source_file_id。
4. 搜索结果带 source_refs。
5. 低置信 OCR 内容标记需人工确认。

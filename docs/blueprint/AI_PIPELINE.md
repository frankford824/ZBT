# AI 与文档处理流水线

## 文档处理

上传文件后，Go 保存 file_asset 和 MinIO object，创建任务。Python 按文件类型处理：PDF 使用 PyMuPDF，Word 使用 python-docx，Excel 使用 openpyxl，PPT 使用 python-pptx，旧格式通过 LibreOffice 转换，图片和扫描 PDF 走 OCRProvider。

输出统一中间文档模型，随后清洗、结构感知切片、embedding、写入 knowledge_chunks 和 pgvector。

## 逐章生成

输入 bid_document、bid_part、bid_chapter、tender_parse_result、requirement_refs、selected_knowledge_refs、retrieved_chunks、previous_chapter_summaries 和 template_style。

输出 Tiptap JSON、source_refs、self_check、needs_human_input、model metadata、token usage 和 trace_id。

事实性内容没有引用时必须标记 needs_human_input。

## 合规检查

Go 执行确定性规则检查。Python 执行语义一致性、评分优化、模糊废标风险和修复建议。LLM 不得直接输出 fail。

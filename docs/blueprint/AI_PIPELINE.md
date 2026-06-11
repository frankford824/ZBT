# AI 与文档处理流水线

## 文档处理

上传文件后，Go 保存 file_asset 和 MinIO object，创建任务。Python 按文件类型处理：PDF 使用 PyMuPDF，Word 使用 python-docx，Excel 使用 openpyxl，PPT 使用 python-pptx，旧格式通过 LibreOffice 转换，图片和扫描 PDF 走 OCRProvider。

当前一期实现路径：前端通过 Go 获取 MinIO 预签名 URL，上传后调用 confirm；Go 将 ready 的 file_asset 转成 knowledge_document。用户点击处理后，Go 创建 ai_tasks，调用 Python AI 服务 `/tasks/knowledge-process`，Python 通过 ModelRouter 选择处理路线并返回外部 task_id。Python 后台任务读取 MinIO 对象，已支持 text/plain、PDF 和 Word 的最小文本抽取与切片；完成后通过 HMAC 签名回调 Go，Go 验签后更新 ai_tasks、knowledge_documents.parse_status，并将切片写入 knowledge_chunks。

输出统一中间文档模型，随后清洗、结构感知切片、embedding、写入 knowledge_chunks 和 pgvector。

## 逐章生成

输入 bid_document、bid_part、bid_chapter、tender_parse_result、requirement_refs、selected_knowledge_refs、retrieved_chunks、previous_chapter_summaries 和 template_style。

输出 Tiptap JSON、source_refs、self_check、needs_human_input、model metadata、token usage 和 trace_id。

当前一期实现路径：Go 在 `POST /chapters/:chapterId/regenerate` 中预生成外部 task_id，写入 `ai_tasks(resource_type='bid_chapter')`，并将章节状态置为 `generating`。Python `/tasks/chapter-generate` 返回 202 + task_id，在后台执行 ModelRouter；完成后通过 HMAC 回调 Go。Go 根据 `external_task_id` 定位任务，更新任务状态、章节内容、`bid_chapter_versions` 和 `knowledge_references`。前端通过 `GET /ai-tasks/:taskId` 轮询刷新章节，后续再补 SSE。

事实性内容没有引用时必须标记 needs_human_input。

## 合规检查

Go 执行确定性规则检查。Python 执行语义一致性、评分优化、模糊废标风险和修复建议。LLM 不得直接输出 fail。

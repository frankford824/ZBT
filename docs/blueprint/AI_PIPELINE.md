# AI 与文档处理流水线

外部同类项目方法论已转成 ZBT 自有任务清单，见 `docs/blueprint/AI_IMPLEMENTATION_CHECKLIST.md`。该清单只复用解析、OCR、Skill Pipeline、问题矩阵和来源引用的方案思路，不复制外部代码。

## 文档处理

上传文件后，Go 保存 file_asset 和 MinIO object，创建任务。Python 按文件类型处理：PDF 使用 PyMuPDF，Word 使用 python-docx，Excel 使用 openpyxl，PPT 使用 python-pptx，旧格式通过 LibreOffice 转换，图片和扫描 PDF 走 OCRProvider。

当前一期实现路径：前端通过 Go 获取 MinIO 预签名 URL，上传后调用 confirm；Go 将 ready 的 file_asset 转成 knowledge_document。用户点击处理后，Go 创建 ai_tasks，签名调用 Python AI 服务 `/tasks/knowledge-process`，Python 通过 ModelRouter 选择处理路线并返回外部 task_id。Python 后台任务读取 MinIO 对象，已支持 text/plain、PDF 文本层/版面块/表格候选、docx 段落和表格、xlsx/xlsm、pptx/pptm；空文本 PDF 可通过 `OCR_HTTP_ENDPOINT` 接外部 OCR，未配置 OCR 时标记 `ocr_required` 而不伪装为成功。随后通过 `knowledge_embedding` 路由生成 embedding，并随 HMAC 签名回调 Go。Go 验签后更新 ai_tasks、knowledge_documents.parse_status，并将切片和 embedding 写入 knowledge_chunks / pgvector。

输出统一中间文档模型，随后清洗、结构感知切片、embedding、写入 knowledge_chunks 和 pgvector。随仓路由默认以 OpenAI-compatible embedding Provider 为主路径；未配置真实 Key 时才走显式 Mock fallback。`AI_EMBEDDING_PROVIDER` / `AI_EMBEDDING_MODEL` 可覆盖为 DashScope、DeepSeek 兼容网关或本地 BGE 类服务。

## 逐章生成

输入 bid_document、bid_part、bid_chapter、tender_parse_result、requirement_refs、selected_knowledge_refs、retrieved_chunks、previous_chapter_summaries 和 template_style。

输出 Tiptap JSON、source_refs、self_check、needs_human_input、model metadata、token usage 和 trace_id。

阶段闸门使用 `bid_pipeline_gates` 统一记录。招标文件上传或重新解析时写入 `interpret=pending`；AI 解析完成后进入 `needs_review`；人工确认解析结果后写入 `interpret=passed`；解析失败写入 `blocked`。大纲生成前 Go 会强制检查 `interpret=passed`，通过后才允许生成目录和响应计划；大纲生成成功后写入 `plan=passed`。整标生成前强制检查 `plan=passed`，生成 job 汇总时按章节步骤结果写入 `generate=pending|passed|blocked`，其中任一章节失败或取消都会阻断生成阶段。导出前强制检查 `generate=passed`，导出回调写入 `format=pending|passed|blocked`。历史数据中已有 `confirmed` 解析结果、已有目录大纲或已有完成章节内容但缺少闸门记录时，会按真实业务状态自动补齐对应阶段。

当前一期实现路径：Go 在解析回调和人工确认时把 `structured_result.requirement_items` 同步到 `bid_requirement_items`，并通过 `GET /bids/:id/requirements` 给前端展示和筛选。Go 在 `POST /chapters/:chapterId/regenerate` 中预生成外部 task_id，检索当前租户 `knowledge_chunks`，将真实 chunk/document 引用写入 `retrieved_knowledge_refs`，并从当前招标解析结果中按章节标题选取 `requirement_refs` 后创建 `ai_tasks(resource_type='bid_chapter')`，再将章节状态置为 `generating`。整标逐章生成和章节 AI 动作复用同一需求提取逻辑。Go 签名调用 Python `/tasks/chapter-generate`；Python 返回 202 + task_id 后在后台执行 ModelRouter。ModelRouter 默认优先使用 OpenAI-compatible 主 Provider，真实 Provider 不健康且允许 Mock fallback 时才降级；`AI_LLM_PROVIDER` / `AI_LLM_MODEL` 可覆盖具体厂商和模型。OpenAI-compatible Provider 要求模型在 `self_check.requirement_coverage` 中逐条返回 `requirement_id`、覆盖状态、证据和引用；模型遗漏该结构时服务端补一个需复核覆盖矩阵，避免回调结果缺少追踪链路。完成后通过 HMAC 回调 Go，Go 根据 `external_task_id` 定位任务，更新任务状态、章节内容、`bid_chapter_versions` 和 `knowledge_references`；章节版本的 `model_metadata` 会保存 `self_check` 和 `requirement_coverage`，前端编辑器用业务口径展示“响应覆盖”。前端通过 `/bids/:id/generation/stream` 订阅 SSE 进度，并保留 `GET /ai-tasks/:taskId` 轮询兜底。

事实性内容没有引用时必须标记 needs_human_input。

## 合规检查

Go 执行确定性规则检查。Python 执行语义一致性、评分优化、模糊废标风险和修复建议。LLM 不得直接输出 fail。

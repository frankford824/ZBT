# RAG 设计

一期使用 PostgreSQL + pgvector，保留 VectorStore 抽象。

检索流程：

1. query = 章节标题 + 招标要求 + 当前任务上下文。
2. metadata filter = tenant_id、doc_type、tag、category、expire_at。
3. pgvector 召回 Top 30。
4. PostgreSQL 全文关键词召回 Top 30。
5. RRF 融合。
6. RerankProvider 精排。
7. 取 Top 5-8 构造上下文。
8. LLM 生成并返回 source_refs。
9. 落库 knowledge_references。

生成内容使用类似 {{ref:chunk_id}} 的中间标记，落库时解析为 source_refs。

当前实现状态：text/plain、PDF 和 Word 已具备最小文本抽取、切片和 knowledge_chunks 入库能力。Python 文档处理任务会通过 `knowledge_embedding` 路由生成 MockProvider embedding，Go 回调入库到 `knowledge_chunks.embedding vector(1024)`，并建立 HNSW cosine 索引。`POST /knowledge/search` 会调用 Python `/embeddings/knowledge` 生成 query embedding，在当前租户内分别进行 pgvector Top K 和 PostgreSQL 全文关键词 Top K 召回，然后按 RRF 融合候选，再调用 Python `/rerank/knowledge` 走 `knowledge_rerank` 路由和 RerankProvider 精排；AI 服务不可用时仍保留 RRF/关键词兜底。章节重新生成已异步任务化，Go 会在创建任务前检索当前租户 `knowledge_chunks` 并传入 `retrieved_knowledge_refs`；Python 生成完成回调 Go 后保存 AI 返回的 source_refs，若 chunk/document 同租户存在则将 `knowledge_references.source_document_id` 与 `chunk_id` 解析为真实 UUID 并标记 resolved。真实 embedding Provider 和真实 rerank Provider 仍可按 ModelRouter 配置替换。

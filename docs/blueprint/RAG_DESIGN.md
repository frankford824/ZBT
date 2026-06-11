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

# 模型网关

所有模型调用经过 Python AI 服务的 ModelRouter。

## Provider

LLMProvider：complete、generate_json、stream、count_tokens、health_check。

EmbeddingProvider：embed_text、embed_batch、get_dimensions、health_check。

RerankProvider：rerank、health_check。

OCRProvider：parse_pdf、parse_image、extract_layout、extract_tables、health_check。

ModelRouter：resolve、fallback、log_call、quota_status、enforce_quota。

`log_call()` 维护 Python AI 服务运行期内存账本，用于把调用返回中的 `estimated_cost` 按租户聚合，并输出当前预算状态快照；持久化审计仍由 Go 后端写入 `ai_call_logs`。`enforce_quota()` 基于 `quotas.default_tenant_monthly_budget` 和 `quotas.per_tenant_monthly_budget` 判断租户是否还能继续使用 Provider-backed 路由。随仓策略为 `downgrade_then_block`：租户预算耗尽后优先降级到 `mock/local` 这类零外部模型成本 Provider；没有可降级候选时明确拒绝路由解析。

当前已落地 `/embeddings/knowledge`，Go 后端在知识库搜索时调用该端点获取 query embedding。该端点通过 `knowledge_embedding` 路由解析 Provider；随仓配置以 OpenAI-compatible embedding 为主路径，未配置真实 Key 时才走显式 Mock fallback。`AI_EMBEDDING_PROVIDER` / `AI_EMBEDDING_MODEL` 可覆盖到 OpenAI-compatible、DashScope 或 Local BGE。

当前已落地 `/rerank/knowledge`，Go 后端在知识库搜索完成 RRF 融合后调用该端点精排候选。该端点通过 `knowledge_rerank` 路由解析 RerankProvider；随仓配置以 OpenAI-compatible Provider 为主路径，未配置真实 Key 时才走显式 Mock fallback。`AI_RERANK_PROVIDER` / `AI_RERANK_MODEL` 可覆盖到真实 rerank provider。

## Provider 类型

预留 OpenAI-compatible、Anthropic、DashScope、Gemini、DeepSeek、Local、Mock LLM；OpenAI-compatible、DashScope、Local BGE、Mock embedding；Cohere、Jina、Local BGE、Mock rerank；PaddleOCR、Cloud OCR、Azure Document Intelligence、Mistral OCR、Mock OCR。

模型名称不写死在代码中，通过 model_routing.yaml 和环境变量配置。Go 调用 Python AI 服务时携带 `X-ZBT-Timestamp` 和 `X-ZBT-Signature`；AI 服务在 `AI_SERVICE_HMAC_SECRET` 非空时强制验签，健康检查端点保持公开。

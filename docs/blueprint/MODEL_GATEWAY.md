# 模型网关

所有模型调用经过 Python AI 服务的 ModelRouter。

## Provider

LLMProvider：complete、generate_json、stream、count_tokens、health_check。

EmbeddingProvider：embed_text、embed_batch、get_dimensions、health_check。

RerankProvider：rerank、health_check。

OCRProvider：parse_pdf、parse_image、extract_layout、extract_tables、health_check。

ModelRouter：resolve、fallback、log_call、enforce_quota。

当前已落地 `/embeddings/knowledge`，Go 后端在知识库搜索时调用该端点获取 query embedding。该端点通过 `knowledge_embedding` 路由解析 Provider；随仓配置以 OpenAI-compatible embedding 为主路径，未配置真实 Key 时才走显式 Mock fallback。`AI_EMBEDDING_PROVIDER` / `AI_EMBEDDING_MODEL` 可覆盖到 OpenAI-compatible、DashScope 或 Local BGE。

当前已落地 `/rerank/knowledge`，Go 后端在知识库搜索完成 RRF 融合后调用该端点精排候选。该端点通过 `knowledge_rerank` 路由解析 RerankProvider；随仓配置以 OpenAI-compatible Provider 为主路径，未配置真实 Key 时才走显式 Mock fallback。`AI_RERANK_PROVIDER` / `AI_RERANK_MODEL` 可覆盖到真实 rerank provider。

## Provider 类型

预留 OpenAI-compatible、Anthropic、DashScope、Gemini、DeepSeek、Local、Mock LLM；OpenAI-compatible、DashScope、Local BGE、Mock embedding；Cohere、Jina、Local BGE、Mock rerank；PaddleOCR、Cloud OCR、Azure Document Intelligence、Mistral OCR、Mock OCR。

模型名称不写死在代码中，通过 model_routing.yaml 和环境变量配置。Go 调用 Python AI 服务时携带 `X-ZBT-Timestamp` 和 `X-ZBT-Signature`；AI 服务在 `AI_SERVICE_HMAC_SECRET` 非空时强制验签，健康检查端点保持公开。

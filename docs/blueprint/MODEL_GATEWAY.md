# 模型网关

所有模型调用经过 Python AI 服务的 ModelRouter。

## Provider

LLMProvider：complete、generate_json、stream、count_tokens、health_check。

EmbeddingProvider：embed_text、embed_batch、get_dimensions、health_check。

RerankProvider：rerank、health_check。

OCRProvider：parse_pdf、parse_image、extract_layout、extract_tables、health_check。

ModelRouter：resolve、fallback、log_call、enforce_quota。

当前已落地 `/embeddings/knowledge`，Go 后端在知识库搜索时调用该端点获取 query embedding。该端点通过 `knowledge_embedding` 路由解析 Provider，当前使用 Mock embedding，后续可替换为 OpenAI-compatible、DashScope 或 Local BGE。

## Provider 类型

预留 OpenAI-compatible、Anthropic、DashScope、Gemini、DeepSeek、Local、Mock LLM；OpenAI-compatible、DashScope、Local BGE、Mock embedding；Cohere、Jina、Local BGE、Mock rerank；PaddleOCR、Cloud OCR、Azure Document Intelligence、Mistral OCR、Mock OCR。

模型名称不写死在代码中，通过 model_routing.yaml 和环境变量配置。

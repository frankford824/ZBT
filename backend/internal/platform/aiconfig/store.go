package aiconfig

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	platformdb "github.com/frankford824/ZBT/backend/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidRequest = errors.New("invalid ai config request")

const (
	maxAIModelConfigJSONBytes = 16 * 1024
	maxAIConfigTextRunes      = 160
	maxAIConfigEndpointRunes  = 2048
	maxAIConfigMonthlyBudget  = 10000000
	maxAIConfigRate           = 1000000
)

var aiPricingKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_@./:*+-]{1,160}$`)

type Store struct {
	pool *pgxpool.Pool
}

type Config struct {
	ID                  string                 `json:"id"`
	Enabled             bool                   `json:"enabled"`
	LLMProvider         string                 `json:"llm_provider"`
	LLMModel            string                 `json:"llm_model"`
	EmbeddingProvider   string                 `json:"embedding_provider"`
	EmbeddingModel      string                 `json:"embedding_model"`
	RerankProvider      string                 `json:"rerank_provider"`
	RerankModel         string                 `json:"rerank_model"`
	OCRProvider         string                 `json:"ocr_provider"`
	OCREndpoint         string                 `json:"ocr_endpoint"`
	MonthlyBudget       float64                `json:"monthly_budget"`
	Pricing             map[string]PricingRate `json:"pricing"`
	MockFallbackAllowed bool                   `json:"mock_fallback_allowed"`
	Metadata            map[string]any         `json:"metadata"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

type UpsertRequest struct {
	Enabled             bool                   `json:"enabled"`
	LLMProvider         string                 `json:"llm_provider"`
	LLMModel            string                 `json:"llm_model"`
	EmbeddingProvider   string                 `json:"embedding_provider"`
	EmbeddingModel      string                 `json:"embedding_model"`
	RerankProvider      string                 `json:"rerank_provider"`
	RerankModel         string                 `json:"rerank_model"`
	OCRProvider         string                 `json:"ocr_provider"`
	OCREndpoint         string                 `json:"ocr_endpoint"`
	MonthlyBudget       float64                `json:"monthly_budget"`
	Pricing             map[string]PricingRate `json:"pricing"`
	MockFallbackAllowed bool                   `json:"mock_fallback_allowed"`
	Metadata            map[string]any         `json:"metadata"`
}

type PricingRate struct {
	InputPer1K   float64 `json:"input_per_1k,omitempty"`
	OutputPer1K  float64 `json:"output_per_1k,omitempty"`
	InputPer1M   float64 `json:"input_per_1m,omitempty"`
	OutputPer1M  float64 `json:"output_per_1m,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	DisplayName  string  `json:"display_name,omitempty"`
	LastReviewed string  `json:"last_reviewed,omitempty"`
}

type ProviderOption struct {
	ProviderKey  string   `json:"provider_key"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Capabilities []string `json:"capabilities"`
	SecretKeys   []string `json:"secret_keys"`
}

type OCROption struct {
	ProviderKey string `json:"provider_key"`
	Name        string `json:"name"`
	EndpointKey string `json:"endpoint_key"`
}

type RouteOption struct {
	TaskType   string `json:"task_type"`
	Name       string `json:"name"`
	Capability string `json:"capability"`
	Track      string `json:"track"`
}

type ProviderUsage struct {
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	Calls         int     `json:"calls"`
	EstimatedCost float64 `json:"estimated_cost"`
}

type CostSummary struct {
	TotalCalls      int             `json:"total_calls"`
	SuccessfulCalls int             `json:"successful_calls"`
	FailedCalls     int             `json:"failed_calls"`
	InputTokens     int             `json:"input_tokens"`
	OutputTokens    int             `json:"output_tokens"`
	EstimatedCost   float64         `json:"estimated_cost"`
	MonthlyBudget   float64         `json:"monthly_budget"`
	Currency        string          `json:"currency"`
	ProviderUsage   []ProviderUsage `json:"provider_usage"`
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func DefaultConfig() Config {
	now := time.Now().UTC()
	return Config{
		LLMProvider:         "openai_compatible_primary",
		LLMModel:            "gpt-4o-mini",
		EmbeddingProvider:   "openai_compatible_primary",
		EmbeddingModel:      "text-embedding-3-large",
		RerankProvider:      "openai_compatible_primary",
		RerankModel:         "gpt-4o-mini",
		OCRProvider:         "http_ocr",
		MonthlyBudget:       0,
		Pricing:             map[string]PricingRate{},
		Metadata:            map[string]any{},
		CreatedAt:           now,
		UpdatedAt:           now,
		MockFallbackAllowed: false,
	}
}

func ProviderCatalog() []ProviderOption {
	return []ProviderOption{
		{ProviderKey: "openai_compatible_primary", Name: "主模型服务", Category: "通用模型", Capabilities: []string{"文本理解", "章节生成", "资料匹配"}, SecretKeys: []string{"OPENAI_API_KEY"}},
		{ProviderKey: "cloudflare_ai_gateway", Name: "Cloudflare AI Gateway", Category: "网关聚合", Capabilities: []string{"统一路由", "调用观测", "供应商切换"}, SecretKeys: []string{"CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_API_TOKEN"}},
		{ProviderKey: "deepseek", Name: "DeepSeek", Category: "文本模型", Capabilities: []string{"文件解读", "章节生成", "合规判断"}, SecretKeys: []string{"DEEPSEEK_API_KEY"}},
		{ProviderKey: "dashscope", Name: "通义千问", Category: "文本模型", Capabilities: []string{"文件解读", "章节生成", "资料处理"}, SecretKeys: []string{"DASHSCOPE_API_KEY"}},
		{ProviderKey: "local", Name: "本地处理", Category: "内置能力", Capabilities: []string{"文档切分", "版式导出"}, SecretKeys: []string{}},
		{ProviderKey: "mock", Name: "演练模式", Category: "测试替身", Capabilities: []string{"流程演示"}, SecretKeys: []string{}},
	}
}

func OCRCatalog() []OCROption {
	return []OCROption{
		{ProviderKey: "http_ocr", Name: "HTTP OCR 服务", EndpointKey: "OCR_HTTP_ENDPOINT"},
		{ProviderKey: "mineru", Name: "MinerU", EndpointKey: "MINERU_HTTP_ENDPOINT"},
		{ProviderKey: "paddleocr", Name: "PaddleOCR", EndpointKey: "PADDLEOCR_HTTP_ENDPOINT"},
		{ProviderKey: "local", Name: "本地文本抽取", EndpointKey: ""},
		{ProviderKey: "openai_compatible_primary", Name: "主模型视觉识别", EndpointKey: "OPENAI_BASE_URL"},
		{ProviderKey: "cloudflare_ai_gateway", Name: "Cloudflare 视觉识别", EndpointKey: "CLOUDFLARE_AI_GATEWAY_OPENAI_BASE_URL"},
	}
}

func RouteCatalog() []RouteOption {
	return []RouteOption{
		{TaskType: "tender_parse", Name: "招标文件解读", Capability: "文件理解", Track: "llm"},
		{TaskType: "outline_generate", Name: "目录大纲生成", Capability: "方案规划", Track: "llm"},
		{TaskType: "chapter_generate", Name: "章节生成", Capability: "内容生成", Track: "llm"},
		{TaskType: "chapter_self_check", Name: "章节自检", Capability: "质量校验", Track: "llm"},
		{TaskType: "compliance_check", Name: "合规判断", Capability: "合规语义", Track: "llm"},
		{TaskType: "rewrite_assistant", Name: "改写助手", Capability: "内容改写", Track: "llm"},
		{TaskType: "cost_advice", Name: "成本建议", Capability: "成本分析", Track: "llm"},
		{TaskType: "knowledge_embedding", Name: "资料向量化", Capability: "资料整理", Track: "embedding"},
		{TaskType: "knowledge_rerank", Name: "资料匹配重排", Capability: "资料匹配", Track: "rerank"},
		{TaskType: "document_ocr", Name: "扫描件识别", Capability: "图文识别", Track: "ocr"},
		{TaskType: "knowledge_process", Name: "文档切分", Capability: "资料入库", Track: "local"},
		{TaskType: "document_export", Name: "版式导出", Capability: "版式生成", Track: "local"},
	}
}

func (s *Store) Get(ctx context.Context, tenantID string) (Config, error) {
	config := DefaultConfig()
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		found, err := scanConfig(tx.QueryRow(ctx, `
			select id::text, enabled, llm_provider, llm_model, embedding_provider, embedding_model,
				rerank_provider, rerank_model, ocr_provider, ocr_endpoint, monthly_budget::float8,
				pricing, mock_fallback_allowed, metadata, created_at, updated_at
			from ai_model_configs
			where tenant_id = $1
		`, tenantID))
		if err != nil {
			return err
		}
		config = found
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return config, nil
	}
	return config, err
}

func (s *Store) Upsert(ctx context.Context, tenantID string, req UpsertRequest) (Config, error) {
	normalized, err := NormalizeRequest(req)
	if err != nil {
		return Config{}, err
	}
	pricingRaw, err := marshalPricingJSON(normalized.Pricing)
	if err != nil {
		return Config{}, err
	}
	metadataRaw, err := marshalMetadataJSON(normalized.Metadata)
	if err != nil {
		return Config{}, err
	}
	var config Config
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		created, err := scanConfig(tx.QueryRow(ctx, `
			insert into ai_model_configs (
				tenant_id, enabled, llm_provider, llm_model, embedding_provider, embedding_model,
				rerank_provider, rerank_model, ocr_provider, ocr_endpoint, monthly_budget,
				pricing, mock_fallback_allowed, metadata
			)
			values (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11,
				$12::jsonb, $13, $14::jsonb
			)
			on conflict (tenant_id) do update
			set enabled = excluded.enabled,
				llm_provider = excluded.llm_provider,
				llm_model = excluded.llm_model,
				embedding_provider = excluded.embedding_provider,
				embedding_model = excluded.embedding_model,
				rerank_provider = excluded.rerank_provider,
				rerank_model = excluded.rerank_model,
				ocr_provider = excluded.ocr_provider,
				ocr_endpoint = excluded.ocr_endpoint,
				monthly_budget = excluded.monthly_budget,
				pricing = excluded.pricing,
				mock_fallback_allowed = excluded.mock_fallback_allowed,
				metadata = excluded.metadata,
				updated_at = now()
			returning id::text, enabled, llm_provider, llm_model, embedding_provider, embedding_model,
				rerank_provider, rerank_model, ocr_provider, ocr_endpoint, monthly_budget::float8,
				pricing, mock_fallback_allowed, metadata, created_at, updated_at
		`, tenantID, normalized.Enabled, normalized.LLMProvider, normalized.LLMModel,
			normalized.EmbeddingProvider, normalized.EmbeddingModel, normalized.RerankProvider,
			normalized.RerankModel, normalized.OCRProvider, normalized.OCREndpoint,
			normalized.MonthlyBudget, pricingRaw, normalized.MockFallbackAllowed, metadataRaw))
		if err != nil {
			return err
		}
		config = created
		return nil
	})
	return config, err
}

func (s *Store) CostSummary(ctx context.Context, tenantID string) (CostSummary, error) {
	summary := CostSummary{Currency: "CNY", ProviderUsage: []ProviderUsage{}}
	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var total, success, failed, inputTokens, outputTokens int64
		var estimatedCost float64
		if err := tx.QueryRow(ctx, `
			select
				count(*),
				count(*) filter (where status in ('done', 'success', 'succeeded')),
				count(*) filter (where status in ('failed', 'error')),
				coalesce(sum(input_tokens), 0),
				coalesce(sum(output_tokens), 0),
				coalesce(sum(estimated_cost), 0)::float8
			from ai_call_logs
			where tenant_id = $1
				and created_at >= date_trunc('month', now())
		`, tenantID).Scan(&total, &success, &failed, &inputTokens, &outputTokens, &estimatedCost); err != nil {
			return err
		}
		summary.TotalCalls = int(total)
		summary.SuccessfulCalls = int(success)
		summary.FailedCalls = int(failed)
		summary.InputTokens = int(inputTokens)
		summary.OutputTokens = int(outputTokens)
		summary.EstimatedCost = roundMoney(estimatedCost)
		_ = tx.QueryRow(ctx, `
			select coalesce(monthly_budget, 0)::float8
			from ai_model_configs
			where tenant_id = $1
		`, tenantID).Scan(&summary.MonthlyBudget)
		rows, err := tx.Query(ctx, `
			select provider, model, count(*), coalesce(sum(estimated_cost), 0)::float8
			from ai_call_logs
			where tenant_id = $1
				and created_at >= date_trunc('month', now())
			group by provider, model
			order by coalesce(sum(estimated_cost), 0) desc, count(*) desc
			limit 8
		`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item ProviderUsage
			var calls int64
			if err := rows.Scan(&item.Provider, &item.Model, &calls, &item.EstimatedCost); err != nil {
				return err
			}
			item.Calls = int(calls)
			item.EstimatedCost = roundMoney(item.EstimatedCost)
			summary.ProviderUsage = append(summary.ProviderUsage, item)
		}
		return rows.Err()
	})
	summary.MonthlyBudget = roundMoney(summary.MonthlyBudget)
	return summary, err
}

func NormalizeRequest(req UpsertRequest) (Config, error) {
	defaults := DefaultConfig()
	config := Config{
		Enabled:             req.Enabled,
		LLMProvider:         defaultString(req.LLMProvider, defaults.LLMProvider),
		LLMModel:            defaultString(req.LLMModel, defaults.LLMModel),
		EmbeddingProvider:   defaultString(req.EmbeddingProvider, defaults.EmbeddingProvider),
		EmbeddingModel:      defaultString(req.EmbeddingModel, defaults.EmbeddingModel),
		RerankProvider:      defaultString(req.RerankProvider, defaults.RerankProvider),
		RerankModel:         defaultString(req.RerankModel, defaults.RerankModel),
		OCRProvider:         defaultString(req.OCRProvider, defaults.OCRProvider),
		OCREndpoint:         strings.TrimSpace(req.OCREndpoint),
		MonthlyBudget:       roundMoney(req.MonthlyBudget),
		MockFallbackAllowed: req.MockFallbackAllowed,
		Metadata:            normalizeMetadata(req.Metadata),
	}
	if !validModelProvider(config.LLMProvider) ||
		!validModelProvider(config.EmbeddingProvider) ||
		!validModelProvider(config.RerankProvider) ||
		!validOCRProvider(config.OCRProvider) {
		return Config{}, ErrInvalidRequest
	}
	for _, value := range []string{config.LLMModel, config.EmbeddingModel, config.RerankModel} {
		if !validAIConfigText(value) {
			return Config{}, ErrInvalidRequest
		}
	}
	if !validEndpoint(config.OCREndpoint) ||
		req.MonthlyBudget < 0 ||
		req.MonthlyBudget > maxAIConfigMonthlyBudget {
		return Config{}, ErrInvalidRequest
	}
	pricing, err := NormalizePricing(req.Pricing)
	if err != nil {
		return Config{}, err
	}
	config.Pricing = pricing
	if _, err := marshalMetadataJSON(config.Metadata); err != nil {
		return Config{}, err
	}
	return config, nil
}

func NormalizePricing(pricing map[string]PricingRate) (map[string]PricingRate, error) {
	result := map[string]PricingRate{}
	for rawKey, rawRate := range pricing {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		if !aiPricingKeyPattern.MatchString(key) {
			return nil, ErrInvalidRequest
		}
		rate := PricingRate{
			InputPer1K:   roundMoney(rawRate.InputPer1K),
			OutputPer1K:  roundMoney(rawRate.OutputPer1K),
			InputPer1M:   roundMoney(rawRate.InputPer1M),
			OutputPer1M:  roundMoney(rawRate.OutputPer1M),
			Currency:     normalizeCurrency(rawRate.Currency),
			DisplayName:  truncateText(rawRate.DisplayName, maxAIConfigTextRunes),
			LastReviewed: truncateText(rawRate.LastReviewed, 32),
		}
		if !validRate(rate.InputPer1K) || !validRate(rate.OutputPer1K) || !validRate(rate.InputPer1M) || !validRate(rate.OutputPer1M) {
			return nil, ErrInvalidRequest
		}
		if rate.InputPer1K == 0 && rate.OutputPer1K == 0 && rate.InputPer1M == 0 && rate.OutputPer1M == 0 {
			continue
		}
		if rate.Currency == "" {
			rate.Currency = "CNY"
		}
		result[key] = rate
	}
	raw, err := json.Marshal(result)
	if err != nil || len(raw) > maxAIModelConfigJSONBytes {
		return nil, ErrInvalidRequest
	}
	return result, nil
}

func (s *Store) withTenant(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := platformdb.WithTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func scanConfig(row interface{ Scan(dest ...any) error }) (Config, error) {
	var config Config
	var pricingRaw, metadataRaw []byte
	err := row.Scan(
		&config.ID, &config.Enabled, &config.LLMProvider, &config.LLMModel,
		&config.EmbeddingProvider, &config.EmbeddingModel, &config.RerankProvider,
		&config.RerankModel, &config.OCRProvider, &config.OCREndpoint,
		&config.MonthlyBudget, &pricingRaw, &config.MockFallbackAllowed,
		&metadataRaw, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return Config{}, err
	}
	pricing, err := unmarshalPricingJSON(pricingRaw)
	if err != nil {
		return Config{}, err
	}
	config.Pricing = pricing
	metadata, err := unmarshalMetadataJSON(metadataRaw)
	if err != nil {
		return Config{}, err
	}
	config.Metadata = metadata
	config.MonthlyBudget = roundMoney(config.MonthlyBudget)
	return config, nil
}

func marshalPricingJSON(pricing map[string]PricingRate) ([]byte, error) {
	normalized, err := NormalizePricing(pricing)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil || len(raw) > maxAIModelConfigJSONBytes {
		return nil, ErrInvalidRequest
	}
	return raw, nil
}

func unmarshalPricingJSON(raw []byte) (map[string]PricingRate, error) {
	if len(raw) == 0 {
		return map[string]PricingRate{}, nil
	}
	var pricing map[string]PricingRate
	if err := json.Unmarshal(raw, &pricing); err != nil {
		return nil, err
	}
	if pricing == nil {
		pricing = map[string]PricingRate{}
	}
	return NormalizePricing(pricing)
}

func marshalMetadataJSON(metadata map[string]any) ([]byte, error) {
	normalized := normalizeMetadata(metadata)
	raw, err := json.Marshal(normalized)
	if err != nil || len(raw) > maxAIModelConfigJSONBytes {
		return nil, ErrInvalidRequest
	}
	return raw, nil
}

func unmarshalMetadataJSON(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return normalizeMetadata(metadata), nil
}

func normalizeMetadata(metadata map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range metadata {
		key = truncateText(key, 80)
		if key == "" {
			continue
		}
		switch typed := value.(type) {
		case string:
			result[key] = truncateText(typed, maxAIConfigTextRunes)
		case bool, float64, int, int64, nil:
			result[key] = typed
		}
	}
	return result
}

func validModelProvider(value string) bool {
	switch strings.TrimSpace(value) {
	case "openai_compatible_primary", "cloudflare_ai_gateway", "deepseek", "dashscope", "mock", "local":
		return true
	default:
		return false
	}
}

func validOCRProvider(value string) bool {
	switch strings.TrimSpace(value) {
	case "http_ocr", "mineru", "paddleocr", "local", "openai_compatible_primary", "cloudflare_ai_gateway":
		return true
	default:
		return false
	}
}

func validAIConfigText(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len([]rune(value)) <= maxAIConfigTextRunes && !strings.ContainsAny(value, "\r\n\t")
}

func validEndpoint(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if len([]rune(value)) > maxAIConfigEndpointRunes || strings.ContainsAny(value, "\r\n\t") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https")
}

func validRate(value float64) bool {
	return value >= 0 && value <= maxAIConfigRate && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func roundMoney(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*10000) / 10000
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func truncateText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func normalizeCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if value != "CNY" && value != "USD" {
		return "CNY"
	}
	return value
}

func SortedPricingKeys(pricing map[string]PricingRate) []string {
	keys := make([]string, 0, len(pricing))
	for key := range pricing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func EstimateCostFromPricing(pricing map[string]PricingRate, provider, model string, inputTokens, outputTokens int) (float64, bool) {
	if inputTokens <= 0 && outputTokens <= 0 {
		return 0, false
	}
	for _, key := range pricingLookupKeys(provider, model) {
		rate, ok := pricing[key]
		if !ok {
			continue
		}
		inputPer1K := rate.InputPer1K
		outputPer1K := rate.OutputPer1K
		if inputPer1K == 0 && rate.InputPer1M > 0 {
			inputPer1K = rate.InputPer1M / 1000
		}
		if outputPer1K == 0 && rate.OutputPer1M > 0 {
			outputPer1K = rate.OutputPer1M / 1000
		}
		cost := roundMoney((float64(inputTokens)*inputPer1K + float64(outputTokens)*outputPer1K) / 1000)
		return cost, cost > 0
	}
	return 0, false
}

func pricingLookupKeys(provider, model string) []string {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	lowerProvider := strings.ToLower(provider)
	lowerModel := strings.ToLower(model)
	keys := []string{
		provider + "/" + model,
		model,
		provider + "/*",
		"*",
		lowerProvider + "/" + lowerModel,
		lowerModel,
		lowerProvider + "/*",
	}
	seen := map[string]bool{}
	result := []string{}
	for _, key := range keys {
		if key == "/" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, key)
	}
	return result
}

func ScanTenantPricing(row interface{ Scan(dest ...any) error }) (map[string]PricingRate, error) {
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, err
	}
	return unmarshalPricingJSON(raw)
}

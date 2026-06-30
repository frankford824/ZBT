package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/frankford824/ZBT/backend/internal/platform/aiconfig"
	"github.com/frankford824/ZBT/backend/internal/platform/aihttp"
	"github.com/frankford824/ZBT/backend/internal/platform/tenant"
	"github.com/gin-gonic/gin"
)

type aiConfigOverview struct {
	Config          aiconfig.Config           `json:"config"`
	Runtime         aiRuntimeStatus           `json:"runtime"`
	Summary         aiconfig.CostSummary      `json:"summary"`
	ProviderCatalog []aiconfig.ProviderOption `json:"provider_catalog"`
	OCRCatalog      []aiconfig.OCROption      `json:"ocr_catalog"`
	RouteCatalog    []aiconfig.RouteOption    `json:"route_catalog"`
}

type aiRuntimeStatus struct {
	ServiceReachable     bool              `json:"service_reachable"`
	ServiceStatus        string            `json:"service_status"`
	ProviderHealth       map[string]bool   `json:"provider_health"`
	Active               aiRuntimeModelSet `json:"active"`
	Routes               []aiRuntimeRoute  `json:"routes"`
	Secrets              []aiSecretStatus  `json:"secrets"`
	RuntimePricingKeys   []string          `json:"runtime_pricing_keys"`
	MockFallbackAllowed  bool              `json:"mock_fallback_allowed"`
	MockProvidersEnabled bool              `json:"mock_providers_enabled"`
	SavedConfigEnabled   bool              `json:"saved_config_enabled"`
	SavedConfigActive    bool              `json:"saved_config_active"`
	PendingDeployFields  []string          `json:"pending_deploy_fields"`
	ErrorMessage         string            `json:"error_message"`
	CheckedAt            time.Time         `json:"checked_at"`
}

type aiRuntimeModelSet struct {
	LLMProvider       string `json:"llm_provider"`
	LLMModel          string `json:"llm_model"`
	EmbeddingProvider string `json:"embedding_provider"`
	EmbeddingModel    string `json:"embedding_model"`
	RerankProvider    string `json:"rerank_provider"`
	RerankModel       string `json:"rerank_model"`
	OCRProvider       string `json:"ocr_provider"`
	OCREndpoint       string `json:"ocr_endpoint"`
}

type aiRuntimeRoute struct {
	TaskType       string `json:"task_type"`
	Name           string `json:"name"`
	Capability     string `json:"capability"`
	Track          string `json:"track"`
	ActiveProvider string `json:"active_provider"`
	ActiveModel    string `json:"active_model"`
	SavedProvider  string `json:"saved_provider"`
	SavedModel     string `json:"saved_model"`
}

type aiSecretStatus struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
}

type aiConfigCheckResult struct {
	Runtime aiRuntimeStatus `json:"runtime"`
	Checks  []aiConfigCheck `json:"checks"`
}

type aiConfigCheck struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type aiRuntimeSnapshot struct {
	Status               string            `json:"status"`
	Providers            map[string]bool   `json:"providers"`
	Active               aiRuntimeModelSet `json:"active"`
	Secrets              []aiSecretStatus  `json:"secrets"`
	RuntimePricingKeys   []string          `json:"runtime_pricing_keys"`
	MockFallbackAllowed  bool              `json:"mock_fallback_allowed"`
	MockProvidersEnabled bool              `json:"mock_providers_enabled"`
	CheckedAt            time.Time         `json:"checked_at"`
}

func (s *server) getAIConfig(c *gin.Context) {
	overview, err := s.aiConfigOverview(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, overview, err)
}

func (s *server) upsertAIConfig(c *gin.Context) {
	var req aiconfig.UpsertRequest
	if !bindJSON(c, &req) {
		return
	}
	if _, err := s.aiConfigStore.Upsert(c.Request.Context(), tenant.FromContext(c.Request.Context()), req); err != nil {
		respond(c, nil, err)
		return
	}
	overview, err := s.aiConfigOverview(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	respond(c, overview, err)
}

func (s *server) checkAIConfig(c *gin.Context) {
	config, err := s.aiConfigStore.Get(c.Request.Context(), tenant.FromContext(c.Request.Context()))
	if err != nil {
		respond(c, nil, err)
		return
	}
	runtime := s.aiRuntimeStatus(c.Request.Context(), config)
	respond(c, aiConfigCheckResult{Runtime: runtime, Checks: aiReadinessChecks(config, runtime)}, nil)
}

func (s *server) aiConfigOverview(ctx context.Context, tenantID string) (aiConfigOverview, error) {
	config, err := s.aiConfigStore.Get(ctx, tenantID)
	if err != nil {
		return aiConfigOverview{}, err
	}
	summary, err := s.aiConfigStore.CostSummary(ctx, tenantID)
	if err != nil {
		return aiConfigOverview{}, err
	}
	return aiConfigOverview{
		Config:          config,
		Runtime:         s.aiRuntimeStatus(ctx, config),
		Summary:         summary,
		ProviderCatalog: aiconfig.ProviderCatalog(),
		OCRCatalog:      aiconfig.OCRCatalog(),
		RouteCatalog:    aiconfig.RouteCatalog(),
	}, nil
}

func (s *server) aiRuntimeStatus(ctx context.Context, config aiconfig.Config) aiRuntimeStatus {
	snapshot, healthErr := s.fetchAIRuntimeSnapshot(ctx)
	if snapshot.Status == "" {
		snapshot.Status = "ok"
	}
	if snapshot.Providers == nil {
		snapshot.Providers = map[string]bool{}
	}
	if snapshot.Secrets == nil {
		snapshot.Secrets = []aiSecretStatus{}
	}
	if snapshot.RuntimePricingKeys == nil {
		snapshot.RuntimePricingKeys = []string{}
	}
	if snapshot.CheckedAt.IsZero() {
		snapshot.CheckedAt = time.Now().UTC()
	}
	pending := aiPendingDeployFields(config, snapshot.Active)
	return aiRuntimeStatus{
		ServiceReachable:     healthErr == nil,
		ServiceStatus:        snapshot.Status,
		ProviderHealth:       snapshot.Providers,
		Active:               snapshot.Active,
		Routes:               aiRuntimeRoutes(config, snapshot.Active),
		Secrets:              snapshot.Secrets,
		RuntimePricingKeys:   snapshot.RuntimePricingKeys,
		MockFallbackAllowed:  snapshot.MockFallbackAllowed,
		MockProvidersEnabled: snapshot.MockProvidersEnabled,
		SavedConfigEnabled:   config.Enabled,
		SavedConfigActive:    config.Enabled && len(pending) == 0,
		PendingDeployFields:  pending,
		ErrorMessage:         healthErrorMessage(healthErr),
		CheckedAt:            snapshot.CheckedAt,
	}
}

func (s *server) fetchAIRuntimeSnapshot(ctx context.Context) (aiRuntimeSnapshot, error) {
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	url := strings.TrimRight(s.cfg.AIServiceURL, "/") + "/models/health"
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, url, nil)
	if err != nil {
		return aiRuntimeSnapshot{Status: "unreachable"}, err
	}
	aihttp.Sign(req, nil, s.cfg.AIServiceHMACSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return aiRuntimeSnapshot{Status: "unreachable"}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aiRuntimeSnapshot{Status: "unreachable"}, fmt.Errorf("AI service returned HTTP %d", resp.StatusCode)
	}
	var payload aiRuntimeSnapshot
	if err := aihttp.DecodeJSONLimit(resp.Body, &payload, 1024*1024); err != nil {
		return aiRuntimeSnapshot{Status: "unreachable"}, err
	}
	if payload.Status == "" {
		payload.Status = "ok"
	}
	return payload, nil
}

func aiPendingDeployFields(config aiconfig.Config, active aiRuntimeModelSet) []string {
	if !config.Enabled {
		return []string{}
	}
	fields := []string{}
	compare := func(label, saved, running string) {
		if strings.TrimSpace(saved) != strings.TrimSpace(running) {
			fields = append(fields, label)
		}
	}
	compare("文件理解模型", config.LLMProvider+"/"+config.LLMModel, active.LLMProvider+"/"+active.LLMModel)
	compare("资料整理模型", config.EmbeddingProvider+"/"+config.EmbeddingModel, active.EmbeddingProvider+"/"+active.EmbeddingModel)
	compare("资料匹配模型", config.RerankProvider+"/"+config.RerankModel, active.RerankProvider+"/"+active.RerankModel)
	compare("OCR 服务", config.OCRProvider, active.OCRProvider)
	if strings.TrimSpace(config.OCREndpoint) != "" {
		compare("OCR 地址", config.OCREndpoint, active.OCREndpoint)
	}
	sort.Strings(fields)
	return fields
}

func aiRuntimeRoutes(config aiconfig.Config, active aiRuntimeModelSet) []aiRuntimeRoute {
	routes := make([]aiRuntimeRoute, 0, len(aiconfig.RouteCatalog()))
	for _, item := range aiconfig.RouteCatalog() {
		route := aiRuntimeRoute{
			TaskType:   item.TaskType,
			Name:       item.Name,
			Capability: item.Capability,
			Track:      item.Track,
		}
		switch item.Track {
		case "llm":
			route.ActiveProvider, route.ActiveModel = active.LLMProvider, active.LLMModel
			route.SavedProvider, route.SavedModel = config.LLMProvider, config.LLMModel
		case "embedding":
			route.ActiveProvider, route.ActiveModel = active.EmbeddingProvider, active.EmbeddingModel
			route.SavedProvider, route.SavedModel = config.EmbeddingProvider, config.EmbeddingModel
		case "rerank":
			route.ActiveProvider, route.ActiveModel = active.RerankProvider, active.RerankModel
			route.SavedProvider, route.SavedModel = config.RerankProvider, config.RerankModel
		case "ocr":
			route.ActiveProvider, route.ActiveModel = active.OCRProvider, "OCR"
			route.SavedProvider, route.SavedModel = config.OCRProvider, "OCR"
		default:
			route.ActiveProvider, route.ActiveModel = "local", item.Name
			route.SavedProvider, route.SavedModel = "local", item.Name
		}
		routes = append(routes, route)
	}
	return routes
}

func aiReadinessChecks(config aiconfig.Config, runtime aiRuntimeStatus) []aiConfigCheck {
	checks := []aiConfigCheck{}
	if runtime.ServiceReachable {
		checks = append(checks, aiConfigCheck{Key: "ai_service", Name: "智能服务连接", Status: "passed", Message: "服务可达"})
	} else {
		checks = append(checks, aiConfigCheck{Key: "ai_service", Name: "智能服务连接", Status: "failed", Message: "服务暂不可达"})
	}
	checks = append(checks, providerCheck("llm_provider", "文件理解模型", config.LLMProvider, runtime))
	checks = append(checks, providerCheck("embedding_provider", "资料整理模型", config.EmbeddingProvider, runtime))
	checks = append(checks, providerCheck("rerank_provider", "资料匹配模型", config.RerankProvider, runtime))
	checks = append(checks, ocrCheck(config, runtime))
	if len(config.Pricing) > 0 || len(runtime.RuntimePricingKeys) > 0 {
		checks = append(checks, aiConfigCheck{Key: "pricing", Name: "费用估算", Status: "passed", Message: "已配置单价"})
	} else {
		checks = append(checks, aiConfigCheck{Key: "pricing", Name: "费用估算", Status: "warning", Message: "缺少单价时费用会显示为 0"})
	}
	if config.MockFallbackAllowed || runtime.MockFallbackAllowed || runtime.MockProvidersEnabled {
		checks = append(checks, aiConfigCheck{Key: "fallback", Name: "演练回退", Status: "warning", Message: "仍允许演练模式回退"})
	} else {
		checks = append(checks, aiConfigCheck{Key: "fallback", Name: "演练回退", Status: "passed", Message: "生产回退已关闭"})
	}
	if config.Enabled && len(runtime.PendingDeployFields) > 0 {
		checks = append(checks, aiConfigCheck{Key: "deploy", Name: "保存配置生效", Status: "warning", Message: "保存配置与运行配置不一致"})
	} else if config.Enabled {
		checks = append(checks, aiConfigCheck{Key: "deploy", Name: "保存配置生效", Status: "passed", Message: "保存配置与运行配置一致"})
	} else {
		checks = append(checks, aiConfigCheck{Key: "deploy", Name: "保存配置生效", Status: "warning", Message: "保存配置尚未启用"})
	}
	return checks
}

func providerCheck(key, name, provider string, runtime aiRuntimeStatus) aiConfigCheck {
	if provider == "local" {
		return aiConfigCheck{Key: key, Name: name, Status: "passed", Message: "使用本地处理"}
	}
	if provider == "mock" {
		return aiConfigCheck{Key: key, Name: name, Status: "warning", Message: "当前选择演练模式"}
	}
	if !secretReadyForProvider(provider, runtime.Secrets) {
		return aiConfigCheck{Key: key, Name: name, Status: "failed", Message: "缺少授权配置"}
	}
	if ok, exists := runtime.ProviderHealth[provider]; exists && !ok {
		return aiConfigCheck{Key: key, Name: name, Status: "failed", Message: "供应商不可用"}
	}
	return aiConfigCheck{Key: key, Name: name, Status: "passed", Message: "授权项已配置"}
}

func ocrCheck(config aiconfig.Config, runtime aiRuntimeStatus) aiConfigCheck {
	if config.OCRProvider == "local" {
		return aiConfigCheck{Key: "ocr", Name: "扫描件识别", Status: "warning", Message: "本地抽取无法覆盖扫描件"}
	}
	endpoint := strings.TrimSpace(config.OCREndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(runtime.Active.OCREndpoint)
	}
	if endpoint == "" && config.OCRProvider != "openai_compatible_primary" && config.OCRProvider != "cloudflare_ai_gateway" {
		return aiConfigCheck{Key: "ocr", Name: "扫描件识别", Status: "failed", Message: "缺少识别服务地址"}
	}
	if config.OCRProvider == "openai_compatible_primary" || config.OCRProvider == "cloudflare_ai_gateway" {
		return providerCheck("ocr", "扫描件识别", config.OCRProvider, runtime)
	}
	if ok, exists := runtime.ProviderHealth[config.OCRProvider]; exists && !ok {
		return aiConfigCheck{Key: "ocr", Name: "扫描件识别", Status: "failed", Message: "识别服务不可用"}
	}
	return aiConfigCheck{Key: "ocr", Name: "扫描件识别", Status: "passed", Message: "基础配置已就绪"}
}

func secretReadyForProvider(provider string, secrets []aiSecretStatus) bool {
	if provider == "local" || provider == "mock" {
		return true
	}
	if provider == "cloudflare_ai_gateway" {
		return secretConfigured(secrets, "CLOUDFLARE_API_TOKEN") && secretConfigured(secrets, "CLOUDFLARE_ACCOUNT_ID")
	}
	required := map[string][]string{
		"openai_compatible_primary": {"OPENAI_API_KEY"},
		"deepseek":                  {"DEEPSEEK_API_KEY"},
		"dashscope":                 {"DASHSCOPE_API_KEY"},
	}
	needed := required[provider]
	if len(needed) == 0 {
		return false
	}
	for _, key := range needed {
		found := false
		for _, secret := range secrets {
			if secret.Key == key && secret.Configured {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func secretConfigured(secrets []aiSecretStatus, key string) bool {
	for _, secret := range secrets {
		if secret.Key == key && secret.Configured {
			return true
		}
	}
	return false
}

func healthErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return "智能服务暂不可达"
}

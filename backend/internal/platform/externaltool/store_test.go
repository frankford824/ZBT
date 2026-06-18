package externaltool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeConfigRequiresHTTPStreamableEndpoint(t *testing.T) {
	enabled := true
	cfg, err := normalizeConfig("Handaas_Bidding", UpsertConfigRequest{
		Name:         "招投标数据",
		Endpoint:     "https://example.com/mcp",
		Enabled:      &enabled,
		AllowedTools: []string{"bid_bigdata_bid_search", "bid_bigdata_bid_search", "bid_bigdata_bid_win_stats"},
		TimeoutMS:    3000,
		Metadata:     map[string]any{"cost_per_call": 0.01},
	})
	if err != nil {
		t.Fatalf("normalize config: %v", err)
	}
	if cfg.ProviderKey != "handaas-bidding" || !cfg.Enabled || cfg.Transport != TransportStreamableHTTP {
		t.Fatalf("unexpected normalized config: %+v", cfg)
	}
	if len(cfg.AllowedTools) != 2 ||
		!toolAllowed(cfg.AllowedTools, "bid_bigdata_bid_search") ||
		!toolAllowed(cfg.AllowedTools, "bid_bigdata_bid_win_stats") {
		t.Fatalf("expected sorted unique allowed tools, got %#v", cfg.AllowedTools)
	}
	if _, err := normalizeConfig("bad", UpsertConfigRequest{Endpoint: "file:///tmp/mcp.sock"}); err == nil {
		t.Fatal("expected non-http endpoint to be rejected")
	}
	if _, err := normalizeConfig("bad", UpsertConfigRequest{Enabled: &enabled}); err == nil {
		t.Fatal("expected enabled external tool to require an endpoint")
	}
}

func TestNormalizeConfigNormalizesCostMetadata(t *testing.T) {
	cfg, err := normalizeConfig("handaas-bidding", UpsertConfigRequest{
		Endpoint:      "https://example.com/mcp",
		MonthlyBudget: 100.123456,
		Metadata: map[string]any{
			"cost_per_call": "12.34567",
			"token":         "raw-secret",
			"notes":         "should not be stored",
		},
	})
	if err != nil {
		t.Fatalf("normalize config: %v", err)
	}
	if cfg.MonthlyBudget != 100.1235 {
		t.Fatalf("expected rounded monthly budget, got %.8f", cfg.MonthlyBudget)
	}
	if got := cfg.Metadata["cost_per_call"]; got != 12.3457 {
		t.Fatalf("expected rounded cost_per_call metadata, got %#v", got)
	}
	if _, ok := cfg.Metadata["token"]; ok {
		t.Fatalf("expected unknown metadata keys to be dropped, got %#v", cfg.Metadata)
	}
	if len(cfg.Metadata) != 1 {
		t.Fatalf("expected only governed metadata keys, got %#v", cfg.Metadata)
	}
}

func TestNormalizeConfigRejectsInvalidExternalToolMoney(t *testing.T) {
	for _, req := range []UpsertConfigRequest{
		{Endpoint: "https://example.com/mcp", MonthlyBudget: -1},
		{Endpoint: "https://example.com/mcp", MonthlyBudget: math.Inf(1)},
		{Endpoint: "https://example.com/mcp", MonthlyBudget: maxExternalToolMonthlyBudget + 0.0001},
		{Endpoint: "https://example.com/mcp", Metadata: map[string]any{"cost_per_call": -0.01}},
		{Endpoint: "https://example.com/mcp", Metadata: map[string]any{"cost_per_call": math.Inf(1)}},
		{Endpoint: "https://example.com/mcp", Metadata: map[string]any{"cost_per_call": "Infinity"}},
		{Endpoint: "https://example.com/mcp", Metadata: map[string]any{"cost_per_call": "not-a-number"}},
		{Endpoint: "https://example.com/mcp", Metadata: map[string]any{"cost_per_call": maxExternalToolCostPerCall + 0.0001}},
		{Endpoint: "https://example.com/mcp", Metadata: map[string]any{"cost_per_call": map[string]any{"value": 1}}},
	} {
		if _, err := normalizeConfig("handaas-bidding", req); err == nil {
			t.Fatalf("expected invalid money request to be rejected: %#v", req)
		}
	}
}

func TestMarshalExternalToolMetadataJSONRejectsInvalidAndOversizedValues(t *testing.T) {
	for name, value := range map[string]map[string]any{
		"invalid number": {"bad": math.NaN()},
		"unsupported":    {"bad": func() {}},
		"oversized":      {"payload": strings.Repeat("审", maxExternalToolAuditMetadataJSONBytes)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := marshalExternalToolMetadataJSON(value, maxExternalToolAuditMetadataJSONBytes); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("expected invalid external tool metadata to be rejected, got %v", err)
			}
		})
	}
	if raw, err := marshalExternalToolMetadataJSON(nil, maxExternalToolAuditMetadataJSONBytes); err != nil || string(raw) != "{}" {
		t.Fatalf("expected nil metadata to normalize to empty JSON, raw=%q err=%v", raw, err)
	}
}

func TestNormalizeConfigRejectsUnsafeExternalEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"http://localhost:9000/mcp",
		"http://127.0.0.1:9000/mcp",
		"http://10.0.0.8/mcp",
		"http://172.16.0.8/mcp",
		"http://192.168.1.20/mcp",
		"http://[::1]/mcp",
		"http://minio:9000/mcp",
		"http://metadata.local/mcp",
		"http://100.64.0.1/mcp",
		"http://192.0.2.1/mcp",
		"http://198.18.0.1/mcp",
		"http://198.51.100.1/mcp",
		"http://203.0.113.1/mcp",
		"http://240.0.0.1/mcp",
		"http://[2001:db8::1]/mcp",
		"https://user:pass@example.com/mcp",
		"https://example.com@127.0.0.1/mcp",
		"https://example.com/mcp?token=secret",
		"https://example.com/mcp?api_key=secret",
		"https://example.com/mcp?access_token=secret",
		"//example.com/mcp",
	} {
		if _, err := normalizeConfig("handaas-bidding", UpsertConfigRequest{Endpoint: endpoint}); err == nil {
			t.Fatalf("expected unsafe external endpoint %q to be rejected", endpoint)
		}
	}
}

func TestCostPerCallIgnoresInvalidStoredMetadata(t *testing.T) {
	for _, metadata := range []map[string]any{
		{"cost_per_call": math.Inf(1)},
		{"cost_per_call": "1e999"},
		{"cost_per_call": "not-a-number"},
		{"cost_per_call": maxExternalToolCostPerCall + 0.0001},
		{"cost_per_call": []any{1}},
	} {
		if got := costPerCall(metadata); got != 0 {
			t.Fatalf("expected invalid stored cost metadata to be ignored, got %.8f for %#v", got, metadata)
		}
	}
	if got := costPerCall(map[string]any{"cost_per_call": "0.123456"}); got != 0.1235 {
		t.Fatalf("expected valid stored cost metadata to be rounded, got %.8f", got)
	}
}

func TestNormalizeInvokeRequestRejectsOversizedAndNonJSONArguments(t *testing.T) {
	normalized, err := normalizeInvokeRequest(InvokeRequest{
		ToolName:     " bid_search ",
		Arguments:    map[string]any{"keyword": "智慧交通"},
		ResourceType: " bid ",
	})
	if err != nil {
		t.Fatalf("normalize invoke request: %v", err)
	}
	if normalized.ToolName != "bid_search" || normalized.ResourceType != "bid" {
		t.Fatalf("expected trimmed invoke request, got %+v", normalized)
	}
	if normalized.Arguments["keyword"] != "智慧交通" {
		t.Fatalf("expected arguments to be preserved, got %#v", normalized.Arguments)
	}
	for name, req := range map[string]InvokeRequest{
		"oversized arguments": {
			ToolName:  "bid_search",
			Arguments: map[string]any{"payload": strings.Repeat("招", maxExternalToolArgumentsJSONBytes)},
		},
		"non json arguments": {
			ToolName:  "bid_search",
			Arguments: map[string]any{"bad": func() {}},
		},
		"invalid number arguments": {
			ToolName:  "bid_search",
			Arguments: map[string]any{"bad": math.Inf(1)},
		},
		"oversized resource type": {
			ToolName:     "bid_search",
			ResourceType: strings.Repeat("类", maxExternalToolResourceTypeRunes+1),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeInvokeRequest(req); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("expected invalid request, got %v", err)
			}
		})
	}
}

func TestMarshalExternalToolArgumentsJSONAndRequestHashRejectInvalidValues(t *testing.T) {
	raw, err := marshalExternalToolArgumentsJSON(nil)
	if err != nil || string(raw) != "{}" {
		t.Fatalf("expected nil arguments to normalize to empty JSON, raw=%q err=%v", raw, err)
	}
	hash, err := requestHash(nil)
	if err != nil {
		t.Fatalf("expected nil arguments to hash, got %v", err)
	}
	if len(hash) != 64 {
		t.Fatalf("expected sha256 request hash, got %q", hash)
	}
	for name, arguments := range map[string]map[string]any{
		"invalid number": {"bad": math.Inf(-1)},
		"unsupported":    {"bad": func() {}},
		"oversized":      {"payload": strings.Repeat("参", maxExternalToolArgumentsJSONBytes)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := marshalExternalToolArgumentsJSON(arguments); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("expected invalid arguments JSON to be rejected, got %v", err)
			}
			if _, err := requestHash(arguments); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("expected invalid request hash arguments to be rejected, got %v", err)
			}
		})
	}
}

func TestExternalToolHTTPClientRejectsLocalhostDial(t *testing.T) {
	var received atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer server.Close()

	client := newExternalToolHTTPClient()
	resp, err := client.Get(server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected guarded external tool client to reject localhost endpoint")
	}
	if received.Load() {
		t.Fatal("guarded external tool client reached localhost handler")
	}
}

func TestProviderPresetsIncludeGovernedIndustryMCPs(t *testing.T) {
	presets := ProviderPresets()
	keys := map[string]bool{}
	for _, preset := range presets {
		keys[preset.ProviderKey] = true
		if !preset.ReadOnly {
			t.Fatalf("external provider preset must be read-only: %+v", preset)
		}
		if preset.Transport != TransportStreamableHTTP {
			t.Fatalf("unexpected preset transport: %+v", preset)
		}
		if len(preset.DefaultAllowedTools) == 0 {
			t.Fatalf("expected default tool allowlist for preset: %+v", preset)
		}
	}
	for _, key := range []string{"handaas-bidding", "autorfp", "qlows", "bidcraft-compliance", "bidcraft-win-strategy", "loopio"} {
		if !keys[key] {
			t.Fatalf("expected provider preset %s", key)
		}
	}
}

func TestKnownProviderAppliesDefaultsAndRejectsUnsafeConfig(t *testing.T) {
	cfg, err := normalizeConfig("handaas-bidding", UpsertConfigRequest{
		Endpoint: "https://example.com/mcp",
	})
	if err != nil {
		t.Fatalf("normalize known provider: %v", err)
	}
	if cfg.Name != "旷湖招投标大数据" {
		t.Fatalf("expected preset name, got %q", cfg.Name)
	}
	if !toolAllowed(cfg.AllowedTools, "bid_bigdata_bid_search") {
		t.Fatalf("expected preset allowlist, got %#v", cfg.AllowedTools)
	}
	if _, err := normalizeConfig("handaas-bidding", UpsertConfigRequest{
		Endpoint:        "https://example.com/mcp",
		AllowedTools:    []string{"upload_full_tender_document"},
		RedactionPolicy: "summary_only",
	}); err == nil {
		t.Fatal("expected strict preset to reject unknown tools")
	}
	if _, err := normalizeConfig("handaas-bidding", UpsertConfigRequest{
		Endpoint:        "https://example.com/mcp",
		RedactionPolicy: "disabled",
	}); err == nil {
		t.Fatal("expected known provider to reject disabled redaction")
	}
}

func TestSummarizeArgumentsDoesNotPersistRawValues(t *testing.T) {
	summary := summarizeArguments(map[string]any{
		"keyword": "某客户完整招标文件原文",
		"filters": map[string]any{
			"region": "浙江",
		},
		"limit": 10,
	})
	if summary == "" {
		t.Fatal("expected request summary")
	}
	if summary == "keyword=某客户完整招标文件原文" || strings.Contains(summary, "某客户完整招标文件原文") {
		t.Fatalf("summary leaked raw sensitive text: %s", summary)
	}
	if !strings.Contains(summary, "keyword=string") || !strings.Contains(summary, "filters=object") {
		t.Fatalf("expected structural summary, got %s", summary)
	}
}

func TestSummarizeValueDoesNotPersistRawExternalResponse(t *testing.T) {
	summary := summarizeValue(map[string]any{
		"items": []any{
			map[string]any{
				"title":  "某客户机密采购项目",
				"phone":  "13800138000",
				"email":  "buyer@example.com",
				"amount": 9900000,
			},
		},
		"next_token": "raw-page-token",
	})
	for _, leaked := range []string{"某客户机密采购项目", "13800138000", "buyer@example.com", "raw-page-token", "9900000"} {
		if strings.Contains(summary, leaked) {
			t.Fatalf("response summary leaked raw value %q: %s", leaked, summary)
		}
	}
	for _, expected := range []string{`"items"`, `"type":"array"`, `"count":1`, `"next_token"`} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected structural response summary to contain %s, got %s", expected, summary)
		}
	}
}

func TestSafeErrorRedactsExternalEndpointAndSecrets(t *testing.T) {
	message := safeError(errString(`Post "https://api.example.com/mcp?token=secret-token": external tool error: api_key=raw-key failed`))
	for _, leaked := range []string{"https://api.example.com", "secret-token", "raw-key"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("safe error leaked %q: %s", leaked, message)
		}
	}
	if !strings.Contains(message, "[external-endpoint]") || !strings.Contains(message, "api_key=[redacted]") {
		t.Fatalf("expected redacted endpoint and secret marker, got %s", message)
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}

func TestCallStreamableHTTPUsesMCPToolsCallEnvelope(t *testing.T) {
	t.Setenv("EXTERNAL_TOOL_HANDAAS_BIDDING_TOKEN", "secret-token")
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("expected authorization header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected JSON content type, got %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"ok","result":{"items":[{"title":"公告"}]}}`))
	}))
	defer server.Close()

	store := &Store{client: server.Client()}
	result, metadata, err := store.callStreamableHTTP(context.Background(), Config{
		ProviderKey: "handaas-bidding",
		Endpoint:    server.URL,
		TimeoutMS:   2000,
	}, InvokeRequest{
		ToolName:  "bid_search",
		Arguments: map[string]any{"keyword": "智慧交通"},
	})
	if err != nil {
		t.Fatalf("call streamable http: %v", err)
	}
	if metadata["http_status"] != 200 {
		t.Fatalf("expected http status metadata, got %#v", metadata)
	}
	if captured["method"] != "tools/call" {
		t.Fatalf("expected MCP tools/call method, got %#v", captured)
	}
	params := captured["params"].(map[string]any)
	if params["name"] != "bid_search" {
		t.Fatalf("expected tool name in params, got %#v", params)
	}
	resultMap := result.(map[string]any)
	items := resultMap["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected result payload, got %#v", result)
	}
}

func TestCallStreamableHTTPRejectsNonJSONArgumentsBeforeOutboundRequest(t *testing.T) {
	var received atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer server.Close()

	store := &Store{client: server.Client()}
	_, _, err := store.callStreamableHTTP(context.Background(), Config{
		ProviderKey: "bad-json",
		Endpoint:    server.URL,
		TimeoutMS:   2000,
	}, InvokeRequest{ToolName: "bid_search", Arguments: map[string]any{"bad": func() {}}})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
	if received.Load() {
		t.Fatal("non-json arguments should be rejected before outbound request")
	}
}

func TestCallStreamableHTTPRejectsOversizedResponseWithoutLeakingBody(t *testing.T) {
	secretFragment := []byte("secret external tool response body")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bytes.Repeat(secretFragment, maxExternalToolResponseBytes/len(secretFragment)+1))
	}))
	defer server.Close()

	store := &Store{client: server.Client()}
	_, metadata, err := store.callStreamableHTTP(context.Background(), Config{
		ProviderKey: "oversized-response",
		Endpoint:    server.URL,
		TimeoutMS:   2000,
	}, InvokeRequest{ToolName: "bid_search", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("expected oversized response error")
	}
	if metadata["http_status"] != 200 {
		t.Fatalf("expected http status metadata, got %#v", metadata)
	}
	message := err.Error()
	if strings.Contains(message, string(secretFragment)) {
		t.Fatalf("oversized response error leaked body fragment: %s", message)
	}
	if !strings.Contains(message, "too large") {
		t.Fatalf("expected controlled oversized response error, got %s", message)
	}
}

func TestCallStreamableHTTPHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer server.Close()

	store := &Store{client: server.Client()}
	_, _, err := store.callStreamableHTTP(context.Background(), Config{
		ProviderKey: "slow",
		Endpoint:    server.URL,
		TimeoutMS:   1,
	}, InvokeRequest{ToolName: "bid_search", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

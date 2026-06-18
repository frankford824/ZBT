package externaltool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

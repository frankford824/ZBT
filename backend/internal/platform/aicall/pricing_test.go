package aicall

import (
	"database/sql"
	"testing"
	"time"
)

func TestEstimateCostUsesConfiguredProviderModelPricing(t *testing.T) {
	t.Setenv("AI_MODEL_PRICING_JSON", `{
		"deepseek/deepseek-chat": {"input_per_1k": 0.001, "output_per_1k": 0.002}
	}`)

	cost := estimateCost("deepseek", "deepseek-chat", 1000, 500)
	if cost != 0.002 {
		t.Fatalf("expected configured provider/model cost 0.002, got %.6f", cost)
	}
}

func TestEstimateCostSupportsProviderWildcardAndPerMillionRates(t *testing.T) {
	t.Setenv("AI_MODEL_PRICING_JSON", `{
		"openai_compatible_primary/*": {"input_per_1m": 2, "output_per_1m": 8}
	}`)

	cost := estimateCost("openai_compatible_primary", "custom-model", 1000, 1000)
	if cost != 0.01 {
		t.Fatalf("expected wildcard per-million cost 0.01, got %.6f", cost)
	}
}

func TestEstimateCostReturnsZeroWhenPricingMissing(t *testing.T) {
	t.Setenv("AI_MODEL_PRICING_JSON", `{}`)

	if cost := estimateCost("mock", "mock-model", 1000, 1000); cost != 0 {
		t.Fatalf("expected missing pricing to keep zero cost, got %.6f", cost)
	}
}

func TestRecordFromTaskUsesModelMetadataFallbackAndPricing(t *testing.T) {
	t.Setenv("AI_MODEL_PRICING_JSON", `{
		"mock/fallback-json-model": {"input_per_1k": 0.001, "output_per_1k": 0.002}
	}`)

	input := recordFromTask(
		"tenant-1",
		"external-task-1",
		"local-task-1",
		sql.NullString{String: "user-1", Valid: true},
		"tender_parse",
		"done",
		"bid_parse_result",
		"resource-1",
		map[string]any{"provider": "mock", "model": "fallback-json-model"},
		map[string]any{
			"trace_id": "trace-1",
			"model_metadata": map[string]any{
				"provider":      "mock",
				"model":         "fallback-json-model",
				"fallback_from": "deepseek_primary",
			},
			"token_usage": map[string]any{
				"input_tokens":  1000,
				"output_tokens": 500,
			},
		},
		"",
		time.Unix(100, 0),
		sql.NullTime{Time: time.Unix(103, 0), Valid: true},
	)
	input = normalizeRecord(input)

	if input.FallbackFrom != "deepseek_primary" {
		t.Fatalf("expected fallback source from model metadata, got %q", input.FallbackFrom)
	}
	if input.TraceID != "trace-1" {
		t.Fatalf("expected trace id from task result, got %q", input.TraceID)
	}
	if input.LatencyMS != 3000 {
		t.Fatalf("expected callback latency, got %d", input.LatencyMS)
	}
	if input.EstimatedCost != 0.002 {
		t.Fatalf("expected estimated cost 0.002, got %.6f", input.EstimatedCost)
	}
}

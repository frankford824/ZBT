package aicall

import (
	"database/sql"
	"math"
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

func TestEstimateCostSanitizesInvalidPricingRates(t *testing.T) {
	t.Setenv("AI_MODEL_PRICING_JSON", `{
		"bad/model": {"input_per_1k": -5, "output_per_1k": -9},
		"partial/model": {"input_per_1k": -5, "output_per_1k": 0.002}
	}`)

	if cost := estimateCost("bad", "model", 1000, 1000); cost != 0 {
		t.Fatalf("expected negative pricing rates to be ignored, got %.6f", cost)
	}
	if cost := estimateCost("partial", "model", 1000, 1000); cost != 0.002 {
		t.Fatalf("expected positive output rate to be retained, got %.6f", cost)
	}
}

func TestNormalizeRecordSanitizesInvalidExplicitCostAndFallsBackToPricing(t *testing.T) {
	t.Setenv("AI_MODEL_PRICING_JSON", `{
		"deepseek/deepseek-chat": {"input_per_1k": 0.001, "output_per_1k": 0.002}
	}`)

	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		input := normalizeRecord(RecordInput{
			TraceID:       "trace-1",
			TaskType:      "chapter_generate",
			Provider:      "deepseek",
			Model:         "deepseek-chat",
			InputTokens:   1000,
			OutputTokens:  500,
			EstimatedCost: value,
			Status:        "done",
		})
		if input.EstimatedCost != 0.002 {
			t.Fatalf("expected invalid explicit cost %v to fall back to pricing, got %.6f", value, input.EstimatedCost)
		}
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

func TestRecordFromTaskParsesStringTokenAndCostFields(t *testing.T) {
	input := recordFromTask(
		"tenant-1",
		"external-task-1",
		"local-task-1",
		sql.NullString{},
		"knowledge_embedding",
		"done",
		"knowledge_document",
		"resource-1",
		map[string]any{"provider": "openai_compatible_primary", "model": "embedding-model"},
		map[string]any{
			"model_metadata": map[string]any{
				"provider":       "openai_compatible_primary",
				"model":          "embedding-model",
				"estimated_cost": "0.0125",
			},
			"token_usage": map[string]any{
				"input_tokens":  "1200",
				"output_tokens": "0",
			},
		},
		"",
		time.Unix(100, 0),
		sql.NullTime{Time: time.Unix(103, 0), Valid: true},
	)
	input = normalizeRecord(input)

	if input.InputTokens != 1200 {
		t.Fatalf("expected string input tokens to parse, got %d", input.InputTokens)
	}
	if input.OutputTokens != 0 {
		t.Fatalf("expected string output tokens to parse, got %d", input.OutputTokens)
	}
	if input.EstimatedCost != 0.0125 {
		t.Fatalf("expected explicit string estimated cost, got %.6f", input.EstimatedCost)
	}
}

func TestRecordFromTaskReadsTopLevelEstimatedCost(t *testing.T) {
	input := recordFromTask(
		"tenant-1",
		"external-task-1",
		"local-task-1",
		sql.NullString{},
		"chapter_generate",
		"done",
		"bid_chapter",
		"resource-1",
		map[string]any{"provider": "deepseek", "model": "deepseek-chat"},
		map[string]any{
			"estimated_cost": "0.0315",
			"model_metadata": map[string]any{
				"provider": "deepseek",
				"model":    "deepseek-chat",
			},
			"token_usage": map[string]any{
				"input_tokens":  2000,
				"output_tokens": 1000,
			},
		},
		"",
		time.Unix(100, 0),
		sql.NullTime{Time: time.Unix(103, 0), Valid: true},
	)
	input = normalizeRecord(input)

	if input.EstimatedCost != 0.0315 {
		t.Fatalf("expected top-level estimated cost, got %.6f", input.EstimatedCost)
	}
}

func TestRecordFromTaskPrefersModelMetadataEstimatedCost(t *testing.T) {
	input := recordFromTask(
		"tenant-1",
		"external-task-1",
		"local-task-1",
		sql.NullString{},
		"chapter_generate",
		"done",
		"bid_chapter",
		"resource-1",
		map[string]any{"provider": "deepseek", "model": "deepseek-chat"},
		map[string]any{
			"estimated_cost": 0.0315,
			"model_metadata": map[string]any{
				"provider":       "deepseek",
				"model":          "deepseek-chat",
				"estimated_cost": 0.0125,
			},
			"token_usage": map[string]any{
				"input_tokens":  2000,
				"output_tokens": 1000,
			},
		},
		"",
		time.Unix(100, 0),
		sql.NullTime{Time: time.Unix(103, 0), Valid: true},
	)
	input = normalizeRecord(input)

	if input.EstimatedCost != 0.0125 {
		t.Fatalf("expected model metadata estimated cost, got %.6f", input.EstimatedCost)
	}
}

func TestNormalizeStatusCanonicalizesKnownValues(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: " ", want: "done"},
		{input: " SUCCESS ", want: "done"},
		{input: "succeeded", want: "done"},
		{input: "ERROR", want: "failed"},
		{input: "canceled", want: "cancelled"},
		{input: " RUNNING ", want: "running"},
	} {
		if got := normalizeStatus(tc.input); got != tc.want {
			t.Fatalf("expected %q to normalize to %q, got %q", tc.input, tc.want, got)
		}
	}
}

func TestNormalizeStatusRejectsUnsupportedValues(t *testing.T) {
	if got := normalizeStatus("partial_success"); got != "" {
		t.Fatalf("expected unsupported status to be rejected, got %q", got)
	}
}

func TestShouldUpdateExistingLogUsesCallbackStatusOrdering(t *testing.T) {
	for _, tc := range []struct {
		current string
		next    string
		want    bool
	}{
		{current: "queued", next: "running", want: true},
		{current: "running", next: "done", want: true},
		{current: "running", next: "queued", want: false},
		{current: "done", next: "failed", want: false},
		{current: "failed", next: "done", want: false},
	} {
		if got := shouldUpdateExistingLog(tc.current, tc.next); got != tc.want {
			t.Fatalf("expected %q -> %q update=%v, got %v", tc.current, tc.next, tc.want, got)
		}
	}
}

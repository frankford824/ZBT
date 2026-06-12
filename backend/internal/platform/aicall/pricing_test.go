package aicall

import "testing"

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

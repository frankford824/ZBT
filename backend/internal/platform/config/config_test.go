package config

import "testing"

func TestLoadUsesDevelopmentAIHMACSecretWhenUnset(t *testing.T) {
	t.Setenv("AI_SERVICE_HMAC_SECRET", "")

	cfg := Load()

	if cfg.AIServiceHMACSecret != DefaultAIServiceHMACSecret {
		t.Fatalf("expected default AI service HMAC secret")
	}
}

func TestLoadAllowsAIHMACSecretOverride(t *testing.T) {
	t.Setenv("AI_SERVICE_HMAC_SECRET", "custom-secret")

	cfg := Load()

	if cfg.AIServiceHMACSecret != "custom-secret" {
		t.Fatalf("expected configured AI service HMAC secret")
	}
}

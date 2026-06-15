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

func TestValidateAllowsDevelopmentDefaultsOutsideProduction(t *testing.T) {
	clearProductionEnv(t)
	cfg := Load()
	cfg.DatabaseURL = "postgres://example"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected development config to validate: %v", err)
	}
}

func TestValidateRejectsDevelopmentSecretsInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	cfg := Load()
	cfg.DatabaseURL = "postgres://example"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config with development secrets to be rejected")
	}
}

func TestValidateAllowsProductionOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "prod-jwt-secret")
	t.Setenv("AI_SERVICE_HMAC_SECRET", "prod-ai-secret")
	t.Setenv("MINIO_ACCESS_KEY", "prod-minio-access")
	t.Setenv("MINIO_SECRET_KEY", "prod-minio-secret")
	cfg := Load()
	cfg.DatabaseURL = "postgres://example"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected production config with explicit secrets to validate: %v", err)
	}
}

func TestValidateRejectsDevelopmentMinIOCredentialsInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "prod-jwt-secret")
	t.Setenv("AI_SERVICE_HMAC_SECRET", "prod-ai-secret")
	cfg := Load()
	cfg.DatabaseURL = "postgres://example"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config with development MinIO credentials to be rejected")
	}
}

func clearProductionEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "ZBT_ENV", "ENVIRONMENT", "GIN_MODE"} {
		t.Setenv(key, "")
	}
}

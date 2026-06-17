package config

import (
	"testing"
	"time"
)

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

func TestLoadUsesConfiguredJWTAccessTTL(t *testing.T) {
	t.Setenv("JWT_ACCESS_TTL", "15m")

	cfg := Load()

	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Fatalf("expected configured JWT access TTL, got %s", cfg.JWTAccessTTL)
	}
}

func TestLoadBoundsJWTAccessTTL(t *testing.T) {
	t.Setenv("JWT_ACCESS_TTL", "1s")
	if cfg := Load(); cfg.JWTAccessTTL != DefaultJWTAccessTTL {
		t.Fatalf("expected tiny JWT access TTL to fall back, got %s", cfg.JWTAccessTTL)
	}

	t.Setenv("JWT_ACCESS_TTL", "999h")
	if cfg := Load(); cfg.JWTAccessTTL != maxJWTAccessTTL {
		t.Fatalf("expected huge JWT access TTL to clamp, got %s", cfg.JWTAccessTTL)
	}
}

func TestLoadAllowsBucketEnsureOverride(t *testing.T) {
	t.Setenv("MINIO_ENSURE_BUCKET", "false")

	cfg := Load()

	if cfg.MinIOEnsureBucket {
		t.Fatal("expected MINIO_ENSURE_BUCKET=false to disable bucket initialization")
	}
}

func TestLoadEnablesBucketEnsureByDefault(t *testing.T) {
	t.Setenv("MINIO_ENSURE_BUCKET", "")

	cfg := Load()

	if !cfg.MinIOEnsureBucket {
		t.Fatal("expected bucket initialization to be enabled by default")
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
	t.Setenv("JWT_SECRET", "prod-jwt-secret-value")
	t.Setenv("AI_SERVICE_HMAC_SECRET", "prod-ai-secret-value")
	t.Setenv("MINIO_ACCESS_KEY", "prod-minio-access-value")
	t.Setenv("MINIO_SECRET_KEY", "prod-minio-secret-value")
	cfg := Load()
	cfg.DatabaseURL = "postgres://example"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected production config with explicit secrets to validate: %v", err)
	}
}

func TestValidateRejectsDevelopmentMinIOCredentialsInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "prod-jwt-secret-value")
	t.Setenv("AI_SERVICE_HMAC_SECRET", "prod-ai-secret-value")
	cfg := Load()
	cfg.DatabaseURL = "postgres://example"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config with development MinIO credentials to be rejected")
	}
}

func TestValidateRejectsShortProductionSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("AI_SERVICE_HMAC_SECRET", "prod-ai-secret-value")
	t.Setenv("MINIO_ACCESS_KEY", "prod-minio-access-value")
	t.Setenv("MINIO_SECRET_KEY", "prod-minio-secret-value")
	cfg := Load()
	cfg.DatabaseURL = "postgres://example"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config with short JWT secret to be rejected")
	}
}

func clearProductionEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "ZBT_ENV", "ENVIRONMENT", "GIN_MODE"} {
		t.Setenv(key, "")
	}
}

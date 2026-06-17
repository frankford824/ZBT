package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

const (
	DefaultAIServiceHMACSecret = "dev-only-zbt-ai-callback-secret"
	DefaultJWTSecret           = "dev-only-zbt-jwt-secret"
	DefaultJWTAccessTTL        = 8 * time.Hour
	DefaultMinIOAccessKey      = "zbt_minio"
	DefaultMinIOSecretKey      = "zbt_minio_secret"
	minProductionSecretLength  = 16
	minJWTAccessTTL            = time.Minute
	maxJWTAccessTTL            = 24 * time.Hour
)

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	MigrationDatabaseURL string
	RedisURL             string
	AIServiceURL         string
	AIServiceHMACSecret  string
	AICallbackURL        string
	MinIOEndpoint        string
	MinIOPublicEndpoint  string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOUseSSL          bool
	MinIORegion          string
	MinIOBucket          string
	MinIOEnsureBucket    bool
	JWTSecret            string
	JWTAccessTTL         time.Duration
	DefaultTenantID      string
}

func Load() Config {
	return Config{
		HTTPAddr:             env("HTTP_ADDR", ":8080"),
		DatabaseURL:          env("DATABASE_URL", ""),
		MigrationDatabaseURL: env("MIGRATION_DATABASE_URL", env("DATABASE_URL", "")),
		RedisURL:             env("REDIS_URL", "redis://redis:6379/0"),
		AIServiceURL:         env("AI_SERVICE_URL", "http://ai-service:8000"),
		AIServiceHMACSecret:  env("AI_SERVICE_HMAC_SECRET", DefaultAIServiceHMACSecret),
		AICallbackURL:        env("AI_CALLBACK_URL", "http://backend:8080/api/v1/ai/callbacks/tasks"),
		MinIOEndpoint:        env("MINIO_ENDPOINT", "minio:9000"),
		MinIOPublicEndpoint:  env("MINIO_PUBLIC_ENDPOINT", env("MINIO_ENDPOINT", "minio:9000")),
		MinIOAccessKey:       env("MINIO_ACCESS_KEY", DefaultMinIOAccessKey),
		MinIOSecretKey:       env("MINIO_SECRET_KEY", DefaultMinIOSecretKey),
		MinIOUseSSL:          envBool("MINIO_USE_SSL", false),
		MinIORegion:          env("MINIO_REGION", "us-east-1"),
		MinIOBucket:          env("MINIO_BUCKET", "zbt-files"),
		MinIOEnsureBucket:    envBool("MINIO_ENSURE_BUCKET", true),
		JWTSecret:            env("JWT_SECRET", DefaultJWTSecret),
		JWTAccessTTL:         envDuration("JWT_ACCESS_TTL", DefaultJWTAccessTTL, minJWTAccessTTL, maxJWTAccessTTL),
		DefaultTenantID:      env("DEFAULT_TENANT_ID", "00000000-0000-4000-8000-000000000001"),
	}
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	if !ProductionMode() {
		return nil
	}
	if insecureSecret(cfg.JWTSecret, DefaultJWTSecret) {
		return errors.New("JWT_SECRET must be set to a non-development value in production")
	}
	if insecureSecret(cfg.AIServiceHMACSecret, DefaultAIServiceHMACSecret) {
		return errors.New("AI_SERVICE_HMAC_SECRET must be set to a non-development value in production")
	}
	if insecureSecret(cfg.MinIOAccessKey, DefaultMinIOAccessKey) {
		return errors.New("MINIO_ACCESS_KEY must be set to a non-development value in production")
	}
	if insecureSecret(cfg.MinIOSecretKey, DefaultMinIOSecretKey) {
		return errors.New("MINIO_SECRET_KEY must be set to a non-development value in production")
	}
	return nil
}

func ProductionMode() bool {
	for _, key := range []string{"APP_ENV", "ZBT_ENV", "ENVIRONMENT", "GIN_MODE"} {
		value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		switch value {
		case "prod", "production", "release":
			return true
		}
	}
	return false
}

func insecureSecret(value, developmentDefault string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == developmentDefault || len(value) < minProductionSecretLength
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "TRUE" || value == "yes" || value == "YES"
}

func envDuration(key string, fallback, minimum, maximum time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < minimum {
		return fallback
	}
	if duration > maximum {
		return maximum
	}
	return duration
}

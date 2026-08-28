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

	// CollectorHMACSecret 为空表示未启用采集接入：POST /api/v1/platform/tenders/ingest
	// 直接返回 503 且不做任何写入。故意不设开发默认值，没配密钥就等于没开这个入口。
	CollectorHMACSecret string
	// PlatformTenderPoolPublic 控制公共标讯池只读接口是否对租户可见，测试期打开、上线前收回。
	PlatformTenderPoolPublic bool

	// ZizhiAPIURL / ZizhiAPIKey 指向局域网内的资质库检索服务（zizhi-api）。
	// 两者任一为空即视为未接入，资质同步接口返回 503 而不是静默失败。
	// 该服务只被读取，资质原件的权威副本始终在公司 NAS 上。
	ZizhiAPIURL string
	ZizhiAPIKey string
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

		CollectorHMACSecret:      env("COLLECTOR_HMAC_SECRET", ""),
		PlatformTenderPoolPublic: envBool("PLATFORM_TENDER_POOL_PUBLIC", false),

		ZizhiAPIURL: env("ZIZHI_API_URL", ""),
		ZizhiAPIKey: env("ZIZHI_API_KEY", ""),
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

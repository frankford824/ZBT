package config

import "os"

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	MigrationDatabaseURL string
	RedisURL             string
	AIServiceURL         string
	AIServiceHMACSecret  string
	MinIOEndpoint        string
	MinIOPublicEndpoint  string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOUseSSL          bool
	MinIORegion          string
	MinIOBucket          string
	JWTSecret            string
	DefaultTenantID      string
}

func Load() Config {
	return Config{
		HTTPAddr:             env("HTTP_ADDR", ":8080"),
		DatabaseURL:          env("DATABASE_URL", ""),
		MigrationDatabaseURL: env("MIGRATION_DATABASE_URL", env("DATABASE_URL", "")),
		RedisURL:             env("REDIS_URL", "redis://redis:6379/0"),
		AIServiceURL:         env("AI_SERVICE_URL", "http://ai-service:8000"),
		AIServiceHMACSecret:  env("AI_SERVICE_HMAC_SECRET", ""),
		MinIOEndpoint:        env("MINIO_ENDPOINT", "minio:9000"),
		MinIOPublicEndpoint:  env("MINIO_PUBLIC_ENDPOINT", env("MINIO_ENDPOINT", "minio:9000")),
		MinIOAccessKey:       env("MINIO_ACCESS_KEY", "zbt_minio"),
		MinIOSecretKey:       env("MINIO_SECRET_KEY", "zbt_minio_secret"),
		MinIOUseSSL:          envBool("MINIO_USE_SSL", false),
		MinIORegion:          env("MINIO_REGION", "us-east-1"),
		MinIOBucket:          env("MINIO_BUCKET", "zbt-files"),
		JWTSecret:            env("JWT_SECRET", "dev-only-zbt-jwt-secret"),
		DefaultTenantID:      env("DEFAULT_TENANT_ID", "00000000-0000-4000-8000-000000000001"),
	}
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

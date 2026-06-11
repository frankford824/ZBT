package config

import "os"

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	RedisURL            string
	AIServiceURL        string
	AIServiceHMACSecret string
	MinIOBucket         string
}

func Load() Config {
	return Config{
		HTTPAddr:            env("HTTP_ADDR", ":8080"),
		DatabaseURL:         env("DATABASE_URL", ""),
		RedisURL:            env("REDIS_URL", "redis://redis:6379/0"),
		AIServiceURL:        env("AI_SERVICE_URL", "http://ai-service:8000"),
		AIServiceHMACSecret: env("AI_SERVICE_HMAC_SECRET", ""),
		MinIOBucket:         env("MINIO_BUCKET", "zbt-files"),
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

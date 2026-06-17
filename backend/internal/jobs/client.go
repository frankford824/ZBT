package jobs

import (
	"crypto/tls"
	"net/url"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
)

type Client struct {
	client *asynq.Client
}

func NewClient(redisAddr string) *Client {
	return &Client{client: asynq.NewClient(redisClientOpt(redisAddr))}
}

func (c *Client) Close() error {
	return c.client.Close()
}

func redisClientOpt(raw string) asynq.RedisClientOpt {
	value := strings.TrimSpace(raw)
	if value == "" {
		return asynq.RedisClientOpt{Addr: "redis:6379"}
	}
	if !strings.Contains(value, "://") {
		return asynq.RedisClientOpt{Addr: value}
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return asynq.RedisClientOpt{Addr: value}
	}
	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return asynq.RedisClientOpt{Addr: value}
	}

	opt := asynq.RedisClientOpt{Addr: parsed.Host}
	if parsed.User != nil {
		opt.Username = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			opt.Password = password
		}
	}
	if db := strings.Trim(parsed.Path, "/"); db != "" {
		if parsedDB, err := strconv.Atoi(db); err == nil && parsedDB >= 0 {
			opt.DB = parsedDB
		}
	}
	if parsed.Scheme == "rediss" {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opt
}

package jobs

import "testing"

func TestRedisClientOptUsesHostPortAddress(t *testing.T) {
	opt := redisClientOpt(" redis:6379 ")

	if opt.Addr != "redis:6379" {
		t.Fatalf("unexpected redis addr: %q", opt.Addr)
	}
	if opt.DB != 0 || opt.Username != "" || opt.Password != "" || opt.TLSConfig != nil {
		t.Fatalf("unexpected redis options: %+v", opt)
	}
}

func TestRedisClientOptParsesRedisURL(t *testing.T) {
	opt := redisClientOpt("redis://user:secret@redis.example:6380/3")

	if opt.Addr != "redis.example:6380" {
		t.Fatalf("unexpected redis addr: %q", opt.Addr)
	}
	if opt.Username != "user" || opt.Password != "secret" {
		t.Fatalf("unexpected redis credentials: username=%q password=%q", opt.Username, opt.Password)
	}
	if opt.DB != 3 {
		t.Fatalf("unexpected redis db: %d", opt.DB)
	}
	if opt.TLSConfig != nil {
		t.Fatal("expected redis:// to avoid TLS")
	}
}

func TestRedisClientOptParsesRedissURL(t *testing.T) {
	opt := redisClientOpt("rediss://:secret@redis.example:6380/2")

	if opt.Addr != "redis.example:6380" {
		t.Fatalf("unexpected redis addr: %q", opt.Addr)
	}
	if opt.Username != "" || opt.Password != "secret" {
		t.Fatalf("unexpected redis credentials: username=%q password=%q", opt.Username, opt.Password)
	}
	if opt.DB != 2 {
		t.Fatalf("unexpected redis db: %d", opt.DB)
	}
	if opt.TLSConfig == nil {
		t.Fatal("expected rediss:// to enable TLS")
	}
}

func TestRedisClientOptIgnoresInvalidURLDatabase(t *testing.T) {
	opt := redisClientOpt("redis://redis.example/not-a-db")

	if opt.Addr != "redis.example" {
		t.Fatalf("unexpected redis addr: %q", opt.Addr)
	}
	if opt.DB != 0 {
		t.Fatalf("expected invalid db to fall back to 0, got %d", opt.DB)
	}
}

func TestRedisClientOptFallsBackForUnsupportedURLScheme(t *testing.T) {
	raw := "http://redis.example:6379/0"
	opt := redisClientOpt(raw)

	if opt.Addr != raw {
		t.Fatalf("expected unsupported URL to remain raw, got %q", opt.Addr)
	}
}

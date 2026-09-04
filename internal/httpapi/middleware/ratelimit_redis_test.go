package middleware

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisBackend(t *testing.T, burst int) *redisBackend {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	return newRedisBackend(client, burst)
}

func TestRedisBackend_AllowsUpToBurstPerWindow(t *testing.T) {
	backend := newTestRedisBackend(t, 3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		allowed, err := backend.allow(ctx, "1.2.3.4")
		if err != nil {
			t.Fatalf("allow() returned error: %v", err)
		}
		if !allowed {
			t.Fatalf("request %d expected to be allowed within burst, was denied", i+1)
		}
	}

	allowed, err := backend.allow(ctx, "1.2.3.4")
	if err != nil {
		t.Fatalf("allow() returned error: %v", err)
	}
	if allowed {
		t.Fatal("request exceeding burst expected to be denied, was allowed")
	}
}

func TestRedisBackend_TracksClientsIndependently(t *testing.T) {
	backend := newTestRedisBackend(t, 1)
	ctx := context.Background()

	if allowed, err := backend.allow(ctx, "1.1.1.1"); err != nil || !allowed {
		t.Fatalf("first client's first request should be allowed, got allowed=%v err=%v", allowed, err)
	}
	if allowed, err := backend.allow(ctx, "1.1.1.1"); err != nil || allowed {
		t.Fatalf("first client's second request should be denied, got allowed=%v err=%v", allowed, err)
	}
	if allowed, err := backend.allow(ctx, "2.2.2.2"); err != nil || !allowed {
		t.Fatalf("second (different) client should still be allowed, got allowed=%v err=%v", allowed, err)
	}
}

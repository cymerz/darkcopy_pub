package access

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimitStore(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()
	store := NewRedisRateLimitStore(rdb)
	key := "test-ip-key"
	ttl := 15 * time.Minute

	// Verify not rate limited initially
	limited, err := store.IsRateLimited(ctx, key, 3, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limited {
		t.Errorf("expected not rate limited initially")
	}

	// Record failed attempt 1
	err = store.RecordFailedAttempt(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify not rate limited yet (threshold 3)
	limited, err = store.IsRateLimited(ctx, key, 3, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limited {
		t.Errorf("expected not rate limited at 1 attempt")
	}

	// Record failed attempt 2 & 3
	_ = store.RecordFailedAttempt(ctx, key, ttl)
	_ = store.RecordFailedAttempt(ctx, key, ttl)

	// Verify rate limited (threshold 3 met)
	limited, err = store.IsRateLimited(ctx, key, 3, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !limited {
		t.Errorf("expected rate limited at 3 attempts")
	}

	// Reset rate limit
	err = store.ResetRateLimit(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify not rate limited anymore
	limited, err = store.IsRateLimited(ctx, key, 3, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limited {
		t.Errorf("expected not rate limited after reset")
	}
}

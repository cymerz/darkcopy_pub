package quota

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisCounter(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	counter := NewRedisCounter(rdb)
	counter.now = func() time.Time { return now }

	// First attempt under limit
	allowed, remaining := counter.Allow("127.0.0.1", 3)
	if !allowed {
		t.Errorf("expected allowed=true, got false")
	}
	if remaining != 2 {
		t.Errorf("expected remaining=2, got %d", remaining)
	}

	// Second attempt under limit
	allowed, remaining = counter.Allow("127.0.0.1", 3)
	if !allowed {
		t.Errorf("expected allowed=true, got false")
	}
	if remaining != 1 {
		t.Errorf("expected remaining=1, got %d", remaining)
	}

	// Third attempt under limit
	allowed, remaining = counter.Allow("127.0.0.1", 3)
	if !allowed {
		t.Errorf("expected allowed=true, got false")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}

	// Fourth attempt exceeds limit
	allowed, remaining = counter.Allow("127.0.0.1", 3)
	if allowed {
		t.Errorf("expected allowed=false, got true")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}
}

func TestRedisSizeCounter(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	counter := NewRedisSizeCounter(rdb)
	counter.now = func() time.Time { return now }

	// Allow upload within size limit
	allowed, remaining := counter.Allow("127.0.0.1", 100, 300)
	if !allowed {
		t.Errorf("expected allowed=true, got false")
	}
	if remaining != 200 {
		t.Errorf("expected remaining=200, got %d", remaining)
	}

	// Allow upload that hits the limit exactly
	allowed, remaining = counter.Allow("127.0.0.1", 200, 300)
	if !allowed {
		t.Errorf("expected allowed=true, got false")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}

	// Exceed size limit
	allowed, remaining = counter.Allow("127.0.0.1", 50, 300)
	if allowed {
		t.Errorf("expected allowed=false, got true")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}
}

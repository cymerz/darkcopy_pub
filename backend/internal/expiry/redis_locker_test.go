package expiry

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLocker(t *testing.T) {
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
	locker := NewRedisLocker(rdb)
	key := "test-lock-key"
	ttl := 5 * time.Second

	// Attempt to acquire lock first time
	acquired, err := locker.AcquireLock(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Errorf("expected to acquire lock first time")
	}

	// Attempt to acquire lock second time (should fail as it is already locked)
	acquired, err = locker.AcquireLock(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acquired {
		t.Errorf("expected to fail acquiring lock second time")
	}

	// Release lock
	err = locker.ReleaseLock(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Attempt to acquire lock again after release (should succeed)
	acquired, err = locker.AcquireLock(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acquired {
		t.Errorf("expected to acquire lock after release")
	}
}

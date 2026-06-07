// Package access provides the access controller implementation with bcrypt
// password verification and pluggable rate limiting storage.
package access

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor used for bcrypt hashing.
	BcryptCost = 10

	// RateLimitThreshold is the number of consecutive failed attempts before blocking.
	RateLimitThreshold = 5

	// RateLimitTTL is the duration for which rate limit entries are valid.
	RateLimitTTL = 15 * time.Minute
)

// RateLimitStore defines the backend storage interface for password rate limits.
type RateLimitStore interface {
	RecordFailedAttempt(ctx context.Context, key string, ttl time.Duration) error
	IsRateLimited(ctx context.Context, key string, threshold int, ttl time.Duration) (bool, error)
	ResetRateLimit(ctx context.Context, key string) error
}

// rateLimitEntry tracks failed attempts for a given IP+resource combination.
type rateLimitEntry struct {
	count       int
	lastAttempt time.Time
}

// InMemoryRateLimitStore implements RateLimitStore using sync.Map.
type InMemoryRateLimitStore struct {
	rateLimits sync.Map
	now        func() time.Time
}

// NewInMemoryRateLimitStore creates a new InMemoryRateLimitStore.
func NewInMemoryRateLimitStore(now func() time.Time) *InMemoryRateLimitStore {
	if now == nil {
		now = time.Now
	}
	return &InMemoryRateLimitStore{
		now: now,
	}
}

// RecordFailedAttempt records a failed password attempt in memory.
func (s *InMemoryRateLimitStore) RecordFailedAttempt(ctx context.Context, key string, ttl time.Duration) error {
	now := s.now()
	val, loaded := s.rateLimits.Load(key)
	if loaded {
		entry := val.(*rateLimitEntry)
		if now.Sub(entry.lastAttempt) > ttl {
			entry.count = 1
			entry.lastAttempt = now
		} else {
			entry.count++
			entry.lastAttempt = now
		}
		s.rateLimits.Store(key, entry)
	} else {
		s.rateLimits.Store(key, &rateLimitEntry{
			count:       1,
			lastAttempt: now,
		})
	}
	return nil
}

// IsRateLimited checks rate limit status in memory.
func (s *InMemoryRateLimitStore) IsRateLimited(ctx context.Context, key string, threshold int, ttl time.Duration) (bool, error) {
	val, ok := s.rateLimits.Load(key)
	if !ok {
		return false, nil
	}
	entry := val.(*rateLimitEntry)
	if s.now().Sub(entry.lastAttempt) > ttl {
		s.rateLimits.Delete(key)
		return false, nil
	}
	return entry.count >= threshold, nil
}

// ResetRateLimit clears rate limit status in memory.
func (s *InMemoryRateLimitStore) ResetRateLimit(ctx context.Context, key string) error {
	s.rateLimits.Delete(key)
	return nil
}

// RedisRateLimitStore implements RateLimitStore using Redis.
type RedisRateLimitStore struct {
	rdb redis.Cmdable
}

// NewRedisRateLimitStore creates a new RedisRateLimitStore.
func NewRedisRateLimitStore(rdb redis.Cmdable) *RedisRateLimitStore {
	return &RedisRateLimitStore{
		rdb: rdb,
	}
}

// RecordFailedAttempt records a failed password attempt in Redis.
func (s *RedisRateLimitStore) RecordFailedAttempt(ctx context.Context, key string, ttl time.Duration) error {
	rkey := "ratelimit:failed:" + key
	pipe := s.rdb.Pipeline()
	pipe.Incr(ctx, rkey)
	pipe.Expire(ctx, rkey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// IsRateLimited checks rate limit status in Redis.
func (s *RedisRateLimitStore) IsRateLimited(ctx context.Context, key string, threshold int, ttl time.Duration) (bool, error) {
	rkey := "ratelimit:failed:" + key
	val, err := s.rdb.Get(ctx, rkey).Int()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return val >= threshold, nil
}

// ResetRateLimit clears rate limit status in Redis.
func (s *RedisRateLimitStore) ResetRateLimit(ctx context.Context, key string) error {
	rkey := "ratelimit:failed:" + key
	return s.rdb.Del(ctx, rkey).Err()
}

// Compile-time check that Controller implements AccessController.
var _ AccessController = (*Controller)(nil)

// Controller is the concrete implementation of AccessController.
// It delegates rate limit storage to a RateLimitStore.
type Controller struct {
	store RateLimitStore
	now   func() time.Time
}

// NewController creates a new Controller instance.
func NewController(store RateLimitStore) *Controller {
	c := &Controller{
		now: time.Now,
	}
	if store == nil {
		store = &InMemoryRateLimitStore{
			now: func() time.Time { return c.now() },
		}
	}
	c.store = store
	return c
}

// CheckAccess verifies a password against a bcrypt hash.
// It returns AccessGranted if the password matches, or AccessDenied otherwise.
func (c *Controller) CheckAccess(ctx context.Context, passwordHash string, password string) (AccessResult, error) {
	if passwordHash == "" {
		return AccessGranted, nil
	}

	err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err == nil {
		return AccessGranted, nil
	}

	if err == bcrypt.ErrMismatchedHashAndPassword {
		return AccessDenied, nil
	}

	return AccessDenied, fmt.Errorf("access: failed to compare password: %w", err)
}

// RecordFailedAttempt records a failed password attempt.
func (c *Controller) RecordFailedAttempt(ctx context.Context, ip string, resourceSlug string) error {
	key := fmt.Sprintf("%s:%s", ip, resourceSlug)
	return c.store.RecordFailedAttempt(ctx, key, RateLimitTTL)
}

// IsRateLimited checks whether the given IP+resource combination has exceeded the threshold.
func (c *Controller) IsRateLimited(ctx context.Context, ip string, resourceSlug string) (bool, error) {
	key := fmt.Sprintf("%s:%s", ip, resourceSlug)
	return c.store.IsRateLimited(ctx, key, RateLimitThreshold, RateLimitTTL)
}

// ResetRateLimit clears the rate limit entry.
func (c *Controller) ResetRateLimit(ctx context.Context, ip string, resourceSlug string) {
	key := fmt.Sprintf("%s:%s", ip, resourceSlug)
	_ = c.store.ResetRateLimit(ctx, key)
}

// HashPassword hashes a plaintext password using bcrypt with cost factor 10.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("access: failed to hash password: %w", err)
	}
	return string(hash), nil
}


// Package access provides the access controller implementation with bcrypt
// password verification and in-memory rate limiting.
package access

import (
	"context"
	"fmt"
	"sync"
	"time"

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

// rateLimitEntry tracks failed attempts for a given IP+resource combination.
type rateLimitEntry struct {
	count     int
	lastAttempt time.Time
}

// Compile-time check that Controller implements AccessController.
var _ AccessController = (*Controller)(nil)

// Controller is the concrete implementation of AccessController.
// It uses bcrypt for password verification and an in-memory sync.Map for rate limiting.
type Controller struct {
	// rateLimits stores rate limit entries keyed by "{ip}:{resource_slug}".
	rateLimits sync.Map

	// now is a function that returns the current time. It can be overridden for testing.
	now func() time.Time
}

// NewController creates a new Controller instance.
func NewController() *Controller {
	return &Controller{
		now: time.Now,
	}
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

// RecordFailedAttempt records a failed password attempt for the given IP and resource.
// The key format is "{ip}:{resourceSlug}".
func (c *Controller) RecordFailedAttempt(ctx context.Context, ip string, resourceSlug string) error {
	key := fmt.Sprintf("%s:%s", ip, resourceSlug)

	now := c.now()

	val, loaded := c.rateLimits.Load(key)
	if loaded {
		entry := val.(*rateLimitEntry)
		// If the entry has expired, reset it.
		if now.Sub(entry.lastAttempt) > RateLimitTTL {
			entry.count = 1
			entry.lastAttempt = now
		} else {
			entry.count++
			entry.lastAttempt = now
		}
		c.rateLimits.Store(key, entry)
	} else {
		c.rateLimits.Store(key, &rateLimitEntry{
			count:       1,
			lastAttempt: now,
		})
	}

	return nil
}

// IsRateLimited checks whether the given IP+resource combination has exceeded
// the rate limit threshold (5 consecutive failed attempts within 15 minutes).
func (c *Controller) IsRateLimited(ctx context.Context, ip string, resourceSlug string) (bool, error) {
	key := fmt.Sprintf("%s:%s", ip, resourceSlug)

	val, ok := c.rateLimits.Load(key)
	if !ok {
		return false, nil
	}

	entry := val.(*rateLimitEntry)

	// If the entry has expired (older than TTL), it's no longer rate limited.
	if c.now().Sub(entry.lastAttempt) > RateLimitTTL {
		c.rateLimits.Delete(key)
		return false, nil
	}

	return entry.count >= RateLimitThreshold, nil
}

// ResetRateLimit clears the rate limit entry for the given IP and resource.
// This is called when a successful authentication occurs.
func (c *Controller) ResetRateLimit(ctx context.Context, ip string, resourceSlug string) {
	key := fmt.Sprintf("%s:%s", ip, resourceSlug)
	c.rateLimits.Delete(key)
}

// HashPassword hashes a plaintext password using bcrypt with cost factor 10.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("access: failed to hash password: %w", err)
	}
	return string(hash), nil
}

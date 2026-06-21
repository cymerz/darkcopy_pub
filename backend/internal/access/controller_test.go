package access

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "mysecretpassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Verify the hash is valid bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		t.Errorf("HashPassword() produced invalid hash: %v", err)
	}

	// Verify cost factor is 10
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost() error = %v", err)
	}
	if cost != BcryptCost {
		t.Errorf("HashPassword() cost = %d, want %d", cost, BcryptCost)
	}
}

func TestCheckAccess_EmptyHash(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	result, err := ctrl.CheckAccess(ctx, "", "anypassword")
	if err != nil {
		t.Fatalf("CheckAccess() error = %v", err)
	}
	if result != AccessGranted {
		t.Errorf("CheckAccess() with empty hash = %v, want AccessGranted", result)
	}
}

func TestCheckAccess_CorrectPassword(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	password := "correctpassword"
	hash, _ := HashPassword(password)

	result, err := ctrl.CheckAccess(ctx, hash, password)
	if err != nil {
		t.Fatalf("CheckAccess() error = %v", err)
	}
	if result != AccessGranted {
		t.Errorf("CheckAccess() with correct password = %v, want AccessGranted", result)
	}
}

func TestCheckAccess_WrongPassword(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	password := "correctpassword"
	hash, _ := HashPassword(password)

	result, err := ctrl.CheckAccess(ctx, hash, "wrongpassword")
	if err != nil {
		t.Fatalf("CheckAccess() error = %v", err)
	}
	if result != AccessDenied {
		t.Errorf("CheckAccess() with wrong password = %v, want AccessDenied", result)
	}
}

func TestRecordFailedAttempt_IncreasesCount(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	ip := "192.168.1.1"
	resource := "abc12345"

	for i := 0; i < 3; i++ {
		if err := ctrl.RecordFailedAttempt(ctx, ip, resource); err != nil {
			t.Fatalf("RecordFailedAttempt() error = %v", err)
		}
	}

	limited, err := ctrl.IsRateLimited(ctx, ip, resource)
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if limited {
		t.Error("IsRateLimited() = true after 3 attempts, want false")
	}
}

func TestIsRateLimited_AfterThreshold(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	ip := "192.168.1.1"
	resource := "abc12345"

	// Record exactly 5 failed attempts
	for i := 0; i < RateLimitThreshold; i++ {
		if err := ctrl.RecordFailedAttempt(ctx, ip, resource); err != nil {
			t.Fatalf("RecordFailedAttempt() error = %v", err)
		}
	}

	limited, err := ctrl.IsRateLimited(ctx, ip, resource)
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if !limited {
		t.Error("IsRateLimited() = false after 5 attempts, want true")
	}
}

func TestIsRateLimited_DifferentIPsAreIndependent(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	resource := "abc12345"

	// Rate limit IP1
	for i := 0; i < RateLimitThreshold; i++ {
		ctrl.RecordFailedAttempt(ctx, "192.168.1.1", resource)
	}

	// IP2 should not be limited
	limited, err := ctrl.IsRateLimited(ctx, "192.168.1.2", resource)
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if limited {
		t.Error("IsRateLimited() = true for different IP, want false")
	}
}

func TestIsRateLimited_DifferentResourcesAreIndependent(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	ip := "192.168.1.1"

	// Rate limit resource1
	for i := 0; i < RateLimitThreshold; i++ {
		ctrl.RecordFailedAttempt(ctx, ip, "resource1")
	}

	// resource2 should not be limited
	limited, err := ctrl.IsRateLimited(ctx, ip, "resource2")
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if limited {
		t.Error("IsRateLimited() = true for different resource, want false")
	}
}

func TestIsRateLimited_ExpiresAfterTTL(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	ip := "192.168.1.1"
	resource := "abc12345"

	// Set time to "now"
	currentTime := time.Now()
	ctrl.now = func() time.Time { return currentTime }

	// Record 5 failed attempts
	for i := 0; i < RateLimitThreshold; i++ {
		ctrl.RecordFailedAttempt(ctx, ip, resource)
	}

	// Verify rate limited
	limited, _ := ctrl.IsRateLimited(ctx, ip, resource)
	if !limited {
		t.Fatal("IsRateLimited() = false, want true before TTL expires")
	}

	// Advance time past TTL
	ctrl.now = func() time.Time { return currentTime.Add(RateLimitTTL + 1*time.Second) }

	limited, err := ctrl.IsRateLimited(ctx, ip, resource)
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if limited {
		t.Error("IsRateLimited() = true after TTL expired, want false")
	}
}

func TestRecordFailedAttempt_ResetsAfterTTL(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	ip := "192.168.1.1"
	resource := "abc12345"

	// Set time to "now"
	currentTime := time.Now()
	ctrl.now = func() time.Time { return currentTime }

	// Record 4 failed attempts
	for i := 0; i < 4; i++ {
		ctrl.RecordFailedAttempt(ctx, ip, resource)
	}

	// Advance time past TTL
	ctrl.now = func() time.Time { return currentTime.Add(RateLimitTTL + 1*time.Second) }

	// Record 1 more attempt — should reset counter since TTL expired
	ctrl.RecordFailedAttempt(ctx, ip, resource)

	limited, err := ctrl.IsRateLimited(ctx, ip, resource)
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if limited {
		t.Error("IsRateLimited() = true after TTL reset, want false (only 1 attempt after reset)")
	}
}

func TestResetRateLimit(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	ip := "192.168.1.1"
	resource := "abc12345"

	// Rate limit the IP
	for i := 0; i < RateLimitThreshold; i++ {
		ctrl.RecordFailedAttempt(ctx, ip, resource)
	}

	// Verify rate limited
	limited, _ := ctrl.IsRateLimited(ctx, ip, resource)
	if !limited {
		t.Fatal("IsRateLimited() = false, want true before reset")
	}

	// Reset
	ctrl.ResetRateLimit(ctx, ip, resource)

	// Should no longer be limited
	limited, err := ctrl.IsRateLimited(ctx, ip, resource)
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if limited {
		t.Error("IsRateLimited() = true after reset, want false")
	}
}

func TestIsRateLimited_NoEntry(t *testing.T) {
	ctrl := NewController(nil)
	ctx := context.Background()

	limited, err := ctrl.IsRateLimited(ctx, "1.2.3.4", "nonexistent")
	if err != nil {
		t.Fatalf("IsRateLimited() error = %v", err)
	}
	if limited {
		t.Error("IsRateLimited() = true for non-existent entry, want false")
	}
}

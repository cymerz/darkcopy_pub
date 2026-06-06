package urlgen

import (
	"context"
	"errors"
	"regexp"
	"testing"
)

func TestGenerateSlug_Format(t *testing.T) {
	gen := NewGenerator(nil)
	slug, err := gen.GenerateSlug(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slug) != 8 {
		t.Errorf("expected slug length 8, got %d: %q", len(slug), slug)
	}

	matched, _ := regexp.MatchString(`^[a-z0-9]{8}$`, slug)
	if !matched {
		t.Errorf("slug %q does not match expected pattern [a-z0-9]{8}", slug)
	}
}

func TestGenerateSlug_UniqueResults(t *testing.T) {
	gen := NewGenerator(nil)
	slugs := make(map[string]bool)

	for i := 0; i < 100; i++ {
		slug, err := gen.GenerateSlug(context.Background())
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if slugs[slug] {
			t.Errorf("duplicate slug generated: %q", slug)
		}
		slugs[slug] = true
	}
}

func TestGenerateSlug_RetriesOnCollision(t *testing.T) {
	attempts := 0
	// First 3 calls return "exists", then return "not exists"
	slugExists := func(ctx context.Context, slug string) (bool, error) {
		attempts++
		if attempts <= 3 {
			return true, nil
		}
		return false, nil
	}

	gen := NewGenerator(slugExists)
	slug, err := gen.GenerateSlug(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slug == "" {
		t.Error("expected non-empty slug")
	}

	if attempts != 4 {
		t.Errorf("expected 4 attempts, got %d", attempts)
	}
}

func TestGenerateSlug_ReturnsErrorAfterMaxRetries(t *testing.T) {
	// Always return "exists" to simulate persistent collision
	slugExists := func(ctx context.Context, slug string) (bool, error) {
		return true, nil
	}

	gen := NewGenerator(slugExists)
	_, err := gen.GenerateSlug(context.Background())

	if !errors.Is(err, ErrSlugGenerationFailed) {
		t.Errorf("expected ErrSlugGenerationFailed, got: %v", err)
	}
}

func TestGenerateSlug_ReturnsErrorFromSlugExistsFunc(t *testing.T) {
	expectedErr := errors.New("database error")
	slugExists := func(ctx context.Context, slug string) (bool, error) {
		return false, expectedErr
	}

	gen := NewGenerator(slugExists)
	_, err := gen.GenerateSlug(context.Background())

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected database error, got: %v", err)
	}
}

func TestGenerateSlug_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	gen := NewGenerator(func(ctx context.Context, slug string) (bool, error) {
		return true, nil
	})

	_, err := gen.GenerateSlug(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestGenerateSlug_ExactlyMaxRetriesAttempts(t *testing.T) {
	attempts := 0
	slugExists := func(ctx context.Context, slug string) (bool, error) {
		attempts++
		return true, nil
	}

	gen := NewGenerator(slugExists)
	_, err := gen.GenerateSlug(context.Background())

	if !errors.Is(err, ErrSlugGenerationFailed) {
		t.Fatalf("expected ErrSlugGenerationFailed, got: %v", err)
	}

	if attempts != 10 {
		t.Errorf("expected exactly 10 attempts, got %d", attempts)
	}
}

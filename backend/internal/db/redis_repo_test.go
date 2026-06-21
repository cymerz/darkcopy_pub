package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/gthbn/pastebin/internal/paste"
)

func TestRedisCachingAndBuffering(t *testing.T) {
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
	pasteRepo := NewPasteRepo(nil).WithRedis(rdb)
	fileRepo := NewFileRepo(nil).WithRedis(rdb)

	// Test 1: GetBySlug from cache
	dummyPaste := &paste.Paste{
		ID:         uuid.New(),
		Slug:       "test-cache-slug",
		Title:      "Cached Paste",
		Content:    "Content in cache",
		Visibility: "public",
		CreatedAt:  time.Now().UTC(),
	}
	cachedVal, err := json.Marshal(dummyPaste)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	err = rdb.Set(ctx, "paste:cache:test-cache-slug", cachedVal, 10*time.Minute).Err()
	if err != nil {
		t.Fatalf("redis set failed: %v", err)
	}

	p, err := pasteRepo.GetBySlug(ctx, "test-cache-slug")
	if err != nil {
		t.Fatalf("GetBySlug failed: %v", err)
	}
	if p.Title != dummyPaste.Title || p.Content != dummyPaste.Content {
		t.Errorf("expected cached paste, got %+v", p)
	}

	// Test 2: Invalidate cache on DeletePasteBySlug
	// Clean slate key setup
	err = rdb.Set(ctx, "paste:cache:test-delete-slug", cachedVal, 10*time.Minute).Err()
	if err != nil {
		t.Fatalf("redis set failed: %v", err)
	}

	// Call DeletePasteBySlug. Since pool is nil, this will try database and error/panic,
	// but cache invalidation runs BEFORE the database query. Let's catch/recover or just expect
	// the key to be gone from Redis anyway after. Let's write a mock or wrap with defer to verify key is deleted.
	defer func() {
		exists, _ := rdb.Exists(ctx, "paste:cache:test-delete-slug").Result()
		if exists != 0 {
			t.Errorf("expected cache key to be deleted on deletion attempt")
		}
	}()
	_, _ = pasteRepo.DeletePasteBySlug(ctx, "test-delete-slug")

	// Test 3: IncrementViews buffering
	err = pasteRepo.IncrementViews(ctx, "test-view-slug")
	if err != nil {
		t.Fatalf("IncrementViews failed: %v", err)
	}
	viewCount, err := rdb.HGet(ctx, "paste:views", "test-view-slug").Int()
	if err != nil {
		t.Fatalf("HGet failed: %v", err)
	}
	if viewCount != 1 {
		t.Errorf("expected viewCount = 1, got %d", viewCount)
	}

	// Test 4: IncrementDownloads buffering
	err = fileRepo.IncrementDownloads(ctx, "test-download-slug")
	if err != nil {
		t.Fatalf("IncrementDownloads failed: %v", err)
	}
	downloadCount, err := rdb.HGet(ctx, "file:downloads", "test-download-slug").Int()
	if err != nil {
		t.Fatalf("HGet failed: %v", err)
	}
	if downloadCount != 1 {
		t.Errorf("expected downloadCount = 1, got %d", downloadCount)
	}
}

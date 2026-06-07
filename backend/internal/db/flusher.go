package db

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// StartFlusher runs a periodic background task to flush buffered views and downloads
// from Redis to the PostgreSQL database.
func StartFlusher(ctx context.Context, rdb *redis.Client, pool *pgxpool.Pool, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				flushViews(ctx, rdb, pool, logger)
				flushDownloads(ctx, rdb, pool, logger)
			}
		}
	}()
}

func flushViews(ctx context.Context, rdb *redis.Client, pool *pgxpool.Pool, logger *slog.Logger) {
	// Atomic rename to ensure we don't drop increments that happen during flushing
	err := rdb.Rename(ctx, "paste:views", "paste:views:flush").Err()
	if err == redis.Nil {
		// Nothing to flush
		return
	} else if err != nil {
		// Key might not exist because no increments happened yet
		return
	}

	views, err := rdb.HGetAll(ctx, "paste:views:flush").Result()
	if err != nil {
		logger.Error("failed to HGetAll paste:views:flush", "error", err)
		return
	}

	if len(views) == 0 {
		_ = rdb.Del(ctx, "paste:views:flush")
		return
	}

	// Update DB
	tx, err := pool.Begin(ctx)
	if err != nil {
		logger.Error("failed to start database transaction for views flush", "error", err)
		return
	}
	defer tx.Rollback(ctx)

	for slug, countStr := range views {
		count, cerr := strconv.ParseInt(countStr, 10, 64)
		if cerr != nil {
			logger.Warn("invalid view count in redis", "slug", slug, "value", countStr)
			continue
		}

		_, dbErr := tx.Exec(ctx, "UPDATE pastes SET views = views + $1 WHERE slug = $2", count, slug)
		if dbErr != nil {
			logger.Error("failed to update paste views in transaction", "slug", slug, "error", dbErr)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error("failed to commit views flush transaction", "error", err)
		return
	}

	if err := rdb.Del(ctx, "paste:views:flush").Err(); err != nil {
		logger.Error("failed to delete paste:views:flush key", "error", err)
	}
}

func flushDownloads(ctx context.Context, rdb *redis.Client, pool *pgxpool.Pool, logger *slog.Logger) {
	// Atomic rename to ensure we don't drop increments that happen during flushing
	err := rdb.Rename(ctx, "file:downloads", "file:downloads:flush").Err()
	if err == redis.Nil {
		// Nothing to flush
		return
	} else if err != nil {
		// Key might not exist because no increments happened yet
		return
	}

	downloads, err := rdb.HGetAll(ctx, "file:downloads:flush").Result()
	if err != nil {
		logger.Error("failed to HGetAll file:downloads:flush", "error", err)
		return
	}

	if len(downloads) == 0 {
		_ = rdb.Del(ctx, "file:downloads:flush")
		return
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		logger.Error("failed to start database transaction for downloads flush", "error", err)
		return
	}
	defer tx.Rollback(ctx)

	for slug, countStr := range downloads {
		count, cerr := strconv.ParseInt(countStr, 10, 64)
		if cerr != nil {
			logger.Warn("invalid download count in redis", "slug", slug, "value", countStr)
			continue
		}

		_, dbErr := tx.Exec(ctx, "UPDATE files SET downloads = downloads + $1 WHERE slug = $2", count, slug)
		if dbErr != nil {
			logger.Error("failed to update file downloads in transaction", "slug", slug, "error", dbErr)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error("failed to commit downloads flush transaction", "error", err)
		return
	}

	if err := rdb.Del(ctx, "file:downloads:flush").Err(); err != nil {
		logger.Error("failed to delete file:downloads:flush key", "error", err)
	}
}

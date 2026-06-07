package quota

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCounter implements handler.DailyQuota using Redis.
type RedisCounter struct {
	rdb redis.Cmdable
	now func() time.Time
}

// NewRedisCounter creates a new RedisCounter.
func NewRedisCounter(rdb redis.Cmdable) *RedisCounter {
	return &RedisCounter{
		rdb: rdb,
		now: time.Now,
	}
}

// Allow reports whether an action for key is permitted under limit,
// and if so records it by incrementing the daily Redis key.
func (c *RedisCounter) Allow(key string, limit int) (bool, int) {
	if limit <= 0 {
		return true, -1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Construct daily key: quota:daily:YYYY-MM-DD:key
	day := c.now().UTC().Format("2006-01-02")
	rkey := "quota:daily:" + day + ":" + key

	// Lua script to check limit, increment, and set TTL if new
	var allowScript = redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])

		local current = redis.call('GET', key)
		if current and tonumber(current) >= limit then
			return {0, 0}
		end

		local newVal = redis.call('INCR', key)
		if newVal == 1 then
			redis.call('EXPIRE', key, ttl)
		end

		return {1, limit - newVal}
	`);

	// Calculate TTL until tomorrow UTC 00:00:00 + 2 hours buffer
	now := c.now().UTC()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	ttl := tomorrow.Sub(now) + 2*time.Hour

	res, err := allowScript.Run(ctx, c.rdb, []string{rkey}, limit, int(ttl.Seconds())).Result()
	if err != nil {
		// Log error if needed, but fall back to allowing to prevent system blackout
		return true, -1
	}

	results, ok := res.([]interface{})
	if !ok || len(results) < 2 {
		return true, -1
	}

	allowed := results[0].(int64) == 1
	remaining := int(results[1].(int64))
	return allowed, remaining
}

// RedisSizeCounter implements handler.DailySizeQuota using Redis.
type RedisSizeCounter struct {
	rdb redis.Cmdable
	now func() time.Time
}

// NewRedisSizeCounter creates a new RedisSizeCounter.
func NewRedisSizeCounter(rdb redis.Cmdable) *RedisSizeCounter {
	return &RedisSizeCounter{
		rdb: rdb,
		now: time.Now,
	}
}

// Allow reports whether an upload of size bytes for key is permitted under limitBytes,
// and if so records it by adding the size to the daily Redis key.
func (c *RedisSizeCounter) Allow(key string, size int64, limitBytes int64) (bool, int64) {
	if limitBytes <= 0 {
		return true, -1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Construct daily key: quota:size:YYYY-MM-DD:key
	day := c.now().UTC().Format("2006-01-02")
	rkey := "quota:size:" + day + ":" + key

	// Lua script to check size limit, increment, and set TTL if new
	var sizeScript = redis.NewScript(`
		local key = KEYS[1]
		local size = tonumber(ARGV[1])
		local limit = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])

		local current = redis.call('GET', key)
		local currentVal = 0
		if current then
			currentVal = tonumber(current)
		end

		if currentVal + size > limit then
			return {0, limit - currentVal}
		end

		local newVal = redis.call('INCRBY', key, size)
		if currentVal == 0 then
			redis.call('EXPIRE', key, ttl)
		end

		return {1, limit - newVal}
	`);

	// Calculate TTL until tomorrow UTC 00:00:00 + 2 hours buffer
	now := c.now().UTC()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	ttl := tomorrow.Sub(now) + 2*time.Hour

	res, err := sizeScript.Run(ctx, c.rdb, []string{rkey}, size, limitBytes, int(ttl.Seconds())).Result()
	if err != nil {
		// Fallback to allowing in case of Redis failure
		return true, -1
	}

	results, ok := res.([]interface{})
	if !ok || len(results) < 2 {
		return true, -1
	}

	allowed := results[0].(int64) == 1
	remaining := results[1].(int64)
	return allowed, remaining
}

// Package quota provides simple in-memory per-IP daily counters used to enforce
// "N actions per IP per day" limits (e.g. max pastes/uploads per day). Counts
// reset at UTC midnight and are kept in memory only — they are best-effort and
// reset on restart, which is acceptable for abuse mitigation.
package quota

import (
	"sync"
	"time"
)

// Counter tracks how many times each key (typically an IP + action) has acted
// on the current UTC day.
type Counter struct {
	mu    sync.Mutex
	day   string
	counts map[string]int
	now   func() time.Time
}

// NewCounter creates an empty Counter.
func NewCounter() *Counter {
	return &Counter{
		counts: make(map[string]int),
		now:    time.Now,
	}
}

// dayKey returns the current UTC day as YYYY-MM-DD.
func (c *Counter) dayKey() string {
	return c.now().UTC().Format("2006-01-02")
}

// rollIfNeeded resets all counts when the UTC day has changed. Caller must hold
// the lock.
func (c *Counter) rollIfNeeded() {
	today := c.dayKey()
	if c.day != today {
		c.day = today
		c.counts = make(map[string]int)
	}
}

// Allow reports whether an action for key is permitted given limit, and if so
// records it (incrementing the count). A limit of 0 means unlimited. The
// returned remaining is how many actions are left after this one (or -1 when
// unlimited).
func (c *Counter) Allow(key string, limit int) (allowed bool, remaining int) {
	if limit <= 0 {
		return true, -1
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.rollIfNeeded()

	if c.counts[key] >= limit {
		return false, 0
	}
	c.counts[key]++
	return true, limit - c.counts[key]
}

// Count returns the current count for key on the current UTC day.
func (c *Counter) Count(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rollIfNeeded()
	return c.counts[key]
}

// SizeCounter tracks total bytes uploaded on the current UTC day.
type SizeCounter struct {
	mu     sync.Mutex
	day    string
	counts map[string]int64
	now    func() time.Time
}

// NewSizeCounter creates an empty SizeCounter.
func NewSizeCounter() *SizeCounter {
	return &SizeCounter{
		counts: make(map[string]int64),
		now:    time.Now,
	}
}

func (c *SizeCounter) dayKey() string {
	return c.now().UTC().Format("2006-01-02")
}

func (c *SizeCounter) rollIfNeeded() {
	today := c.dayKey()
	if c.day != today {
		c.day = today
		c.counts = make(map[string]int64)
	}
}

// Allow reports whether an upload of size bytes for key is permitted given limitBytes,
// and if so records it (adding the size to the count). A limitBytes of 0 means unlimited.
// The returned remaining is how many bytes are left after this one (or -1 when unlimited).
func (c *SizeCounter) Allow(key string, size int64, limitBytes int64) (allowed bool, remaining int64) {
	if limitBytes <= 0 {
		return true, -1
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.rollIfNeeded()

	if c.counts[key]+size > limitBytes {
		return false, limitBytes - c.counts[key]
	}
	c.counts[key] += size
	return true, limitBytes - c.counts[key]
}

// Count returns the current total bytes for key on the current UTC day.
func (c *SizeCounter) Count(key string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rollIfNeeded()
	return c.counts[key]
}

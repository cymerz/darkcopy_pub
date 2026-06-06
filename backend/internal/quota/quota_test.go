package quota

import (
	"testing"
	"time"
)

func TestAllow_UnlimitedWhenLimitZero(t *testing.T) {
	c := NewCounter()
	for i := 0; i < 1000; i++ {
		if allowed, remaining := c.Allow("ip", 0); !allowed || remaining != -1 {
			t.Fatalf("limit 0 should be unlimited; got allowed=%v remaining=%d", allowed, remaining)
		}
	}
}

func TestAllow_EnforcesLimit(t *testing.T) {
	c := NewCounter()
	limit := 3

	for i := 1; i <= limit; i++ {
		allowed, remaining := c.Allow("ip", limit)
		if !allowed {
			t.Fatalf("action %d should be allowed", i)
		}
		if remaining != limit-i {
			t.Errorf("action %d: expected remaining %d, got %d", i, limit-i, remaining)
		}
	}

	// Next one must be denied.
	if allowed, remaining := c.Allow("ip", limit); allowed || remaining != 0 {
		t.Errorf("expected denial after limit reached; got allowed=%v remaining=%d", allowed, remaining)
	}
}

func TestAllow_KeysAreIndependent(t *testing.T) {
	c := NewCounter()
	c.Allow("a", 1)
	if allowed, _ := c.Allow("b", 1); !allowed {
		t.Error("different key should have its own counter")
	}
	if allowed, _ := c.Allow("a", 1); allowed {
		t.Error("key 'a' should already be at its limit")
	}
}

func TestAllow_ResetsOnNewDay(t *testing.T) {
	c := NewCounter()
	day1 := time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC)
	day2 := time.Date(2024, 1, 2, 0, 5, 0, 0, time.UTC)

	c.now = func() time.Time { return day1 }
	if allowed, _ := c.Allow("ip", 1); !allowed {
		t.Fatal("first action on day1 should be allowed")
	}
	if allowed, _ := c.Allow("ip", 1); allowed {
		t.Fatal("second action on day1 should be denied")
	}

	// New UTC day → counters reset.
	c.now = func() time.Time { return day2 }
	if allowed, _ := c.Allow("ip", 1); !allowed {
		t.Error("action on day2 should be allowed after reset")
	}
}

func TestCount(t *testing.T) {
	c := NewCounter()
	c.Allow("ip", 5)
	c.Allow("ip", 5)
	if got := c.Count("ip"); got != 2 {
		t.Errorf("expected count 2, got %d", got)
	}
}

func TestSizeCounter_EnforcesLimitAndResets(t *testing.T) {
	sc := NewSizeCounter()
	limit := int64(100)

	// First 40 bytes allowed
	if allowed, remaining := sc.Allow("ip", 40, limit); !allowed || remaining != 60 {
		t.Errorf("expected allowed, got allowed=%v, remaining=%d", allowed, remaining)
	}

	// Another 50 bytes allowed
	if allowed, remaining := sc.Allow("ip", 50, limit); !allowed || remaining != 10 {
		t.Errorf("expected allowed, got allowed=%v, remaining=%d", allowed, remaining)
	}

	// 20 bytes denied (exceeds limit 100)
	if allowed, remaining := sc.Allow("ip", 20, limit); allowed || remaining != 10 {
		t.Errorf("expected denied, got allowed=%v, remaining=%d", allowed, remaining)
	}

	// Unlimited check
	if allowed, remaining := sc.Allow("ip2", 1000, 0); !allowed || remaining != -1 {
		t.Errorf("expected allowed for unlimited, got allowed=%v, remaining=%d", allowed, remaining)
	}

	// Check day roll-over
	day1 := time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC)
	day2 := time.Date(2024, 1, 2, 0, 5, 0, 0, time.UTC)

	sc.now = func() time.Time { return day1 }
	if allowed, _ := sc.Allow("day", 80, 100); !allowed {
		t.Fatal("expected day1 allowed")
	}
	if allowed, _ := sc.Allow("day", 30, 100); allowed {
		t.Fatal("expected day1 denied (exceeds 100)")
	}

	sc.now = func() time.Time { return day2 }
	if allowed, _ := sc.Allow("day", 50, 100); !allowed {
		t.Error("expected day2 allowed after reset")
	}
}

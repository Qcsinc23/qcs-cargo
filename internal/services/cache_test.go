package services

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// TestMemoryCache_BoundedSize is the MED-10 regression test. The
// previous implementation grew the underlying map without bound; the
// fix caps the entry count at MemoryCache.cap and evicts LRU entries
// once the cap is reached.
func TestMemoryCache_BoundedSize(t *testing.T) {
	c := NewMemoryCacheWithCap(128)
	ctx := context.Background()

	const writes = 5000
	for i := 0; i < writes; i++ {
		if err := c.Set(ctx, "k:"+strconv.Itoa(i), []byte("v"), time.Minute); err != nil {
			t.Fatalf("set %d: %v", i, err)
		}
	}

	if got := c.Len(); got > c.Cap() {
		t.Fatalf("expected bounded size <= %d after %d writes, got %d", c.Cap(), writes, got)
	}
	if got := c.Len(); got == 0 {
		t.Fatalf("expected cache to retain entries, got 0")
	}
}

// TestMemoryCache_EvictsOldestFirst confirms the eviction order: the
// oldest entry that has not been touched should be the first to go
// when the cap is reached.
func TestMemoryCache_EvictsOldestFirst(t *testing.T) {
	c := NewMemoryCacheWithCap(3)
	ctx := context.Background()

	for _, k := range []string{"a", "b", "c"} {
		if err := c.Set(ctx, k, []byte(k), time.Minute); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}

	// Inserting a 4th key must evict "a" (oldest).
	if err := c.Set(ctx, "d", []byte("d"), time.Minute); err != nil {
		t.Fatalf("set d: %v", err)
	}

	if _, ok, _ := c.Get(ctx, "a"); ok {
		t.Fatalf("expected oldest key 'a' to be evicted")
	}
	for _, k := range []string{"b", "c", "d"} {
		if _, ok, _ := c.Get(ctx, k); !ok {
			t.Fatalf("expected key %q to be retained", k)
		}
	}
}

// TestMemoryCache_GetPromotesLRU verifies that Get on an entry moves it
// to the most-recently-used end of the list, so subsequent eviction
// drops a different key.
func TestMemoryCache_GetPromotesLRU(t *testing.T) {
	c := NewMemoryCacheWithCap(3)
	ctx := context.Background()

	for _, k := range []string{"a", "b", "c"} {
		if err := c.Set(ctx, k, []byte(k), time.Minute); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}

	// Touch "a" so it is no longer the oldest.
	if _, ok, _ := c.Get(ctx, "a"); !ok {
		t.Fatalf("expected to read 'a' before eviction")
	}

	// Inserting a 4th key now evicts "b" (the new oldest).
	if err := c.Set(ctx, "d", []byte("d"), time.Minute); err != nil {
		t.Fatalf("set d: %v", err)
	}

	if _, ok, _ := c.Get(ctx, "b"); ok {
		t.Fatalf("expected 'b' to be evicted after touching 'a'")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok, _ := c.Get(ctx, k); !ok {
			t.Fatalf("expected key %q to be retained", k)
		}
	}
}

// TestMemoryCache_SetUpdatesInPlace ensures that overwriting an existing
// key does not leak LRU list entries and does not evict other keys.
func TestMemoryCache_SetUpdatesInPlace(t *testing.T) {
	c := NewMemoryCacheWithCap(2)
	ctx := context.Background()

	_ = c.Set(ctx, "a", []byte("1"), time.Minute)
	_ = c.Set(ctx, "b", []byte("1"), time.Minute)

	for i := 0; i < 100; i++ {
		_ = c.Set(ctx, "a", []byte(strconv.Itoa(i)), time.Minute)
	}

	if got := c.Len(); got != 2 {
		t.Fatalf("expected 2 entries after repeated overwrite, got %d", got)
	}
	if _, ok, _ := c.Get(ctx, "b"); !ok {
		t.Fatalf("expected 'b' to survive overwrites of 'a'")
	}
	val, ok, _ := c.Get(ctx, "a")
	if !ok || string(val) != "99" {
		t.Fatalf("expected latest value for 'a', got %q ok=%v", string(val), ok)
	}
}

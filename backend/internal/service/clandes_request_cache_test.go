//go:build unit

package service

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestClandesRequestCache_GetAndDelete_ReturnsReleaseFunc locks in the fix for the
// silent slot leak where getAndDelete used to nil out ctx.ReleaseFunc on the shared
// pointer before returning it to the caller, causing the caller's nil-check to
// always skip the release and leave concurrency slots held until the 15-minute
// Redis TTL swept them.
func TestClandesRequestCache_GetAndDelete_ReturnsReleaseFunc(t *testing.T) {
	t.Parallel()

	c := newClandesRequestCache()
	var released atomic.Int32
	c.set("req-1", &clandesRequestContext{
		StartTime:   time.Now(),
		ReleaseFunc: func() { released.Add(1) },
	})

	ctx, release, ok := c.getAndDelete("req-1")
	require.True(t, ok, "cache hit expected")
	require.NotNil(t, ctx, "context pointer expected")
	require.NotNil(t, release, "release closure expected")
	require.Equal(t, int32(0), released.Load(), "release must not fire before caller invokes it")

	release()
	require.Equal(t, int32(1), released.Load(), "release must fire exactly once")

	// Subsequent lookups must miss (entry deleted from cache).
	_, _, ok = c.getAndDelete("req-1")
	require.False(t, ok, "entry must be deleted after getAndDelete")
}

// TestClandesRequestCache_GetAndDelete_DoesNotDoubleRelease asserts that
// getAndDelete transfers ownership: after it returns, the entry's OnEvicted
// callback (fired by cache.Delete) must not re-invoke the release closure.
func TestClandesRequestCache_GetAndDelete_DoesNotDoubleRelease(t *testing.T) {
	t.Parallel()

	c := newClandesRequestCache()
	var released atomic.Int32
	c.set("req-1", &clandesRequestContext{
		StartTime:   time.Now(),
		ReleaseFunc: func() { released.Add(1) },
	})

	_, release, ok := c.getAndDelete("req-1")
	require.True(t, ok)
	require.NotNil(t, release)
	release()

	// OnEvicted must have observed ReleaseFunc=nil and stayed silent.
	require.Equal(t, int32(1), released.Load(), "release must fire exactly once across getAndDelete + OnEvicted")
}

// TestClandesRequestCache_GetAndDelete_Miss returns (nil, nil, false) for
// unknown request IDs — protecting callers from nil deref on `release`.
func TestClandesRequestCache_GetAndDelete_Miss(t *testing.T) {
	t.Parallel()

	c := newClandesRequestCache()
	ctx, release, ok := c.getAndDelete("does-not-exist")
	require.False(t, ok)
	require.Nil(t, ctx)
	require.Nil(t, release)
}

// TestClandesRequestCache_FlushAll_ReleasesAll covers the disconnect cleanup path.
func TestClandesRequestCache_FlushAll_ReleasesAll(t *testing.T) {
	t.Parallel()

	c := newClandesRequestCache()
	var released atomic.Int32
	for _, id := range []string{"a", "b", "c"} {
		c.set(id, &clandesRequestContext{
			StartTime:   time.Now(),
			ReleaseFunc: func() { released.Add(1) },
		})
	}

	n := c.flushAll()
	require.Equal(t, 3, n, "flushAll should release every in-flight slot")
	require.Equal(t, int32(3), released.Load())

	// Second call sees an empty cache and releases nothing.
	require.Equal(t, 0, c.flushAll())
	require.Equal(t, int32(3), released.Load())
}

// TestClandesRequestCache_FlushAll_DoesNotDoubleReleaseOnEviction makes sure that
// entries cleared by flushAll don't re-fire via the go-cache OnEvicted callback.
func TestClandesRequestCache_FlushAll_DoesNotDoubleReleaseOnEviction(t *testing.T) {
	t.Parallel()

	c := newClandesRequestCache()
	var released atomic.Int32
	c.set("a", &clandesRequestContext{
		StartTime:   time.Now(),
		ReleaseFunc: func() { released.Add(1) },
	})

	require.Equal(t, 1, c.flushAll())
	require.Equal(t, int32(1), released.Load())
}

// TestClandesRequestCache_TTLAlignment asserts the TTL constant matches the Redis
// slot TTL so late ReportUsage calls still find a live cache entry instead of
// falling through to "no cached context" while the slot is still held in Redis.
func TestClandesRequestCache_TTLAlignment(t *testing.T) {
	t.Parallel()
	require.Equal(t, 15*time.Minute, clandesRequestCacheTTL,
		"clandes reqCache TTL must match concurrency_cache.defaultSlotTTLMinutes (15m)")
}
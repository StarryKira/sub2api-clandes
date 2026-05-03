package service

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// clandesRequestContext stores the billing context for an in-flight request routed through clandes.
// It is keyed by the requestId from routeRequest and consumed in reportUsage.
type clandesRequestContext struct {
	APIKey       *APIKey
	User         *User
	Account      *Account
	Subscription *UserSubscription
	GroupID      *int64
	StartTime    time.Time
	UserAgent    string
	ReleaseFunc  func() // concurrency slot release; called in ReportUsage or on disconnect cleanup
}

// clandesRequestCache is a short-lived cache bridging routeRequest and reportUsage calls.
// TTL matches the Redis concurrency slot TTL (see concurrency_cache.go defaultSlotTTLMinutes):
// if the cache expired before ReportUsage arrived, the cached billing context is discarded
// and the concurrency slot would have to be reclaimed from Redis by age — keeping the two
// horizons aligned means the in-process OnEvicted path stays authoritative for the entire
// lifetime of the slot, so late ReportUsage calls never produce "no cached context" drops
// while the slot is still considered alive server-side.
type clandesRequestCache struct {
	cache *gocache.Cache
}

const clandesRequestCacheTTL = 15 * time.Minute

func newClandesRequestCache() *clandesRequestCache {
	c := gocache.New(clandesRequestCacheTTL, clandesRequestCacheTTL*2)
	// Release concurrency slots when entries expire without a ReportUsage call
	// (e.g. clandes crashed mid-request and never reported back).
	c.OnEvicted(func(_ string, v any) {
		if ctx, ok := v.(*clandesRequestContext); ok && ctx.ReleaseFunc != nil {
			ctx.ReleaseFunc()
		}
	})
	return &clandesRequestCache{cache: c}
}

func (c *clandesRequestCache) set(requestID string, ctx *clandesRequestContext) {
	c.cache.Set(requestID, ctx, clandesRequestCacheTTL)
}

// getAndDelete retrieves and removes the context atomically.
// Returns (nil, nil, false) if not found.
//
// The release closure (may be nil for entries whose slot was already reclaimed) is returned
// as a separate value because go-cache's OnEvicted callback fires on Delete as well as on
// TTL eviction; if the release closure were left on the returned struct, the OnEvicted
// hook would double-release it. The caller invokes the returned release exactly once.
func (c *clandesRequestCache) getAndDelete(requestID string) (*clandesRequestContext, func(), bool) {
	v, ok := c.cache.Get(requestID)
	if !ok {
		return nil, nil, false
	}
	ctx, castOK := v.(*clandesRequestContext)
	if !castOK || ctx == nil {
		c.cache.Delete(requestID)
		return nil, nil, false
	}
	release := ctx.ReleaseFunc
	ctx.ReleaseFunc = nil // hand ownership to caller; OnEvicted now sees nil
	c.cache.Delete(requestID)
	return ctx, release, true
}

// flushAll releases all in-flight concurrency slots and clears the cache.
// Called when the clandes connection drops to clean up orphaned slots.
func (c *clandesRequestCache) flushAll() int {
	items := c.cache.Items()
	count := 0
	for key, item := range items {
		if ctx, ok := item.Object.(*clandesRequestContext); ok && ctx.ReleaseFunc != nil {
			ctx.ReleaseFunc()
			ctx.ReleaseFunc = nil // prevent OnEvicted from double-releasing
			count++
		}
		c.cache.Delete(key)
	}
	return count
}

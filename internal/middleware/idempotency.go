package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Pass 2 audit fix M-9 (server side): in-memory idempotency cache for the
// warehouse mutating endpoints. Pairs with the per-entry Idempotency-Key the
// service worker emits on offline replay (internal/static/sw.js). When a
// replay arrives with a key the server has already processed within
// idempotencyTTL, the cached response is returned and the handler is not
// re-run, preventing duplicate writes (e.g. a manifest created twice).
//
// The cache is intentionally process-local. With the current single-replica
// deployment that is sufficient. If/when the deployment is scaled
// horizontally, swap in a Redis-backed implementation that uses the same key
// space ("idemp:" + key).

const idempotencyTTL = 1 * time.Hour
const idempotencyMaxKeys = 4096

type idempotencyEntry struct {
	createdAt time.Time
	status    int
	body      []byte
}

var (
	idempotencyMu    sync.Mutex
	idempotencyCache = map[string]idempotencyEntry{}
)

// IdempotencyMiddleware short-circuits handlers when the request supplies an
// Idempotency-Key header that has been seen recently. Only POST/PATCH requests
// are considered. Bodies of >256KB are not cached (we still let the request
// through but do not deduplicate it). Errors (status >= 500) are not cached
// so the client can legitimately retry.
func IdempotencyMiddleware(c *fiber.Ctx) error {
	if c.Method() != fiber.MethodPost && c.Method() != fiber.MethodPatch {
		return c.Next()
	}
	key := strings.TrimSpace(c.Get("Idempotency-Key"))
	if key == "" {
		return c.Next()
	}
	scoped := c.Path() + "|" + key

	idempotencyMu.Lock()
	if entry, ok := idempotencyCache[scoped]; ok && time.Since(entry.createdAt) < idempotencyTTL {
		idempotencyMu.Unlock()
		c.Set("X-Idempotent-Replay", "true")
		c.Status(entry.status)
		if len(entry.body) > 0 {
			return c.Send(entry.body)
		}
		return nil
	}
	idempotencyMu.Unlock()

	if err := c.Next(); err != nil {
		return err
	}

	status := c.Response().StatusCode()
	if status >= 200 && status < 500 {
		body := append([]byte(nil), c.Response().Body()...)
		if len(body) <= 256*1024 {
			idempotencyMu.Lock()
			if len(idempotencyCache) >= idempotencyMaxKeys {
				// DEF-011 (backlog) fix: drop the single oldest entry on
				// each insert past the cap. O(n) per eviction but only
				// triggered when full, instead of the previous "scan the
				// entire map every insert past the cap" pattern that
				// produced consistent O(n) work under sustained load.
				evictOldestLocked()
			}
			idempotencyCache[scoped] = idempotencyEntry{
				createdAt: time.Now(),
				status:    status,
				body:      body,
			}
			idempotencyMu.Unlock()
		}
	}
	return nil
}

// evictOldestLocked removes the single oldest entry from idempotencyCache.
// Caller must hold idempotencyMu.
func evictOldestLocked() {
	var oldestKey string
	var oldestAt time.Time
	for k, v := range idempotencyCache {
		if oldestKey == "" || v.createdAt.Before(oldestAt) {
			oldestKey = k
			oldestAt = v.createdAt
		}
	}
	if oldestKey != "" {
		delete(idempotencyCache, oldestKey)
	}
}

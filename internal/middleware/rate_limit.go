package middleware

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// rateLimiter is a simple in-memory rate limiter
type rateLimiter struct {
	attempts map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

type lockoutEntry struct {
	failures    []time.Time
	lockedUntil time.Time
}

type authLockoutTracker struct {
	entries    map[string]lockoutEntry
	mu         sync.RWMutex
	threshold  int
	window     time.Duration
	lockPeriod time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func newAuthLockoutTracker(threshold int, window, lockPeriod time.Duration) *authLockoutTracker {
	return &authLockoutTracker{
		entries:    make(map[string]lockoutEntry),
		threshold:  threshold,
		window:     window,
		lockPeriod: lockPeriod,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old attempts
	valid := []time.Time{}
	for _, t := range rl.attempts[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		delete(rl.attempts, key)
	} else {
		rl.attempts[key] = valid
	}

	if len(valid) >= rl.limit {
		return false
	}
	rl.attempts[key] = append(rl.attempts[key], now)
	return true
}

func (rl *rateLimiter) remaining(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	count := 0
	for _, t := range rl.attempts[key] {
		if t.After(cutoff) {
			count++
		}
	}

	remaining := rl.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Global rate limiters for auth endpoints
var (
	ipLimiter      = newRateLimiter(10, time.Hour)
	emailLimiter   = newRateLimiter(5, time.Hour)
	authAttemptLog = newAuthLockoutTracker(5, 15*time.Minute, 30*time.Minute)
)

// ResetAuthRateLimitersForTest clears in-memory limiter state between tests.
func ResetAuthRateLimitersForTest() {
	ipLimiter.mu.Lock()
	ipLimiter.attempts = make(map[string][]time.Time)
	ipLimiter.mu.Unlock()

	emailLimiter.mu.Lock()
	emailLimiter.attempts = make(map[string][]time.Time)
	emailLimiter.mu.Unlock()

	authAttemptLog.mu.Lock()
	authAttemptLog.entries = make(map[string]lockoutEntry)
	authAttemptLog.mu.Unlock()
}

// containsAuthRoute checks if the path is an auth-related route
func containsAuthRoute(path string) bool {
	return path == "/api/v1/auth" || strings.HasPrefix(path, "/api/v1/auth/")
}

// AuthRateLimit creates rate limiter for auth endpoints: 5 per email per hour, 10 per IP per hour
func AuthRateLimit(c *fiber.Ctx) error {
	// Skip for non-auth routes
	path := c.Path()
	if !containsAuthRoute(path) {
		return c.Next()
	}

	ip := c.IP()

	// Check IP-based rate limit
	if !ipLimiter.allow(ip) {
		remaining := ipLimiter.remaining(ip)
		c.Set("X-RateLimit-Limit", "10")
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Set("X-RateLimit-Reset", time.Now().Add(time.Hour).Format(time.RFC3339))
		return c.Status(429).JSON(fiber.Map{
			"error": fiber.Map{"code": "RATE_LIMITED", "message": "Too many requests from this IP. Please try again later."},
		})
	}

	// For POST requests with email in body
	if c.Method() == "POST" {
		var body struct {
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err == nil && body.Email != "" {
			emailKey := strings.ToLower(strings.TrimSpace(body.Email))
			if !emailLimiter.allow(emailKey) {
				remaining := emailLimiter.remaining(emailKey)
				c.Set("X-RateLimit-Limit", "5")
				c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
				c.Set("X-RateLimit-Reset", time.Now().Add(time.Hour).Format(time.RFC3339))
				return c.Status(429).JSON(fiber.Map{
					"error": fiber.Map{"code": "RATE_LIMITED", "message": "Too many attempts for this email. Please try again later."},
				})
			}
		}
	}

	return c.Next()
}

// CheckAuthAttemptLockout returns whether a key is currently locked and when lockout expires.
func CheckAuthAttemptLockout(key string) (bool, time.Time) {
	authAttemptLog.mu.Lock()
	defer authAttemptLog.mu.Unlock()

	key = strings.TrimSpace(key)
	if key == "" {
		return false, time.Time{}
	}
	entry, ok := authAttemptLog.entries[key]
	if !ok {
		return false, time.Time{}
	}
	now := time.Now()
	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		return true, entry.lockedUntil
	}
	// Clean stale/expired entry.
	delete(authAttemptLog.entries, key)
	return false, time.Time{}
}

// RecordAuthAttemptFailure records a failed auth attempt and applies lockout when threshold is exceeded.
func RecordAuthAttemptFailure(key string) {
	authAttemptLog.mu.Lock()
	defer authAttemptLog.mu.Unlock()

	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	now := time.Now()
	entry := authAttemptLog.entries[key]
	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		authAttemptLog.entries[key] = entry
		return
	}

	cutoff := now.Add(-authAttemptLog.window)
	filtered := make([]time.Time, 0, len(entry.failures)+1)
	for _, t := range entry.failures {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	filtered = append(filtered, now)
	entry.failures = filtered
	if len(filtered) >= authAttemptLog.threshold {
		entry.lockedUntil = now.Add(authAttemptLog.lockPeriod)
	}
	authAttemptLog.entries[key] = entry
}

// ClearAuthAttemptFailures clears lockout/failure state for a successful auth attempt key.
func ClearAuthAttemptFailures(key string) {
	authAttemptLog.mu.Lock()
	defer authAttemptLog.mu.Unlock()
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	delete(authAttemptLog.entries, key)
}

package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthAttemptLockoutLifecycle(t *testing.T) {
	key := "test-lockout-key"
	ClearAuthAttemptFailures(key)
	locked, _ := CheckAuthAttemptLockout(key)
	assert.False(t, locked)

	for i := 0; i < 5; i++ {
		RecordAuthAttemptFailure(key)
	}
	locked, until := CheckAuthAttemptLockout(key)
	assert.True(t, locked)
	assert.True(t, until.After(time.Now()))

	ClearAuthAttemptFailures(key)
	locked, _ = CheckAuthAttemptLockout(key)
	assert.False(t, locked)
}

// TestAuthRateLimit_DoesNotConsumePerEmailQuota is the SEC-002 regression
// test. Before the fix, AuthRateLimit parsed the request body and consumed
// a per-email quota for any POST containing an "email" field. An attacker
// could therefore lock the legitimate user out of recovery flows by burning
// their per-email allowance from a different IP. After the fix, only the
// per-IP coarse limiter applies in middleware; per-account throttling is
// done in handlers via services.CheckAndRecordAuthRequest. We assert this
// by sending many POSTs for one email from one IP, then confirming that a
// fresh IP for the same email is not blocked.
func TestAuthRateLimit_DoesNotConsumePerEmailQuota(t *testing.T) {
	ResetAuthRateLimitersForTest()
	app := fiber.New()
	app.Use(AuthRateLimit)
	app.Post("/api/v1/auth/magic-link/request", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	attackerIP := "198.51.100.20:1234"
	body := []byte(`{"email":"victim@example.com"}`)
	for i := 0; i < 9; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link/request", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = attackerIP
		resp, err := app.Test(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	victimIP := "198.51.100.21:1234"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link/request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = victimIP
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode,
		"a fresh IP must not be blocked just because the email was seen from another IP",
	)
}


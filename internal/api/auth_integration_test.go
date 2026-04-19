//go:build integration

package api_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/stretchr/testify/require"
)

// TestForgotPassword_TimingConstant exercises Pass 3 audit fix DEF-009.
//
// The forgot-password handler used to short-circuit when the supplied
// email did not exist (cheap: ~1ms) but performed a DB write + email
// enqueue when it did exist (expensive: tens to hundreds of ms). The
// delta let an attacker enumerate registered emails by timing the
// response. The fix forces every request through one bcrypt comparison
// at cost 12 (~250ms) and pushes the email-send step to a background
// goroutine, so the handler returns in roughly the same wall-clock
// time regardless of whether the account exists.
//
// This test verifies:
//  1. The HTTP status code and response body bytes are byte-identical
//     between the existing-email and missing-email branches on every
//     iteration (no enumeration via response shape).
//  2. The median wall-clock time for the existing-email branch and the
//     missing-email branch differ by at most 50% of the larger median
//     (no enumeration via response timing).
func TestForgotPassword_TimingConstant(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	app := setupTestApp(t)

	const iterations = 5

	existingEmails := make([]string, 0, iterations)
	for i := 0; i < iterations; i++ {
		email := fmt.Sprintf("forgot-timing-%d@example.com", i)
		_, err := services.Register(
			context.Background(),
			fmt.Sprintf("Forgot Timing %d", i),
			email,
			"+15551234567",
			"StrongPass1!",
		)
		require.NoError(t, err, "pre-register iteration %d", i)
		existingEmails = append(existingEmails, email)
	}

	// Warm up: prime any lazy globals (sql prepared stmts, bcrypt
	// asm dispatch, fiber router caches) with one unrelated POST so
	// the first measured request below is not unfairly slow.
	warmReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/password/forgot",
		bytes.NewReader([]byte(`{"email":"warmup@example.com"}`)),
	)
	warmReq.Header.Set("Content-Type", "application/json")
	warmResp, err := app.Test(warmReq, 10000)
	require.NoError(t, err, "warmup request")
	_, _ = io.Copy(io.Discard, warmResp.Body)
	_ = warmResp.Body.Close()

	resetThrottleState(t)

	exists := make([]time.Duration, 0, iterations)
	missing := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		resetThrottleState(t)

		statusExists, bodyExists, durExists := timeForgotPassword(t, app, existingEmails[i])
		statusMissing, bodyMissing, durMissing := timeForgotPassword(
			t, app,
			fmt.Sprintf("does-not-exist-%d@example.com", i),
		)

		require.Equalf(t, statusExists, statusMissing,
			"iteration %d: status code differs (exists=%d missing=%d) — would leak account existence",
			i, statusExists, statusMissing,
		)
		require.Equalf(t, bodyExists, bodyMissing,
			"iteration %d: response body bytes differ — would leak account existence\nexists=%q\nmissing=%q",
			i, string(bodyExists), string(bodyMissing),
		)

		exists = append(exists, durExists)
		missing = append(missing, durMissing)
	}

	medExists := median(exists)
	medMissing := median(missing)

	larger := medExists
	if medMissing > larger {
		larger = medMissing
	}
	delta := medExists - medMissing
	if delta < 0 {
		delta = -delta
	}
	threshold := larger / 2

	t.Logf(
		"forgot-password timing: median(exists)=%s median(missing)=%s delta=%s threshold(50%% of larger)=%s",
		medExists, medMissing, delta, threshold,
	)

	require.LessOrEqualf(t, delta, threshold,
		"forgot-password timing channel exceeds 50%% of larger median: median(exists)=%s median(missing)=%s delta=%s threshold=%s — DEF-009 regression",
		medExists, medMissing, delta, threshold,
	)
}

// timeForgotPassword issues one POST /api/v1/auth/password/forgot and
// returns the status code, fully-buffered response body, and the
// wall-clock duration of the call. The 10s timeout is well above the
// expected ~250ms (bcrypt cost 12) plus DB lookup so we never trip the
// Fiber default 1s test timeout.
func timeForgotPassword(t *testing.T, app interface {
	Test(req *http.Request, msTimeout ...int) (*http.Response, error)
}, email string) (int, []byte, time.Duration) {
	t.Helper()
	payload := []byte(fmt.Sprintf(`{"email":%q}`, email))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/forgot", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := app.Test(req, 10000)
	require.NoErrorf(t, err, "forgot-password request for %s", email)
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	dur := time.Since(start)
	require.NoErrorf(t, readErr, "read body for %s", email)
	return resp.StatusCode, body, dur
}

// resetThrottleState clears every rate-limit surface that gates the
// forgot-password endpoint:
//
//   - auth_request_log (services.CheckAndRecordAuthRequest):
//     per-account 3/15min and per-IP 20/15min on the bucket name.
//   - middleware.ipLimiter (10/hour per source IP) and the auth
//     attempt-failure tracker, both reset via the package-private
//     ResetAuthRateLimitersForTest helper.
//
// Without resetting all three the test would start returning 429 mid-loop
// (httptest re-uses 0.0.0.0 / 127.0.0.1 for every request, so all calls
// share the same IP bucket) and we would no longer be measuring the
// bcrypt path.
func resetThrottleState(t *testing.T) {
	t.Helper()
	_, err := db.DB().Exec(`DELETE FROM auth_request_log`)
	require.NoError(t, err, "reset auth_request_log")
	middleware.ResetAuthRateLimitersForTest()
}

func median(xs []time.Duration) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(xs))
	copy(sorted, xs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

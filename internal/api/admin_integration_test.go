//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustAdminAccessToken(t *testing.T) string {
	t.Helper()

	secret := os.Getenv("JWT_SECRET")
	require.GreaterOrEqual(t, len(secret), 32)

	claims := services.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			Subject:   testdata.AdminID,
			ID:        uuid.NewString(),
		},
		UserID: testdata.AdminID,
		Email:  "admin@qcs-cargo.com",
		Role:   "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Pass 2 audit fix M-2: use the full JWT secret (no truncation).
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func seedObservabilityInsightsEvents(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339)
	events := []struct {
		category   string
		eventName  string
		userID     interface{}
		requestID  interface{}
		path       interface{}
		method     interface{}
		statusCode interface{}
		durationMS interface{}
		value      interface{}
	}{
		{"analytics", "page_view", testdata.AdminID, "req-analytics-1", "/api/v1/admin/dashboard", "GET", 200, 45.0, nil},
		{"analytics", "search_used", testdata.CustomerAliceID, "req-analytics-2", "/api/v1/admin/search", "GET", 200, 52.0, nil},
		{"performance", "http_request", testdata.AdminID, "req-perf-1", "/api/v1/admin/search", "GET", 200, 180.0, nil},
		{"performance", "http_request", testdata.AdminID, "req-perf-2", "/api/v1/admin/ship-requests", "GET", 502, 1325.0, nil},
		{"performance", "http_request", testdata.AdminID, "req-perf-3", "/api/v1/admin/ship-requests", "GET", 200, 880.0, nil},
		{"error", "db_timeout", testdata.AdminID, "req-err-1", "/api/v1/admin/search", "GET", 500, nil, nil},
		{"error", "validation_failed", testdata.CustomerAliceID, "req-err-2", "/api/v1/ship-requests", "POST", 422, nil, nil},
		{"business", "ship_request_paid", testdata.CustomerAliceID, "req-biz-1", "/api/v1/ship-requests", "POST", 200, nil, 17.50},
	}

	for _, e := range events {
		_, err := db.DB().Exec(`
			INSERT INTO observability_events (
				id, category, event_name, user_id, request_id, path, method, status_code, duration_ms, value, metadata_json, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, uuid.NewString(), e.category, e.eventName, e.userID, e.requestID, e.path, e.method, e.statusCode, e.durationMS, e.value, "{}", now)
		require.NoError(t, err)
	}
}

func TestAdminSearch_EmptyQueryReturnsEmptyCollections(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/search", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Users         []map[string]interface{} `json:"users"`
		ShipRequests  []map[string]interface{} `json:"ship_requests"`
		LockerPackage []map[string]interface{} `json:"locker_packages"`
		Page          int                      `json:"page"`
		Limit         int                      `json:"limit"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Len(t, body.Users, 0)
	assert.Len(t, body.ShipRequests, 0)
	assert.Len(t, body.LockerPackage, 0)
	assert.Equal(t, 1, body.Page)
	assert.Equal(t, 20, body.Limit)
}

func TestAdminSearch_PaginatesResults(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/search?q=a&limit=1&page=1", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/search?q=a&limit=1&page=2", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var page1 struct {
		Users []map[string]interface{} `json:"users"`
		Page  int                      `json:"page"`
		Limit int                      `json:"limit"`
	}
	var page2 struct {
		Users []map[string]interface{} `json:"users"`
		Page  int                      `json:"page"`
		Limit int                      `json:"limit"`
	}
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&page1))
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&page2))

	require.Len(t, page1.Users, 1)
	require.Len(t, page2.Users, 1)
	assert.Equal(t, 1, page1.Page)
	assert.Equal(t, 2, page2.Page)
	assert.Equal(t, 1, page1.Limit)
	assert.Equal(t, 1, page2.Limit)
	assert.NotEqual(t, page1.Users[0]["id"], page2.Users[0]["id"])
}

func TestAdminSearch_ShipRequestMappingContract(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/search?q=QCS-PAID-001&limit=5&page=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		ShipRequests []map[string]interface{} `json:"ship_requests"`
		Page         int                      `json:"page"`
		Limit        int                      `json:"limit"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(t, body.ShipRequests)

	sr := body.ShipRequests[0]
	assert.Equal(t, "QCS-PAID-001", sr["confirmation_code"])
	assert.Equal(t, "paid", sr["status"])
	assert.Equal(t, "guyana", sr["destination_id"])
	assert.Equal(t, "standard", sr["service_type"])
	assert.NotEmpty(t, sr["id"])
	assert.NotEmpty(t, sr["user_id"])
	assert.NotEmpty(t, sr["recipient_id"])
	assert.Equal(t, 1, body.Page)
	assert.Equal(t, 5, body.Limit)
}

func TestAdminSearch_LimitIsBoundedToMax(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/search?q=a&limit=999&page=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Users []map[string]interface{} `json:"users"`
		Limit int                      `json:"limit"`
		Page  int                      `json:"page"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, 100, body.Limit)
	assert.Equal(t, 1, body.Page)
	assert.LessOrEqual(t, len(body.Users), 100)
}

func TestAdminSystemHealth_ReturnsMonitoringSnapshot(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system-health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "operational", body.Data["status"])
	assert.Equal(t, true, body.Data["db_ok"])
	assert.Equal(t, "/metrics", body.Data["metrics_endpoint"])
	_, hasUsers := body.Data["users"]
	assert.True(t, hasUsers)
	_, hasPendingShip := body.Data["pending_ship_count"]
	assert.True(t, hasPendingShip)
	_, hasGeneratedAt := body.Data["generated_at"]
	assert.True(t, hasGeneratedAt)
}

func TestAdminSystemHealth_RequiresAdminAuth(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system-health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAdminDestinations_ListAndUpdate(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/destinations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var listed struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listed))
	require.GreaterOrEqual(t, len(listed.Data), 5)

	payload := strings.NewReader(`{"usd_per_lb":3.65,"transit_days_min":4,"transit_days_max":6,"sort_order":11}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/destinations/guyana", payload)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var updated struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))
	assert.Equal(t, "guyana", updated.Data["id"])
	assert.Equal(t, 3.65, updated.Data["usd_per_lb"])
	assert.Equal(t, float64(4), updated.Data["transit_days_min"])
	assert.Equal(t, float64(6), updated.Data["transit_days_max"])
}

func TestAdminInsights_ReturnsConsolidatedSummaries(t *testing.T) {
	app := setupTestApp(t)
	seedObservabilityInsightsEvents(t)
	token := mustAdminAccessToken(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/insights?window_days=30&slow_ms=400&slow_limit=3", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, float64(30), body.Data["window_days"])
	_, hasGeneratedAt := body.Data["generated_at"]
	assert.True(t, hasGeneratedAt)

	analytics, ok := body.Data["analytics"].(map[string]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, analytics["total_events"], float64(1))
	_, hasUniqueUsers := analytics["unique_users"]
	assert.True(t, hasUniqueUsers)

	performance, ok := body.Data["performance"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(400), performance["slow_threshold_ms"])
	_, hasTotalRequests := performance["total_requests"]
	assert.True(t, hasTotalRequests)
	topSlow, ok := performance["top_slow_routes"].([]interface{})
	require.True(t, ok)
	require.NotEmpty(t, topSlow)
	firstSlow, ok := topSlow[0].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, firstSlow["path"])
	_, hasMethod := firstSlow["method"]
	assert.True(t, hasMethod)
	_, hasRequestCount := firstSlow["request_count"]
	assert.True(t, hasRequestCount)
	_, hasAvgDuration := firstSlow["avg_duration_ms"]
	assert.True(t, hasAvgDuration)

	errors, ok := body.Data["errors"].(map[string]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, errors["total_errors"], float64(1))
	_, hasServerErrors := errors["server_errors"]
	assert.True(t, hasServerErrors)
	_, hasClientErrors := errors["client_errors"]
	assert.True(t, hasClientErrors)

	business, ok := body.Data["business"].(map[string]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, business["total_events"], float64(1))
	_, hasTotalValue := business["total_value"]
	assert.True(t, hasTotalValue)
	_, hasAvgValue := business["avg_value"]
	assert.True(t, hasAvgValue)
}

func TestAdminInsights_WindowDaysBounded(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/insights?window_days=999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, float64(90), body.Data["window_days"])
}

func TestAdminInsights_RequiresAdminAuth(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/insights", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestRequireAuth_UsesLiveDBRoleNotJWTClaim is the INC-004 regression
// test. RequireAuth now reads the user's role from the live DB row, not
// the role embedded in the access-token JWT claim. A user demoted in the
// DB is therefore denied admin routes on their next request, instead of
// retaining admin access until the token expires (≤15 min).
//
// We mint an access token with role="admin" (the standard helper), then
// flip the user's role in the DB to "customer" before the call. With the
// fix in place, /admin/dashboard must return 403 even though the JWT
// still claims admin.
func TestRequireAuth_UsesLiveDBRoleNotJWTClaim(t *testing.T) {
	app := setupTestApp(t)
	token := mustAdminAccessToken(t)

	// Sanity: the admin token works against /admin/dashboard before
	// demotion.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "admin should be allowed before demotion")

	// Demote the admin in the DB without changing the JWT.
	_, err = db.DB().Exec(
		`UPDATE users SET role = 'customer', updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), testdata.AdminID,
	)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"demoted admin must be blocked from admin routes immediately, not after token expiry",
	)
	// Drop unused vars to satisfy strict linters.
	_ = strings.TrimSpace
}

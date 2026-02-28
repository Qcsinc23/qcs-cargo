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
	signed, err := token.SignedString([]byte(secret)[:32])
	require.NoError(t, err)
	return signed
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

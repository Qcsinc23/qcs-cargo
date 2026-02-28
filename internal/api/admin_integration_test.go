//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

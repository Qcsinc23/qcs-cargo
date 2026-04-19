//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionsRevokeAll_KeepCurrentSession(t *testing.T) {
	app := setupTestApp(t)

	accessCurrent, refreshCurrent := issueAuthTokens(t, testdata.CustomerAliceID)
	_, _ = issueAuthTokens(t, testdata.CustomerAliceID) // create another active session

	keepSessionID, err := services.ValidateRefreshToken(refreshCurrent)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions", bytes.NewReader([]byte(`{"keep_session_id":"`+keepSessionID+`"}`)))
	req.Header.Set("Authorization", "Bearer "+accessCurrent)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	sessions, err := db.Queries().ListSessionsByUser(context.Background(), gen.ListSessionsByUserParams{
		UserID:    testdata.CustomerAliceID,
		ExpiresAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, keepSessionID, sessions[0].ID)
}

func TestAccountDeactivate_RevokesSessionsAndBlocksRoutes(t *testing.T) {
	app := setupTestApp(t)
	accessToken, refreshToken := issueAuthTokens(t, testdata.CustomerAliceID)
	sessionID, err := services.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/deactivate", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	user, err := db.Queries().GetUserByID(context.Background(), testdata.CustomerAliceID)
	require.NoError(t, err)
	assert.Equal(t, "inactive", user.Status)

	sessions, err := db.Queries().ListSessionsByUser(context.Background(), gen.ListSessionsByUserParams{
		UserID:    testdata.CustomerAliceID,
		ExpiresAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, err)
	assert.Len(t, sessions, 0)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAccountDelete_AnonymizesAndBlacklistsAccessToken(t *testing.T) {
	app := setupTestApp(t)
	accessToken, refreshToken := issueAuthTokens(t, testdata.CustomerAliceID)
	sessionID, err := services.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	claims, err := services.ValidateAccessTokenClaims(accessToken)
	require.NoError(t, err)
	require.NotEmpty(t, claims.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/delete", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	user, err := db.Queries().GetUserByID(context.Background(), testdata.CustomerAliceID)
	require.NoError(t, err)
	assert.Equal(t, "deleted", user.Status)
	assert.Equal(t, "Deleted User", user.Name)
	assert.Equal(t, "deleted+"+testdata.CustomerAliceID+"@qcs.invalid", user.Email)
	assert.False(t, user.Phone.Valid)
	assert.False(t, user.SuiteCode.Valid)
	assert.False(t, user.PasswordHash.Valid)
	assert.False(t, user.AddressStreet.Valid)
	assert.False(t, user.AddressCity.Valid)
	assert.False(t, user.AddressState.Valid)
	assert.False(t, user.AddressZip.Valid)
	assert.Equal(t, 0, user.EmailVerified)

	sessions, err := db.Queries().ListSessionsByUser(context.Background(), gen.ListSessionsByUserParams{
		UserID:    testdata.CustomerAliceID,
		ExpiresAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, err)
	assert.Len(t, sessions, 0)

	blacklisted, err := services.IsTokenBlacklisted(context.Background(), claims.ID)
	require.NoError(t, err)
	assert.True(t, blacklisted)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthMagicLinkVerify_RejectsInactiveAccount(t *testing.T) {
	app := setupTestApp(t)
	now := time.Now().UTC().Format(time.RFC3339)
	require.NoError(t, db.Queries().UpdateUserStatus(context.Background(), gen.UpdateUserStatusParams{
		Status:    "inactive",
		UpdatedAt: now,
		ID:        testdata.CustomerAliceID,
	}))

	rawToken, err := services.RequestMagicLink(context.Background(), testdata.CustomerAliceID, "")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link/verify", bytes.NewReader([]byte(`{"token":"`+rawToken+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ACCOUNT_INACTIVE", errObj["code"])
}

func TestRequireAuth_RejectsInactiveUserStatus(t *testing.T) {
	app := setupTestApp(t)
	now := time.Now().UTC().Format(time.RFC3339)
	require.NoError(t, db.Queries().UpdateUserStatus(context.Background(), gen.UpdateUserStatusParams{
		Status:    "inactive",
		UpdatedAt: now,
		ID:        testdata.CustomerAliceID,
	}))

	secret := os.Getenv("JWT_SECRET")
	require.GreaterOrEqual(t, len(secret), 32)
	claims := services.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			Subject:   testdata.CustomerAliceID,
			ID:        uuid.NewString(),
		},
		UserID: testdata.CustomerAliceID,
		Email:  "alice@test.com",
		Role:   "customer",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Pass 2 audit fix M-2: getJWTSecret no longer truncates JWT_SECRET
	// to 32 bytes, so the signing key here must be the full secret too.
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ACCOUNT_INACTIVE", errObj["code"])
}

//go:build integration

package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationsStream_RejectsAccessTokenQueryString(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream?access_token="+accessToken, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestNotificationsStream_AllowsRefreshCookieAuth(t *testing.T) {
	app := setupTestApp(t)
	_, refreshToken := issueAuthTokens(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream", nil)
	req.AddCookie(&http.Cookie{Name: "qcs_refresh", Value: refreshToken})
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "event: snapshot")
}

func TestNotificationsStream_RejectsRevokedBearerToken(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	claims, err := services.ValidateAccessTokenClaims(accessToken)
	require.NoError(t, err)
	require.NotNil(t, claims.ExpiresAt)
	require.NoError(t, services.BlacklistToken(context.Background(), claims.ID, claims.ExpiresAt.Time))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

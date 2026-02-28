//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func issueAuthTokens(t *testing.T, userID string) (string, string) {
	t.Helper()
	rawToken, err := services.RequestMagicLink(context.Background(), userID, "")
	require.NoError(t, err)

	_, accessToken, refreshToken, err := services.VerifyMagicLink(context.Background(), rawToken)
	require.NoError(t, err)

	return accessToken, refreshToken
}

func TestAuthMagicLinkRequest_EnumerationSafeResponse(t *testing.T) {
	app := setupTestApp(t)

	payload := []byte(`{"email":"missing-user@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link/request", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "If an account with that email exists, you will receive a sign-in link shortly.", body.Data.Message)
}

func TestAuthLogout_Returns204NoContent(t *testing.T) {
	app := setupTestApp(t)
	accessToken, refreshToken := issueAuthTokens(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: "qcs_refresh", Value: refreshToken})

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Len(t, body, 0)
	assert.Contains(t, resp.Header.Get("Set-Cookie"), "qcs_refresh=")
}

func TestBookingsCreateAndList_WithAuth(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	scheduledDate := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	payload := []byte(`{
		"service_type":"standard",
		"destination_id":"guyana",
		"recipient_id":"` + testdata.RecipientGeorgetown + `",
		"scheduled_date":"` + scheduledDate + `",
		"time_slot":"morning",
		"weight_lbs":4.5,
		"length_in":10,
		"width_in":8,
		"height_in":6,
		"value_usd":120,
		"add_insurance":true
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/bookings", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&listResp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listResp.Data), 1)
}

func TestLockerListAndServiceRequestValidation_WithAuth(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/locker?status=stored", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&listResp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listResp.Data), 1)

	payload := []byte(`{"service_type":"invalid-type"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/locker/"+testdata.PkgAliceStored1+"/service-request", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

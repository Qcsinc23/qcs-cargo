//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth_ReturnsOK(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Status string `json:"status"`
		DB     string `json:"db"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "ok", body.Status)
	assert.Equal(t, "ok", body.DB)
}

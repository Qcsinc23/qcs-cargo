//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlatformReadinessAndRuntime(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/readiness", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var readiness struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&readiness))
	assert.NotEmpty(t, readiness.Data.Status)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/platform/runtime", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdminModerationQueueLifecycle(t *testing.T) {
	app := setupTestApp(t)
	adminToken, _ := issueAuthTokens(t, testdata.AdminID)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/moderation", bytes.NewReader([]byte(`{"resource_type":"contact_submission","resource_id":"contact-123","notes":"review for spam"}`)))
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))
	require.NotEmpty(t, created.Data.ID)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/moderation", nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/moderation/"+created.Data.ID, bytes.NewReader([]byte(`{"status":"approved","notes":"clean"}`)))
	updateReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	require.NoError(t, err)
	defer updateResp.Body.Close()
	require.Equal(t, http.StatusOK, updateResp.StatusCode)
}

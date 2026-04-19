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

func TestParcelFeatures_ConsolidationPhotosExportAndImport(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	previewReq := httptest.NewRequest(http.MethodPost, "/api/v1/parcel/consolidation-preview", bytes.NewReader([]byte(`{"locker_package_ids":["`+testdata.PkgAliceStored1+`","`+testdata.PkgAliceStored2+`"],"destination_id":"guyana"}`)))
	previewReq.Header.Set("Authorization", "Bearer "+accessToken)
	previewReq.Header.Set("Content-Type", "application/json")
	previewResp, err := app.Test(previewReq)
	require.NoError(t, err)
	defer previewResp.Body.Close()
	require.Equal(t, http.StatusOK, previewResp.StatusCode)

	var previewBody struct {
		Data struct {
			PackageCount int     `json:"package_count"`
			Savings      float64 `json:"estimated_savings_lbs"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(previewResp.Body).Decode(&previewBody))
	assert.Equal(t, 2, previewBody.Data.PackageCount)
	assert.GreaterOrEqual(t, previewBody.Data.Savings, 0.0)

	photosReq := httptest.NewRequest(http.MethodGet, "/api/v1/parcel/photos", nil)
	photosReq.Header.Set("Authorization", "Bearer "+accessToken)
	photosResp, err := app.Test(photosReq)
	require.NoError(t, err)
	defer photosResp.Body.Close()
	require.Equal(t, http.StatusOK, photosResp.StatusCode)

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/data/export?format=json", nil)
	exportReq.Header.Set("Authorization", "Bearer "+accessToken)
	exportResp, err := app.Test(exportReq)
	require.NoError(t, err)
	defer exportResp.Body.Close()
	require.Equal(t, http.StatusOK, exportResp.StatusCode)

	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/data/recipients/import", bytes.NewReader([]byte(`{"rows":[{"name":"Office Georgetown","destination_id":"guyana","street":"12 Water St","city":"Georgetown"},{"name":"Home Kingston","destination_id":"jamaica","street":"44 Hope Rd","city":"Kingston"}]}`)))
	importReq.Header.Set("Authorization", "Bearer "+accessToken)
	importReq.Header.Set("Content-Type", "application/json")
	importResp, err := app.Test(importReq)
	require.NoError(t, err)
	defer importResp.Body.Close()
	require.Equal(t, http.StatusOK, importResp.StatusCode)

	var importBody struct {
		Data struct {
			ImportedRows int  `json:"imported_rows"`
			MultiAddress bool `json:"multi_address"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(importResp.Body).Decode(&importBody))
	assert.Equal(t, 2, importBody.Data.ImportedRows)
	assert.True(t, importBody.Data.MultiAddress)
}

func TestParcelFeatures_CustomsSignatureAndLoyalty(t *testing.T) {
	// Pass 2 audit fix H-5: customs-docs now validates file_url against an
	// upload host allowlist; configure one for the test fixture.
	t.Setenv("UPLOAD_HOST_ALLOWLIST", "cdn.example.com")
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	customsReq := httptest.NewRequest(http.MethodPost, "/api/v1/parcel/customs-docs", bytes.NewReader([]byte(`{"ship_request_id":"`+testdata.ShipReqAliceDraft+`","doc_type":"invoice","file_name":"invoice.pdf","file_url":"https://cdn.example.com/invoice.pdf","mime_type":"application/pdf","size_bytes":512}`)))
	customsReq.Header.Set("Authorization", "Bearer "+accessToken)
	customsReq.Header.Set("Content-Type", "application/json")
	customsResp, err := app.Test(customsReq)
	require.NoError(t, err)
	defer customsResp.Body.Close()
	require.Equal(t, http.StatusCreated, customsResp.StatusCode)

	signReq := httptest.NewRequest(http.MethodPost, "/api/v1/parcel/delivery-signature", bytes.NewReader([]byte(`{"ship_request_id":"`+testdata.ShipReqAliceDraft+`","signer_name":"Alice","signature_data":"data:image/png;base64,abc123"}`)))
	signReq.Header.Set("Authorization", "Bearer "+accessToken)
	signReq.Header.Set("Content-Type", "application/json")
	signResp, err := app.Test(signReq)
	require.NoError(t, err)
	defer signResp.Body.Close()
	require.Equal(t, http.StatusOK, signResp.StatusCode)

	loyaltyReq := httptest.NewRequest(http.MethodGet, "/api/v1/parcel/loyalty-summary", nil)
	loyaltyReq.Header.Set("Authorization", "Bearer "+accessToken)
	loyaltyResp, err := app.Test(loyaltyReq)
	require.NoError(t, err)
	defer loyaltyResp.Body.Close()
	require.Equal(t, http.StatusOK, loyaltyResp.StatusCode)
}

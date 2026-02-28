//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicDestinations_List_DBBacked(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/destinations", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.GreaterOrEqual(t, len(body.Data), 5)

	var hasGuyana bool
	for _, d := range body.Data {
		if d["id"] == "guyana" {
			hasGuyana = true
			assert.Equal(t, "Guyana", d["name"])
			assert.Equal(t, "GY", d["code"])
			assert.Equal(t, "Georgetown", d["capital"])
			assert.Equal(t, "3-5 days", d["transit"])
		}
	}
	assert.True(t, hasGuyana)
}

func TestPublicDestinations_GetByID_DBBacked(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/destinations/guyana", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "guyana", body.Data["id"])
	assert.Equal(t, "Guyana", body.Data["name"])
	assert.Equal(t, "3-5 days", body.Data["transit"])
}

func TestPublicDestinations_GetByID_NotFound(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/destinations/not-real", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, strings.EqualFold(body.Error.Code, "NOT_FOUND"))
}

func TestPublicTrack_ReturnsShipmentByTrackingNumber(t *testing.T) {
	app := setupTestApp(t)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.DB().Exec(`
		INSERT INTO shipments
			(id, destination_id, manifest_id, ship_request_id, tracking_number, status, total_weight, package_count, carrier, estimated_delivery, actual_delivery, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "ship_pub_001", "guyana", nil, testdata.ShipReqAliceShipped, "QCS-TRK-001", "in_transit", 12.5, 2, "QCS Cargo", "2026-03-05", nil, now, now)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/track/QCS-TRK-001", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "QCS-TRK-001", body.Data["tracking_number"])
	assert.Equal(t, "in_transit", body.Data["status"])
	assert.Equal(t, "guyana", body.Data["destination_id"])
	assert.Equal(t, "QCS Cargo", body.Data["carrier"])
}

func TestPublicTrack_ReturnsShipRequestByConfirmationCode(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/track/QCS-SHIP-001", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "QCS-SHIP-001", body.Data["confirmation_code"])
	assert.Equal(t, "QCS-SHIP-001", body.Data["tracking_number"])
	assert.Equal(t, "shipped", body.Data["status"])
	assert.Equal(t, "guyana", body.Data["destination_id"])
}

func TestPublicTrack_ReturnsNotFound(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/track/FAKE-000", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestShippingCalculator_InvalidNumericInputsReturn400(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calculator?dest=guyana&weight=abc", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/calculator?dest=guyana&weight=5&l=bad", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestShippingCalculator_AcceptsDestinationAliasQueryParam(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calculator?destination=guyana&weight=5", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

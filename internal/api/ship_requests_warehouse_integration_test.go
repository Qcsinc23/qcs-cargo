//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShipRequestCreateCustomsAndPay_UsesPersistedPricing(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	createPayload := []byte(`{
		"destination_id":"guyana",
		"service_type":"standard",
		"recipient_id":"` + testdata.RecipientGeorgetown + `",
		"locker_package_ids":["` + testdata.PkgAliceStored3 + `"]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ship-requests", bytes.NewReader(createPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	require.NotEmpty(t, created.Data.ID)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/ship-requests/"+created.Data.ID+"/estimate", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var estimate struct {
		Data struct {
			Total float64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&estimate))
	assert.Greater(t, estimate.Data.Total, 0.0)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/ship-requests/"+created.Data.ID+"/customs", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	prevStripe := os.Getenv("STRIPE_SECRET_KEY")
	require.NoError(t, os.Setenv("STRIPE_SECRET_KEY", ""))
	defer func() {
		_ = os.Setenv("STRIPE_SECRET_KEY", prevStripe)
	}()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/ship-requests/"+created.Data.ID+"/pay", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
}

func TestRecipientOwnershipValidation_BookingsAndShipRequests(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerBobID)

	bookingPayload := []byte(`{
		"service_type":"standard",
		"destination_id":"guyana",
		"recipient_id":"` + testdata.RecipientGeorgetown + `",
		"scheduled_date":"2099-01-01",
		"time_slot":"morning",
		"weight_lbs":1
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(bookingPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	shipPayload := []byte(`{
		"destination_id":"guyana",
		"service_type":"standard",
		"recipient_id":"` + testdata.RecipientGeorgetown + `",
		"locker_package_ids":["` + testdata.PkgBobStored1 + `"]
	}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/ship-requests", bytes.NewReader(shipPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWarehouseManifestCreate_RollsBackOnItemFailure(t *testing.T) {
	app := setupTestApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.StaffWarehouseID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/warehouse/manifests", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var before struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&before))

	payload := []byte(`{
		"destination_id":"guyana",
		"ship_request_ids":["` + testdata.ShipReqAlicePaid + `","ship_request_missing"]
	}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/manifests", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/warehouse/manifests", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var after struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&after))
	assert.Len(t, after.Data, len(before.Data))

	req = httptest.NewRequest(http.MethodGet, "/api/v1/ship-requests/"+testdata.ShipReqAlicePaid, nil)
	req.Header.Set("Authorization", "Bearer "+issueAccessTokenForUser(t, testdata.CustomerAliceID))
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var shipRequest struct {
		Data struct {
			ShipRequest struct {
				Status string `json:"status"`
			} `json:"ship_request"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&shipRequest))
	assert.Equal(t, "paid", shipRequest.Data.ShipRequest.Status)
}

func issueAccessTokenForUser(t *testing.T, userID string) string {
	t.Helper()
	accessToken, _ := issueAuthTokens(t, userID)
	return accessToken
}

// TestAdminShipRequestReconcile_OperatesOnDifferentUserRequest is the
// regression test for DEF-001. Before the fix, shipRequestReconcile filtered
// by the calling admin's user_id, so it could not be used to reconcile
// stuck customer payments (its only legitimate use). This test asserts that
// an admin can reconcile a ship request owned by a different user and that
// the row's payment_status flips to "paid".
func TestAdminShipRequestReconcile_OperatesOnDifferentUserRequest(t *testing.T) {
	app := setupTestApp(t)

	// Seeded fixture sreq_alice_draft1 belongs to Alice and is in "draft"
	// state; we stage it as pending_payment + an unpaid payment_status so
	// the reconcile transition is meaningful.
	_, err := db.DB().Exec(
		`UPDATE ship_requests SET status = ?, payment_status = ?, updated_at = ? WHERE id = ?`,
		"pending_payment", "pending", time.Now().UTC().Format(time.RFC3339), testdata.ShipReqAliceDraft,
	)
	require.NoError(t, err)

	adminToken := issueAccessTokenForUser(t, testdata.AdminID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ship-requests/"+testdata.ShipReqAliceDraft+"/reconcile", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var status, paymentStatus string
	err = db.DB().QueryRow(
		`SELECT status, payment_status FROM ship_requests WHERE id = ?`,
		testdata.ShipReqAliceDraft,
	).Scan(&status, &paymentStatus)
	require.NoError(t, err)
	assert.Equal(t, "paid", status)
	assert.Equal(t, "paid", paymentStatus)
}

// TestAdminShipRequestReconcile_NonAdminRejected guards the route gate.
func TestAdminShipRequestReconcile_NonAdminRejected(t *testing.T) {
	app := setupTestApp(t)

	customerToken := issueAccessTokenForUser(t, testdata.CustomerBobID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ship-requests/"+testdata.ShipReqAlicePaid+"/reconcile", nil)
	req.Header.Set("Authorization", "Bearer "+customerToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

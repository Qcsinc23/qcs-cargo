//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWarehouseReceiveFromBooking_PackageCountCapped is the Pass 2.5
// MED-14 regression. Before the fix the handler trusted body.package_count
// and would attempt to insert that many locker_package rows in a single
// transaction. Confirm the cap (>100) is rejected with 400 and that
// package_count<1 is also rejected.
func TestWarehouseReceiveFromBooking_PackageCountCapped(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	// First seed a booking row owned by a customer with a suite code so
	// the handler can advance past its booking + user lookups before
	// hitting the cap. We don't actually expect the insert to run; the
	// validation must short-circuit before the loop.
	bookingID := "bkg_med14_test01"
	_, err := db.DB().Exec(`
		INSERT INTO bookings
			(id, user_id, confirmation_code, status, service_type, destination_id,
			 scheduled_date, time_slot, created_at, updated_at)
		VALUES (?, ?, 'QCS-MED14-001', 'pending', 'standard', 'guyana',
			datetime('now'), 'morning', datetime('now'), datetime('now'))
	`, bookingID, testdata.CustomerAliceID)
	require.NoError(t, err)

	// 1000 packages > the 100 cap → 400.
	payload := []byte(fmt.Sprintf(`{"booking_id":%q,"package_count":1000}`, bookingID))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/packages/receive-from-booking", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+staffToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Zero packages → 400 (the legacy code would silently no-op).
	payload2 := []byte(fmt.Sprintf(`{"booking_id":%q,"package_count":0}`, bookingID))
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/packages/receive-from-booking", bytes.NewReader(payload2))
	req2.Header.Set("Authorization", "Bearer "+staffToken)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)
}

// TestWarehouseExceptionResolve_RejectsUnknownAction is the Pass 2.5
// LOW-04 regression. The handler previously coerced any unknown action
// to "matched"; it must now return 400 for anything outside the
// allowlist (match|return|dispose).
func TestWarehouseExceptionResolve_RejectsUnknownAction(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	// Seed a minimal unmatched_packages row so the handler doesn't
	// 404 before reaching the action switch.
	exceptionID := "unmatch_low04_001"
	_, err := db.DB().Exec(`
		INSERT INTO unmatched_packages (id, status, received_at, created_at)
		VALUES (?, 'pending', datetime('now'), datetime('now'))
	`, exceptionID)
	require.NoError(t, err)

	payload := []byte(`{"action":"floob"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/exceptions/"+exceptionID+"/resolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+staffToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestNotificationsPushSubscribe_RejectsHijackedEndpoint is the Pass 2.5
// MED-16 regression. Once the new globally-unique constraint is in place
// (migration 20260422120000), a second user attempting to register an
// endpoint already owned by another account must get a 409, not a
// silent reassignment.
func TestNotificationsPushSubscribe_RejectsHijackedEndpoint(t *testing.T) {
	app := setupTestApp(t)
	aliceToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)
	bobToken, _ := issueAuthTokens(t, testdata.CustomerBobID)

	endpoint := "https://push.example.com/sub/abc-med16"
	body := map[string]any{
		"endpoint": endpoint,
		"keys":     map[string]string{"p256dh": "p2-key", "auth": "auth-key"},
	}
	payload, _ := json.Marshal(body)

	// Alice registers the endpoint first.
	reqA := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/push/subscribe", bytes.NewReader(payload))
	reqA.Header.Set("Authorization", "Bearer "+aliceToken)
	reqA.Header.Set("Content-Type", "application/json")
	respA, err := app.Test(reqA)
	require.NoError(t, err)
	defer respA.Body.Close()
	require.Equal(t, http.StatusCreated, respA.StatusCode, "alice subscribe should succeed")

	// Bob tries to register the same endpoint → must be rejected.
	reqB := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/push/subscribe", bytes.NewReader(payload))
	reqB.Header.Set("Authorization", "Bearer "+bobToken)
	reqB.Header.Set("Content-Type", "application/json")
	respB, err := app.Test(reqB)
	require.NoError(t, err)
	defer respB.Body.Close()
	assert.Equal(t, http.StatusConflict, respB.StatusCode, "bob's hijack attempt must be rejected")

	// And the row must still belong to Alice in the DB.
	var owner string
	require.NoError(t, db.DB().QueryRow(
		`SELECT user_id FROM push_subscriptions WHERE endpoint = ?`,
		endpoint,
	).Scan(&owner))
	assert.Equal(t, testdata.CustomerAliceID, owner)
}

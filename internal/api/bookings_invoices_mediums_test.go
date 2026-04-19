//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBookingCreate_IdempotencyKeyDeduped is the Pass 2.5 MED-06
// regression. POST /bookings now sits behind IdempotencyMiddleware:
// two POSTs that supply the same Idempotency-Key header within the
// cache TTL must replay the cached response (same body bytes) and not
// create a second booking row.
func TestBookingCreate_IdempotencyKeyDeduped(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	scheduledDate := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	payload := []byte(`{
		"service_type":"standard",
		"destination_id":"guyana",
		"recipient_id":"` + testdata.RecipientGeorgetown + `",
		"scheduled_date":"` + scheduledDate + `",
		"time_slot":"morning",
		"weight_lbs":2.0,
		"length_in":4,
		"width_in":4,
		"height_in":4,
		"value_usd":50,
		"add_insurance":false
	}`)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(payload))
	req1.Header.Set("Authorization", "Bearer "+aliceToken)
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "test-idemp-bk-001")
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusCreated, resp1.StatusCode)

	var firstBody struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&firstBody))
	firstID, _ := firstBody.Data["id"].(string)
	require.NotEmpty(t, firstID)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(payload))
	req2.Header.Set("Authorization", "Bearer "+aliceToken)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "test-idemp-bk-001")
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusCreated, resp2.StatusCode)
	assert.Equal(t, "true", resp2.Header.Get("X-Idempotent-Replay"))

	var secondBody struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&secondBody))
	secondID, _ := secondBody.Data["id"].(string)
	assert.Equal(t, firstID, secondID, "second POST with same Idempotency-Key must return the same booking id")

	// Confirm only ONE row was inserted under Alice for this combo.
	var count int
	err = db.DB().QueryRow(
		`SELECT COUNT(*) FROM bookings WHERE user_id = ? AND scheduled_date = ? AND time_slot = 'morning' AND total = ?`,
		testdata.CustomerAliceID, scheduledDate, firstBody.Data["total"],
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Idempotency-Key must dedupe the second POST at the middleware layer")
}

// TestBookingCreate_RecipientErrorHasFixedMessage is the Pass 2.5
// MED-07 regression. The handler used to surface raw err.Error() on
// the recipient-validation failure path, which can leak internal SQL
// or driver detail. The response message must now be a fixed
// customer-facing string.
func TestBookingCreate_RecipientErrorHasFixedMessage(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	scheduledDate := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	// Use a recipient that exists but is bound to a *different* destination
	// than the booking. validateBookingRecipient returns a non-sql.ErrNoRows
	// error in that branch ("recipient destination does not match ..."),
	// which is exactly the path that used to leak err.Error().
	payload := []byte(`{
		"service_type":"standard",
		"destination_id":"jamaica",
		"recipient_id":"` + testdata.RecipientGeorgetown + `",
		"scheduled_date":"` + scheduledDate + `",
		"time_slot":"morning",
		"weight_lbs":1,"length_in":4,"width_in":4,"height_in":4,"value_usd":10,"add_insurance":false
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "VALIDATION_ERROR", body.Error.Code)
	assert.Equal(t, "recipient validation failed", body.Error.Message,
		"MED-07: error envelope must use a fixed customer-facing message, not the raw err.Error()")
}

// TestBookingList_PaginationCapped is the Pass 2.5 MED-08 regression.
// bookingList caps the response slice at maxLimit (100) regardless of
// the requested ?limit value and reports the pagination envelope.
func TestBookingList_PaginationCapped(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	now := time.Now().UTC().Format(time.RFC3339)
	scheduled := time.Now().UTC().AddDate(0, 0, 5).Format("2006-01-02")
	const extra = 120
	for i := 0; i < extra; i++ {
		_, err := db.DB().Exec(`
			INSERT INTO bookings (
				id, user_id, confirmation_code, status, service_type, destination_id, recipient_id,
				scheduled_date, time_slot, special_instructions,
				weight_lbs, length_in, width_in, height_in, value_usd, add_insurance,
				subtotal, discount, insurance, total,
				payment_status, stripe_payment_intent_id, created_at, updated_at
			) VALUES (?, ?, ?, 'pending', 'standard', 'guyana', NULL,
				?, 'morning', NULL,
				1, 1, 1, 1, 0, 0, 0, 0, 0, 0,
				'pending', NULL, ?, ?)`,
			fmt.Sprintf("bkg_pag_%03d", i),
			testdata.CustomerAliceID,
			fmt.Sprintf("BK-PAG%03d", i),
			scheduled, now, now,
		)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bookings?limit=99999", nil)
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data  []map[string]any `json:"data"`
		Page  int              `json:"page"`
		Limit int              `json:"limit"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.LessOrEqual(t, len(body.Data), 100, "page size must be capped at maxLimit")
	assert.Equal(t, 100, body.Limit)
	assert.Equal(t, 1, body.Page)
	assert.GreaterOrEqual(t, body.Total, extra, "total must reflect the full set, not the page")
}

// TestInvoiceList_PaginationAndNoUserID covers Pass 2.5 MED-08
// (pagination envelope) and LOW-02 (drop user_id from the per-invoice
// payload returned by the GET /invoices/:id handler).
func TestInvoiceList_PaginationAndNoUserID(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	now := time.Now().UTC().Format(time.RFC3339)
	const extra = 30
	invoiceIDs := make([]string, 0, extra)
	for i := 0; i < extra; i++ {
		id := fmt.Sprintf("inv_pag_%03d", i)
		invoiceIDs = append(invoiceIDs, id)
		_, err := db.DB().Exec(`
			INSERT INTO invoices (
				id, user_id, booking_id, ship_request_id, invoice_number,
				subtotal, tax, total, status, due_date, paid_at, notes,
				created_at
			) VALUES (?, ?, NULL, NULL, ?, 0, 0, 0, 'pending', NULL, NULL, NULL, ?)`,
			id, testdata.CustomerAliceID, fmt.Sprintf("INV-PAG%03d", i), now,
		)
		require.NoError(t, err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices?limit=10", nil)
	listReq.Header.Set("Authorization", "Bearer "+aliceToken)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listBody struct {
		Data  []map[string]any `json:"data"`
		Limit int              `json:"limit"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listBody))
	assert.LessOrEqual(t, len(listBody.Data), 10)
	assert.Equal(t, 10, listBody.Limit)
	assert.GreaterOrEqual(t, listBody.Total, extra)

	// Now fetch a single invoice and assert the per-invoice payload
	// shape from invoiceToMap no longer includes user_id (LOW-02).
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+invoiceIDs[0], nil)
	getReq.Header.Set("Authorization", "Bearer "+aliceToken)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var getBody struct {
		Data struct {
			Invoice map[string]any `json:"invoice"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&getBody))
	_, hasUserID := getBody.Data.Invoice["user_id"]
	assert.False(t, hasUserID, "LOW-02: invoice payload must not include user_id")
}

// TestRecipientsDelete_NotFoundForeignID is the Pass 2.5 LOW-01
// regression. DELETE /recipients/:id with an id that does not belong
// to (or does not exist for) the caller must return 404, not 200.
func TestRecipientsDelete_NotFoundForeignID(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	// Use an id Alice definitely does not own.
	bogusID := "rcpt_does_not_exist_999"
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/recipients/"+bogusID, nil)
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Sanity: a real recipient owned by Alice (not referenced by any
	// ship_request — the seeded RecipientGeorgetown is FK'd to several
	// rows so we insert a fresh standalone one for this assertion)
	// still deletes successfully.
	freshID := "rcpt_low01_delete_ok"
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.DB().Exec(`
		INSERT INTO recipients (id, user_id, name, destination_id, street, city, is_default, use_count, created_at, updated_at)
		VALUES (?, ?, 'Deletable Recipient', 'guyana', '1 Test St', 'Georgetown', 0, 0, ?, ?)`,
		freshID, testdata.CustomerAliceID, now, now,
	)
	require.NoError(t, err)

	okReq := httptest.NewRequest(http.MethodDelete, "/api/v1/recipients/"+freshID, nil)
	okReq.Header.Set("Authorization", "Bearer "+aliceToken)
	okResp, err := app.Test(okReq)
	require.NoError(t, err)
	defer okResp.Body.Close()
	assert.Equal(t, http.StatusOK, okResp.StatusCode)
}

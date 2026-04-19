//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecipientsList_PaginationBounded is the MED-02 regression. The
// recipientsList handler must respect the maxRecipientLimit cap even
// when the caller asks for a much larger page size, and must return the
// pagination envelope (page/limit/total) so callers can iterate.
func TestRecipientsList_PaginationBounded(t *testing.T) {
	app := setupTestApp(t)

	// Seed many recipients so the cap is observable. The default seed
	// only inserts two; we add another 250 to push past
	// maxRecipientLimit (200).
	const extra = 250
	for i := 0; i < extra; i++ {
		_, err := db.DB().Exec(
			`INSERT INTO recipients (id, user_id, name, destination_id, street, city, is_default, use_count, created_at, updated_at)
			 VALUES (?, ?, ?, 'guyana', '1 Test St', 'Georgetown', 0, 0, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')`,
			fmt.Sprintf("rcpt_pag_%03d", i),
			testdata.CustomerAliceID,
			fmt.Sprintf("Recipient %03d", i),
		)
		require.NoError(t, err)
	}

	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipients?limit=99999", nil)
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
	assert.LessOrEqual(t, len(body.Data), 200, "page size must be capped at maxRecipientLimit")
	assert.Equal(t, 200, body.Limit, "reported limit must be the enforced cap")
	assert.GreaterOrEqual(t, body.Total, extra, "total must reflect the full set, not the page")
	assert.Equal(t, 1, body.Page)
}

// TestRecipientsCreate_RejectsHTMLInName is the MED-03 regression for
// the name field. services.ValidateName already rejects HTML
// metacharacters; this ensures the recipients create handler invokes it.
func TestRecipientsCreate_RejectsHTMLInName(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	payload := []byte(`{
        "name": "<script>alert(1)</script>",
        "destination_id": "guyana",
        "street": "123 Test St",
        "city": "Georgetown"
    }`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipients", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "name with HTML metacharacters must be rejected")
}

// TestRecipientsCreate_RejectsHTMLInStreet is the MED-03 regression for
// the address fields, which historically had no XSS validation.
func TestRecipientsCreate_RejectsHTMLInStreet(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	payload := []byte(`{
        "name": "Jane Doe",
        "destination_id": "guyana",
        "street": "<img src=x onerror=alert(1)>",
        "city": "Georgetown"
    }`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipients", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "street with HTML metacharacters must be rejected")
}

// TestRecipientsUpdate_RejectsClearingRequiredField is the MED-04
// regression. A PATCH that explicitly sends empty strings for required
// fields must not be allowed to blank out the row.
func TestRecipientsUpdate_RejectsClearingRequiredField(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	payload := []byte(`{"street": ""}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/recipients/"+testdata.RecipientGeorgetown, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "PATCH that empties a required field must be rejected")
}

// TestLockerServiceRequest_NotesCap is the MED-05 regression. The notes
// field on /locker/:id/service-request must cap at 1000 chars.
func TestLockerServiceRequest_NotesCap(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	longNotes := strings.Repeat("a", 1001)
	payload := []byte(fmt.Sprintf(`{"service_type":"photo","notes":%q}`, longNotes))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/locker/"+testdata.PkgAliceStored1+"/service-request", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "oversized notes must be rejected")
}

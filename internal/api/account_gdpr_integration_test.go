//go:build integration

package api_test

import (
	"database/sql"
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

// TestAccountDelete_AnonymizesAllUserScopedTables is the CRIT-03
// regression test. The previous implementation only anonymized the
// users row; recipients, locker_packages, ship_requests, etc. were
// left intact, making the customer-facing "personal data anonymized"
// message materially false.
func TestAccountDelete_AnonymizesAllUserScopedTables(t *testing.T) {
	app := setupTestApp(t)

	// Seed Alice with extra PII rows beyond the default seed.
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.DB().Exec(
		`INSERT INTO recipients (id, user_id, name, destination_id, street, city, is_default, use_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, 0, ?, ?)`,
		"rcpt_anon_test", testdata.CustomerAliceID, "Test Recipient", "guyana", "456 Test St", "Georgetown", now, now,
	)
	require.NoError(t, err)

	// Confirm pre-state.
	var preCount int
	require.NoError(t, db.DB().QueryRow(
		`SELECT COUNT(*) FROM recipients WHERE user_id = ?`, testdata.CustomerAliceID,
	).Scan(&preCount))
	assert.Greater(t, preCount, 0, "test fixture must seed recipients before delete")

	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/delete", nil)
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Sessions / mfa / api_keys / cookie_consents must be hard-deleted.
	hardDelete := []struct {
		name  string
		query string
	}{
		{"sessions", `SELECT COUNT(*) FROM sessions WHERE user_id = ?`},
		{"user_mfa", `SELECT COUNT(*) FROM user_mfa WHERE user_id = ?`},
		{"api_keys", `SELECT COUNT(*) FROM api_keys WHERE user_id = ?`},
		{"cookie_consents", `SELECT COUNT(*) FROM cookie_consents WHERE user_id = ?`},
	}
	for _, tt := range hardDelete {
		var count int
		require.NoError(t, db.DB().QueryRow(tt.query, testdata.CustomerAliceID).Scan(&count), tt.name)
		assert.Equal(t, 0, count, "table %s should be empty for deleted user", tt.name)
	}

	// Recipients are soft-deleted (scrub PII + set deleted_at) because
	// ship_requests FK into them. Verify PII has been replaced.
	rows, err := db.DB().Query(`SELECT name, deleted_at FROM recipients WHERE user_id = ?`, testdata.CustomerAliceID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var rname string
		var deletedAt *string
		require.NoError(t, rows.Scan(&rname, &deletedAt))
		assert.Equal(t, "[deleted]", rname, "recipient name must be scrubbed")
		assert.NotNil(t, deletedAt, "recipient must have deleted_at set")
	}

	// users row anonymized.
	var name, email string
	require.NoError(t, db.DB().QueryRow(
		`SELECT name, email FROM users WHERE id = ?`, testdata.CustomerAliceID,
	).Scan(&name, &email))
	assert.Equal(t, "Deleted User", name)
	assert.Contains(t, email, "deleted+", "email should be anonymized to deleted+<id>@qcs.invalid")

	// locker_packages sender_name scrubbed (rows kept for audit).
	var anySender string
	err = db.DB().QueryRow(
		`SELECT sender_name FROM locker_packages WHERE user_id = ? LIMIT 1`, testdata.CustomerAliceID,
	).Scan(&anySender)
	if err == nil {
		assert.Equal(t, "[deleted]", anySender)
	}

	// Pass 3 HIGH-01: the audit row must be written in the SAME tx
	// as the scrub. After commit it must be visible in admin_activity
	// with the canonical "auth.account.delete" event type.
	var auditCount int
	require.NoError(t, db.DB().QueryRow(
		`SELECT COUNT(*) FROM admin_activity WHERE actor_id = ? AND action = ?`,
		testdata.CustomerAliceID, "auth.account.delete",
	).Scan(&auditCount))
	assert.Equal(t, 1, auditCount, "exactly one auth.account.delete audit row must be written in the same tx as the anonymization")

	// Pass 3 CRIT-01: extended scrub coverage. Spot-check a handful of
	// the newly-covered tables that we know the seed populates so the
	// regression cannot recur.
	var siNotes sql.NullString
	require.NoError(t, db.DB().QueryRow(
		`SELECT notes FROM service_requests WHERE user_id = ? LIMIT 1`, testdata.CustomerAliceID,
	).Scan(&siNotes))
	assert.False(t, siNotes.Valid, "service_requests.notes must be NULL for deleted user")
}

// TestComplianceGDPRDeleteRequest_ProcessedSynchronously is the CRIT-04
// regression test. Previously this endpoint just appended an audit row
// with no anonymization processor existing anywhere in the codebase.
// Now the same anonymization runs synchronously and the audit row is
// marked processed.
func TestComplianceGDPRDeleteRequest_ProcessedSynchronously(t *testing.T) {
	app := setupTestApp(t)

	bobToken := issueAccessTokenForUser(t, testdata.CustomerBobID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/gdpr/delete-request", nil)
	req.Header.Set("Authorization", "Bearer "+bobToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "processed", body.Data["status"], "delete_request must be processed inline, not just logged")
	assert.NotEmpty(t, body.Data["processed_at"], "processed_at timestamp must be set")

	// Bob's users row must be anonymized.
	var name, email string
	require.NoError(t, db.DB().QueryRow(
		`SELECT name, email FROM users WHERE id = ?`, testdata.CustomerBobID,
	).Scan(&name, &email))
	assert.Equal(t, "Deleted User", name)
	assert.Contains(t, email, "deleted+")

	// Pass 3 HIGH-01: a "gdpr.delete_request.processed" audit row
	// must be written in the SAME tx as the scrub.
	var auditCount int
	require.NoError(t, db.DB().QueryRow(
		`SELECT COUNT(*) FROM admin_activity WHERE actor_id = ? AND action = ?`,
		testdata.CustomerBobID, "gdpr.delete_request.processed",
	).Scan(&auditCount))
	assert.Equal(t, 1, auditCount, "exactly one gdpr.delete_request.processed audit row must be present")
}

// TestAppendResourceVersion_RedactsGDPRPayload is the audit-redaction
// half of CRIT-04. The version row is preserved (the audit trail of
// "request happened" remains) but the body is replaced with a marker.
func TestAppendResourceVersion_RedactsGDPRPayload(t *testing.T) {
	app := setupTestApp(t)

	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	// Create an export request (does NOT trigger anonymization, only logs intent).
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/gdpr/export-request", nil)
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Inspect the resource_versions row.
	var dataJSON string
	require.NoError(t, db.DB().QueryRow(
		`SELECT data_json FROM resource_versions WHERE resource_type = 'gdpr_request' AND changed_by = ? ORDER BY created_at DESC LIMIT 1`,
		testdata.CustomerAliceID,
	).Scan(&dataJSON))

	assert.True(t, strings.Contains(dataJSON, "_redacted"), "GDPR audit payload must be redacted")
	assert.False(t, strings.Contains(dataJSON, testdata.CustomerAliceID), "redacted payload must not contain user_id verbatim")
}

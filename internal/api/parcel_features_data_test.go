//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/require"
)

// TestDataExport_CoversAllUserScopedCollections is the Pass 2.5 HIGH-07
// drift-guard. The /data/export response is the GDPR Article 15 data
// portability surface, and it MUST include a top-level key for every
// user-scoped table. When a new user-scoped table lands in a migration,
// this test fails until exportUserRows + dataExportUser are updated to
// expose the new collection.
//
// Update expectedKeys (and userExportData / dataExportUser) when adding
// or removing a user-scoped table.
func TestDataExport_CoversAllUserScopedCollections(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data/export?format=json", nil)
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	expectedKeys := []string{
		"generated_at",
		"user_id",
		"user_profile",
		"recipients",
		"locker_packages",
		"ship_requests",
		"bookings",
		"customs_preclearance_docs",
		"delivery_signatures",
		"locker_photos",
		"assisted_purchase_requests",
		"notification_prefs",
		"cookie_consents",
		"loyalty_ledger",
		"user_mfa",
		"api_keys",
		"service_requests",
		"storage_fees",
		"invoices",
		"templates",
		"inbound_tracking",
		"in_app_notifications",
		"push_subscriptions",
		"parcel_consolidation_previews",
		"data_import_jobs",
	}

	have := []string{}
	for k := range body.Data {
		have = append(have, k)
	}
	sort.Strings(have)

	for _, want := range expectedKeys {
		if _, ok := body.Data[want]; !ok {
			t.Errorf("data export missing top-level key %q (have: %v)", want, have)
		}
	}
}

// TestDataExport_NeverLeaksAPIKeyHashOrMFASecrets is the explicit
// negative assertion for HIGH-07: even though api_keys and user_mfa now
// appear in the export, the raw key hash, raw key, and MFA secret/OTP
// fields must never be returned. This is enforced both by the SELECT
// statements (which omit those columns) and by the export struct shape
// (which has no field for them) — this test catches accidental
// reintroduction via SELECT * or struct refactors.
func TestDataExport_NeverLeaksAPIKeyHashOrMFASecrets(t *testing.T) {
	app := setupTestApp(t)
	aliceToken := issueAccessTokenForUser(t, testdata.CustomerAliceID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data/export?format=json", nil)
	req.Header.Set("Authorization", "Bearer "+aliceToken)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var raw map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))

	flat := flattenJSONKeys(raw, "")
	forbidden := []string{
		"key_hash",
		"key_hash_sha256",
		"raw_key",
		"secret",
		"otp_code_hash",
		"otp_secret",
		"password_hash",
		"signature_data",
		"p256dh",
		"auth_secret",
	}
	for _, key := range forbidden {
		for _, present := range flat {
			if present == key {
				t.Errorf("data export must never expose key %q (full key path appeared in response)", key)
			}
		}
	}
}

// flattenJSONKeys walks any JSON-decoded value and returns the flat list
// of leaf keys it observed. Used to assert that a forbidden field name
// never appears anywhere in a deeply-nested response.
func flattenJSONKeys(v interface{}, prefix string) []string {
	out := []string{}
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			out = append(out, k)
			out = append(out, flattenJSONKeys(val, prefix+"."+k)...)
		}
	case []interface{}:
		for _, val := range t {
			out = append(out, flattenJSONKeys(val, prefix)...)
		}
	}
	return out
}

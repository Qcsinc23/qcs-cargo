//go:build integration

package api_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/api"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSecurityComplianceApp(t *testing.T) *fiber.App {
	t.Helper()
	return setupTestApp(t)
}

func TestSecurityMFAFlow_HappyAndDeny(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	app := setupSecurityComplianceApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	setupReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/mfa/setup", bytes.NewReader([]byte(`{"method":"email_otp"}`)))
	setupReq.Header.Set("Authorization", "Bearer "+accessToken)
	setupReq.Header.Set("Content-Type", "application/json")
	setupResp, err := app.Test(setupReq)
	require.NoError(t, err)
	defer setupResp.Body.Close()
	require.Equal(t, http.StatusOK, setupResp.StatusCode)

	challengeReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/mfa/challenge", bytes.NewReader([]byte(`{}`)))
	challengeReq.Header.Set("Authorization", "Bearer "+accessToken)
	challengeReq.Header.Set("Content-Type", "application/json")
	challengeResp, err := app.Test(challengeReq)
	require.NoError(t, err)
	defer challengeResp.Body.Close()
	require.Equal(t, http.StatusOK, challengeResp.StatusCode)

	var challengeBody struct {
		Data struct {
			OTPCode string `json:"otp_code"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(challengeResp.Body).Decode(&challengeBody))
	require.Len(t, challengeBody.Data.OTPCode, 6)

	badVerifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/mfa/verify", bytes.NewReader([]byte(`{"code":"000000"}`)))
	badVerifyReq.Header.Set("Authorization", "Bearer "+accessToken)
	badVerifyReq.Header.Set("Content-Type", "application/json")
	badVerifyResp, err := app.Test(badVerifyReq)
	require.NoError(t, err)
	defer badVerifyResp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, badVerifyResp.StatusCode)

	goodVerifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/mfa/verify", bytes.NewReader([]byte(`{"code":"`+challengeBody.Data.OTPCode+`"}`)))
	goodVerifyReq.Header.Set("Authorization", "Bearer "+accessToken)
	goodVerifyReq.Header.Set("Content-Type", "application/json")
	goodVerifyResp, err := app.Test(goodVerifyReq)
	require.NoError(t, err)
	defer goodVerifyResp.Body.Close()
	require.Equal(t, http.StatusOK, goodVerifyResp.StatusCode)

	// Pass 2 audit fix H-2: disabling MFA now requires step-up. Issue a
	// fresh challenge and present the new OTP to disable cleanly. A bare
	// disable without a code must be rejected.
	bareDisableReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/mfa/disable", nil)
	bareDisableReq.Header.Set("Authorization", "Bearer "+accessToken)
	bareDisableResp, err := app.Test(bareDisableReq)
	require.NoError(t, err)
	defer bareDisableResp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, bareDisableResp.StatusCode)

	stepUpChallengeReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/mfa/challenge", bytes.NewReader([]byte(`{}`)))
	stepUpChallengeReq.Header.Set("Authorization", "Bearer "+accessToken)
	stepUpChallengeReq.Header.Set("Content-Type", "application/json")
	stepUpChallengeResp, err := app.Test(stepUpChallengeReq)
	require.NoError(t, err)
	defer stepUpChallengeResp.Body.Close()
	require.Equal(t, http.StatusOK, stepUpChallengeResp.StatusCode)
	var stepUpChallengeBody struct {
		Data struct {
			OTPCode string `json:"otp_code"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(stepUpChallengeResp.Body).Decode(&stepUpChallengeBody))
	require.Len(t, stepUpChallengeBody.Data.OTPCode, 6)

	disableReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/mfa/disable", bytes.NewReader([]byte(`{"code":"`+stepUpChallengeBody.Data.OTPCode+`"}`)))
	disableReq.Header.Set("Authorization", "Bearer "+accessToken)
	disableReq.Header.Set("Content-Type", "application/json")
	disableResp, err := app.Test(disableReq)
	require.NoError(t, err)
	defer disableResp.Body.Close()
	require.Equal(t, http.StatusOK, disableResp.StatusCode)
}

func TestSecurityAPIKeysAndIPFilter_HappyAndDeny(t *testing.T) {
	app := setupSecurityComplianceApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	createOneReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/api-keys", bytes.NewReader([]byte(`{"name":"default-allow-key"}`)))
	createOneReq.Header.Set("Authorization", "Bearer "+accessToken)
	createOneReq.Header.Set("Content-Type", "application/json")
	createOneResp, err := app.Test(createOneReq)
	require.NoError(t, err)
	defer createOneResp.Body.Close()
	require.Equal(t, http.StatusCreated, createOneResp.StatusCode)

	var createdOne struct {
		Data struct {
			ID     string `json:"id"`
			APIKey string `json:"api_key"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(createOneResp.Body).Decode(&createdOne))
	require.NotEmpty(t, createdOne.Data.ID)
	require.NotEmpty(t, createdOne.Data.APIKey)

	createTwoReq := httptest.NewRequest(http.MethodPost, "/api/v1/security/api-keys", bytes.NewReader([]byte(`{"name":"denied-key"}`)))
	createTwoReq.Header.Set("Authorization", "Bearer "+accessToken)
	createTwoReq.Header.Set("Content-Type", "application/json")
	createTwoResp, err := app.Test(createTwoReq)
	require.NoError(t, err)
	defer createTwoResp.Body.Close()
	require.Equal(t, http.StatusCreated, createTwoResp.StatusCode)

	var createdTwo struct {
		Data struct {
			ID     string `json:"id"`
			APIKey string `json:"api_key"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(createTwoResp.Body).Decode(&createdTwo))

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.DB().Exec(`
INSERT INTO ip_access_rules (id, api_key_id, cidr, action, enabled, created_at, updated_at)
VALUES (?, ?, ?, 'deny', 1, ?, ?)
`, "iprule_deny_1", createdTwo.Data.ID, "203.0.113.55/32", now, now)
	require.NoError(t, err)

	machineApp := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})
	machineApp.Use(middleware.EnforceAPIKeyIPAccess)
	machineApp.Get("/machine", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	denyReq := httptest.NewRequest(http.MethodGet, "/machine", nil)
	denyReq.Header.Set("X-API-Key", createdTwo.Data.APIKey)
	denyReq.Header.Set("X-Forwarded-For", "203.0.113.55")
	denyReq.RemoteAddr = "203.0.113.55:44321"
	denyResp, err := machineApp.Test(denyReq)
	require.NoError(t, err)
	defer denyResp.Body.Close()
	assert.Equal(t, http.StatusForbidden, denyResp.StatusCode)

	happyReq := httptest.NewRequest(http.MethodGet, "/machine", nil)
	happyReq.Header.Set("X-API-Key", createdOne.Data.APIKey)
	happyReq.Header.Set("X-Forwarded-For", "198.51.100.10")
	happyReq.RemoteAddr = "198.51.100.10:4567"
	happyResp, err := machineApp.Test(happyReq)
	require.NoError(t, err)
	defer happyResp.Body.Close()
	assert.Equal(t, http.StatusOK, happyResp.StatusCode)
}

func TestSecurityFeatureFlags_AdminSetCustomerReadAndDeny(t *testing.T) {
	app := setupSecurityComplianceApp(t)
	customerToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)
	adminToken, _ := issueAuthTokens(t, testdata.AdminID)

	denyReq := httptest.NewRequest(http.MethodPut, "/api/v1/security/feature-flags/miss-032", bytes.NewReader([]byte(`{"enabled":true}`)))
	denyReq.Header.Set("Authorization", "Bearer "+customerToken)
	denyReq.Header.Set("Content-Type", "application/json")
	denyResp, err := app.Test(denyReq)
	require.NoError(t, err)
	defer denyResp.Body.Close()
	assert.Equal(t, http.StatusForbidden, denyResp.StatusCode)

	setReq := httptest.NewRequest(http.MethodPut, "/api/v1/security/feature-flags/miss-032", bytes.NewReader([]byte(`{"enabled":true,"rollout_percent":25,"description":"wave 11 test flag"}`)))
	setReq.Header.Set("Authorization", "Bearer "+adminToken)
	setReq.Header.Set("Content-Type", "application/json")
	setResp, err := app.Test(setReq)
	require.NoError(t, err)
	defer setResp.Body.Close()
	assert.Equal(t, http.StatusOK, setResp.StatusCode)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/security/feature-flags/miss-032", nil)
	getReq.Header.Set("Authorization", "Bearer "+customerToken)
	getResp, err := app.Test(getReq)
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var body struct {
		Data struct {
			Enabled        bool `json:"enabled"`
			RolloutPercent int  `json:"rollout_percent"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&body))
	assert.True(t, body.Data.Enabled)
	assert.Equal(t, 25, body.Data.RolloutPercent)
}

func TestComplianceCookieGDPRRecipientRestoreAndVersionHistory(t *testing.T) {
	app := setupSecurityComplianceApp(t)
	accessToken, _ := issueAuthTokens(t, testdata.CustomerAliceID)

	getConsentReq := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/cookie-consent", nil)
	getConsentReq.Header.Set("Authorization", "Bearer "+accessToken)
	getConsentResp, err := app.Test(getConsentReq)
	require.NoError(t, err)
	defer getConsentResp.Body.Close()
	require.Equal(t, http.StatusOK, getConsentResp.StatusCode)

	setConsentReq := httptest.NewRequest(http.MethodPut, "/api/v1/compliance/cookie-consent", bytes.NewReader([]byte(`{"consent_version":"v2","functional":true,"analytics":true,"marketing":false}`)))
	setConsentReq.Header.Set("Authorization", "Bearer "+accessToken)
	setConsentReq.Header.Set("Content-Type", "application/json")
	setConsentResp, err := app.Test(setConsentReq)
	require.NoError(t, err)
	defer setConsentResp.Body.Close()
	require.Equal(t, http.StatusOK, setConsentResp.StatusCode)

	exportReq := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/gdpr/export-request", nil)
	exportReq.Header.Set("Authorization", "Bearer "+accessToken)
	exportResp, err := app.Test(exportReq)
	require.NoError(t, err)
	defer exportResp.Body.Close()
	require.Equal(t, http.StatusCreated, exportResp.StatusCode)

	var exportBody struct {
		Data struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(exportResp.Body).Decode(&exportBody))
	require.NotEmpty(t, exportBody.Data.RequestID)

	versionsReq := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/version-history/gdpr_request/"+exportBody.Data.RequestID, nil)
	versionsReq.Header.Set("Authorization", "Bearer "+accessToken)
	versionsResp, err := app.Test(versionsReq)
	require.NoError(t, err)
	defer versionsResp.Body.Close()
	require.Equal(t, http.StatusOK, versionsResp.StatusCode)

	var versionsBody struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(versionsResp.Body).Decode(&versionsBody))
	require.NotEmpty(t, versionsBody.Data)

	gdprListReq := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/gdpr/requests", nil)
	gdprListReq.Header.Set("Authorization", "Bearer "+accessToken)
	gdprListResp, err := app.Test(gdprListReq)
	require.NoError(t, err)
	defer gdprListResp.Body.Close()
	require.Equal(t, http.StatusOK, gdprListResp.StatusCode)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.DB().ExecContext(context.Background(), `
UPDATE recipients
SET deleted_at = ?
WHERE id = ? AND user_id = ?
`, now, testdata.RecipientGeorgetown, testdata.CustomerAliceID)
	require.NoError(t, err)

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/recipients/"+testdata.RecipientGeorgetown+"/restore", nil)
	restoreReq.Header.Set("Authorization", "Bearer "+accessToken)
	restoreResp, err := app.Test(restoreReq)
	require.NoError(t, err)
	defer restoreResp.Body.Close()
	require.Equal(t, http.StatusOK, restoreResp.StatusCode)

	var deletedAt sql.NullString
	err = db.DB().QueryRowContext(context.Background(), `SELECT deleted_at FROM recipients WHERE id = ?`, testdata.RecipientGeorgetown).Scan(&deletedAt)
	require.NoError(t, err)
	assert.False(t, deletedAt.Valid)

	conflictReq := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/recipients/"+testdata.RecipientGeorgetown+"/restore", nil)
	conflictReq.Header.Set("Authorization", "Bearer "+accessToken)
	conflictResp, err := app.Test(conflictReq)
	require.NoError(t, err)
	defer conflictResp.Body.Close()
	assert.Equal(t, http.StatusConflict, conflictResp.StatusCode)
}

func hashForTest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

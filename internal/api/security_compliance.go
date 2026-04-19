package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RegisterSecurityCompliance mounts security/compliance endpoints.
// Caller is responsible for wiring under /api/v1.
func RegisterSecurityCompliance(g fiber.Router) {
	security := g.Group("/security")
	security.Post("/mfa/setup", middleware.RequireAuth, securityMFASetup)
	security.Post("/mfa/challenge", middleware.RequireAuth, securityMFAChallenge)
	security.Post("/mfa/verify", middleware.RequireAuth, securityMFAVerify)
	security.Post("/mfa/disable", middleware.RequireAuth, securityMFADisable)

	security.Post("/api-keys", middleware.RequireAuth, securityAPIKeyCreate)
	security.Get("/api-keys", middleware.RequireAuth, securityAPIKeyList)
	security.Post("/api-keys/:id/revoke", middleware.RequireAuth, securityAPIKeyRevoke)
	security.Post("/api-keys/:id/rotate", middleware.RequireAuth, securityAPIKeyRotate)

	// Pass 2.5 MED-17: feature flags expose enable/rollout state for
	// every server-side flag (including ones gated to staff/admin
	// surfaces). The list endpoint must be admin-only so a customer
	// cannot enumerate which features exist or read rollout percentages
	// that hint at unreleased behaviour. The :key GET remains broadly
	// readable so feature checks from authenticated clients still work
	// (rollout_percent is the public surface those clients already act
	// on); the PUT remains admin-only.
	security.Get("/feature-flags", middleware.RequireAuth, middleware.RequireAdmin, securityFeatureFlagsList)
	security.Get("/feature-flags/:key", middleware.RequireAuth, securityFeatureFlagGet)
	security.Put("/feature-flags/:key", middleware.RequireAuth, middleware.RequireAdmin, securityFeatureFlagSet)

	compliance := g.Group("/compliance")
	compliance.Get("/cookie-consent", middleware.RequireAuth, complianceCookieConsentGet)
	compliance.Put("/cookie-consent", middleware.RequireAuth, complianceCookieConsentSet)

	compliance.Post("/gdpr/export-request", middleware.RequireAuth, complianceGDPRExportRequest)
	compliance.Post("/gdpr/delete-request", middleware.RequireAuth, complianceGDPRDeleteRequest)
	compliance.Get("/gdpr/requests", middleware.RequireAuth, complianceGDPRRequests)

	compliance.Post("/recipients/:id/restore", middleware.RequireAuth, complianceRecipientRestore)
	compliance.Get("/version-history/:resource_type/:resource_id", middleware.RequireAuth, complianceVersionHistory)
}

func securityMFASetup(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	var body struct {
		Method string `json:"method"`
	}
	if err := c.BodyParser(&body); err != nil && len(c.Body()) > 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}

	// Pass 3 D1: TOTP enrolment was previously accepted but verification
	// returned 501 IMPLEMENTATION_PENDING, leaving the user stranded with an
	// unusable factor. Until RFC 6238 verify is wired end-to-end, only
	// email_otp is a valid method on this endpoint.
	method := strings.TrimSpace(body.Method)
	if method == "" {
		method = "email_otp"
	}
	if method != "email_otp" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "method must be email_otp"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	secret, err := randomTokenHex(16)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to setup MFA"))
	}
	// Pass 2 audit fix H-3: encrypt the TOTP shared secret before persisting
	// so a read-only DB compromise (backup leak, replica access) does not
	// hand attackers a working seed for every MFA-protected account. The raw
	// secret is still returned to the client once at setup time so the user
	// can scan/store it.
	storedSecret, err := services.EncryptSecret(secret)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to setup MFA"))
	}

	_, err = db.DB().ExecContext(c.Context(), `
INSERT INTO user_mfa (id, user_id, method, secret, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, 0, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    method = excluded.method,
    secret = excluded.secret,
    enabled = 0,
    updated_at = excluded.updated_at
`, uuid.NewString(), userID, method, storedSecret, now, now)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to setup MFA"))
	}

	resp := fiber.Map{
		"method":      method,
		"mfa_enabled": false,
	}

	return c.JSON(fiber.Map{"status": "success", "data": resp})
}

func securityMFAChallenge(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	var mfaID, method string
	err := db.DB().QueryRowContext(c.Context(), `
SELECT id, method
FROM user_mfa
WHERE user_id = ?
`, userID).Scan(&mfaID, &method)
	if err == sql.ErrNoRows {
		mfaID = uuid.NewString()
		method = "email_otp"
		secret, secErr := randomTokenHex(16)
		if secErr != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create MFA challenge"))
		}
		// Pass 2 audit fix H-3: encrypt-at-rest.
		storedSecret, encErr := services.EncryptSecret(secret)
		if encErr != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create MFA challenge"))
		}
		_, err = db.DB().ExecContext(c.Context(), `
INSERT INTO user_mfa (id, user_id, method, secret, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, 0, ?, ?)
`, mfaID, userID, method, storedSecret, nowStr, nowStr)
	}
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create MFA challenge"))
	}

	code, err := randomOTPCode()
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create MFA challenge"))
	}

	expiresAt := now.Add(10 * time.Minute).Format(time.RFC3339)
	_, err = db.DB().ExecContext(c.Context(), `
UPDATE user_mfa
SET otp_code_hash = ?, otp_expires_at = ?, failed_attempts = 0, last_challenge_at = ?, updated_at = ?
WHERE user_id = ?
`, hashString(code), expiresAt, nowStr, nowStr, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create MFA challenge"))
	}

	// Pass 2.5 HIGH-02 / Pass 3 D1: enqueue the OTP via the outbound email
	// worker so production users actually receive the code. email_otp is now
	// the only supported method, so there is no longer a TOTP branch to skip.
	// Email delivery is best-effort: the OTP row is already persisted, so a
	// transient enqueue failure should not fail the request — the user can
	// retry.
	if u, lookupErr := db.Queries().GetUserByID(c.Context(), userID); lookupErr == nil && strings.TrimSpace(u.Email) != "" {
		if enqErr := services.EnqueueEmail(c.Context(), services.TemplateMFAChallengeCode, u.Email, map[string]any{"code": code}); enqErr != nil {
			log.Printf("[mfa challenge] enqueue email for user %s: %v", userID, enqErr)
		}
	} else if lookupErr != nil {
		log.Printf("[mfa challenge] lookup user %s for email enqueue: %v", userID, lookupErr)
	}

	data := fiber.Map{
		"challenge_id": mfaID,
		"method":       method,
		"expires_at":   expiresAt,
	}
	if services.AllowDebugAuthArtifacts() {
		// Explicit local/test fallback when external email/SMS transport is not configured.
		data["otp_code"] = code
	}

	return c.JSON(fiber.Map{"status": "success", "data": data})
}

func securityMFAVerify(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	code := strings.TrimSpace(body.Code)
	if code == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "code required"))
	}

	var method string
	var otpHash, otpExpiresAt sql.NullString
	var failedAttempts int
	err := db.DB().QueryRowContext(c.Context(), `
SELECT method, otp_code_hash, otp_expires_at, failed_attempts
FROM user_mfa
WHERE user_id = ?
`, userID).Scan(&method, &otpHash, &otpExpiresAt, &failedAttempts)
	if err == sql.ErrNoRows {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "MFA not configured"))
	}
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to verify MFA"))
	}
	// Pass 3 D1: TOTP enrolment is no longer accepted at /mfa/setup, but a
	// row may still exist from before the cutover (or from a direct DB
	// write). Reject explicitly with a 400 so the user re-enrols on email_otp
	// rather than getting stuck against the email-OTP hash comparison below.
	if strings.EqualFold(strings.TrimSpace(method), "totp") {
		return c.Status(400).JSON(ErrorResponse{}.withCode(
			"VALIDATION_ERROR",
			"TOTP is no longer a supported MFA method. Please re-enroll using email_otp.",
		))
	}
	if !otpHash.Valid || strings.TrimSpace(otpHash.String) == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "No active MFA challenge"))
	}
	if otpExpiresAt.Valid && strings.TrimSpace(otpExpiresAt.String) != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339, otpExpiresAt.String)
		if parseErr == nil && time.Now().UTC().After(expiresAt) {
			return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "MFA code expired"))
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if hashString(code) != strings.TrimSpace(otpHash.String) {
		_, _ = db.DB().ExecContext(c.Context(), `
UPDATE user_mfa
SET failed_attempts = failed_attempts + 1,
    otp_code_hash = CASE WHEN failed_attempts + 1 >= 5 THEN NULL ELSE otp_code_hash END,
    otp_expires_at = CASE WHEN failed_attempts + 1 >= 5 THEN NULL ELSE otp_expires_at END,
    updated_at = ?
WHERE user_id = ?
`, now, userID)
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Invalid MFA code"))
	}

	_, err = db.DB().ExecContext(c.Context(), `
UPDATE user_mfa
SET enabled = 1,
    failed_attempts = 0,
    otp_code_hash = NULL,
    otp_expires_at = NULL,
    last_verified_at = ?,
    updated_at = ?
WHERE user_id = ?
`, now, now, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to verify MFA"))
	}

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"mfa_enabled": true}})
}

// securityMFADisable disables MFA for the current user.
//
// Pass 2 audit fix H-2: requires step-up — the caller must provide a current,
// unused OTP code (within the same 10-minute challenge window the user just
// completed) so a brief session compromise cannot silently strip MFA from
// the victim account. If MFA was never enabled the call is a no-op.
//
// Pass 2.5 MED-20: accept an alternative password step-up. If the caller
// supplies a non-empty `password` field and the user has a stored
// password hash, a successful bcrypt comparison authorises the disable
// just like a current OTP would. This unblocks users who lost access to
// their MFA method (e.g. inbox compromised, OTP delivery broken) and
// can still prove account ownership with their password. Falls back to
// the OTP path when password is absent or the user has no password set.
func securityMFADisable(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	var body struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil && len(c.Body()) > 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	code := strings.TrimSpace(body.Code)
	password := body.Password // intentionally NOT trimmed; passwords may have leading/trailing spaces

	// Look up MFA state. If the user has no row or MFA is disabled there is
	// nothing to do (idempotent). If MFA is enabled we require a current OTP
	// or a valid password (MED-20).
	var (
		mfaEnabled              int
		otpHash, otpExpiresAtNS sql.NullString
	)
	err := db.DB().QueryRowContext(c.Context(), `
SELECT enabled, otp_code_hash, otp_expires_at
FROM user_mfa
WHERE user_id = ?
`, userID).Scan(&mfaEnabled, &otpHash, &otpExpiresAtNS)
	if err == sql.ErrNoRows || (err == nil && mfaEnabled == 0) {
		// Nothing to disable.
		return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"mfa_enabled": false}})
	}
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to disable MFA"))
	}

	// Pass 2.5 MED-20: try password step-up first when supplied. We
	// compare against the user's stored bcrypt hash. A successful match
	// is sufficient to authorise the disable; we then skip the OTP
	// branch entirely. A supplied-but-wrong password falls through to
	// the OTP path so we don't reveal whether the password was correct.
	passwordVerified := false
	if password != "" {
		ok, vErr := services.VerifyUserPassword(c.Context(), userID, password)
		if vErr != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to disable MFA"))
		}
		passwordVerified = ok
	}

	if !passwordVerified {
		if code == "" {
			return c.Status(401).JSON(ErrorResponse{}.withCode("MFA_REQUIRED", "current MFA code or password required to disable MFA"))
		}
		if !otpHash.Valid || strings.TrimSpace(otpHash.String) == "" {
			return c.Status(401).JSON(ErrorResponse{}.withCode("MFA_REQUIRED", "no active MFA challenge; request a new code"))
		}
		if otpExpiresAtNS.Valid && strings.TrimSpace(otpExpiresAtNS.String) != "" {
			if exp, parseErr := time.Parse(time.RFC3339, otpExpiresAtNS.String); parseErr == nil && time.Now().UTC().After(exp) {
				return c.Status(401).JSON(ErrorResponse{}.withCode("MFA_REQUIRED", "MFA code expired; request a new code"))
			}
		}
		if hashString(code) != strings.TrimSpace(otpHash.String) {
			return c.Status(401).JSON(ErrorResponse{}.withCode("MFA_INVALID", "invalid MFA code"))
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.DB().ExecContext(c.Context(), `
UPDATE user_mfa
SET enabled = 0,
    otp_code_hash = NULL,
    otp_expires_at = NULL,
    failed_attempts = 0,
    updated_at = ?
WHERE user_id = ?
`, now, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to disable MFA"))
	}

	// Best-effort notification to the user that MFA was disabled.
	if u, lookupErr := db.Queries().GetUserByID(c.Context(), userID); lookupErr == nil {
		if mailErr := services.SendSecurityAlert(u.Email, "MFA disabled", "Multi-factor authentication was disabled on your account. If you did not do this, contact support immediately."); mailErr != nil {
			log.Printf("[security] failed to send MFA-disabled email to %s: %v", u.Email, mailErr)
		}
	}

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"mfa_enabled": false}})
}

func securityAPIKeyCreate(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	var body struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ExpiresAt string   `json:"expires_at"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name required"))
	}

	expiresAt := strings.TrimSpace(body.ExpiresAt)
	if expiresAt != "" {
		if _, err := time.Parse(time.RFC3339, expiresAt); err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "expires_at must be RFC3339"))
		}
	}

	rawKey, keyPrefix, keyHash, err := buildAPIKeyMaterial()
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create API key"))
	}

	scopesJSON, err := json.Marshal(body.Scopes)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create API key"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	apiKeyID := uuid.NewString()
	_, err = db.DB().ExecContext(c.Context(), `
INSERT INTO api_keys (
    id, user_id, name, key_prefix, key_hash, scopes_json,
    last_used_at, expires_at, revoked_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, NULL, ?, NULL, ?, ?)
`, apiKeyID, userID, name, keyPrefix, keyHash, string(scopesJSON), nullIfEmpty(expiresAt), now, now)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create API key"))
	}

	_ = appendResourceVersion(c.Context(), "api_key", apiKeyID, "created", userID, fiber.Map{
		"name":       name,
		"key_prefix": keyPrefix,
	})

	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{
		"id":         apiKeyID,
		"name":       name,
		"key_prefix": keyPrefix,
		"api_key":    rawKey,
		"expires_at": nullJSONString(expiresAt),
		"created_at": now,
	}})
}

func securityAPIKeyList(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	rows, err := db.DB().QueryContext(c.Context(), `
SELECT id, name, key_prefix, scopes_json, last_used_at, expires_at, revoked_at, created_at, updated_at
FROM api_keys
WHERE user_id = ?
ORDER BY created_at DESC
`, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list API keys"))
	}
	defer rows.Close()

	data := make([]fiber.Map, 0)
	for rows.Next() {
		var id, name, keyPrefix, scopesJSON, createdAt, updatedAt string
		var lastUsedAt, expiresAt, revokedAt sql.NullString
		if err := rows.Scan(&id, &name, &keyPrefix, &scopesJSON, &lastUsedAt, &expiresAt, &revokedAt, &createdAt, &updatedAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list API keys"))
		}
		var scopes []string
		if strings.TrimSpace(scopesJSON) != "" {
			_ = json.Unmarshal([]byte(scopesJSON), &scopes)
		}
		if scopes == nil {
			scopes = []string{}
		}
		data = append(data, fiber.Map{
			"id":           id,
			"name":         name,
			"key_prefix":   keyPrefix,
			"scopes":       scopes,
			"last_used_at": nullStringValue(lastUsedAt),
			"expires_at":   nullStringValue(expiresAt),
			"revoked_at":   nullStringValue(revokedAt),
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list API keys"))
	}

	return c.JSON(fiber.Map{"data": data})
}

func securityAPIKeyRevoke(c *fiber.Ctx) error {
	userID := currentUserID(c)
	id := strings.TrimSpace(c.Params("id"))
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.DB().ExecContext(c.Context(), `
UPDATE api_keys
SET revoked_at = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND revoked_at IS NULL
`, now, now, id, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to revoke API key"))
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "API key not found"))
	}

	_ = appendResourceVersion(c.Context(), "api_key", id, "revoked", userID, fiber.Map{"revoked_at": now})
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id, "revoked_at": now}})
}

func securityAPIKeyRotate(c *fiber.Ctx) error {
	userID := currentUserID(c)
	id := strings.TrimSpace(c.Params("id"))
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}

	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to rotate API key"))
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("securityAPIKeyRotate rollback failed: %v", rbErr)
		}
	}()

	var name, scopesJSON string
	var expiresAt sql.NullString
	err = tx.QueryRowContext(c.Context(), `
SELECT name, scopes_json, expires_at
FROM api_keys
WHERE id = ? AND user_id = ? AND revoked_at IS NULL
`, id, userID).Scan(&name, &scopesJSON, &expiresAt)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "API key not found"))
	}
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to rotate API key"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(c.Context(), `
UPDATE api_keys
SET revoked_at = ?, updated_at = ?
WHERE id = ? AND user_id = ?
`, now, now, id, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to rotate API key"))
	}

	rawKey, keyPrefix, keyHash, err := buildAPIKeyMaterial()
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to rotate API key"))
	}

	newID := uuid.NewString()
	_, err = tx.ExecContext(c.Context(), `
INSERT INTO api_keys (
    id, user_id, name, key_prefix, key_hash, scopes_json,
    last_used_at, expires_at, revoked_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, NULL, ?, NULL, ?, ?)
`, newID, userID, name, keyPrefix, keyHash, scopesJSON, expiresAt, now, now)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to rotate API key"))
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to rotate API key"))
	}

	_ = appendResourceVersion(c.Context(), "api_key", id, "rotated", userID, fiber.Map{"rotated_to": newID})
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{
		"id":           newID,
		"name":         name,
		"key_prefix":   keyPrefix,
		"api_key":      rawKey,
		"expires_at":   nullStringValue(expiresAt),
		"rotated_from": id,
	}})
}

func securityFeatureFlagsList(c *fiber.Ctx) error {
	rows, err := db.DB().QueryContext(c.Context(), `
SELECT flag_key, description, enabled, rollout_percent, updated_by, created_at, updated_at
FROM feature_flags
ORDER BY flag_key ASC
`)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list feature flags"))
	}
	defer rows.Close()

	flags := make([]fiber.Map, 0)
	for rows.Next() {
		var key string
		var description, updatedBy sql.NullString
		var enabled, rollout int
		var createdAt, updatedAt string
		if err := rows.Scan(&key, &description, &enabled, &rollout, &updatedBy, &createdAt, &updatedAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list feature flags"))
		}
		flags = append(flags, featureFlagMap(key, description, enabled, rollout, updatedBy, createdAt, updatedAt))
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list feature flags"))
	}
	return c.JSON(fiber.Map{"data": flags})
}

func securityFeatureFlagGet(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	if key == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "key required"))
	}

	flag, err := loadFeatureFlag(c, key)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Feature flag not found"))
	}
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load feature flag"))
	}

	return c.JSON(fiber.Map{"data": flag})
}

func securityFeatureFlagSet(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.Params("key"))
	userID := currentUserID(c)
	if key == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "key required"))
	}

	var body struct {
		Description    *string `json:"description"`
		Enabled        *bool   `json:"enabled"`
		RolloutPercent *int    `json:"rollout_percent"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Description == nil && body.Enabled == nil && body.RolloutPercent == nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "at least one field is required"))
	}

	existing, err := loadFeatureFlag(c, key)
	if err != nil && err != sql.ErrNoRows {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update feature flag"))
	}

	description := sql.NullString{}
	enabled := 0
	rollout := 100
	if err == nil {
		if v, ok := existing["description"].(string); ok && strings.TrimSpace(v) != "" {
			description = sql.NullString{String: v, Valid: true}
		}
		if v, ok := existing["enabled"].(bool); ok && v {
			enabled = 1
		}
		if v, ok := existing["rollout_percent"].(float64); ok {
			rollout = int(v)
		}
	}
	if body.Description != nil {
		d := strings.TrimSpace(*body.Description)
		if d == "" {
			description = sql.NullString{}
		} else {
			description = sql.NullString{String: d, Valid: true}
		}
	}
	if body.Enabled != nil {
		enabled = scBoolToInt(*body.Enabled)
	}
	if body.RolloutPercent != nil {
		rollout = *body.RolloutPercent
	}
	if rollout < 0 || rollout > 100 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "rollout_percent must be between 0 and 100"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.DB().ExecContext(c.Context(), `
INSERT INTO feature_flags (flag_key, description, enabled, rollout_percent, updated_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(flag_key) DO UPDATE SET
    description = excluded.description,
    enabled = excluded.enabled,
    rollout_percent = excluded.rollout_percent,
    updated_by = excluded.updated_by,
    updated_at = excluded.updated_at
`, key, description, enabled, rollout, nullIfEmpty(userID), now, now)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update feature flag"))
	}

	_ = appendResourceVersion(c.Context(), "feature_flag", key, "updated", userID, fiber.Map{
		"enabled":         enabled == 1,
		"rollout_percent": rollout,
		"description":     nullStringValue(description),
	})

	flag, err := loadFeatureFlag(c, key)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load feature flag"))
	}
	return c.JSON(fiber.Map{"status": "success", "data": flag})
}

func loadFeatureFlag(c *fiber.Ctx, key string) (fiber.Map, error) {
	var description, updatedBy sql.NullString
	var enabled, rollout int
	var createdAt, updatedAt string
	err := db.DB().QueryRowContext(c.Context(), `
SELECT flag_key, description, enabled, rollout_percent, updated_by, created_at, updated_at
FROM feature_flags
WHERE flag_key = ?
`, key).Scan(&key, &description, &enabled, &rollout, &updatedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return featureFlagMap(key, description, enabled, rollout, updatedBy, createdAt, updatedAt), nil
}

func featureFlagMap(key string, description sql.NullString, enabled, rollout int, updatedBy sql.NullString, createdAt, updatedAt string) fiber.Map {
	return fiber.Map{
		"key":             key,
		"description":     nullStringValue(description),
		"enabled":         intToBool(enabled),
		"rollout_percent": rollout,
		"updated_by":      nullStringValue(updatedBy),
		"created_at":      createdAt,
		"updated_at":      updatedAt,
	}
}

func buildAPIKeyMaterial() (rawKey, keyPrefix, keyHash string, err error) {
	raw, err := randomTokenHex(24)
	if err != nil {
		return "", "", "", err
	}
	rawKey = "qcs_" + raw
	prefixLen := 12
	if len(rawKey) < prefixLen {
		prefixLen = len(rawKey)
	}
	keyPrefix = rawKey[:prefixLen]
	keyHash = hashString(rawKey)
	return rawKey, keyPrefix, keyHash, nil
}

func randomOTPCode() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	if n < 0 {
		n = -n
	}
	return fmt.Sprintf("%06d", n%1000000), nil
}

func randomTokenHex(size int) (string, error) {
	if size <= 0 {
		return "", nil
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func currentUserID(c *fiber.Ctx) string {
	if userID, ok := c.Locals(middleware.CtxUserID).(string); ok {
		return strings.TrimSpace(userID)
	}
	return ""
}

func scBoolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func intToBool(v int) bool {
	return v != 0
}

func nullIfEmpty(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullStringValue(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	s := strings.TrimSpace(v.String)
	if s == "" {
		return nil
	}
	return s
}

func nullJSONString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

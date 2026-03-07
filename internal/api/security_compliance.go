package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	security.Get("/feature-flags", middleware.RequireAuth, securityFeatureFlagsList)
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

	method := strings.TrimSpace(body.Method)
	if method == "" {
		method = "email_otp"
	}
	if method != "email_otp" && method != "totp" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "method must be email_otp or totp"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	secret, err := randomTokenHex(16)
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
`, uuid.NewString(), userID, method, secret, now, now)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to setup MFA"))
	}

	resp := fiber.Map{
		"method":      method,
		"mfa_enabled": false,
	}
	if method == "totp" {
		resp["secret"] = secret
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
		_, err = db.DB().ExecContext(c.Context(), `
INSERT INTO user_mfa (id, user_id, method, secret, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, 0, ?, ?)
`, mfaID, userID, method, secret, nowStr, nowStr)
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

	var otpHash, otpExpiresAt sql.NullString
	var failedAttempts int
	err := db.DB().QueryRowContext(c.Context(), `
SELECT otp_code_hash, otp_expires_at, failed_attempts
FROM user_mfa
WHERE user_id = ?
`, userID).Scan(&otpHash, &otpExpiresAt, &failedAttempts)
	if err == sql.ErrNoRows {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "MFA not configured"))
	}
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to verify MFA"))
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

func securityMFADisable(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.DB().ExecContext(c.Context(), `
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
	defer tx.Rollback()

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

func complianceCookieConsentGet(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	var id, consentVersion, consentedAt, updatedAt string
	var necessary, functional, analytics, marketing int
	var source, ipAddress, userAgent sql.NullString
	err := db.DB().QueryRowContext(c.Context(), `
SELECT id, consent_version, necessary, functional, analytics, marketing,
       source, ip_address, user_agent, consented_at, updated_at
FROM cookie_consents
WHERE user_id = ?
`, userID).Scan(
		&id, &consentVersion, &necessary, &functional, &analytics, &marketing,
		&source, &ipAddress, &userAgent, &consentedAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return c.JSON(fiber.Map{"data": fiber.Map{
			"id":              "",
			"consent_version": "v1",
			"necessary":       true,
			"functional":      false,
			"analytics":       false,
			"marketing":       false,
			"source":          nil,
			"ip_address":      nil,
			"user_agent":      nil,
			"consented_at":    nil,
			"updated_at":      nil,
		}})
	}
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load cookie consent"))
	}

	return c.JSON(fiber.Map{"data": fiber.Map{
		"id":              id,
		"consent_version": consentVersion,
		"necessary":       intToBool(necessary),
		"functional":      intToBool(functional),
		"analytics":       intToBool(analytics),
		"marketing":       intToBool(marketing),
		"source":          nullStringValue(source),
		"ip_address":      nullStringValue(ipAddress),
		"user_agent":      nullStringValue(userAgent),
		"consented_at":    consentedAt,
		"updated_at":      updatedAt,
	}})
}

func complianceCookieConsentSet(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	var body struct {
		ConsentVersion string `json:"consent_version"`
		Necessary      *bool  `json:"necessary"`
		Functional     bool   `json:"functional"`
		Analytics      bool   `json:"analytics"`
		Marketing      bool   `json:"marketing"`
		Source         string `json:"source"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}

	consentVersion := strings.TrimSpace(body.ConsentVersion)
	if consentVersion == "" {
		consentVersion = "v1"
	}
	necessary := true
	if body.Necessary != nil {
		necessary = *body.Necessary
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.NewString()
	source := strings.TrimSpace(body.Source)
	if source == "" {
		source = "api"
	}

	_, err := db.DB().ExecContext(c.Context(), `
INSERT INTO cookie_consents (
    id, user_id, consent_version, necessary, functional, analytics, marketing,
    source, ip_address, user_agent, consented_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    consent_version = excluded.consent_version,
    necessary = excluded.necessary,
    functional = excluded.functional,
    analytics = excluded.analytics,
    marketing = excluded.marketing,
    source = excluded.source,
    ip_address = excluded.ip_address,
    user_agent = excluded.user_agent,
    consented_at = excluded.consented_at,
    updated_at = excluded.updated_at
`,
		id, userID, consentVersion,
		scBoolToInt(necessary), scBoolToInt(body.Functional), scBoolToInt(body.Analytics), scBoolToInt(body.Marketing),
		source, c.IP(), c.Get(fiber.HeaderUserAgent), now, now,
	)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to save cookie consent"))
	}

	_ = appendResourceVersion(c.Context(), "cookie_consent", userID, "updated", userID, fiber.Map{
		"consent_version": consentVersion,
		"necessary":       necessary,
		"functional":      body.Functional,
		"analytics":       body.Analytics,
		"marketing":       body.Marketing,
	})

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{
		"consent_version": consentVersion,
		"necessary":       necessary,
		"functional":      body.Functional,
		"analytics":       body.Analytics,
		"marketing":       body.Marketing,
		"updated_at":      now,
	}})
}

func complianceGDPRExportRequest(c *fiber.Ctx) error {
	return complianceGDPRCreateRequest(c, "export_request")
}

func complianceGDPRDeleteRequest(c *fiber.Ctx) error {
	return complianceGDPRCreateRequest(c, "delete_request")
}

func complianceGDPRCreateRequest(c *fiber.Ctx, changeType string) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	requestID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	payload := fiber.Map{
		"request_id":   requestID,
		"request_type": changeType,
		"status":       "requested",
		"requested_at": now,
		"user_id":      userID,
	}

	if err := appendResourceVersion(c.Context(), "gdpr_request", requestID, changeType, userID, payload); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create GDPR request"))
	}

	return c.Status(201).JSON(fiber.Map{"status": "success", "data": payload})
}

func complianceGDPRRequests(c *fiber.Ctx) error {
	userID := currentUserID(c)
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}

	rows, err := db.DB().QueryContext(c.Context(), `
SELECT resource_id, change_type, data_json, created_at
FROM resource_versions
WHERE resource_type = 'gdpr_request' AND changed_by = ?
ORDER BY created_at DESC
LIMIT 100
`, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list GDPR requests"))
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var resourceID, changeType, dataJSON, createdAt string
		if err := rows.Scan(&resourceID, &changeType, &dataJSON, &createdAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list GDPR requests"))
		}
		data := map[string]any{}
		if strings.TrimSpace(dataJSON) != "" {
			_ = json.Unmarshal([]byte(dataJSON), &data)
		}
		items = append(items, fiber.Map{
			"resource_id": resourceID,
			"change_type": changeType,
			"data":        data,
			"created_at":  createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list GDPR requests"))
	}

	return c.JSON(fiber.Map{"data": items})
}

func complianceRecipientRestore(c *fiber.Ctx) error {
	userID := currentUserID(c)
	recipientID := strings.TrimSpace(c.Params("id"))
	if userID == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Authorization required"))
	}
	if recipientID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.DB().ExecContext(c.Context(), `
UPDATE recipients
SET deleted_at = NULL, updated_at = ?
WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL
`, now, recipientID, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to restore recipient"))
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		var deletedAt sql.NullString
		err := db.DB().QueryRowContext(c.Context(), `
SELECT deleted_at
FROM recipients
WHERE id = ? AND user_id = ?
`, recipientID, userID).Scan(&deletedAt)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Recipient not found"))
		}
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to restore recipient"))
		}
		return c.Status(409).JSON(ErrorResponse{}.withCode("CONFLICT", "Recipient is not deleted"))
	}

	_ = appendResourceVersion(c.Context(), "recipient", recipientID, "restored", userID, fiber.Map{
		"recipient_id": recipientID,
		"restored_at":  now,
	})

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": recipientID, "restored_at": now}})
}

func complianceVersionHistory(c *fiber.Ctx) error {
	resourceType := strings.TrimSpace(c.Params("resource_type"))
	resourceID := strings.TrimSpace(c.Params("resource_id"))
	userID := currentUserID(c)
	role := strings.TrimSpace(fmt.Sprint(c.Locals(middleware.CtxUserRole)))
	if resourceType == "" || resourceID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "resource_type and resource_id required"))
	}

	query := `
SELECT version_no, change_type, data_json, changed_by, created_at
FROM resource_versions
WHERE resource_type = ? AND resource_id = ?
ORDER BY version_no DESC, created_at DESC
`
	args := []any{resourceType, resourceID}
	if role != "admin" {
		query = `
SELECT version_no, change_type, data_json, changed_by, created_at
FROM resource_versions
WHERE resource_type = ? AND resource_id = ? AND changed_by = ?
ORDER BY version_no DESC, created_at DESC
`
		args = append(args, userID)
	}

	rows, err := db.DB().QueryContext(c.Context(), query, args...)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load version history"))
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var versionNo int
		var changeType, dataJSON, createdAt string
		var changedBy sql.NullString
		if err := rows.Scan(&versionNo, &changeType, &dataJSON, &changedBy, &createdAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load version history"))
		}
		data := map[string]any{}
		if strings.TrimSpace(dataJSON) != "" {
			_ = json.Unmarshal([]byte(dataJSON), &data)
		}
		items = append(items, fiber.Map{
			"version_no":  versionNo,
			"change_type": changeType,
			"data":        data,
			"changed_by":  nullStringValue(changedBy),
			"created_at":  createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load version history"))
	}

	return c.JSON(fiber.Map{"data": items})
}

func appendResourceVersion(ctx any, resourceType, resourceID, changeType, changedBy string, payload any) error {
	ctxValue, ok := ctx.(interface {
		Deadline() (deadline time.Time, ok bool)
		Done() <-chan struct{}
		Err() error
		Value(key any) any
	})
	if !ok {
		return nil
	}
	var nextVersion int
	err := db.DB().QueryRowContext(ctxValue, `
SELECT COALESCE(MAX(version_no), 0) + 1
FROM resource_versions
WHERE resource_type = ? AND resource_id = ?
`, resourceType, resourceID).Scan(&nextVersion)
	if err != nil {
		return err
	}
	if nextVersion <= 0 {
		nextVersion = 1
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctxValue, `
INSERT INTO resource_versions (
    id, resource_type, resource_id, version_no,
    change_type, data_json, changed_by, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`,
		uuid.NewString(), resourceType, resourceID, nextVersion,
		changeType, string(payloadJSON), nullIfEmpty(changedBy), time.Now().UTC().Format(time.RFC3339),
	)
	return err
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

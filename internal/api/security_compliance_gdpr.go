package api

// security_compliance_gdpr.go holds the cookie consent + GDPR + version
// history surface split out from security_compliance.go in Phase 3.3
// (QAL-001). Routes remain registered by RegisterSecurityCompliance.

import (
	"database/sql"
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

	// Pass 2.5 CRIT-04 fix: a delete_request must actually erase data,
	// not merely log intent. Run the same anonymization the customer
	// would get from /api/v1/account/delete; mark the audit row
	// 'processed' once it succeeds.
	if changeType == "delete_request" {
		deletedEmail := "deleted+" + strings.TrimSpace(userID) + "@qcs.invalid"
		if err := services.AnonymizeUserData(c.Context(), userID, "Deleted User", deletedEmail); err != nil {
			log.Printf("[gdpr] anonymize on delete_request user=%s: %v", userID, err)
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to process delete request"))
		}
		payload["status"] = "processed"
		payload["processed_at"] = time.Now().UTC().Format(time.RFC3339)
	}

	if err := appendResourceVersion(c.Context(), "gdpr_request", requestID, changeType, userID, payload); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create GDPR request"))
	}

	return c.Status(201).JSON(fiber.Map{"status": "success", "data": payload})
}

// redactedResourceTypes lists resource_type values whose payload must
// not be stored verbatim in resource_versions. The GDPR endpoint and any
// future PII-bearing audit trail belongs here so the audit table itself
// does not become a permanent backup of the data the user asked to be
// deleted.
//
// Pass 2.5 CRIT-04 fix complement.
var redactedResourceTypes = map[string]struct{}{
	"gdpr_request":   {},
	"cookie_consent": {},
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

	// Pass 2.5 CRIT-04 fix: redact PII for resource types whose audit
	// trail must not retain the very data the user is asking to be
	// erased. We keep the version row itself (so the audit trail of
	// "request happened" is preserved) but strip the body to a marker.
	storedPayload := payload
	if _, redacted := redactedResourceTypes[resourceType]; redacted {
		storedPayload = map[string]any{
			"_redacted":   true,
			"reason":      "pii_excluded_from_audit",
			"change_type": changeType,
		}
	}
	payloadJSON, err := json.Marshal(storedPayload)
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



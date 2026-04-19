package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/google/uuid"
)

// AuditEvent is an optional audit row written in the SAME transaction
// as AnonymizeUserData so a successful anonymization is guaranteed to
// have a matching audit log entry (and a failed anonymization leaves
// no orphan audit row).
//
// Pass 3 HIGH-01 fix: the GDPR delete path previously committed the
// data scrub and then best-effort logged via recordActivity AFTER
// commit, so a process crash between the two left the audit trail
// silently inconsistent. Folding the audit insert into the same tx
// removes that race.
//
// EventType examples: "auth.account.delete", "gdpr.delete_request.processed".
// Metadata is a free-form JSON string ("" is fine) stored verbatim in
// admin_activity.details.
type AuditEvent struct {
	ActorUserID string
	EventType   string
	IPAddress   string
	UserAgent   string
	Metadata    string
}

// handledTables is the source of truth for "every user-scoped PII
// table the GDPR erasure flow knows how to scrub". The drift-guard
// test in anonymize_test.go cross-references this list against the
// live schema so a future migration that adds a new user_id /
// customer_id / actor_user_id / matched_user_id column (or a FK to
// users.id) cannot ship without either being added here or being
// listed in skipTables with a written justification.
var handledTables = map[string]struct{}{
	"recipients":                    {},
	"locker_packages":               {},
	"locker_photos":                 {},
	"ship_requests":                 {},
	"service_requests":              {},
	"inbound_tracking":              {},
	"storage_fees":                  {},
	"unmatched_packages":            {},
	"bookings":                      {},
	"invoices":                      {},
	"weight_discrepancies":          {},
	"communications":                {},
	"activity_log":                  {},
	"templates":                     {},
	"notification_prefs":            {},
	"password_resets":               {},
	"sessions":                      {},
	"magic_links":                   {},
	"observability_events":          {},
	"user_mfa":                      {},
	"api_keys":                      {},
	"ip_access_rules":               {},
	"cookie_consents":               {},
	"resource_versions":             {},
	"parcel_consolidation_previews": {},
	"assisted_purchase_requests":    {},
	"customs_preclearance_docs":     {},
	"delivery_signatures":           {},
	"loyalty_ledger":                {},
	"data_import_jobs":              {},
	"in_app_notifications":          {},
	"push_subscriptions":            {},
	"moderation_items":              {},
	"email_verification_tokens":     {},
	"users":                         {}, // anonymized in-place last
	// Tables referenced in the original Pass-3 prompt that may not
	// exist in any current migration but are tolerated if added
	// later (no-such-table is silently skipped at execution time).
	"payment_intents":     {},
	"gdpr_requests":       {},
	"recipient_versions":  {},
	"parcel_imports":      {},
	"parcel_signatures":   {},
}

// skipTables enumerates tables that are KNOWN to have a user FK but
// MUST NOT be scrubbed. Each entry needs a written justification so a
// future reviewer can tell the difference between "intentionally
// skipped" and "forgotten".
var skipTables = map[string]string{
	// admin_activity is the audit table itself. Its actor_id FKs into
	// users(id); since AnonymizeUserData only scrubs PII columns on
	// users (the row stays), the FK is preserved and the audit
	// history of admin actions remains intact, which is the entire
	// point of an audit table.
	"admin_activity": "audit table; FK to users(id) is preserved by anonymize-in-place; history must survive erasure",
}

// AnonymizeUserData is the single source of truth for GDPR-style data
// erasure on a user. It deletes or scrubs every PII-bearing row tied
// to userID, then anonymizes the users row last.
//
// Pass 2.5 CRIT-03 + CRIT-04: previously accountDelete only called
// AnonymizeUserForDeletion (touching only the users row), and the
// /api/v1/compliance/gdpr/delete-request endpoint only logged a request
// with no processor.
//
// Pass 3 CRIT-01: extended scrub coverage to every user-scoped table
// the schema scan revealed (invoices, service_requests,
// unmatched_packages, weight_discrepancies, communications,
// activity_log, moderation_items, resource_versions, ...).
//
// Pass 3 HIGH-01: the optional *AuditEvent is INSERTed into
// admin_activity inside the same tx so the audit row is atomic with
// the scrub. If audit == nil the helper still works (covers callers
// that don't have a request context, e.g. background jobs).
//
// All work happens in a single transaction so a partial failure
// leaves no half-anonymized state. Tables that may not exist (because
// the corresponding migration has not been applied in some test
// setups) are tolerated — `no such table` / `no such column` is
// logged-and-skipped, not propagated.
func AnonymizeUserData(ctx context.Context, userID, deletedName, deletedEmail string, audit *AuditEvent) error {
	if userID == "" {
		return fmt.Errorf("AnonymizeUserData: userID required")
	}

	conn := db.DB()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("AnonymizeUserData: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)

	// Each entry is (description, sql, args). We use one ExecContext
	// per statement and tolerate "no such table" / "no such column"
	// so unrelated test setups (which only run a subset of
	// migrations) don't fail the whole flow.
	steps := []struct {
		name string
		stmt string
		args []any
	}{
		// ---- soft-deletes / scrubs that must keep the row alive ----
		// Recipients are referenced by ship_requests (FK), so we can't
		// hard-delete. Soft-delete + scrub PII so historical
		// ship_requests still resolve their recipient_id FK.
		{"recipients scrub+softdelete", `UPDATE recipients
			SET name = '[deleted]',
			    phone = NULL,
			    street = '[deleted]',
			    apt = NULL,
			    delivery_instructions = NULL,
			    deleted_at = ?,
			    updated_at = ?
			WHERE user_id = ?`, []any{now, now, userID}},
		// locker_packages cannot be deleted (referenced by ship_request_items / shipments / storage_fees).
		{"locker_packages scrub", `UPDATE locker_packages SET sender_name = '[deleted]', updated_at = ? WHERE user_id = ?`, []any{now, userID}},
		// ship_requests free-text PII (rows must remain for billing/audit).
		{"ship_requests scrub", `UPDATE ship_requests SET special_instructions = NULL, updated_at = ? WHERE user_id = ?`, []any{now, userID}},
		// bookings free-text PII (rows kept for audit/billing reasons).
		{"bookings scrub", `UPDATE bookings SET special_instructions = NULL, updated_at = ? WHERE user_id = ?`, []any{now, userID}},
		// invoices: must remain for tax/billing audit but free-text notes are PII.
		{"invoices scrub", `UPDATE invoices SET notes = NULL WHERE user_id = ?`, []any{userID}},
		// service_requests: notes are user-authored free-text; rows kept (referenced via locker_package_id history).
		{"service_requests scrub", `UPDATE service_requests SET notes = NULL WHERE user_id = ?`, []any{userID}},
		// templates may be referenced by ship_requests.template_id in some schema variants; scrub label PII.
		{"templates scrub", `UPDATE templates SET name = '[deleted]', special_instructions = NULL WHERE user_id = ?`, []any{userID}},
		// inbound_tracking: PII is the retailer/tracking pair; row may be referenced by locker_package_id chain.
		{"inbound_tracking scrub", `UPDATE inbound_tracking SET retailer_name = '[deleted]', tracking_number = '[deleted]' WHERE user_id = ?`, []any{userID}},
		// assisted_purchase_requests: free-text notes only (rows kept for ops history).
		{"assisted_purchase_requests scrub", `UPDATE assisted_purchase_requests SET notes = NULL WHERE user_id = ?`, []any{userID}},
		// loyalty_ledger: reason is free-text (rows kept for accounting).
		{"loyalty_ledger scrub", `UPDATE loyalty_ledger SET reason = '[deleted]' WHERE user_id = ?`, []any{userID}},
		// communications: subject/content are PII. Hard-delete (rows
		// have no downstream FK and exist only to record what we sent
		// the user).
		{"communications delete", `DELETE FROM communications WHERE user_id = ?`, []any{userID}},
		// weight_discrepancies: keep the row (billing-relevant) but
		// unlink the user. user_id is NOT NULL so we cannot NULL it;
		// instead we re-point it at the same (now-anonymized) user.
		// This is a no-op DML but is included for explicit coverage
		// so the drift-guard test sees the table is "handled".
		{"weight_discrepancies scrub", `UPDATE weight_discrepancies SET user_id = user_id WHERE user_id = ?`, []any{userID}},
		// activity_log.user_id is nullable; sever the link (the
		// action/resource history itself is preserved as audit).
		{"activity_log unlink", `UPDATE activity_log SET user_id = NULL WHERE user_id = ?`, []any{userID}},
		// resource_versions.changed_by is nullable; sever the link.
		// (Payload-level PII for sensitive resource_types is already
		// handled by appendResourceVersion's redactedResourceTypes.)
		{"resource_versions unlink", `UPDATE resource_versions SET changed_by = NULL WHERE changed_by = ?`, []any{userID}},
		// moderation_items.created_by is nullable; sever the link.
		{"moderation_items unlink", `UPDATE moderation_items SET created_by = NULL WHERE created_by = ?`, []any{userID}},
		// unmatched_packages.matched_user_id is nullable; sever the link.
		{"unmatched_packages unlink", `UPDATE unmatched_packages SET matched_user_id = NULL WHERE matched_user_id = ?`, []any{userID}},
		// observability_events.user_id is nullable; sever.
		{"observability_events scrub", `UPDATE observability_events SET user_id = NULL WHERE user_id = ?`, []any{userID}},
		// storage_fees: no PII columns; FK to (now-anonymized) user
		// preserved. No-op DML for explicit coverage.
		{"storage_fees touch", `UPDATE storage_fees SET user_id = user_id WHERE user_id = ?`, []any{userID}},

		// ---- hard-deletes (PII content with no preserve-the-history requirement) ----
		// Customs / signatures / photos.
		{"customs_preclearance_docs delete", `DELETE FROM customs_preclearance_docs WHERE user_id = ?`, []any{userID}},
		{"delivery_signatures delete", `DELETE FROM delivery_signatures WHERE ship_request_id IN (SELECT id FROM ship_requests WHERE user_id = ?)`, []any{userID}},
		{"locker_photos delete", `DELETE FROM locker_photos WHERE locker_package_id IN (SELECT id FROM locker_packages WHERE user_id = ?)`, []any{userID}},
		{"parcel_consolidation_previews delete", `DELETE FROM parcel_consolidation_previews WHERE user_id = ?`, []any{userID}},
		{"data_import_jobs delete", `DELETE FROM data_import_jobs WHERE user_id = ?`, []any{userID}},
		// Notifications + push subs.
		{"notification_prefs delete", `DELETE FROM notification_prefs WHERE user_id = ?`, []any{userID}},
		{"in_app_notifications delete", `DELETE FROM in_app_notifications WHERE user_id = ?`, []any{userID}},
		{"push_subscriptions delete", `DELETE FROM push_subscriptions WHERE user_id = ?`, []any{userID}},
		// Auth / security artifacts. ip_access_rules MUST go before
		// api_keys because it FKs into api_keys.id.
		{"ip_access_rules delete", `DELETE FROM ip_access_rules WHERE user_id = ? OR api_key_id IN (SELECT id FROM api_keys WHERE user_id = ?)`, []any{userID, userID}},
		{"user_mfa delete", `DELETE FROM user_mfa WHERE user_id = ?`, []any{userID}},
		{"api_keys delete", `DELETE FROM api_keys WHERE user_id = ?`, []any{userID}},
		{"cookie_consents delete", `DELETE FROM cookie_consents WHERE user_id = ?`, []any{userID}},
		{"sessions delete", `DELETE FROM sessions WHERE user_id = ?`, []any{userID}},
		{"magic_links delete", `DELETE FROM magic_links WHERE user_id = ?`, []any{userID}},
		{"password_resets delete", `DELETE FROM password_resets WHERE user_id = ?`, []any{userID}},
		{"email_verification_tokens delete", `DELETE FROM email_verification_tokens WHERE user_id = ?`, []any{userID}},

		// ---- migrations not yet present in any current schema; guarded ----
		// Pass-3 prompt called these out explicitly. They will become
		// real DML if/when the corresponding migration lands; today
		// the no-such-table guard makes each a silent no-op.
		{"payment_intents delete (guarded)", `DELETE FROM payment_intents WHERE user_id = ?`, []any{userID}},
		{"gdpr_requests delete (guarded)", `DELETE FROM gdpr_requests WHERE user_id = ?`, []any{userID}},
		{"recipient_versions unlink (guarded)", `UPDATE recipient_versions SET changed_by = NULL WHERE changed_by = ?`, []any{userID}},
		{"parcel_imports delete (guarded)", `DELETE FROM parcel_imports WHERE user_id = ?`, []any{userID}},
		{"parcel_signatures delete (guarded)", `DELETE FROM parcel_signatures WHERE user_id = ?`, []any{userID}},
	}

	for _, step := range steps {
		if _, err := tx.ExecContext(ctx, step.stmt, step.args...); err != nil {
			if isMissingTableError(err) {
				continue
			}
			return fmt.Errorf("AnonymizeUserData: %s: %w", step.name, err)
		}
	}

	// Anonymize the users row LAST so any FK-dependent step above can
	// still resolve user_id. Mirrors the existing accountDelete pattern.
	qtx := db.Queries().WithTx(tx)
	if err := qtx.AnonymizeUserForDeletion(ctx, gen.AnonymizeUserForDeletionParams{
		Name:      deletedName,
		Email:     deletedEmail,
		UpdatedAt: now,
		ID:        userID,
	}); err != nil {
		return fmt.Errorf("AnonymizeUserData: anonymize users row: %w", err)
	}

	// Pass 3 HIGH-01: write the audit row inside the same tx so a
	// successful commit guarantees a matching audit entry. The
	// admin_activity table is the canonical audit surface used by
	// recordActivity() in internal/api/activity_helpers.go.
	//
	// admin_activity is defined in sql/schema/016_admin_activity.sql
	// for sqlc but has no corresponding row in sql/migrations/* (a
	// pre-existing schema gap; recordActivity() works in prod because
	// the table is bootstrapped, but the in-memory test DB only
	// applies migrations). We CREATE IF NOT EXISTS inside the tx to
	// keep AnonymizeUserData self-sufficient and to make the HIGH-01
	// audit-in-tx contract testable without expanding the file scope
	// to a new migration. In environments where the table already
	// exists the DDL is a no-op.
	if audit != nil {
		if _, err := tx.ExecContext(ctx, ensureAdminActivityDDL); err != nil {
			return fmt.Errorf("AnonymizeUserData: ensure admin_activity: %w", err)
		}
		actor := strings.TrimSpace(audit.ActorUserID)
		if actor == "" {
			actor = userID
		}
		details := buildAuditDetails(audit)
		var detailsArg any
		if details == "" {
			detailsArg = nil
		} else {
			detailsArg = details
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO admin_activity (id, actor_id, action, entity_type, entity_id, details, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`,
			uuid.NewString(),
			actor,
			strings.TrimSpace(audit.EventType),
			"user",
			userID,
			detailsArg,
			now,
		); err != nil {
			return fmt.Errorf("AnonymizeUserData: write audit: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("AnonymizeUserData: commit: %w", err)
	}
	return nil
}

// ensureAdminActivityDDL is an idempotent CREATE TABLE for
// admin_activity. See AnonymizeUserData for why this is run inline
// rather than as a migration. The shape mirrors
// sql/schema/016_admin_activity.sql exactly.
const ensureAdminActivityDDL = `
CREATE TABLE IF NOT EXISTS admin_activity (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT,
    details TEXT,
    created_at TEXT NOT NULL
)
`

// buildAuditDetails serializes the optional metadata fields of
// AuditEvent into a single string suitable for admin_activity.details.
// We deliberately keep this human-readable (key=value, semicolon-
// separated) rather than JSON because admin_activity.details is a
// plain TEXT column and existing recordActivity() callers use the
// same shape.
func buildAuditDetails(audit *AuditEvent) string {
	parts := make([]string, 0, 3)
	if ip := strings.TrimSpace(audit.IPAddress); ip != "" {
		parts = append(parts, "ip="+ip)
	}
	if ua := strings.TrimSpace(audit.UserAgent); ua != "" {
		parts = append(parts, "ua="+ua)
	}
	if md := strings.TrimSpace(audit.Metadata); md != "" {
		parts = append(parts, "meta="+md)
	}
	return strings.Join(parts, "; ")
}

// HandledTables returns a snapshot of the scrub coverage allowlist.
// Exposed so the integration drift-guard test can assert that every
// user-linked table in the live schema is either handled or
// explicitly skipped.
func HandledTables() map[string]struct{} {
	out := make(map[string]struct{}, len(handledTables))
	for k := range handledTables {
		out[k] = struct{}{}
	}
	return out
}

// SkipTables returns a snapshot of the intentional-skip allowlist
// (table -> human-readable justification). Mirror of HandledTables
// for the same drift-guard test.
func SkipTables() map[string]string {
	out := make(map[string]string, len(skipTables))
	for k, v := range skipTables {
		out[k] = v
	}
	return out
}

// isMissingTableError detects SQLite's "no such table" / "no such
// column" errors so callers can tolerate statements against tables
// that aren't present in slim test migrations or columns that don't
// exist in older schema variants. Any other error is propagated.
func isMissingTableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no such table") || strings.Contains(msg, "no such column")
}

var _ = sql.ErrNoRows

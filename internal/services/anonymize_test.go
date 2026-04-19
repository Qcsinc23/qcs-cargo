//go:build integration

package services_test

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
)

// TestAnonymizeUserData_DriftGuard scans the live SQLite schema and
// asserts that every non-system table with a user-linked column
// (user_id / customer_id / actor_user_id / matched_user_id /
// changed_by / created_by / actor_id / a FK REFERENCES users(id)) is
// either covered by services.handledTables OR explicitly listed in
// services.skipTables with a written justification.
//
// This is the Pass-3 CRIT-01 drift guard: when a future migration
// adds a new user_id column, this test fails loudly with a
// descriptive list, forcing the migrator to either extend
// AnonymizeUserData or document why the table is exempt. Without
// this guard a schema addition could silently re-create the original
// CRIT-01 bug ("we say we anonymized, but we left a table behind").
func TestAnonymizeUserData_DriftGuard(t *testing.T) {
	conn := testdata.NewTestDB(t)
	db.SetConnForTest(conn)

	tables := listUserScopedTables(t, conn)
	if len(tables) == 0 {
		t.Fatal("drift guard found ZERO user-scoped tables; this means the scanner is broken (the schema definitely has user_id columns)")
	}

	handled := services.HandledTables()
	skip := services.SkipTables()

	var missing []string
	for _, tbl := range tables {
		if _, ok := handled[tbl]; ok {
			continue
		}
		if _, ok := skip[tbl]; ok {
			continue
		}
		missing = append(missing, tbl)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf(`AnonymizeUserData drift detected: %d user-scoped table(s) are neither in services.handledTables nor in services.skipTables.

Missing tables:
  - %s

To fix: either
  1. Add a scrub/delete step for the table in internal/services/anonymize.go and add the table to handledTables, OR
  2. Add the table to skipTables in internal/services/anonymize.go with a one-line justification (e.g. "audit table; FK preserved by anonymize-in-place").

Letting this drift go unfixed reintroduces the CRIT-01 bug: GDPR delete claims "personal data anonymized" while leaving rows behind.`,
			len(missing),
			strings.Join(missing, "\n  - "),
		)
	}
}

// TestAnonymizeUserData_AuditRowInTx asserts the HIGH-01 contract:
// a successful AnonymizeUserData call writes the audit row in the
// SAME transaction as the data scrub, so the admin_activity entry
// is guaranteed to exist iff the scrub committed.
func TestAnonymizeUserData_AuditRowInTx(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	audit := &services.AuditEvent{
		ActorUserID: testdata.CustomerAliceID,
		EventType:   "test.anonymize.audit_in_tx",
		IPAddress:   "127.0.0.1",
		UserAgent:   "drift-guard-test/1.0",
		Metadata:    "via=TestAnonymizeUserData_AuditRowInTx",
	}
	require.NoError(t, services.AnonymizeUserData(
		ctx,
		testdata.CustomerAliceID,
		"Deleted User",
		"deleted+"+testdata.CustomerAliceID+"@qcs.invalid",
		audit,
	))

	// The audit row must exist in admin_activity, with details
	// preserving the metadata + ip + ua we passed.
	var (
		actorID, action, entityType, entityID string
		details                               sql.NullString
	)
	err := conn.QueryRowContext(ctx, `
SELECT actor_id, action, entity_type, entity_id, details
FROM admin_activity
WHERE actor_id = ? AND action = ?
ORDER BY created_at DESC
LIMIT 1
`, testdata.CustomerAliceID, "test.anonymize.audit_in_tx").Scan(
		&actorID, &action, &entityType, &entityID, &details,
	)
	require.NoError(t, err, "audit row must be present in admin_activity inside the same tx as the scrub")
	require.Equal(t, testdata.CustomerAliceID, actorID)
	require.Equal(t, "test.anonymize.audit_in_tx", action)
	require.Equal(t, "user", entityType)
	require.Equal(t, testdata.CustomerAliceID, entityID)
	require.True(t, details.Valid, "audit details must be populated")
	require.Contains(t, details.String, "ip=127.0.0.1")
	require.Contains(t, details.String, "ua=drift-guard-test/1.0")
	require.Contains(t, details.String, "meta=via=TestAnonymizeUserData_AuditRowInTx")
}

// listUserScopedTables introspects the live SQLite schema via
// sqlite_master + PRAGMA table_info + PRAGMA foreign_key_list and
// returns every non-system table that has at least one of:
//   - a column named user_id, customer_id, actor_user_id,
//     matched_user_id, changed_by, created_by, actor_id
//   - a foreign key REFERENCES users(id)
//
// The drift-guard test then compares this list to handledTables +
// skipTables. The set of "user-linked column names" mirrors the
// Pass-3 prompt and the actual conventions used in sql/schema and
// sql/migrations.
func listUserScopedTables(t *testing.T, conn *sql.DB) []string {
	t.Helper()

	tableNames := allUserTables(t, conn)
	userLinkedCols := map[string]struct{}{
		"user_id":         {},
		"customer_id":     {},
		"actor_user_id":   {},
		"matched_user_id": {},
		"changed_by":      {},
		"created_by":      {},
		"actor_id":        {},
	}

	out := make([]string, 0, len(tableNames))
	for _, tbl := range tableNames {
		if hasUserLinkedColumn(t, conn, tbl, userLinkedCols) || hasFKToUsers(t, conn, tbl) {
			out = append(out, tbl)
		}
	}
	sort.Strings(out)
	return out
}

func allUserTables(t *testing.T, conn *sql.DB) []string {
	t.Helper()
	rows, err := conn.Query(`
SELECT name FROM sqlite_master
WHERE type = 'table'
  AND name NOT LIKE 'sqlite_%'
  AND name NOT LIKE '%__new%'
  AND name NOT LIKE '%__old%'
ORDER BY name
`)
	require.NoError(t, err)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	require.NoError(t, rows.Err())
	return names
}

func hasUserLinkedColumn(t *testing.T, conn *sql.DB, table string, want map[string]struct{}) bool {
	t.Helper()
	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk))
		if _, ok := want[name]; ok {
			return true
		}
	}
	require.NoError(t, rows.Err())
	return false
}

func hasFKToUsers(t *testing.T, conn *sql.DB, table string) bool {
	t.Helper()
	rows, err := conn.Query(fmt.Sprintf("PRAGMA foreign_key_list(%q)", table))
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var (
			id       int
			seq      int
			refTable string
			from     string
			to       string
			onUpdate string
			onDelete string
			match    string
		)
		require.NoError(t, rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match))
		if refTable == "users" {
			return true
		}
	}
	require.NoError(t, rows.Err())
	return false
}

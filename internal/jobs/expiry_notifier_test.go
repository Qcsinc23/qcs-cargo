//go:build integration

package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
)

// TestExpiryNotifier_RangePredicateFiresAndDedups is the DEF-003
// regression test. The previous implementation matched on a single
// calendar day, so a missed daily run silently dropped the warning
// permanently. With the range predicate, packages whose expiry falls
// anywhere inside the window get the warning, gated only by the
// sent_notifications dedup table.
//
// We assert two properties:
//
//  1. A package whose expiry sits inside the (today, today+5] window
//     produces a sent_notifications row on first run.
//  2. A second run on unchanged inputs is a no-op (dedup).
//
// RESEND_API_KEY is intentionally unset, so SendStorageWarning5Days
// short-circuits to a successful no-op inside services.resendClient().
// That keeps the test self-contained while still exercising the full
// query + retry + dedup path.
func TestExpiryNotifier_RangePredicateFiresAndDedups(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)
	_ = os.Unsetenv("RESEND_API_KEY")

	in3Days := time.Now().UTC().AddDate(0, 0, 3).Format(time.RFC3339)
	if _, err := conn.Exec(
		`UPDATE locker_packages SET free_storage_expires_at = ? WHERE id = ?`,
		in3Days, testdata.PkgAliceStored1,
	); err != nil {
		t.Fatalf("update package expiry: %v", err)
	}

	if err := RunExpiryNotifierJob(context.Background()); err != nil {
		t.Fatalf("RunExpiryNotifierJob: %v", err)
	}

	var count int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM sent_notifications WHERE notification_type = ? AND resource_id = ?`,
		"storage_warning_5d", testdata.PkgAliceStored1,
	).Scan(&count); err != nil {
		t.Fatalf("count sent_notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 sent_notifications row after first run, got %d", count)
	}

	// Second run must be a no-op via dedup.
	if err := RunExpiryNotifierJob(context.Background()); err != nil {
		t.Fatalf("second RunExpiryNotifierJob: %v", err)
	}
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM sent_notifications WHERE notification_type = ? AND resource_id = ?`,
		"storage_warning_5d", testdata.PkgAliceStored1,
	).Scan(&count); err != nil {
		t.Fatalf("re-count sent_notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("dedup must keep count at 1 on second run, got %d", count)
	}
}

// TestExpiryNotifier_OutsideWindowDoesNotFire complements the previous
// test: if a package's expiry is outside the (today, today+5] window
// the row must not be written. This guards against a regression where
// the range predicate accidentally widens to include all stored
// packages.
func TestExpiryNotifier_OutsideWindowDoesNotFire(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)
	_ = os.Unsetenv("RESEND_API_KEY")

	in10Days := time.Now().UTC().AddDate(0, 0, 10).Format(time.RFC3339)
	if _, err := conn.Exec(
		`UPDATE locker_packages SET free_storage_expires_at = ? WHERE id = ?`,
		in10Days, testdata.PkgAliceStored1,
	); err != nil {
		t.Fatalf("update package expiry: %v", err)
	}

	if err := RunExpiryNotifierJob(context.Background()); err != nil {
		t.Fatalf("RunExpiryNotifierJob: %v", err)
	}

	var count int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM sent_notifications WHERE notification_type = ? AND resource_id = ?`,
		"storage_warning_5d", testdata.PkgAliceStored1,
	).Scan(&count); err != nil {
		t.Fatalf("count sent_notifications: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 sent_notifications rows for out-of-window package, got %d", count)
	}
}

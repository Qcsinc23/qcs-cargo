//go:build integration

package jobs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
)

// TestOutboundEmailQueue_EnqueueAndDrain verifies the Phase 3.2
// happy-path: a row enqueued via services.EnqueueEmail is picked up by
// RunOutboundEmailJob, dispatched through the registered template
// renderer, and marked status='sent'. RESEND_API_KEY is unset so the
// renderer returns nil from inside services.resendClient(), proving the
// full claim/dispatch/ack cycle without a real provider call.
func TestOutboundEmailQueue_EnqueueAndDrain(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)
	t.Setenv("RESEND_API_KEY", "")

	ctx := context.Background()
	if err := services.EnqueueEmail(ctx, services.TemplateStorageWarning5d, "alice@test.com", map[string]any{
		"sender_name": "Amazon",
	}); err != nil {
		t.Fatalf("EnqueueEmail: %v", err)
	}

	// Verify pending row exists.
	var pending int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM outbound_emails WHERE status = 'pending'`).Scan(&pending); err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("expected 1 pending row, got %d", pending)
	}

	if err := RunOutboundEmailJob(ctx); err != nil {
		t.Fatalf("RunOutboundEmailJob: %v", err)
	}

	var sent, pendingAfter int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM outbound_emails WHERE status = 'sent'`).Scan(&sent); err != nil {
		t.Fatalf("count sent: %v", err)
	}
	if err := conn.QueryRow(`SELECT COUNT(*) FROM outbound_emails WHERE status = 'pending'`).Scan(&pendingAfter); err != nil {
		t.Fatalf("count pending after: %v", err)
	}
	if sent != 1 || pendingAfter != 0 {
		t.Fatalf("expected 1 sent / 0 pending after drain, got sent=%d pending=%d", sent, pendingAfter)
	}
}

// TestEnqueueEmail_RejectsUnknownTemplate guards the registry: an
// EnqueueEmail call for a template that has no registered renderer must
// fail at enqueue time rather than land in the queue and then be marked
// 'failed' by the worker.
func TestEnqueueEmail_RejectsUnknownTemplate(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	err := services.EnqueueEmail(context.Background(), services.EmailTemplate("does_not_exist"), "alice@test.com", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

// TestEnqueueEmail_RequiresRecipient mirrors the storage_fee guard.
func TestEnqueueEmail_RequiresRecipient(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	err := services.EnqueueEmail(context.Background(), services.TemplateStorageWarning5d, "  ", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for empty recipient")
	}
}

// TestOutboundEmail_ReapStuckRows is the HIGH-10 regression test. A row
// stuck in 'in_progress' beyond the 5-minute cutoff must be transitioned
// back to 'pending' on the next RunOutboundEmailJob invocation so it can
// be retried.
func TestOutboundEmail_ReapStuckRows(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)
	t.Setenv("RESEND_API_KEY", "")

	ctx := context.Background()
	stuckTime := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	payload := `{"sender_name":"Acme"}`
	_, err := conn.Exec(
		`INSERT INTO outbound_emails (id, template, recipient, payload_json, status, attempt_count, scheduled_at, created_at)
		 VALUES (?, ?, ?, ?, 'in_progress', 0, ?, ?)`,
		"oe_stuck_001", "storage_warning_5d", "alice@test.com", payload, stuckTime, stuckTime,
	)
	if err != nil {
		t.Fatalf("seed stuck row: %v", err)
	}

	if err := RunOutboundEmailJob(ctx); err != nil {
		t.Fatalf("RunOutboundEmailJob: %v", err)
	}

	var status string
	var attemptCount int
	if err := conn.QueryRow(
		`SELECT status, attempt_count FROM outbound_emails WHERE id = ?`,
		"oe_stuck_001",
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("read stuck row: %v", err)
	}
	if status == "in_progress" {
		t.Fatalf("expected stuck row to be reaped (status pending or sent), got status=%q", status)
	}
	if attemptCount < 1 {
		t.Fatalf("expected attempt_count to be incremented by reap (>=1), got %d", attemptCount)
	}
}

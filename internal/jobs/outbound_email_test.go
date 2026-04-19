//go:build integration

package jobs

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
)

// TestOutboundEmailQueue_EnqueueAndDrain verifies the Phase 3.2
// happy-path: a row enqueued via services.EnqueueEmail is picked up by
// RunOutboundEmailJob, dispatched through the registered template
// renderer, and marked status='sent'.
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

func TestEnqueueEmail_RejectsUnknownTemplate(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	err := services.EnqueueEmail(context.Background(), services.EmailTemplate("does_not_exist"), "alice@test.com", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestEnqueueEmail_RequiresRecipient(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	err := services.EnqueueEmail(context.Background(), services.TemplateStorageWarning5d, "  ", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for empty recipient")
	}
}

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
		t.Fatalf("expected stuck row to be reaped, got status=%q", status)
	}
	if attemptCount < 1 {
		t.Fatalf("expected attempt_count to be incremented (>=1), got %d", attemptCount)
	}
}

// --- Pass 3 CRIT-03 + CRIT-04 + CRIT-02 regression tests ------------------

const captureTemplate services.EmailTemplate = "test_capture_pass3"

type captureRecorder struct {
	mu    sync.Mutex
	calls []captureCall
	err   error
}

type captureCall struct {
	recipient      string
	idempotencyKey string
	payload        string
}

func (r *captureRecorder) record(recipient, key string, payload json.RawMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, captureCall{
		recipient:      recipient,
		idempotencyKey: key,
		payload:        string(payload),
	})
	return r.err
}

func (r *captureRecorder) snapshot() []captureCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]captureCall, len(r.calls))
	copy(out, r.calls)
	return out
}

func installCapture(t *testing.T) *captureRecorder {
	t.Helper()
	rec := &captureRecorder{}
	services.RegisterEmailTemplate(captureTemplate, func(ctx context.Context, recipient string, raw json.RawMessage, key string) error {
		return rec.record(recipient, key, raw)
	})
	return rec
}

// TestOutboundEmail_PassesIdempotencyKey is the CRIT-04 regression: the
// worker must hand the row's stable id to the registered sender so the
// downstream Resend call sets the Idempotency-Key header.
func TestOutboundEmail_PassesIdempotencyKey(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)
	t.Setenv("RESEND_API_KEY", "")

	rec := installCapture(t)

	ctx := context.Background()
	if err := services.EnqueueEmail(ctx, captureTemplate, "alice@test.com", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("EnqueueEmail: %v", err)
	}

	var rowID string
	if err := conn.QueryRow(`SELECT id FROM outbound_emails WHERE template = ? ORDER BY created_at DESC LIMIT 1`, string(captureTemplate)).Scan(&rowID); err != nil {
		t.Fatalf("read row id: %v", err)
	}

	if err := RunOutboundEmailJob(ctx); err != nil {
		t.Fatalf("RunOutboundEmailJob: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 send, got %d", len(calls))
	}
	if calls[0].idempotencyKey == "" {
		t.Fatal("idempotency key must not be empty (CRIT-04)")
	}
	if calls[0].idempotencyKey != rowID {
		t.Fatalf("idempotency key must equal outbound_emails.id; got %q want %q", calls[0].idempotencyKey, rowID)
	}
	if calls[0].recipient != "alice@test.com" {
		t.Fatalf("unexpected recipient %q", calls[0].recipient)
	}

	// Stability: a retry on the same row must reuse the same key.
	if _, err := conn.Exec(`UPDATE outbound_emails SET status = 'pending', scheduled_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), rowID); err != nil {
		t.Fatalf("reset row to pending: %v", err)
	}
	if err := RunOutboundEmailJob(ctx); err != nil {
		t.Fatalf("RunOutboundEmailJob (retry): %v", err)
	}
	calls = rec.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 sends after retry, got %d", len(calls))
	}
	if calls[1].idempotencyKey != rowID {
		t.Fatalf("retry must reuse same idempotency key; got %q want %q", calls[1].idempotencyKey, rowID)
	}
}

// TestOutboundEmail_LostClaimRaceSkipsDispatch is the CRIT-03 regression.
// MarkOutboundEmailInProgress now returns rows-affected; when the row is
// no longer 'pending' the UPDATE matches zero rows and the worker MUST
// skip dispatch.
func TestOutboundEmail_LostClaimRaceSkipsDispatch(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)
	t.Setenv("RESEND_API_KEY", "")

	rec := installCapture(t)

	ctx := context.Background()
	if err := services.EnqueueEmail(ctx, captureTemplate, "bob@test.com", map[string]any{}); err != nil {
		t.Fatalf("EnqueueEmail: %v", err)
	}

	var rowID string
	if err := conn.QueryRow(`SELECT id FROM outbound_emails WHERE recipient = 'bob@test.com' ORDER BY created_at DESC LIMIT 1`).Scan(&rowID); err != nil {
		t.Fatalf("read row id: %v", err)
	}

	// Simulate parallel claim winning the race: row is no longer pending.
	if _, err := conn.Exec(`UPDATE outbound_emails SET status = 'sent', sent_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), rowID); err != nil {
		t.Fatalf("simulate competing claim: %v", err)
	}

	if err := RunOutboundEmailJob(ctx); err != nil {
		t.Fatalf("RunOutboundEmailJob: %v", err)
	}
	if calls := rec.snapshot(); len(calls) != 0 {
		t.Fatalf("expected 0 sends for already-sent row, got %d", len(calls))
	}

	// Direct contract check: rows-affected must be 0 on non-pending row,
	// 1 on pending row.
	n, err := db.Queries().MarkOutboundEmailInProgress(ctx, rowID)
	if err != nil {
		t.Fatalf("MarkOutboundEmailInProgress: %v", err)
	}
	if n != 0 {
		t.Fatalf("MarkOutboundEmailInProgress on non-pending row must return 0, got %d", n)
	}

	if _, err := conn.Exec(`UPDATE outbound_emails SET status = 'pending', scheduled_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), rowID); err != nil {
		t.Fatalf("reset to pending: %v", err)
	}
	n, err = db.Queries().MarkOutboundEmailInProgress(ctx, rowID)
	if err != nil {
		t.Fatalf("MarkOutboundEmailInProgress (pending): %v", err)
	}
	if n != 1 {
		t.Fatalf("MarkOutboundEmailInProgress on pending row must return 1, got %d", n)
	}
}

// TestEnqueueEmailTx_RollbackDropsRow is the CRIT-02 regression at the
// services layer: a tx-scoped enqueue whose surrounding tx rolls back
// must NOT leave a row in outbound_emails.
func TestEnqueueEmailTx_RollbackDropsRow(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	if err := services.EnqueueEmailTx(ctx, tx, services.TemplateStorageWarning5d, "carol@test.com", map[string]any{
		"sender_name": "Acme",
	}); err != nil {
		t.Fatalf("EnqueueEmailTx: %v", err)
	}

	var inTx int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM outbound_emails WHERE recipient = 'carol@test.com'`).Scan(&inTx); err != nil {
		t.Fatalf("count inside tx: %v", err)
	}
	if inTx != 1 {
		t.Fatalf("expected row visible inside tx, got %d", inTx)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var afterRollback int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM outbound_emails WHERE recipient = 'carol@test.com'`).Scan(&afterRollback); err != nil {
		t.Fatalf("count after rollback: %v", err)
	}
	if afterRollback != 0 {
		t.Fatalf("rollback must drop the enqueued row; found %d remaining", afterRollback)
	}
}

// TestEnqueueEmailTx_CommitPersistsRow is the positive CRIT-02 case.
func TestEnqueueEmailTx_CommitPersistsRow(t *testing.T) {
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	ctx := context.Background()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	if err := services.EnqueueEmailTx(ctx, tx, services.TemplateStorageWarning5d, "dave@test.com", map[string]any{
		"sender_name": "Acme",
	}); err != nil {
		t.Fatalf("EnqueueEmailTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var afterCommit int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM outbound_emails WHERE recipient = 'dave@test.com' AND status = 'pending'`).Scan(&afterCommit); err != nil {
		t.Fatalf("count after commit: %v", err)
	}
	if afterCommit != 1 {
		t.Fatalf("commit must persist the enqueued row in 'pending'; found %d", afterCommit)
	}
}

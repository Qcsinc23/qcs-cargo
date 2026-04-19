package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
)

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// Phase 3.2 (INC-001 part B) outbound-email worker.
//
// RunOutboundEmailJob drains pending rows from the outbound_emails
// table, dispatching each through the registered template renderer.
// Each row is bound by maxOutboundAttempts; once that budget is
// exhausted the row is marked status='failed' so ops can investigate.
//
// The worker is intentionally lightweight: a single pass per
// invocation, claim up to drainBatchSize rows, run sends serially.
// Single-replica deployment makes that more than enough for the
// volumes a parcel-forwarding business sends. Frequency is controlled
// by the caller (cmd/server runs it on a 1-minute ticker alongside
// the daily jobs).
const (
	drainBatchSize       = 32
	maxOutboundAttempts  = 5
	outboundRescheduleBy = 5 * time.Minute
)

func RunOutboundEmailJob(ctx context.Context) error {
	q := db.Queries()
	rows, err := q.ClaimPendingOutboundEmails(ctx, gen.ClaimPendingOutboundEmailsParams{
		ScheduledAt: time.Now().UTC().Format(time.RFC3339),
		Limit:       drainBatchSize,
	})
	if err != nil {
		return fmt.Errorf("outbound email claim: %w", err)
	}
	for _, row := range rows {
		// Best-effort optimistic claim (avoids re-running rows another
		// invocation might have just picked up). Returns silently on no-op.
		if err := q.MarkOutboundEmailInProgress(ctx, row.ID); err != nil {
			log.Printf("[outbound email] mark in_progress %s: %v", row.ID, err)
			continue
		}
		dispatchOutboundEmail(ctx, row)
	}
	return nil
}

func dispatchOutboundEmail(ctx context.Context, row gen.ClaimPendingOutboundEmailsRow) {
	q := db.Queries()
	template := services.EmailTemplate(row.Template)
	send, ok := services.LookupEmailTemplate(template)
	if !ok {
		log.Printf("[outbound email] %s: unknown template %q, marking failed", row.ID, row.Template)
		_ = q.MarkOutboundEmailFailed(ctx, gen.MarkOutboundEmailFailedParams{
			Status:      "failed",
			LastError:   nullStr(fmt.Sprintf("unknown template %q", row.Template)),
			ScheduledAt: time.Now().UTC().Format(time.RFC3339),
			ID:          row.ID,
		})
		return
	}

	err := send(ctx, row.Recipient, json.RawMessage(row.PayloadJson))
	if err == nil {
		_ = q.MarkOutboundEmailSent(ctx, gen.MarkOutboundEmailSentParams{
			SentAt: nullStr(time.Now().UTC().Format(time.RFC3339)),
			ID:     row.ID,
		})
		return
	}

	middleware.RecordEmailSendFailure(row.Template, "provider_error")
	nextStatus := "pending"
	nextAttempt := int(row.AttemptCount) + 1
	if nextAttempt >= maxOutboundAttempts {
		nextStatus = "failed"
		log.Printf("[outbound email] %s: giving up after %d attempts: %v", row.ID, nextAttempt, err)
	} else {
		log.Printf("[outbound email] %s: attempt %d failed, will retry: %v", row.ID, nextAttempt, err)
	}
	nextScheduled := time.Now().UTC().Add(outboundRescheduleBy * time.Duration(nextAttempt)).Format(time.RFC3339)
	if updateErr := q.MarkOutboundEmailFailed(ctx, gen.MarkOutboundEmailFailedParams{
		Status:      nextStatus,
		LastError:   nullStr(err.Error()),
		ScheduledAt: nextScheduled,
		ID:          row.ID,
	}); updateErr != nil {
		log.Printf("[outbound email] %s: failed to record failure: %v", row.ID, updateErr)
	}
}

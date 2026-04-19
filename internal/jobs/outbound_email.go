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

const (
	drainBatchSize       = 32
	maxOutboundAttempts  = 5
	outboundRescheduleBy = 5 * time.Minute
)

func RunOutboundEmailJob(ctx context.Context) error {
	q := db.Queries()

	reapCutoff := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
	rescheduleAt := time.Now().UTC().Format(time.RFC3339)
	if reaped, err := q.ReapStuckOutboundEmails(ctx, gen.ReapStuckOutboundEmailsParams{
		ScheduledAt:   rescheduleAt,
		ScheduledAt_2: reapCutoff,
	}); err != nil {
		log.Printf("[outbound email] reap stuck rows: %v", err)
	} else if reaped > 0 {
		log.Printf("[outbound email] reaped %d stuck row(s)", reaped)
		middleware.RecordOutboundEmailReaped(int(reaped))
	}

	rows, err := q.ClaimPendingOutboundEmails(ctx, gen.ClaimPendingOutboundEmailsParams{
		ScheduledAt: time.Now().UTC().Format(time.RFC3339),
		Limit:       drainBatchSize,
	})
	if err != nil {
		return fmt.Errorf("outbound email claim: %w", err)
	}
	for _, row := range rows {
		// Pass 3 CRIT-03: rows-affected check. 0 = lost the optimistic-claim
		// race; another worker advanced the row out of 'pending'. Skip
		// dispatch to avoid a duplicate provider send.
		n, err := q.MarkOutboundEmailInProgress(ctx, row.ID)
		if err != nil {
			log.Printf("[outbound email] mark in_progress %s: %v", row.ID, err)
			continue
		}
		if n == 0 {
			log.Printf("[outbound email] %s: lost claim race, skipping", row.ID)
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

	// Pass 3 CRIT-04: pass row.ID as the Resend Idempotency-Key.
	err := send(ctx, row.Recipient, json.RawMessage(row.PayloadJson), row.ID)
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

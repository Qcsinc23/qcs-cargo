package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/google/uuid"
)

// RunExpiryNotifierJob notifies customers about storage expiry milestones:
//   - storage_warning_5d: package expires within the next 5 days
//   - storage_warning_1d: package expires within the next 1 day
//   - storage_final_notice: package has been in the locker >= 55 days
//
// DEF-003 fix: predicates are now ranges, not single-day equality. The
// previous implementation matched only `date(expires_at) = today + 5`
// (etc.), so a missed daily run silently dropped the warning forever.
// With ranges, a catch-up run on day N+2 still sends the day-N warning.
// The sent_notifications table provides per-(type, package, recipient)
// dedup so multiple in-window runs do not produce duplicate emails.
//
// Phase 3.2 (INC-001 part B): notifications are enqueued onto
// outbound_emails and dispatched by the worker. The job marks the
// sent_notifications row immediately on enqueue, since the queue itself
// is the durability guarantee — a transient provider failure stays in
// the queue and is retried by the worker, rather than being silently
// dropped.
func RunExpiryNotifierJob(ctx context.Context) error {
	conn := db.DB()
	queries := db.Queries()
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	in5 := now.AddDate(0, 0, 5).Format("2006-01-02")
	in1 := now.AddDate(0, 0, 1).Format("2006-01-02")
	day55Cutoff := now.AddDate(0, 0, -55).Format("2006-01-02")

	if err := notifyExpiryWindow(ctx, conn, queries, expiryWindow{
		notificationType: string(services.TemplateStorageWarning5d),
		template:         services.TemplateStorageWarning5d,
		query: `
			SELECT id, user_id, sender_name FROM locker_packages
			WHERE status = 'stored'
			  AND free_storage_expires_at IS NOT NULL
			  AND date(free_storage_expires_at) > ?
			  AND date(free_storage_expires_at) <= ?
		`,
		args: []any{today, in5},
	}); err != nil {
		return err
	}

	if err := notifyExpiryWindow(ctx, conn, queries, expiryWindow{
		notificationType: string(services.TemplateStorageWarning1d),
		template:         services.TemplateStorageWarning1d,
		query: `
			SELECT id, user_id, sender_name FROM locker_packages
			WHERE status = 'stored'
			  AND free_storage_expires_at IS NOT NULL
			  AND date(free_storage_expires_at) > ?
			  AND date(free_storage_expires_at) <= ?
		`,
		args: []any{today, in1},
	}); err != nil {
		return err
	}

	if err := notifyExpiryWindow(ctx, conn, queries, expiryWindow{
		notificationType: string(services.TemplateStorageFinalNotice),
		template:         services.TemplateStorageFinalNotice,
		query: `
			SELECT id, user_id, sender_name FROM locker_packages
			WHERE status = 'stored'
			  AND date(arrived_at) <= ?
		`,
		args: []any{day55Cutoff},
	}); err != nil {
		return err
	}

	return nil
}

type expiryWindow struct {
	notificationType string
	template         services.EmailTemplate
	query            string
	args             []any
}

func notifyExpiryWindow(ctx context.Context, conn *sql.DB, queries *gen.Queries, w expiryWindow) error {
	rows, err := conn.QueryContext(ctx, w.query, w.args...)
	if err != nil {
		return fmt.Errorf("expiry notifier %s query: %w", w.notificationType, err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID string
		var senderName sql.NullString
		if err := rows.Scan(&id, &userID, &senderName); err != nil {
			return fmt.Errorf("expiry notifier %s scan: %w", w.notificationType, err)
		}
		to := userEmailForID(ctx, userID)
		if to == "" {
			continue
		}

		count, _ := queries.CheckSentNotification(ctx, gen.CheckSentNotificationParams{
			NotificationType: w.notificationType,
			ResourceID:       id,
			RecipientEmail:   to,
		})
		if count > 0 {
			continue
		}

		sender := "your package"
		if senderName.Valid && senderName.String != "" {
			sender = senderName.String
		}

		if err := services.EnqueueEmail(ctx, w.template, to, map[string]any{
			"sender_name": sender,
		}); err != nil {
			log.Printf("[expiry notifier] %s package %s: enqueue failed: %v", w.notificationType, id, err)
			continue
		}
		_ = queries.CreateSentNotification(ctx, gen.CreateSentNotificationParams{
			ID:               uuid.New().String(),
			NotificationType: w.notificationType,
			ResourceID:       id,
			RecipientEmail:   to,
			SentAt:           time.Now().UTC().Format(time.RFC3339),
		})
	}
	return rows.Err()
}

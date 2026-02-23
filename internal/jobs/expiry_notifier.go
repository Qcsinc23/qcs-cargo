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

// RunExpiryNotifierJob finds stored packages by expiry timing and sends:
// - 5 days before free storage ends: SendStorageWarning5Days
// - 1 day before: SendStorageWarning1Day
// - at day 55 (5 days before disposal): SendStorageFinalNotice
// Uses the global db connection and existing email functions from internal/services/email.go.
func RunExpiryNotifierJob(ctx context.Context) error {
	conn := db.DB()
	queries := db.Queries()
	now := time.Now().UTC()
	in5 := now.AddDate(0, 0, 5).Format("2006-01-02")
	in1 := now.AddDate(0, 0, 1).Format("2006-01-02")
	day55Start := now.AddDate(0, 0, -55).Format("2006-01-02")
	day55End := now.AddDate(0, 0, -54).Format("2006-01-02")

	// 5-day warning
	rows5, err := conn.QueryContext(ctx, `
		SELECT id, user_id, sender_name FROM locker_packages
		WHERE status = 'stored' AND date(free_storage_expires_at) = ?
	`, in5)
	if err != nil {
		return fmt.Errorf("expiry notifier 5-day query: %w", err)
	}
	for rows5.Next() {
		var id, userID string
		var senderName sql.NullString
		if err := rows5.Scan(&id, &userID, &senderName); err != nil {
			rows5.Close()
			return fmt.Errorf("expiry notifier 5-day scan: %w", err)
		}
		to := userEmailForID(ctx, userID)
		if to == "" {
			continue
		}

		// Deduplication check
		count, _ := queries.CheckSentNotification(ctx, gen.CheckSentNotificationParams{
			NotificationType: "storage_warning_5d",
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
		if err := services.SendStorageWarning5Days(to, sender); err != nil {
			log.Printf("[expiry notifier] 5-day warning package %s: %v", id, err)
		} else {
			_ = queries.CreateSentNotification(ctx, gen.CreateSentNotificationParams{
				ID:               uuid.New().String(),
				NotificationType: "storage_warning_5d",
				ResourceID:       id,
				RecipientEmail:   to,
				SentAt:           now.Format(time.RFC3339),
			})
		}
	}
	rows5.Close()

	// 1-day warning
	rows1, err := conn.QueryContext(ctx, `
		SELECT id, user_id, sender_name FROM locker_packages
		WHERE status = 'stored' AND date(free_storage_expires_at) = ?
	`, in1)
	if err != nil {
		return fmt.Errorf("expiry notifier 1-day query: %w", err)
	}
	for rows1.Next() {
		var id, userID string
		var senderName sql.NullString
		if err := rows1.Scan(&id, &userID, &senderName); err != nil {
			rows1.Close()
			return fmt.Errorf("expiry notifier 1-day scan: %w", err)
		}
		to := userEmailForID(ctx, userID)
		if to == "" {
			continue
		}

		// Deduplication check
		count, _ := queries.CheckSentNotification(ctx, gen.CheckSentNotificationParams{
			NotificationType: "storage_warning_1d",
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
		if err := services.SendStorageWarning1Day(to, sender); err != nil {
			log.Printf("[expiry notifier] 1-day warning package %s: %v", id, err)
		} else {
			_ = queries.CreateSentNotification(ctx, gen.CreateSentNotificationParams{
				ID:               uuid.New().String(),
				NotificationType: "storage_warning_1d",
				ResourceID:       id,
				RecipientEmail:   to,
				SentAt:           now.Format(time.RFC3339),
			})
		}
	}
	rows1.Close()

	// Day 55 final notice
	rows55, err := conn.QueryContext(ctx, `
		SELECT id, user_id, sender_name FROM locker_packages
		WHERE status = 'stored'
		  AND date(arrived_at) >= ? AND date(arrived_at) < ?
	`, day55Start, day55End)
	if err != nil {
		return fmt.Errorf("expiry notifier day55 query: %w", err)
	}
	for rows55.Next() {
		var id, userID string
		var senderName sql.NullString
		if err := rows55.Scan(&id, &userID, &senderName); err != nil {
			rows55.Close()
			return fmt.Errorf("expiry notifier day55 scan: %w", err)
		}
		to := userEmailForID(ctx, userID)
		if to == "" {
			continue
		}

		// Deduplication check
		count, _ := queries.CheckSentNotification(ctx, gen.CheckSentNotificationParams{
			NotificationType: "storage_final_notice",
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
		if err := services.SendStorageFinalNotice(to, sender); err != nil {
			log.Printf("[expiry notifier] final notice package %s: %v", id, err)
		} else {
			_ = queries.CreateSentNotification(ctx, gen.CreateSentNotificationParams{
				ID:               uuid.New().String(),
				NotificationType: "storage_final_notice",
				ResourceID:       id,
				RecipientEmail:   to,
				SentAt:           now.Format(time.RFC3339),
			})
		}
	}
	rows55.Close()

	return nil
}

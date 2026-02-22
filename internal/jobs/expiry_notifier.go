package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
)

// RunExpiryNotifierJob finds stored packages by expiry timing and sends:
// - 5 days before free storage ends: SendStorageWarning5Days
// - 1 day before: SendStorageWarning1Day
// - at day 55 (5 days before disposal): SendStorageFinalNotice
// Uses the global db connection and existing email functions from internal/services/email.go.
func RunExpiryNotifierJob(ctx context.Context) error {
	conn := db.DB()
	now := time.Now().UTC()
	in5 := now.AddDate(0, 0, 5).Format("2006-01-02")
	in1 := now.AddDate(0, 0, 1).Format("2006-01-02")
	// Day 55 = 55 days after arrived_at; we compare free_storage_expires_at which is typically
	// arrived_at + free_days (e.g. 30). So "day 55" means 55 days after arrival = free_storage_expires_at + 25.
	// PRD: "find day 55 send SendStorageFinalNotice". So we want packages whose "days since arrival" is 55.
	// free_storage_expires_at = arrived_at + 30, so day 55 = arrived_at + 55 => free_storage_expires_at + 25.
	// So we need packages where arrived_at + 55 days = today, i.e. arrived_at = today - 55.
	day55Start := now.AddDate(0, 0, -55).Format("2006-01-02")
	day55End := now.AddDate(0, 0, -54).Format("2006-01-02")

	// 5-day warning: free_storage_expires_at = in5 (exactly 5 days from now)
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
		sender := "your package"
		if senderName.Valid && senderName.String != "" {
			sender = senderName.String
		}
		if err := services.SendStorageWarning5Days(to, sender); err != nil {
			log.Printf("[expiry notifier] 5-day warning package %s: %v", id, err)
		}
	}
	rows5.Close()
	if err := rows5.Err(); err != nil {
		return fmt.Errorf("expiry notifier 5-day rows: %w", err)
	}

	// 1-day warning: free_storage_expires_at = in1
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
		sender := "your package"
		if senderName.Valid && senderName.String != "" {
			sender = senderName.String
		}
		if err := services.SendStorageWarning1Day(to, sender); err != nil {
			log.Printf("[expiry notifier] 1-day warning package %s: %v", id, err)
		}
	}
	rows1.Close()
	if err := rows1.Err(); err != nil {
		return fmt.Errorf("expiry notifier 1-day rows: %w", err)
	}

	// Day 55 final notice: arrived_at 55 days ago (so "day 55" of storage)
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
		sender := "your package"
		if senderName.Valid && senderName.String != "" {
			sender = senderName.String
		}
		if err := services.SendStorageFinalNotice(to, sender); err != nil {
			log.Printf("[expiry notifier] final notice package %s: %v", id, err)
		}
	}
	rows55.Close()
	if err := rows55.Err(); err != nil {
		return fmt.Errorf("expiry notifier day55 rows: %w", err)
	}

	return nil
}

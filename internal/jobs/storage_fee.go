package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
)

// DefaultDailyStorageFeeAmount is the per-day storage fee in USD after free period (PRD).
const DefaultDailyStorageFeeAmount = 1.50

// RunStorageFeeJob finds locker_packages where free_storage_expires_at < today and status = 'stored',
// inserts a row into storage_fees for each (if not already charged for today), and sends
// SendStorageFeeCharged when Resend is configured. Uses the global db connection.
func RunStorageFeeJob(ctx context.Context) error {
	conn := db.DB()
	today := time.Now().UTC().Format("2006-01-02")

	rows, err := conn.QueryContext(ctx, `
		SELECT id, user_id, sender_name, weight_lbs
		FROM locker_packages
		WHERE status = 'stored'
		  AND date(free_storage_expires_at) < date(?)
		ORDER BY id
	`, today)
	if err != nil {
		return fmt.Errorf("storage fee job query: %w", err)
	}
	defer rows.Close()

	var id, userID string
	var senderName sql.NullString
	var weightLbs sql.NullFloat64

	for rows.Next() {
		if err := rows.Scan(&id, &userID, &senderName, &weightLbs); err != nil {
			return fmt.Errorf("storage fee job scan: %w", err)
		}

		// Skip if we already charged this package for today (idempotent)
		var existing int
		err := conn.QueryRowContext(ctx, `
			SELECT 1 FROM storage_fees WHERE locker_package_id = ? AND fee_date = ?
		`, id, today).Scan(&existing)
		if err == nil {
			continue // already charged today
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("storage fee check existing: %w", err)
		}

		amount := DefaultDailyStorageFeeAmount
		feeID := "sf_" + uuid.New().String()
		createdAt := time.Now().UTC().Format(time.RFC3339)

		_, err = conn.ExecContext(ctx, `
			INSERT INTO storage_fees (id, user_id, locker_package_id, fee_date, amount, invoiced, created_at)
			VALUES (?, ?, ?, ?, ?, 0, ?)
		`, feeID, userID, id, today, amount, createdAt)
		if err != nil {
			return fmt.Errorf("storage fee insert %s: %w", id, err)
		}

		// Notify customer if Resend is configured
		if os.Getenv("RESEND_API_KEY") != "" {
			sender := "your package"
			if senderName.Valid && senderName.String != "" {
				sender = senderName.String
			}
			if err := services.SendStorageFeeCharged(userEmailForID(ctx, userID), sender, amount); err != nil {
				log.Printf("[storage fee job] send email for package %s: %v", id, err)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("storage fee job rows: %w", err)
	}
	return nil
}

func userEmailForID(ctx context.Context, userID string) string {
	u, err := db.Queries().GetUserByID(ctx, userID)
	if err != nil {
		return ""
	}
	return u.Email
}

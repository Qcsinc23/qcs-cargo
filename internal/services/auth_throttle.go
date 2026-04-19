package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
)

// ErrAuthRequestThrottled indicates that a magic-link / password-reset /
// resend-verification request has been refused because too many such requests
// have been issued recently for the same account or IP. Pass 2 audit fix M-4.
var ErrAuthRequestThrottled = errors.New("too many requests; try again later")

// CheckAndRecordAuthRequest enforces a per-bucket sliding window. Returns
// ErrAuthRequestThrottled if the bucket has exceeded `maxAttempts` within
// `window`; otherwise records the attempt and returns nil.
//
// Buckets are arbitrary strings used by callers to distinguish the type of
// request and the principal it targets, e.g.:
//
//	CheckAndRecordAuthRequest(ctx, "magic_link:" + email,         3, 5*time.Minute)
//	CheckAndRecordAuthRequest(ctx, "magic_link_ip:" + clientIP,   10, 5*time.Minute)
//	CheckAndRecordAuthRequest(ctx, "forgot_password:" + email,    3, 15*time.Minute)
//	CheckAndRecordAuthRequest(ctx, "resend_verification:" + email,3, 10*time.Minute)
//
// The implementation also opportunistically prunes expired rows on each call,
// keeping the table compact without a dedicated cleanup job.
//
// Pass 2.5 CRIT-05 + HIGH-06 fix:
//   - The check (SELECT COUNT) and the record (INSERT) are wrapped in a
//     single transaction so concurrent requests cannot all observe count<N
//     before any of them inserts. SQLite's WAL mode plus a single write
//     transaction provides the required serialisability for the per-bucket
//     ceiling to hold under burst concurrency (HIGH-06).
//   - On any DB error (BeginTx, prune Exec, COUNT QueryRow, INSERT Exec, or
//     Commit), this function now returns ErrAuthRequestThrottled rather
//     than nil. The previous fail-open behaviour silently disabled the
//     canonical SEC-002 per-account throttle whenever the DB hiccupped,
//     which is exactly the moment the throttle most needs to hold
//     (CRIT-05). Fail-closed forces callers to behave conservatively
//     (return 429) instead of waving every request through.
func CheckAndRecordAuthRequest(ctx context.Context, bucket string, maxAttempts int, window time.Duration) error {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" || maxAttempts <= 0 || window <= 0 {
		return nil
	}
	since := time.Now().UTC().Add(-window).Format(time.RFC3339)
	pruneCutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

	conn := db.DB()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return ErrAuthRequestThrottled
	}
	defer tx.Rollback() //nolint:errcheck

	// Best-effort prune of fully-expired rows older than 24h to keep the
	// table from growing without bound. Any older window-relative row would
	// already be outside every reasonable rate window.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM auth_request_log WHERE created_at < ?`, pruneCutoff,
	); err != nil {
		return ErrAuthRequestThrottled
	}

	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM auth_request_log WHERE bucket = ? AND created_at >= ?`,
		bucket, since,
	).Scan(&count); err != nil {
		return ErrAuthRequestThrottled
	}
	if count >= maxAttempts {
		return ErrAuthRequestThrottled
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO auth_request_log (id, bucket, created_at) VALUES (?, ?, ?)`,
		uuid.New().String(), bucket, now,
	); err != nil {
		return ErrAuthRequestThrottled
	}
	if err := tx.Commit(); err != nil {
		return ErrAuthRequestThrottled
	}
	return nil
}

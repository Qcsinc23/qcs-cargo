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
func CheckAndRecordAuthRequest(ctx context.Context, bucket string, maxAttempts int, window time.Duration) error {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" || maxAttempts <= 0 || window <= 0 {
		return nil
	}
	since := time.Now().UTC().Add(-window).Format(time.RFC3339)

	conn := db.DB()

	// Best-effort prune of fully-expired rows older than 24h to keep the
	// table from growing without bound. Any older window-relative row would
	// already be outside every reasonable rate window.
	pruneCutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	_, _ = conn.ExecContext(ctx,
		`DELETE FROM auth_request_log WHERE created_at < ?`, pruneCutoff,
	)

	var count int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM auth_request_log WHERE bucket = ? AND created_at >= ?`,
		bucket, since,
	).Scan(&count); err != nil {
		// Throttle table not yet migrated, or transient error: fail open
		// rather than break the auth flow entirely.
		return nil
	}
	if count >= maxAttempts {
		return ErrAuthRequestThrottled
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO auth_request_log (id, bucket, created_at) VALUES (?, ?, ?)`,
		uuid.New().String(), bucket, now,
	); err != nil {
		// As above, fail open on storage error.
		return nil
	}
	return nil
}

package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
)

// TestCheckAndRecordAuthRequest_AtomicUnderConcurrency is the HIGH-06
// regression test. The previous SELECT+INSERT pair allowed a burst to
// pass the throttle by all observing count=0 before any inserted. After
// the fix, exactly maxAttempts requests succeed.
func TestCheckAndRecordAuthRequest_AtomicUnderConcurrency(t *testing.T) {
	conn := testdata.NewTestDB(t)
	db.SetConnForTest(conn)

	const N = 10
	const maxAttempts = 3
	bucket := "test:concurrency"

	var wg sync.WaitGroup
	var allowed, throttled int
	var mu sync.Mutex

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := CheckAndRecordAuthRequest(context.Background(), bucket, maxAttempts, 5*time.Minute)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				allowed++
			} else if errors.Is(err, ErrAuthRequestThrottled) {
				throttled++
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if allowed != maxAttempts {
		t.Errorf("expected exactly %d allowed under atomic check, got %d (throttled=%d)", maxAttempts, allowed, throttled)
	}
}

// TestCheckAndRecordAuthRequest_FailsClosedOnDBError is the CRIT-05
// regression test. A closed DB connection must produce
// ErrAuthRequestThrottled, not silently let the request through.
func TestCheckAndRecordAuthRequest_FailsClosedOnDBError(t *testing.T) {
	conn := testdata.NewTestDB(t)
	db.SetConnForTest(conn)

	if err := conn.Close(); err != nil {
		t.Fatalf("close conn: %v", err)
	}

	err := CheckAndRecordAuthRequest(context.Background(), "test:closed", 3, 5*time.Minute)
	if !errors.Is(err, ErrAuthRequestThrottled) {
		t.Fatalf("expected ErrAuthRequestThrottled on closed DB, got %v", err)
	}
}

// TestCheckAndRecordAuthRequest_BasicAllowAndThrottle ensures the basic
// happy/sad path still works after the rewrite.
func TestCheckAndRecordAuthRequest_BasicAllowAndThrottle(t *testing.T) {
	conn := testdata.NewTestDB(t)
	db.SetConnForTest(conn)

	bucket := fmt.Sprintf("test:basic:%d", time.Now().UnixNano())
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := CheckAndRecordAuthRequest(ctx, bucket, 3, 5*time.Minute); err != nil {
			t.Fatalf("call %d should succeed, got %v", i+1, err)
		}
	}
	err := CheckAndRecordAuthRequest(ctx, bucket, 3, 5*time.Minute)
	if !errors.Is(err, ErrAuthRequestThrottled) {
		t.Fatalf("4th call should be throttled, got %v", err)
	}
}

// TestCheckAndRecordAuthRequest_VolumeCappedPerBucket is the MED-11
// regression test. The 24h age-based prune cannot defend against a
// caller that sustains thousands of attempts inside the retention
// window; the volume cap (authThrottleBucketVolumeCap) forces the
// transaction to keep only the most recent 1000 rows per bucket.
func TestCheckAndRecordAuthRequest_VolumeCappedPerBucket(t *testing.T) {
	conn := testdata.NewTestDB(t)
	db.SetConnForTest(conn)

	bucket := "test:volume_cap"
	ctx := context.Background()

	// Seed 1500 rows directly. Use timestamps inside the rate window so
	// the windowed COUNT comes back > 0 and the volume-prune branch
	// fires (the fix intentionally short-circuits when count == 0).
	now := time.Now().UTC()
	for i := 0; i < 1500; i++ {
		ts := now.Add(-time.Duration(i) * time.Second).Format(time.RFC3339)
		_, err := conn.ExecContext(ctx,
			`INSERT INTO auth_request_log (id, bucket, created_at) VALUES (?, ?, ?)`,
			"row_"+strconv.Itoa(i), bucket, ts,
		)
		if err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	// Use a maxAttempts above the volume cap so the throttle check
	// itself does not refuse the call; we want the volume-prune step
	// to run inside the same transaction as the new INSERT.
	_ = CheckAndRecordAuthRequest(ctx, bucket, 10_000, 1*time.Hour)

	var count int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM auth_request_log WHERE bucket = ?`,
		bucket,
	).Scan(&count); err != nil {
		t.Fatalf("count after prune: %v", err)
	}
	// 1000 retained + the new attempt that just succeeded = 1001 max.
	if count > authThrottleBucketVolumeCap+1 {
		t.Fatalf("expected per-bucket volume cap to keep <= %d rows, got %d",
			authThrottleBucketVolumeCap+1, count)
	}
	if count < authThrottleBucketVolumeCap {
		t.Fatalf("volume prune over-deleted: kept only %d rows", count)
	}
}

// TestCheckAndRecordAuthRequest_VolumeCapSkippedOnEmptyWindow ensures
// the per-bucket COUNT (and the expensive subquery DELETE) is *not*
// run when the windowed count is zero. We can't easily assert "no
// query ran" but we can assert that an empty bucket with no
// in-window rows doesn't get its historical rows dropped just because
// they are out of window - they are pruned only by the 24h prune.
func TestCheckAndRecordAuthRequest_VolumeCapSkippedOnEmptyWindow(t *testing.T) {
	conn := testdata.NewTestDB(t)
	db.SetConnForTest(conn)

	bucket := "test:volume_cap_skipped"
	ctx := context.Background()

	// Insert 5 rows just inside the 24h prune cutoff but outside the
	// 1-minute rate window so the windowed COUNT returns 0.
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ts := now.Add(-30 * time.Minute).Add(-time.Duration(i) * time.Second).Format(time.RFC3339)
		_, err := conn.ExecContext(ctx,
			`INSERT INTO auth_request_log (id, bucket, created_at) VALUES (?, ?, ?)`,
			"old_row_"+strconv.Itoa(i), bucket, ts,
		)
		if err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}

	if err := CheckAndRecordAuthRequest(ctx, bucket, 3, 1*time.Minute); err != nil {
		t.Fatalf("call should succeed, got %v", err)
	}

	var count int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM auth_request_log WHERE bucket = ?`,
		bucket,
	).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	// 5 historical + 1 new = 6 (volume-prune did not engage because
	// the window was empty and the bucket total is well under the cap).
	if count != 6 {
		t.Fatalf("expected 6 rows (5 historical + 1 new), got %d", count)
	}
}

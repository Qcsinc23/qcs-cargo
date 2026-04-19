package services

import (
	"context"
	"errors"
	"fmt"
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

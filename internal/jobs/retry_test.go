package jobs

import (
	"errors"
	"testing"
	"time"
)

// TestRetryEmailSend_SucceedsAfterTransientFailure verifies the retry
// loop returns nil when a transient failure recovers within the attempt
// budget. Backoff is collapsed via tiny intervals to keep the test fast.
func TestRetryEmailSend_SucceedsAfterTransientFailure(t *testing.T) {
	calls := 0
	err := retryEmailSendWithBackoff("test_template", func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	}, 5, 1*time.Millisecond, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 send attempts, got %d", calls)
	}
}

// TestRetryEmailSend_GivesUpAfterMaxAttempts asserts the helper does not
// loop forever and returns the last error after the attempt budget runs
// out. This is the DEF-003/INC-001 guarantee that a permanent provider
// outage is reported, not silently swallowed.
func TestRetryEmailSend_GivesUpAfterMaxAttempts(t *testing.T) {
	wantErr := errors.New("permanent")
	calls := 0
	err := retryEmailSendWithBackoff("test_template", func() error {
		calls++
		return wantErr
	}, 3, 1*time.Millisecond, 5*time.Millisecond)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected permanent err to bubble up, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 send attempts, got %d", calls)
	}
}

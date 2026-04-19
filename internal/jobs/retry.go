package jobs

import (
	"errors"
	"log"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// retryEmailSend wraps a transactional-email call with bounded
// exponential backoff so a transient Resend hiccup does not silently
// drop a customer notification (DEF-003 + INC-001 fix). After every
// failed attempt the jobs.email.send_failures_total counter is
// incremented with the template name and reason, so missed sends are
// observable instead of merely logged.
//
// Until the durable outbound-email queue lands in Phase 3.2, this
// in-job retry is the recovery layer for jobs/email integrations.
//
// Defaults: 3 attempts, 1 s -> 5 s -> 25 s, capped at 30 s. Override via
// retryEmailSendWithBackoff for tests.
func retryEmailSend(template string, send func() error) error {
	return retryEmailSendWithBackoff(template, send, 3, 1*time.Second, 30*time.Second)
}

func retryEmailSendWithBackoff(template string, send func() error, maxAttempts int, initial, max time.Duration) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if initial <= 0 {
		initial = time.Second
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	var lastErr error
	delay := initial
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := send()
		if err == nil {
			if attempt > 1 {
				log.Printf("[email] %s: succeeded after %d attempts", template, attempt)
			}
			return nil
		}
		lastErr = err
		middleware.RecordEmailSendFailure(template, classifyEmailError(err))
		log.Printf("[email] %s: attempt %d/%d failed: %v", template, attempt, maxAttempts, err)
		if attempt == maxAttempts {
			break
		}
		time.Sleep(delay)
		delay *= 5
		if delay > max {
			delay = max
		}
	}
	return lastErr
}

// classifyEmailError reduces an email-provider error to a small enum so
// the failure counter has bounded label cardinality. Unknown errors are
// labeled "other".
func classifyEmailError(err error) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, errEmptyRecipient) {
		return "empty_recipient"
	}
	return "other"
}

// errEmptyRecipient is returned from helpers when no recipient address is
// available for a notification (e.g. account anonymized via GDPR delete).
var errEmptyRecipient = errors.New("empty recipient")

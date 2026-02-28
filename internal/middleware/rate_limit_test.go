package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthAttemptLockoutLifecycle(t *testing.T) {
	key := "test-lockout-key"
	ClearAuthAttemptFailures(key)
	locked, _ := CheckAuthAttemptLockout(key)
	assert.False(t, locked)

	for i := 0; i < 5; i++ {
		RecordAuthAttemptFailure(key)
	}
	locked, until := CheckAuthAttemptLockout(key)
	assert.True(t, locked)
	assert.True(t, until.After(time.Now()))

	ClearAuthAttemptFailures(key)
	locked, _ = CheckAuthAttemptLockout(key)
	assert.False(t, locked)
}


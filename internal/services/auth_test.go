package services_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSuiteCode(t *testing.T) {
	// Format: QCS- + 6 alphanumeric (PRD 8.10)
	prefix := "QCS-"
	re := regexp.MustCompile(`^QCS-[A-Z0-9]{6}$`)

	for i := 0; i < 20; i++ {
		code, err := services.GenerateSuiteCode()
		require.NoError(t, err)
		assert.Len(t, code, len(prefix)+6, "suite code should be QCS- + 6 chars")
		assert.Regexp(t, re, code, "suite code should match QCS-XXXXXX format")
		assert.True(t, strings.HasPrefix(code, prefix))
	}
}

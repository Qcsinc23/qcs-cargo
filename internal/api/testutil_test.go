//go:build integration

package api_test

import (
	"os"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/api"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/gofiber/fiber/v2"
)

func init() {
	// Test config: fake keys so handlers don't call real Stripe/Resend
	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-integration-tests")
	_ = os.Setenv("STRIPE_SECRET_KEY", "sk_test_fake")
	_ = os.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_fake")
	_ = os.Setenv("RESEND_API_KEY", "re_test_fake")
	_ = os.Setenv("FROM_EMAIL", "test@qcs-cargo.com")
	_ = os.Setenv("APP_URL", "http://localhost:3000")
	_ = os.Setenv("QCS_OBSERVABILITY_DISABLED", "1")
}

// setupTestApp creates a Fiber app with all API routes, backed by a seeded
// in-memory SQLite database. Uses NewSeededDB(t) and test config above.
func setupTestApp(t *testing.T) *fiber.App {
	t.Helper()
	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)
	app := fiber.New(fiber.Config{
		ErrorHandler: api.ErrorHandler,
	})
	api.RegisterAPIRoutes(app)
	return app
}

package api

import (
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RegisterAPIRoutes registers all API routes (v1 and Stripe webhook) on the app.
// Used by cmd/server and by integration tests.
func RegisterAPIRoutes(app *fiber.App) {
	apiGroup := app.Group("/api", middleware.PropagateRequestID)
	RegisterStripeWebhook(apiGroup)

	v1 := apiGroup.Group("/v1")
	limitMiddleware := limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
	})
	v1.Use(func(c *fiber.Ctx) error {
		if c.Path() == "/api/v1/health" {
			return c.Next()
		}
		return limitMiddleware(c)
	})

	// Auth routes get stricter rate limiting on top of the global limiter.
	v1.Use("/auth", middleware.AuthRateLimit)
	// CSRF protection for cookie-authenticated mutation requests.
	v1.Use(middleware.CSRFProtection)

	RegisterHealth(v1)
	RegisterAuth(v1)
	RegisterPublic(v1)
	RegisterLocker(v1)
	RegisterRecipients(v1)
	RegisterShipRequests(v1)
	RegisterShipments(v1)
	RegisterInvoices(v1)
	RegisterTemplates(v1)
	RegisterBookings(v1)
	RegisterInboundTracking(v1)
	v1.Get("/me", middleware.RequireAuth, Me)
	RegisterMe(v1)
	RegisterAdmin(v1)
	RegisterWarehouse(v1)
	RegisterNotifications(v1)
	RegisterSessions(v1)
	RegisterAccount(v1)
	RegisterBlog(v1)
}

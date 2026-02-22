package api

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
)

// RegisterStripeWebhook mounts POST /webhooks/stripe. Must be mounted on a router that serves /api (e.g. app.Group("/api")).
// Raw body is required for signature verification; do not use body parser middleware for this route.
func RegisterStripeWebhook(g fiber.Router) {
	g.Post("/webhooks/stripe", stripeWebhookHandler)
}

func stripeWebhookHandler(c *fiber.Ctx) error {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		log.Print("[stripe webhook] STRIPE_WEBHOOK_SECRET not set")
		return c.Status(500).SendString("Webhook not configured")
	}
	payload := c.Body()
	sig := c.Get("Stripe-Signature")
	if sig == "" {
		return c.Status(400).SendString("Missing Stripe-Signature")
	}
	event, err := webhook.ConstructEvent(payload, sig, secret)
	if err != nil {
		log.Printf("[stripe webhook] signature verification failed: %v", err)
		return c.Status(400).SendString("Invalid signature")
	}
	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := pi.UnmarshalJSON(event.Data.Raw); err != nil {
			log.Printf("[stripe webhook] payment_intent parse: %v", err)
			return c.Status(500).SendString("Parse error")
		}
		if err := reconcileShipRequestByPaymentIntent(c, pi.ID); err != nil {
			log.Printf("[stripe webhook] reconcile: %v", err)
			return c.Status(500).SendString("Reconcile failed")
		}
	case "payment_intent.payment_failed", "payment_intent.canceled", "payment_intent.requires_action":
		// Log only; no DB update
		log.Printf("[stripe webhook] %s: %s", event.Type, event.ID)
	default:
		log.Printf("[stripe webhook] unhandled event type: %s", event.Type)
	}
	return c.SendStatus(200)
}

func reconcileShipRequestByPaymentIntent(c *fiber.Ctx, paymentIntentID string) error {
	sr, err := db.Queries().GetShipRequestByStripePaymentIntentID(c.Context(), sql.NullString{String: paymentIntentID, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[stripe webhook] no ship request for payment_intent %s", paymentIntentID)
			return nil // idempotent: already reconciled or unknown
		}
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return db.Queries().UpdateShipRequestPaymentReconcile(c.Context(), gen.UpdateShipRequestPaymentReconcileParams{
		PaymentStatus: sql.NullString{String: "paid", Valid: true},
		Status:        "paid",
		UpdatedAt:     now,
		ID:            sr.ID,
		UserID:        sr.UserID,
	})
}

package api

import (
	"database/sql"
	"errors"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

// errAmountMismatch is returned when a Stripe payment_intent.succeeded event
// reports an amount or currency that does not match the ship request total.
// We surface a 400 to Stripe so the event is not retried indefinitely; an
// alert log entry is also emitted.
var errAmountMismatch = errors.New("payment amount or currency mismatch")

// RegisterStripeWebhook mounts POST /webhooks/stripe. Must be mounted on a router that serves /api (e.g. app.Group("/api")).
// Raw body is required for signature verification; do not use body parser middleware for this route.
func RegisterStripeWebhook(g fiber.Router) {
	g.Post("/webhooks/stripe", stripeWebhookHandler)
}

func stripeWebhookHandler(c *fiber.Ctx) error {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		log.Print("[stripe webhook] STRIPE_WEBHOOK_SECRET not set; rejecting webhook with 503")
		return c.Status(fiber.StatusServiceUnavailable).SendString("Webhook not configured")
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
		if err := reconcileShipRequestByPaymentIntent(c, &pi); err != nil {
			if errors.Is(err, errAmountMismatch) {
				// 400 so Stripe does not keep retrying. The reconciliation
				// helper has already logged the mismatch and recorded an
				// observability event for ops to investigate.
				return c.Status(400).SendString("Amount mismatch")
			}
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

// reconcileShipRequestByPaymentIntent marks the ship request paid only when
// the PaymentIntent's amount, currency, and status all match expectations, and
// only sends a "Paid" email exactly once per ship request even if Stripe
// retries the webhook. Pass 2 audit fixes C-3 (amount/currency verification)
// and L-4 (idempotent paid email).
func reconcileShipRequestByPaymentIntent(c *fiber.Ctx, pi *stripe.PaymentIntent) error {
	if pi == nil {
		return errors.New("nil payment intent")
	}
	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		// We should only get here for the .succeeded event, but be defensive.
		log.Printf("[stripe webhook] payment_intent %s status=%s, ignoring", pi.ID, pi.Status)
		return nil
	}
	sr, err := db.Queries().GetShipRequestByStripePaymentIntentID(c.Context(), sql.NullString{String: pi.ID, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[stripe webhook] no ship request for payment_intent %s", pi.ID)
			return nil // idempotent: already reconciled or unknown
		}
		return err
	}

	expectedCents := int64(math.Round(sr.Total * 100))
	if pi.Amount != expectedCents || !strings.EqualFold(string(pi.Currency), "usd") {
		log.Printf("[stripe webhook][ALERT] amount/currency mismatch for ship_request %s pi=%s pi_amount=%d pi_currency=%s expected=%d expected_currency=usd",
			sr.ID, pi.ID, pi.Amount, pi.Currency, expectedCents)
		// Mark the ship request for manual review rather than silently flipping
		// to paid. We do not fail the request so the customer is not double-blocked.
		now := time.Now().UTC().Format(time.RFC3339)
		_, _ = db.DB().ExecContext(c.Context(),
			`UPDATE ship_requests SET payment_status = ?, updated_at = ? WHERE id = ? AND payment_status IS NOT 'paid'`,
			"review_required", now, sr.ID,
		)
		return errAmountMismatch
	}

	// Atomic state transition: only flip to paid if not already paid, and only
	// then send the paid email. Without this, every retried webhook event for
	// the same successful PI would re-send a "Paid" email.
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.DB().ExecContext(c.Context(), `
UPDATE ship_requests
SET status = 'paid',
    payment_status = 'paid',
    updated_at = ?,
    paid_email_sent_at = COALESCE(paid_email_sent_at, ?)
WHERE id = ? AND user_id = ? AND COALESCE(payment_status, '') <> 'paid'
`, now, now, sr.ID, sr.UserID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// Already reconciled; nothing more to do.
		return nil
	}

	u, err := db.Queries().GetUserByID(c.Context(), sr.UserID)
	if err == nil {
		if err := services.SendShipRequestPaid(u.Email, sr.ConfirmationCode); err != nil {
			log.Printf("[stripe webhook] send payment success email failed for ship_request %s user %s: %v", sr.ID, sr.UserID, err)
		}
	}
	return nil
}

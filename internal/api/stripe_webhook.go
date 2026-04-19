package api

import (
	"database/sql"
	"errors"
	"fmt"
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

// RegisterStripeWebhook mounts POST /webhooks/stripe.
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
				return c.Status(400).SendString("Amount mismatch")
			}
			log.Printf("[stripe webhook] reconcile: %v", err)
			return c.Status(500).SendString("Reconcile failed")
		}
	case "payment_intent.payment_failed", "payment_intent.canceled", "payment_intent.requires_action":
		log.Printf("[stripe webhook] %s: %s", event.Type, event.ID)
	default:
		log.Printf("[stripe webhook] unhandled event type: %s", event.Type)
	}
	return c.SendStatus(200)
}

func reconcileShipRequestByPaymentIntent(c *fiber.Ctx, pi *stripe.PaymentIntent) error {
	if pi == nil {
		return errors.New("nil payment intent")
	}
	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		log.Printf("[stripe webhook] payment_intent %s status=%s, ignoring", pi.ID, pi.Status)
		return nil
	}
	sr, err := db.Queries().GetShipRequestByStripePaymentIntentID(c.Context(), sql.NullString{String: pi.ID, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[stripe webhook] no ship request for payment_intent %s", pi.ID)
			return nil
		}
		return err
	}

	expectedCents := int64(math.Round(sr.Total * 100))
	if pi.Amount != expectedCents || !strings.EqualFold(string(pi.Currency), "usd") {
		log.Printf("[stripe webhook][ALERT] amount/currency mismatch for ship_request %s pi=%s pi_amount=%d pi_currency=%s expected=%d expected_currency=usd",
			sr.ID, pi.ID, pi.Amount, pi.Currency, expectedCents)
		now := time.Now().UTC().Format(time.RFC3339)
		_, _ = db.DB().ExecContext(c.Context(),
			`UPDATE ship_requests SET payment_status = ?, updated_at = ? WHERE id = ? AND payment_status IS NOT 'paid'`,
			"review_required", now, sr.ID,
		)
		return errAmountMismatch
	}

	// Pass 3 CRIT-02 fix: the ship_requests UPDATE that flips payment_status
	// to 'paid' AND the outbound_emails INSERT that enqueues the customer's
	// "paid" confirmation email must share a single atomic boundary.
	// Previously they were two separate ExecContext calls; if the process
	// died between them, the customer would be marked paid but never
	// receive the confirmation, with no observable signal because the
	// worker only ever drains rows that exist. Stripe also retries this
	// webhook on non-200, so we want the failure mode to be
	// "tx aborts -> 500 -> Stripe retries" rather than "partial commit
	// -> silent data loss".
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(c.Context(), `
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
		// Already reconciled; commit no-op so Stripe stops retrying.
		return tx.Commit()
	}

	u, err := db.Queries().WithTx(tx).GetUserByID(c.Context(), sr.UserID)
	if err != nil {
		return fmt.Errorf("stripe webhook: lookup user %s for ship_request %s: %w", sr.UserID, sr.ID, err)
	}

	if err := services.EnqueueEmailTx(c.Context(), tx, services.TemplateShipRequestPaid, u.Email, map[string]any{
		"confirmation_code": sr.ConfirmationCode,
	}); err != nil {
		log.Printf("[stripe webhook] enqueue payment success email failed for ship_request %s user %s: %v", sr.ID, sr.UserID, err)
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("stripe webhook: commit reconciliation tx for ship_request %s: %w", sr.ID, err)
	}
	return nil
}

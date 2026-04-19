package api

// ship_requests_payment.go holds the Stripe-PaymentIntent + admin
// reconciliation paths split out from ship_requests.go in Phase 3.3
// (QAL-001). The CRUD lifecycle for ship requests stays in
// ship_requests.go; everything money-adjacent lives here so the
// payment surface can be reasoned about in isolation.

import (
	"database/sql"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentintent"
)

// shipRequestPay creates (or returns) a Stripe PaymentIntent for the ship
// request. Pass 2 audit fixes:
//
//   - C-2: rejects already-paid requests, reuses an existing reusable
//     PaymentIntent when one is on file, and sends a deterministic
//     Idempotency-Key so a network retry does not create a second charge.
//   - M-3: uses math.Round when converting the float total to cents so amounts
//     such as $19.99 are not silently truncated to $19.98.
func shipRequestPay(c *fiber.Ctx) error {
	const maxPaymentCents = 5_000_000 // $50,000 safety guardrail

	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	sr, err := db.Queries().GetShipRequestByID(c.Context(), gen.GetShipRequestByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
	}
	if sr.Status == "paid" {
		return c.Status(409).JSON(ErrorResponse{}.withCode("ALREADY_PAID", "Ship request is already paid"))
	}
	if sr.Status != "pending_payment" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Ship request must be pending_payment before payment"))
	}
	amountCents := int64(math.Round(sr.Total * 100))
	if amountCents < 50 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Minimum charge is $0.50"))
	}
	if amountCents > maxPaymentCents {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Payment amount exceeds maximum allowed"))
	}
	secretKey := os.Getenv("STRIPE_SECRET_KEY")
	if secretKey == "" {
		return c.Status(501).JSON(fiber.Map{
			"error":         "payment_not_configured",
			"message":       "Stripe is not configured. Set STRIPE_SECRET_KEY for live payments.",
			"client_secret": nil,
		})
	}
	stripe.Key = secretKey

	// C-2: if we already have a PaymentIntent on file and it is still
	// reusable (requires_payment_method, requires_confirmation, requires_action,
	// processing) we hand the existing client_secret back to the user instead
	// of opening a second intent that could lead to a duplicate charge.
	if sr.StripePaymentIntentID.Valid && strings.TrimSpace(sr.StripePaymentIntentID.String) != "" {
		existing, getErr := paymentintent.Get(sr.StripePaymentIntentID.String, nil)
		if getErr == nil && existing != nil && isReusablePaymentIntent(existing) && existing.Amount == amountCents && string(existing.Currency) == strings.ToLower(string(stripe.CurrencyUSD)) {
			return c.JSON(fiber.Map{"data": fiber.Map{"client_secret": existing.ClientSecret}})
		}
		// Otherwise (canceled, expired, amount changed, lookup failed) fall
		// through to creating a fresh intent. We do not attempt to cancel the
		// old one here so Stripe retains a record; webhook reconciliation
		// guards against amount drift (see C-3 in stripe_webhook.go).
	}

	// C-2: deterministic Idempotency-Key keyed off the ship request and its
	// last update time. A double-submit (e.g. user double-clicks "Pay") within
	// the same logical state collapses to one PaymentIntent at Stripe.
	idemKey := "ship_request:" + sr.ID + ":" + sr.UpdatedAt
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Metadata: map[string]string{
			"ship_request_id":   sr.ID,
			"ship_request_user": sr.UserID,
		},
	}
	params.IdempotencyKey = stripe.String(idemKey)
	pi, err := paymentintent.New(params)
	if err != nil {
		log.Printf("[ship_requests] paymentintent.New failed for ship_request %s: %v", sr.ID, err)
		return c.Status(502).JSON(ErrorResponse{}.withCode("PAYMENT_PROVIDER_ERROR", "Failed to create payment intent"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.Queries().UpdateShipRequestPaymentIntent(c.Context(), gen.UpdateShipRequestPaymentIntentParams{
		StripePaymentIntentID: sql.NullString{String: pi.ID, Valid: true},
		UpdatedAt:             now,
		ID:                    id,
		UserID:                userID,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to save payment intent"))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"client_secret": pi.ClientSecret}})
}

// isReusablePaymentIntent reports whether a Stripe PaymentIntent is in a state
// where the existing client_secret can still be used to complete payment.
func isReusablePaymentIntent(pi *stripe.PaymentIntent) bool {
	if pi == nil {
		return false
	}
	switch pi.Status {
	case stripe.PaymentIntentStatusRequiresPaymentMethod,
		stripe.PaymentIntentStatusRequiresConfirmation,
		stripe.PaymentIntentStatusRequiresAction,
		stripe.PaymentIntentStatusProcessing:
		return true
	default:
		return false
	}
}

// shipRequestReconcile is admin-only (route is gated by RequireAdmin) and
// must operate on any user's ship request, not only those owned by the
// calling admin. DEF-001 fix: previously this used UserID-scoped queries,
// which made the endpoint non-functional for its purpose (manual
// reconciliation of stuck customer payments). It now looks up and updates
// by id only, and records the action under the admin's user_id for audit.
func shipRequestReconcile(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	adminUserID := c.Locals(middleware.CtxUserID).(string)
	if _, err := db.Queries().GetShipRequestByIDForAdmin(c.Context(), id); err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.Queries().UpdateShipRequestPaymentReconcileForAdmin(c.Context(), gen.UpdateShipRequestPaymentReconcileForAdminParams{
		PaymentStatus: sql.NullString{String: "paid", Valid: true},
		Status:        "paid",
		UpdatedAt:     now,
		ID:            id,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to reconcile payment"))
	}
	recordActivity(c.Context(), adminUserID, "admin.ship_request.reconcile", "ship_request", id, "manual")
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"payment_status": "paid"}})
}

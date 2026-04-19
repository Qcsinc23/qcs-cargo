package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentintent"
)

// RegisterShipRequests mounts ship-request routes. All require auth.
func RegisterShipRequests(g fiber.Router) {
	g.Get("/ship-requests", middleware.RequireAuth, shipRequestList)
	g.Get("/ship-requests/:id", middleware.RequireAuth, shipRequestGetByID)
	g.Post("/ship-requests", middleware.RequireAuth, shipRequestCreate)
	g.Post("/ship-requests/:id/customs", middleware.RequireAuth, shipRequestSubmitCustoms)
	g.Get("/ship-requests/:id/estimate", middleware.RequireAuth, shipRequestEstimate)
	g.Post("/ship-requests/:id/pay", middleware.RequireAuth, shipRequestPay)
	g.Post("/ship-requests/:id/reconcile", middleware.RequireAuth, middleware.RequireAdmin, shipRequestReconcile)
}

func shipRequestList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	list, err := db.Queries().ListShipRequestsByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list ship requests"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func shipRequestGetByID(c *fiber.Ctx) error {
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
	items, err := db.Queries().ListShipRequestItemsByShipRequestID(c.Context(), sr.ID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load items"))
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"ship_request": sr,
			"items":        items,
		},
	})
}

// customsItemBody is one element of POST /ship-requests/:id/customs body.
type customsItemBody struct {
	ID              string   `json:"id"` // ship_request_item id
	Description     string   `json:"description"`
	Value           *float64 `json:"value"`
	Quantity        *int64   `json:"quantity"`
	HsCode          string   `json:"hs_code"`
	CountryOfOrigin string   `json:"country_of_origin"`
	WeightLbs       *float64 `json:"weight_lbs"`
}

func shipRequestSubmitCustoms(c *fiber.Ctx) error {
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
	if sr.Status != "draft" && sr.Status != "pending_customs" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Ship request must be draft or pending_customs to submit customs"))
	}
	var body []customsItemBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body: expected array of customs items"))
	}
	items, err := db.Queries().ListShipRequestItemsByShipRequestID(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load items"))
	}
	itemByID := make(map[string]gen.ShipRequestItem)
	for _, it := range items {
		itemByID[it.ID] = it
	}

	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start transaction"))
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("shipRequestSubmitCustoms rollback failed: %v", rbErr)
		}
	}()
	qtx := db.Queries().WithTx(tx)

	now := time.Now().UTC().Format(time.RFC3339)
	for _, b := range body {
		if b.ID == "" {
			continue
		}
		_, ok := itemByID[b.ID]
		if !ok {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "unknown item id: "+b.ID))
		}
		if b.Value != nil && *b.Value < 0 {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "customs value cannot be negative"))
		}
		if b.Quantity != nil && *b.Quantity < 0 {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "customs quantity cannot be negative"))
		}
		if b.WeightLbs != nil && *b.WeightLbs < 0 {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "customs weight cannot be negative"))
		}
		arg := gen.UpdateShipRequestItemCustomsParams{
			ID:            b.ID,
			ShipRequestID: id,
		}
		if b.Description != "" {
			arg.CustomsDescription = sql.NullString{String: b.Description, Valid: true}
		}
		if b.Value != nil {
			arg.CustomsValue = sql.NullFloat64{Float64: *b.Value, Valid: true}
		}
		if b.Quantity != nil {
			arg.CustomsQuantity = sql.NullInt64{Int64: *b.Quantity, Valid: true}
		}
		if b.HsCode != "" {
			arg.CustomsHsCode = sql.NullString{String: b.HsCode, Valid: true}
		}
		if b.CountryOfOrigin != "" {
			arg.CustomsCountryOfOrigin = sql.NullString{String: b.CountryOfOrigin, Valid: true}
		}
		if b.WeightLbs != nil {
			arg.CustomsWeightLbs = sql.NullFloat64{Float64: *b.WeightLbs, Valid: true}
		}
		if err := qtx.UpdateShipRequestItemCustoms(c.Context(), arg); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update item customs"))
		}
	}

	pricingInputs, err := loadShipRequestPricingInputs(c.Context(), qtx, userID, id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to recalculate ship request pricing"))
	}
	pricing := services.CalculateShipmentPricing(sr.DestinationID, sr.ServiceType, pricingInputs)
	if err := qtx.UpdateShipRequestPricing(c.Context(), gen.UpdateShipRequestPricingParams{
		Subtotal:    pricing.Subtotal,
		ServiceFees: pricing.ServiceFees,
		Insurance:   pricing.Insurance,
		Discount:    pricing.Discount,
		Total:       pricing.Total,
		UpdatedAt:   now,
		ID:          id,
		UserID:      userID,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update ship request pricing"))
	}
	customsStatus := sql.NullString{String: "submitted", Valid: true}
	if err := qtx.UpdateShipRequestCustomsStatus(c.Context(), gen.UpdateShipRequestCustomsStatusParams{
		CustomsStatus: customsStatus,
		Status:        "pending_payment",
		UpdatedAt:     now,
		ID:            id,
		UserID:        userID,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update customs status"))
	}
	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit ship request customs updates"))
	}
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"customs_status": "submitted"}})
}

func shipRequestEstimate(c *fiber.Ctx) error {
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
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"subtotal":     sr.Subtotal,
			"service_fees": sr.ServiceFees,
			"insurance":    sr.Insurance,
			"discount":     sr.Discount,
			"total":        sr.Total,
		},
	})
}

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

func shipRequestReconcile(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	_, err := db.Queries().GetShipRequestByID(c.Context(), gen.GetShipRequestByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.Queries().UpdateShipRequestPaymentReconcile(c.Context(), gen.UpdateShipRequestPaymentReconcileParams{
		PaymentStatus: sql.NullString{String: "paid", Valid: true},
		Status:        "paid",
		UpdatedAt:     now,
		ID:            id,
		UserID:        userID,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to reconcile payment"))
	}
	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"payment_status": "paid"}})
}

// createShipRequestBody supports locker_package_ids (simple list) or items (with optional customs).
type createShipRequestBody struct {
	DestinationID       string   `json:"destination_id"`
	ServiceType         string   `json:"service_type"`
	RecipientID         *string  `json:"recipient_id"`
	Consolidate         *bool    `json:"consolidate"`
	SpecialInstructions *string  `json:"special_instructions"`
	LockerPackageIDs    []string `json:"locker_package_ids"`
	Items               []struct {
		LockerPackageID        string   `json:"locker_package_id"`
		CustomsDescription     *string  `json:"customs_description,omitempty"`
		CustomsValue           *float64 `json:"customs_value,omitempty"`
		CustomsQuantity        *int64   `json:"customs_quantity,omitempty"`
		CustomsHsCode          *string  `json:"customs_hs_code,omitempty"`
		CustomsCountryOfOrigin *string  `json:"customs_country_of_origin,omitempty"`
		CustomsWeightLbs       *float64 `json:"customs_weight_lbs,omitempty"`
	} `json:"items"`
}

func shipRequestCreate(c *fiber.Ctx) error {
	var body createShipRequestBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	body.DestinationID = strings.TrimSpace(body.DestinationID)
	body.ServiceType = strings.TrimSpace(body.ServiceType)
	if body.DestinationID == "" || body.ServiceType == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "destination_id and service_type required"))
	}
	if err := services.ValidateDestination(body.DestinationID); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	if !isAllowedShipRequestServiceType(body.ServiceType) {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "invalid service_type"))
	}

	var packageIDs []string
	if len(body.LockerPackageIDs) > 0 {
		packageIDs = body.LockerPackageIDs
	} else if len(body.Items) > 0 {
		for _, it := range body.Items {
			if it.LockerPackageID != "" {
				packageIDs = append(packageIDs, it.LockerPackageID)
			}
		}
	}
	if len(packageIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "locker_package_ids or items required"))
	}
	packageIDs = uniqueNonEmptyStrings(packageIDs)
	if len(packageIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "locker_package_ids or items required"))
	}

	userID := c.Locals(middleware.CtxUserID).(string)

	recipientID, err := validateRecipientForShipRequest(c.Context(), userID, body.DestinationID, body.RecipientID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "recipient not found or not yours"))
		}
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			return c.Status(fiberErr.Code).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", fiberErr.Message))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to validate recipient"))
	}
	pricingInputs, err := collectShipRequestPricingInputs(c.Context(), db.Queries(), userID, packageIDs, nil)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "package not found or not yours"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to validate package"))
	}
	pricing := services.CalculateShipmentPricing(body.DestinationID, body.ServiceType, pricingInputs)

	now := time.Now().UTC().Format(time.RFC3339)
	srID := uuid.New().String()
	confirmationCode := "SR-" + genConfirmationCode()
	specialInstructions := sql.NullString{}
	if body.SpecialInstructions != nil && *body.SpecialInstructions != "" {
		specialInstructions = sql.NullString{String: *body.SpecialInstructions, Valid: true}
	}
	consolidate := 1
	if body.Consolidate != nil && !*body.Consolidate {
		consolidate = 0
	}

	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start transaction"))
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("shipRequestCreate rollback failed: %v", rbErr)
		}
	}()
	qtx := db.Queries().WithTx(tx)

	// Prevent double-shipping: check if any package is already in an active ship request
	for _, pkgID := range packageIDs {
		count, err := qtx.GetActiveShipRequestCountByPackageID(c.Context(), pkgID)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to check package status"))
		}
		if count > 0 {
			return c.Status(409).JSON(ErrorResponse{}.withCode("CONFLICT", "Package "+pkgID+" is already associated with an active ship request"))
		}
	}

	_, err = qtx.CreateShipRequest(c.Context(), gen.CreateShipRequestParams{
		ID:                  srID,
		UserID:              userID,
		ConfirmationCode:    confirmationCode,
		DestinationID:       body.DestinationID,
		RecipientID:         recipientID,
		ServiceType:         body.ServiceType,
		Consolidate:         consolidate,
		SpecialInstructions: specialInstructions,
		Subtotal:            pricing.Subtotal,
		ServiceFees:         pricing.ServiceFees,
		Insurance:           pricing.Insurance,
		Discount:            pricing.Discount,
		Total:               pricing.Total,
		CreatedAt:           now,
		UpdatedAt:           now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create ship request"))
	}

	itemByPkg := make(map[string]struct {
		CustomsDescription     *string
		CustomsValue           *float64
		CustomsQuantity        *int64
		CustomsHsCode          *string
		CustomsCountryOfOrigin *string
		CustomsWeightLbs       *float64
	})
	for _, it := range body.Items {
		if it.LockerPackageID != "" {
			itemByPkg[it.LockerPackageID] = struct {
				CustomsDescription     *string
				CustomsValue           *float64
				CustomsQuantity        *int64
				CustomsHsCode          *string
				CustomsCountryOfOrigin *string
				CustomsWeightLbs       *float64
			}{
				it.CustomsDescription, it.CustomsValue, it.CustomsQuantity,
				it.CustomsHsCode, it.CustomsCountryOfOrigin, it.CustomsWeightLbs,
			}
		}
	}

	for _, pkgID := range packageIDs {
		customs := itemByPkg[pkgID]
		arg := gen.CreateShipRequestItemParams{
			ID:              uuid.New().String(),
			ShipRequestID:   srID,
			LockerPackageID: pkgID,
		}
		if customs.CustomsDescription != nil {
			arg.CustomsDescription = sql.NullString{String: *customs.CustomsDescription, Valid: true}
		}
		if customs.CustomsValue != nil {
			arg.CustomsValue = sql.NullFloat64{Float64: *customs.CustomsValue, Valid: true}
		}
		if customs.CustomsQuantity != nil {
			arg.CustomsQuantity = sql.NullInt64{Int64: *customs.CustomsQuantity, Valid: true}
		}
		if customs.CustomsHsCode != nil {
			arg.CustomsHsCode = sql.NullString{String: *customs.CustomsHsCode, Valid: true}
		}
		if customs.CustomsCountryOfOrigin != nil {
			arg.CustomsCountryOfOrigin = sql.NullString{String: *customs.CustomsCountryOfOrigin, Valid: true}
		}
		if customs.CustomsWeightLbs != nil {
			arg.CustomsWeightLbs = sql.NullFloat64{Float64: *customs.CustomsWeightLbs, Valid: true}
		}
		_, err = qtx.CreateShipRequestItem(c.Context(), arg)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create ship request item"))
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit transaction"))
	}

	return c.Status(201).JSON(fiber.Map{
		"status": "success",
		"data":   fiber.Map{"id": srID, "confirmation_code": confirmationCode},
	})
}

func validateRecipientForShipRequest(ctx context.Context, userID, destinationID string, recipientID *string) (sql.NullString, error) {
	if recipientID == nil || strings.TrimSpace(*recipientID) == "" {
		return sql.NullString{}, nil
	}
	rec, err := db.Queries().GetRecipientByID(ctx, gen.GetRecipientByIDParams{
		ID:     strings.TrimSpace(*recipientID),
		UserID: userID,
	})
	if err != nil {
		return sql.NullString{}, err
	}
	if rec.DestinationID != destinationID {
		return sql.NullString{}, fiber.NewError(fiber.StatusBadRequest, "recipient destination does not match ship request destination")
	}
	return sql.NullString{String: rec.ID, Valid: true}, nil
}

func collectShipRequestPricingInputs(ctx context.Context, queries *gen.Queries, userID string, packageIDs []string, weightOverrides map[string]sql.NullFloat64) ([]services.ShipmentPackageInput, error) {
	inputs := make([]services.ShipmentPackageInput, 0, len(packageIDs))
	for _, pkgID := range packageIDs {
		pkg, err := queries.GetLockerPackageByID(ctx, gen.GetLockerPackageByIDParams{ID: pkgID, UserID: userID})
		if err != nil {
			return nil, err
		}
		input := services.ShipmentPackageInput{}
		if pkg.WeightLbs.Valid {
			input.WeightLbs = pkg.WeightLbs.Float64
		}
		if pkg.LengthIn.Valid {
			input.LengthIn = pkg.LengthIn.Float64
		}
		if pkg.WidthIn.Valid {
			input.WidthIn = pkg.WidthIn.Float64
		}
		if pkg.HeightIn.Valid {
			input.HeightIn = pkg.HeightIn.Float64
		}
		if override, ok := weightOverrides[pkgID]; ok && override.Valid {
			input.WeightLbs = override.Float64
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func loadShipRequestPricingInputs(ctx context.Context, queries *gen.Queries, userID, shipRequestID string) ([]services.ShipmentPackageInput, error) {
	items, err := queries.ListShipRequestItemsByShipRequestID(ctx, shipRequestID)
	if err != nil {
		return nil, err
	}
	packageIDs := make([]string, 0, len(items))
	weightOverrides := make(map[string]sql.NullFloat64, len(items))
	for _, item := range items {
		packageIDs = append(packageIDs, item.LockerPackageID)
		weightOverrides[item.LockerPackageID] = item.CustomsWeightLbs
	}
	return collectShipRequestPricingInputs(ctx, queries, userID, packageIDs, weightOverrides)
}

func isAllowedShipRequestServiceType(serviceType string) bool {
	switch serviceType {
	case "standard", "express", "door_to_door":
		return true
	default:
		return false
	}
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

const confirmationChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// genConfirmationCode returns 8 alphanumeric characters (PRD: SR-{8 alphanumeric}).
func genConfirmationCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i := range out {
		out[i] = confirmationChars[int(b[i])%len(confirmationChars)]
	}
	return string(out)
}

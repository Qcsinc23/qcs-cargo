package api

// bookings_admin.go holds the admin-only booking lifecycle handler split
// out from bookings.go for Pass 2.5 (CRIT-01 + HIGH-01). The customer
// PATCH /bookings/:id handler in bookings.go has been narrowed so it
// cannot mutate payment_status or move a booking through staff workflow
// states. Anything that requires the broader lifecycle (confirmed,
// received, completed) or a payment_status flip (pending -> paid /
// failed / refunded) lives here behind the RequireAdmin gate registered
// in admin.go.

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

// bookingAdminUpdateStatus is admin-only (gated by RequireAdmin in the
// route registration in admin.go). It lets ops drive a booking through
// the operational lifecycle (pending -> confirmed -> received ->
// completed, or cancelled) and update payment_status outside the
// customer's reach. Pass 2.5 fix for CRIT-01 + HIGH-01.
//
// Mirrors the shipRequestReconcile pattern: id-only WHERE clause (no
// user_id scope) because the route is gated upstream by RequireAdmin
// and an admin must be able to act on any user's booking.
func bookingAdminUpdateStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Status        *string `json:"status"`
		PaymentStatus *string `json:"payment_status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	existing, err := db.Queries().GetBookingByIDForAdmin(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Booking not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load booking"))
	}

	status := existing.Status
	if body.Status != nil {
		s := strings.TrimSpace(*body.Status)
		if !isAllowedBookingStatus(s) {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "invalid status"))
		}
		status = s
	}

	paymentStatus := existing.PaymentStatus
	if body.PaymentStatus != nil {
		ps := strings.TrimSpace(*body.PaymentStatus)
		if !isAllowedBookingPaymentStatus(ps) {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "invalid payment_status"))
		}
		paymentStatus = sql.NullString{String: ps, Valid: true}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.Queries().AdminUpdateBookingStatus(c.Context(), gen.AdminUpdateBookingStatusParams{
		Status:        status,
		PaymentStatus: paymentStatus,
		UpdatedAt:     now,
		ID:            id,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update booking"))
	}

	adminID := c.Locals(middleware.CtxUserID).(string)
	recordActivity(c.Context(), adminID, "admin.booking.update", "booking",
		id, fmt.Sprintf("status=%s,payment_status=%s", status, paymentStatus.String))

	updated, err := db.Queries().GetBookingByIDForAdmin(c.Context(), id)
	if err != nil {
		// The row was just updated successfully; a re-read failure here
		// would be unexpected. Return what we know without 500-ing the
		// caller, mirroring the tolerance pattern used in
		// adminShipRequestUpdateStatus.
		return c.JSON(fiber.Map{"data": fiber.Map{
			"id":             id,
			"status":         status,
			"payment_status": paymentStatus.String,
			"updated_at":     now,
		}})
	}
	return c.JSON(fiber.Map{"data": fiber.Map{
		"id":             updated.ID,
		"status":         updated.Status,
		"payment_status": updated.PaymentStatus.String,
		"updated_at":     updated.UpdatedAt,
	}})
}

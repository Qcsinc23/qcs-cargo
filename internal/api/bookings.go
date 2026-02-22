package api

import (
	"crypto/rand"
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterBookings mounts booking routes. All require auth.
func RegisterBookings(g fiber.Router) {
	g.Get("/bookings/time-slots", middleware.RequireAuth, bookingsTimeSlots)
	g.Get("/bookings", middleware.RequireAuth, bookingList)
	g.Get("/bookings/:id", middleware.RequireAuth, bookingGetByID)
	g.Post("/bookings", middleware.RequireAuth, bookingCreate)
	g.Patch("/bookings/:id", middleware.RequireAuth, bookingUpdate)
	g.Delete("/bookings/:id", middleware.RequireAuth, bookingDelete)
}

func bookingList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	list, err := db.Queries().ListBookingsByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list bookings"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func bookingGetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	b, err := db.Queries().GetBookingByID(c.Context(), gen.GetBookingByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Booking not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load booking"))
	}
	return c.JSON(fiber.Map{"data": b})
}

func bookingCreate(c *fiber.Ctx) error {
	var body struct {
		ServiceType          string   `json:"service_type"`
		DestinationID        string   `json:"destination_id"`
		RecipientID          *string  `json:"recipient_id"`
		ScheduledDate        string   `json:"scheduled_date"`
		TimeSlot             string   `json:"time_slot"`
		SpecialInstructions  *string  `json:"special_instructions"`
		Subtotal             float64  `json:"subtotal"`
		Discount             float64  `json:"discount"`
		Insurance            float64  `json:"insurance"`
		Total                float64  `json:"total"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.ServiceType == "" || body.DestinationID == "" || body.ScheduledDate == "" || body.TimeSlot == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "service_type, destination_id, scheduled_date, time_slot required"))
	}

	userID := c.Locals(middleware.CtxUserID).(string)
	now := time.Now().UTC().Format(time.RFC3339)
	confirmationCode := "BK-" + genBookingConfirmationCode()
	recipientID := sql.NullString{}
	if body.RecipientID != nil && *body.RecipientID != "" {
		recipientID = sql.NullString{String: *body.RecipientID, Valid: true}
	}
	specialInstructions := sql.NullString{}
	if body.SpecialInstructions != nil && *body.SpecialInstructions != "" {
		specialInstructions = sql.NullString{String: *body.SpecialInstructions, Valid: true}
	}

	b, err := db.Queries().CreateBooking(c.Context(), gen.CreateBookingParams{
		ID:                  uuid.New().String(),
		UserID:              userID,
		ConfirmationCode:    confirmationCode,
		Status:              "pending",
		ServiceType:         body.ServiceType,
		DestinationID:       body.DestinationID,
		RecipientID:         recipientID,
		ScheduledDate:       body.ScheduledDate,
		TimeSlot:            body.TimeSlot,
		SpecialInstructions: specialInstructions,
		Subtotal:            body.Subtotal,
		Discount:            body.Discount,
		Insurance:           body.Insurance,
		Total:               body.Total,
		CreatedAt:           now,
		UpdatedAt:           now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create booking"))
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": b})
}

func bookingUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	existing, err := db.Queries().GetBookingByID(c.Context(), gen.GetBookingByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Booking not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load booking"))
	}

	var body struct {
		Status              *string  `json:"status"`
		SpecialInstructions *string  `json:"special_instructions"`
		Subtotal            *float64 `json:"subtotal"`
		Discount            *float64 `json:"discount"`
		Insurance           *float64 `json:"insurance"`
		Total               *float64 `json:"total"`
		PaymentStatus       *string  `json:"payment_status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}

	status := existing.Status
	if body.Status != nil {
		status = *body.Status
	}
	specialInstructions := existing.SpecialInstructions
	if body.SpecialInstructions != nil {
		specialInstructions = sql.NullString{String: *body.SpecialInstructions, Valid: true}
	}
	subtotal := existing.Subtotal
	if body.Subtotal != nil {
		subtotal = *body.Subtotal
	}
	discount := existing.Discount
	if body.Discount != nil {
		discount = *body.Discount
	}
	insurance := existing.Insurance
	if body.Insurance != nil {
		insurance = *body.Insurance
	}
	total := existing.Total
	if body.Total != nil {
		total = *body.Total
	}
	paymentStatus := existing.PaymentStatus
	if body.PaymentStatus != nil {
		paymentStatus = sql.NullString{String: *body.PaymentStatus, Valid: true}
	}
	now := time.Now().UTC().Format(time.RFC3339)

	b, err := db.Queries().UpdateBooking(c.Context(), gen.UpdateBookingParams{
		Status:              status,
		SpecialInstructions: specialInstructions,
		Subtotal:            subtotal,
		Discount:            discount,
		Insurance:           insurance,
		Total:               total,
		PaymentStatus:       paymentStatus,
		UpdatedAt:           now,
		ID:                  id,
		UserID:              userID,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update booking"))
	}
	return c.JSON(fiber.Map{"data": b})
}

func bookingDelete(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	err := db.Queries().DeleteBooking(c.Context(), gen.DeleteBookingParams{ID: id, UserID: userID})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to delete booking"))
	}
	return c.Status(204).Send(nil)
}

// bookingsTimeSlots returns stub time slots for a given date (no time_slots table).
func bookingsTimeSlots(c *fiber.Ctx) error {
	date := c.Query("date")
	if date == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "date required (YYYY-MM-DD)"))
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "date must be YYYY-MM-DD"))
	}
	slots := []fiber.Map{
		{"id": "morning", "label": "Morning (8am–12pm)", "available": true},
		{"id": "afternoon", "label": "Afternoon (12pm–4pm)", "available": true},
		{"id": "evening", "label": "Evening (4pm–8pm)", "available": true},
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"date": date, "slots": slots}})
}

const bookingConfirmationChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func genBookingConfirmationCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i := range out {
		out[i] = bookingConfirmationChars[int(b[i])%len(bookingConfirmationChars)]
	}
	return string(out)
}

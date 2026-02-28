package api

import (
	"crypto/rand"
	"database/sql"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
		ServiceType         string  `json:"service_type"`
		DestinationID       string  `json:"destination_id"`
		RecipientID         *string `json:"recipient_id"`
		ScheduledDate       string  `json:"scheduled_date"`
		TimeSlot            string  `json:"time_slot"`
		SpecialInstructions *string `json:"special_instructions"`
		WeightLbs           float64 `json:"weight_lbs"`
		LengthIn            float64 `json:"length_in"`
		WidthIn             float64 `json:"width_in"`
		HeightIn            float64 `json:"height_in"`
		ValueUSD            float64 `json:"value_usd"`
		AddInsurance        bool    `json:"add_insurance"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.ServiceType == "" || body.DestinationID == "" || body.ScheduledDate == "" || body.TimeSlot == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "service_type, destination_id, scheduled_date, time_slot required"))
	}
	if err := services.ValidateDestination(body.DestinationID); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	// scheduled_date must be today or future (UTC)
	scheduledDate, err := time.Parse("2006-01-02", body.ScheduledDate)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "invalid scheduled_date format, use YYYY-MM-DD"))
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if scheduledDate.Before(today) {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "scheduled_date cannot be in the past"))
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

	// Server-side pricing calculation
	price := services.CalculatePricing(services.PricingInput{
		DestinationID: body.DestinationID,
		WeightLbs:     body.WeightLbs,
		LengthIn:      body.LengthIn,
		WidthIn:       body.WidthIn,
		HeightIn:      body.HeightIn,
		ServiceType:   body.ServiceType,
		ValueUSD:      body.ValueUSD,
		AddInsurance:  body.AddInsurance,
	})

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
		WeightLbs:           body.WeightLbs,
		LengthIn:            body.LengthIn,
		WidthIn:             body.WidthIn,
		HeightIn:            body.HeightIn,
		ValueUsd:            body.ValueUSD,
		AddInsurance:        boolToInt(body.AddInsurance),
		Subtotal:            price.Subtotal,
		Discount:            price.Discount,
		Insurance:           price.Insurance,
		Total:               price.Total,
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
		WeightLbs           *float64 `json:"weight_lbs"`
		LengthIn            *float64 `json:"length_in"`
		WidthIn             *float64 `json:"width_in"`
		HeightIn            *float64 `json:"height_in"`
		ValueUSD            *float64 `json:"value_usd"`
		AddInsurance        *bool    `json:"add_insurance"`
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

	// Recalculate price with existing values as defaults for fields not provided
	weight := existing.WeightLbs
	if body.WeightLbs != nil {
		weight = *body.WeightLbs
	}
	length := existing.LengthIn
	if body.LengthIn != nil {
		length = *body.LengthIn
	}
	width := existing.WidthIn
	if body.WidthIn != nil {
		width = *body.WidthIn
	}
	height := existing.HeightIn
	if body.HeightIn != nil {
		height = *body.HeightIn
	}
	val := existing.ValueUsd
	if body.ValueUSD != nil {
		val = *body.ValueUSD
	}
	insure := existing.AddInsurance != 0
	if body.AddInsurance != nil {
		insure = *body.AddInsurance
	}

	price := services.CalculatePricing(services.PricingInput{
		DestinationID: existing.DestinationID,
		WeightLbs:     weight,
		LengthIn:      length,
		WidthIn:       width,
		HeightIn:      height,
		ServiceType:   existing.ServiceType,
		ValueUSD:      val,
		AddInsurance:  insure,
	})

	subtotal := existing.Subtotal
	discount := existing.Discount
	insCost := existing.Insurance
	total := existing.Total

	// Only update price if any pricing field was actually provided in the request
	if body.WeightLbs != nil || body.LengthIn != nil || body.WidthIn != nil || body.HeightIn != nil || body.ValueUSD != nil || body.AddInsurance != nil {
		subtotal = price.Subtotal
		discount = price.Discount
		insCost = price.Insurance
		total = price.Total
	}

	paymentStatus := existing.PaymentStatus
	if body.PaymentStatus != nil {
		paymentStatus = sql.NullString{String: *body.PaymentStatus, Valid: true}
	}
	now := time.Now().UTC().Format(time.RFC3339)

	b, err := db.Queries().UpdateBooking(c.Context(), gen.UpdateBookingParams{
		Status:              status,
		SpecialInstructions: specialInstructions,
		WeightLbs:           weight,
		LengthIn:            length,
		WidthIn:             width,
		HeightIn:            height,
		ValueUsd:            val,
		AddInsurance:        boolToInt(insure),
		Subtotal:            subtotal,
		Discount:            discount,
		Insurance:           insCost,
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

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

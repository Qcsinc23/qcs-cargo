package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterInboundTracking mounts inbound tracking routes. All require auth.
func RegisterInboundTracking(g fiber.Router) {
	g.Get("/inbound-tracking", middleware.RequireAuth, inboundTrackingList)
	g.Get("/inbound-tracking/:id", middleware.RequireAuth, inboundTrackingGetByID)
	g.Post("/inbound-tracking", middleware.RequireAuth, inboundTrackingCreate)
	g.Delete("/inbound-tracking/:id", middleware.RequireAuth, inboundTrackingDelete)
}

func inboundTrackingList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	list, err := db.Queries().ListInboundTrackingByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list inbound tracking"))
	}
	// Pass 2.5 MED-08: cap response payload size in Go.
	page, limit, total, slice := paginateInGo(c, len(list))
	list = list[slice.start:slice.end]
	return c.JSON(fiber.Map{
		"data":  list,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

func inboundTrackingGetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	row, err := db.Queries().GetInboundTrackingByID(c.Context(), gen.GetInboundTrackingByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Inbound tracking not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load inbound tracking"))
	}
	return c.JSON(fiber.Map{"data": row})
}

func inboundTrackingCreate(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		Carrier        string  `json:"carrier"`
		TrackingNumber string  `json:"tracking_number"`
		RetailerName   *string `json:"retailer_name"`
		ExpectedItems  *string `json:"expected_items"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Carrier == "" || body.TrackingNumber == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "carrier and tracking_number required"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	_, err := db.Queries().CreateInboundTracking(c.Context(), gen.CreateInboundTrackingParams{
		ID:             id,
		UserID:         userID,
		Carrier:        body.Carrier,
		TrackingNumber: body.TrackingNumber,
		RetailerName:   nullStringPtr(body.RetailerName),
		ExpectedItems:  nullStringPtr(body.ExpectedItems),
		CreatedAt:      now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create inbound tracking"))
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id}})
}

func inboundTrackingDelete(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	err := db.Queries().DeleteInboundTracking(c.Context(), gen.DeleteInboundTrackingParams{ID: id, UserID: userID})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to delete inbound tracking"))
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func nullStringPtr(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

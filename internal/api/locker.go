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

// RegisterLocker mounts locker routes. All require auth.
func RegisterLocker(g fiber.Router) {
	g.Get("/locker/summary", middleware.RequireAuth, lockerSummary)
	g.Get("/locker/:id/service-requests", middleware.RequireAuth, lockerServiceRequestsList)
	g.Post("/locker/:id/photo-request", middleware.RequireAuth, lockerPhotoRequest)
	g.Post("/locker/:id/service-request", middleware.RequireAuth, lockerServiceRequest)
	g.Get("/locker/:id", middleware.RequireAuth, lockerGetByID)
	g.Get("/locker", middleware.RequireAuth, lockerList)
}

func lockerList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	statusFilter := c.Query("status", "")
	list, err := db.Queries().ListLockerPackagesByUser(c.Context(), gen.ListLockerPackagesByUserParams{
		UserID:  userID,
		Column2: statusFilter,
		Status:  statusFilter,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list packages"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func lockerGetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	pkg, err := db.Queries().GetLockerPackageByID(c.Context(), gen.GetLockerPackageByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Package not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package"))
	}
	return c.JSON(fiber.Map{"data": pkg})
}

func lockerSummary(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	row, err := db.Queries().LockerSummaryByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load summary"))
	}
	storedCount := 0
	if row.StoredCount.Valid {
		storedCount = int(row.StoredCount.Float64)
	}
	storedWeight := 0.0
	if w, ok := row.StoredWeight.(float64); ok {
		storedWeight = w
	}
	nextExpiry := ""
	if s, ok := row.NextExpiry.(string); ok {
		nextExpiry = s
	}
	pendingServices := 0
	if row.PendingServices.Valid {
		pendingServices = int(row.PendingServices.Float64)
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"stored_count":      storedCount,
			"stored_weight_lbs": storedWeight,
			"next_expiry":       nextExpiry,
			"pending_services":  pendingServices,
		},
	})
}

func lockerServiceRequestsList(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	_, err := db.Queries().GetLockerPackageByID(c.Context(), gen.GetLockerPackageByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Package not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package"))
	}
	list, err := db.Queries().ListServiceRequestsByLockerPackageID(c.Context(), gen.ListServiceRequestsByLockerPackageIDParams{
		LockerPackageID: id,
		UserID:          userID,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list service requests"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func lockerPhotoRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	_, err := db.Queries().GetLockerPackageByID(c.Context(), gen.GetLockerPackageByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Package not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package"))
	}
	reqID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Queries().CreateServiceRequest(c.Context(), gen.CreateServiceRequestParams{
		ID:              reqID,
		UserID:          userID,
		LockerPackageID: id,
		ServiceType:     "photo",
		CreatedAt:       now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create photo request"))
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": reqID, "service_type": "photo"}})
}

func lockerServiceRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	_, err := db.Queries().GetLockerPackageByID(c.Context(), gen.GetLockerPackageByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Package not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package"))
	}
	var body struct {
		ServiceType string `json:"service_type"`
		Notes       string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	// PRD 8.4 value-added service types
	validTypes := map[string]bool{
		"photo_detail": true, "content_inspection": true, "repackage": true,
		"remove_invoice": true, "fragile_wrap": true, "gift_wrap": true,
		"photo": true, "general": true, // legacy
	}
	if body.ServiceType == "" || !validTypes[body.ServiceType] {
		body.ServiceType = "general"
	}
	reqID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	notes := sql.NullString{}
	if body.Notes != "" {
		notes = sql.NullString{String: body.Notes, Valid: true}
	}
	_, err = db.Queries().CreateServiceRequest(c.Context(), gen.CreateServiceRequestParams{
		ID:              reqID,
		UserID:          userID,
		LockerPackageID: id,
		ServiceType:     body.ServiceType,
		Notes:           notes,
		CreatedAt:       now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create service request"))
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": reqID, "service_type": body.ServiceType}})
}

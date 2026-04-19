package api

import (
	"database/sql"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	defaultLockerLimit = 20
	maxLockerLimit     = 100
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

	limit := int64(c.QueryInt("limit", defaultLockerLimit))
	if limit <= 0 {
		limit = defaultLockerLimit
	}
	if limit > maxLockerLimit {
		limit = maxLockerLimit
	}
	page := int64(c.QueryInt("page", 1))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	list, err := db.Queries().ListLockerPackagesByUserPaged(c.Context(), gen.ListLockerPackagesByUserPagedParams{
		UserID:  userID,
		Column2: statusFilter,
		Status:  statusFilter,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list packages"))
	}
	total, err := db.Queries().CountLockerPackagesByUserFiltered(c.Context(), gen.CountLockerPackagesByUserFilteredParams{
		UserID:  userID,
		Column2: statusFilter,
		Status:  statusFilter,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list packages"))
	}
	out := make([]fiber.Map, 0, len(list))
	for _, p := range list {
		out = append(out, fiber.Map{
			"id":                       p.ID,
			"user_id":                  p.UserID,
			"suite_code":               p.SuiteCode,
			"booking_id":               p.BookingID,
			"tracking_inbound":         p.TrackingInbound,
			"carrier_inbound":          p.CarrierInbound,
			"sender_name":              p.SenderName,
			"sender_address":           p.SenderAddress,
			"weight_lbs":               p.WeightLbs,
			"length_in":                p.LengthIn,
			"width_in":                 p.WidthIn,
			"height_in":                p.HeightIn,
			"arrival_photo_url":        p.ArrivalPhotoUrl,
			"condition":                p.Condition,
			"storage_bay":              p.StorageBay,
			"status":                   p.Status,
			"arrived_at":               p.ArrivedAt,
			"free_storage_expires_at":  p.FreeStorageExpiresAt,
			"disposed_at":              p.DisposedAt,
			"created_at":               p.CreatedAt,
			"updated_at":               p.UpdatedAt,
			"pending_service_requests": p.PendingServiceRequests,
		})
	}
	return c.JSON(fiber.Map{
		"data":   out,
		"page":   page,
		"limit":  limit,
		"total":  total,
		"status": statusFilter,
	})
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
	// Pass 2.5 MED-05: cap free-text notes so a caller cannot persist
	// an unbounded blob into the service_requests table.
	if len(body.Notes) > 1000 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "notes must be 1000 characters or fewer"))
	}
	// PRD 8.4 value-added service types
	validTypes := map[string]bool{
		"photo_detail": true, "content_inspection": true, "repackage": true,
		"remove_invoice": true, "fragile_wrap": true, "gift_wrap": true,
		"photo": true, "general": true, // legacy
	}
	if body.ServiceType == "" || !validTypes[body.ServiceType] {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid service_type"))
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

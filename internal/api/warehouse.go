// Package api: warehouse API. PRD §6.10. Staff or admin only under /api/v1/warehouse.

package api

import (
	"database/sql"
	"log"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RegisterWarehouse mounts warehouse routes under /warehouse. All require auth + staff or admin. PRD §6.10.
func RegisterWarehouse(g fiber.Router) {
	wh := g.Group("/warehouse", middleware.RequireAuth, middleware.RequireStaffOrAdmin)
	wh.Get("/stats", warehouseStats)
	wh.Get("/bookings/today", warehouseBookingsToday)
	wh.Post("/packages/receive-from-booking", warehouseReceiveFromBooking)
	wh.Post("/locker-receive", warehouseLockerReceive)
	wh.Get("/service-queue", warehouseServiceQueue)
	wh.Patch("/service-queue/:id", warehouseServiceQueueUpdate)
	wh.Get("/ship-queue", warehouseShipQueue)
	wh.Patch("/ship-queue/:id/process", warehouseShipQueueProcess)
	wh.Patch("/ship-queue/:id/weighed", warehouseShipQueueWeighed)
	wh.Patch("/ship-queue/:id/staged", warehouseShipQueueStaged)
	wh.Get("/bays", warehouseBays)
	wh.Post("/bays/move", warehouseBaysMove)
	wh.Get("/manifests", warehouseManifestsList)
	wh.Post("/manifests", warehouseManifestsCreate)
	wh.Get("/manifests/:id/documents", warehouseManifestsDocuments)
	wh.Get("/exceptions", warehouseExceptions)
	wh.Post("/exceptions/:id/resolve", warehouseExceptionResolve)
	wh.Get("/packages", warehousePackages)
}

func warehouseStats(c *fiber.Ctx) error {
	row, err := db.Queries().WarehouseStats(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load warehouse stats"))
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"locker_packages_count": row.LockerPackagesCount,
			"ship_requests_count":   row.ShipRequestsCount,
			"bookings_count":        row.BookingsCount,
			"service_queue_count":   row.ServiceQueueCount,
			"unmatched_count":       row.UnmatchedCount,
		},
	})
}

func warehouseLockerReceive(c *fiber.Ctx) error {
	var body struct {
		SuiteCode       string   `json:"suite_code"`
		TrackingInbound *string  `json:"tracking_inbound"`
		CarrierInbound  *string  `json:"carrier_inbound"`
		SenderName      *string  `json:"sender_name"`
		WeightLbs       *float64 `json:"weight_lbs"`
		LengthIn        *float64 `json:"length_in"`
		WidthIn         *float64 `json:"width_in"`
		HeightIn        *float64 `json:"height_in"`
		Condition       *string  `json:"condition"`
		StorageBay      *string  `json:"storage_bay"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.SuiteCode == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "suite_code required"))
	}
	user, err := db.Queries().GetUserBySuiteCode(c.Context(), sql.NullString{String: body.SuiteCode, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Suite code not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to look up user"))
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	freeDays := user.FreeStorageDays
	if freeDays <= 0 {
		freeDays = 30
	}
	expiresAt := now.AddDate(0, 0, freeDays).Format(time.RFC3339)
	id := uuid.New().String()
	arg := gen.CreateLockerPackageParams{
		ID:                   id,
		UserID:               user.ID,
		SuiteCode:            body.SuiteCode,
		TrackingInbound:      toNullString(body.TrackingInbound),
		CarrierInbound:       toNullString(body.CarrierInbound),
		SenderName:           toNullString(body.SenderName),
		WeightLbs:            toNullFloat64(body.WeightLbs),
		LengthIn:             toNullFloat64(body.LengthIn),
		WidthIn:              toNullFloat64(body.WidthIn),
		HeightIn:             toNullFloat64(body.HeightIn),
		ArrivalPhotoUrl:      sql.NullString{},
		Condition:            toNullString(body.Condition),
		StorageBay:           toNullString(body.StorageBay),
		ArrivedAt:            sql.NullString{String: nowStr, Valid: true},
		FreeStorageExpiresAt: sql.NullString{String: expiresAt, Valid: true},
		CreatedAt:            nowStr,
		UpdatedAt:            nowStr,
	}
	pkg, err := db.Queries().CreateLockerPackage(c.Context(), arg)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create locker package"))
	}

	// Notify customer
	if user.Email != "" {
		sender := "your package"
		if body.SenderName != nil && *body.SenderName != "" {
			sender = *body.SenderName
		}
		if err := services.SendPackageArrived(user.Email, sender, pkg.WeightLbs.Float64); err != nil {
			log.Printf("[warehouse] package arrival email failed for user %s locker_package %s: %v", user.ID, pkg.ID, err)
		}
	}

	return c.Status(201).JSON(fiber.Map{"data": pkg})
}

func warehouseServiceQueue(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListServiceQueue(c.Context(), gen.ListServiceQueueParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list service queue"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func warehouseServiceQueueUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Status      string  `json:"status"`
		CompletedBy *string `json:"completed_by"`
		Notes       *string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Status == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "status required"))
	}
	sr, err := db.Queries().GetServiceRequestByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Service request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load service request"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	completedBy := sql.NullString{}
	if body.CompletedBy != nil && *body.CompletedBy != "" {
		completedBy = sql.NullString{String: *body.CompletedBy, Valid: true}
	}
	err = db.Queries().UpdateServiceRequestStatus(c.Context(), gen.UpdateServiceRequestStatusParams{
		Status:      body.Status,
		CompletedBy: completedBy,
		CompletedAt: sql.NullString{String: now, Valid: true},
		Price:       sr.Price,
		ID:          id,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update service request"))
	}
	updated, _ := db.Queries().GetServiceRequestByID(c.Context(), id)
	return c.JSON(fiber.Map{"data": updated})
}

func warehouseShipQueue(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListShipQueuePaid(c.Context(), gen.ListShipQueuePaidParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list ship queue"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func warehousePackages(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	status := c.Query("status", "")
	arg := gen.ListWarehousePackagesParams{
		Column1: "",
		Status:  status,
		Limit:   limit,
		Offset:  offset,
	}
	if status != "" {
		arg.Column1 = status
	}
	list, err := db.Queries().ListWarehousePackages(c.Context(), arg)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list warehouse packages"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func warehouseBookingsToday(c *fiber.Ctx) error {
	list, err := db.Queries().ListBookingsToday(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list today's bookings"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func warehouseReceiveFromBooking(c *fiber.Ctx) error {
	var body struct {
		BookingID    string   `json:"booking_id"`
		PackageCount int      `json:"package_count"`
		WeightLbs    *float64 `json:"weight_lbs"`
		StorageBay   *string  `json:"storage_bay"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.BookingID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "booking_id required"))
	}
	booking, err := db.Queries().GetBookingByIDOnly(c.Context(), body.BookingID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Booking not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load booking"))
	}
	user, err := db.Queries().GetUserByID(c.Context(), booking.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	suiteCode := ""
	if user.SuiteCode.Valid {
		suiteCode = user.SuiteCode.String
	}
	if suiteCode == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "User has no suite code"))
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	freeDays := user.FreeStorageDays
	if freeDays <= 0 {
		freeDays = 30
	}
	expiresAt := now.AddDate(0, 0, freeDays).Format(time.RFC3339)
	bookingIDNull := sql.NullString{String: body.BookingID, Valid: true}
	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start transaction"))
	}
	defer tx.Rollback()
	qtx := db.Queries().WithTx(tx)

	// Mark booking as received
	err = qtx.UpdateBookingStatus(c.Context(), gen.UpdateBookingStatusParams{
		Status:    "received",
		UpdatedAt: nowStr,
		ID:        body.BookingID,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update booking status"))
	}

	created := make([]gen.LockerPackage, 0, body.PackageCount)
	for i := 0; i < body.PackageCount; i++ {
		id := uuid.New().String()
		arg := gen.CreateLockerPackageFromBookingParams{
			ID:                   id,
			UserID:               booking.UserID,
			SuiteCode:            suiteCode,
			BookingID:            bookingIDNull,
			WeightLbs:            toNullFloat64(body.WeightLbs),
			StorageBay:           toNullString(body.StorageBay),
			ArrivedAt:            sql.NullString{String: nowStr, Valid: true},
			FreeStorageExpiresAt: sql.NullString{String: expiresAt, Valid: true},
			CreatedAt:            nowStr,
			UpdatedAt:            nowStr,
		}
		pkg, err := qtx.CreateLockerPackageFromBooking(c.Context(), arg)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create locker package"))
		}
		created = append(created, pkg)
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit transaction"))
	}

	// Notify customer
	if user.Email != "" {
		sender := "your booking"
		weight := 0.0
		if body.WeightLbs != nil {
			weight = *body.WeightLbs
		}
		if err := services.SendPackageArrived(user.Email, sender, weight); err != nil {
			log.Printf("[warehouse] booking-receive email failed for booking %s user %s: %v", body.BookingID, user.ID, err)
		}
	}

	return c.Status(201).JSON(fiber.Map{"data": created})
}

func warehouseShipQueueProcess(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	_, err := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	err = db.Queries().AdminUpdateShipRequestStatus(c.Context(), gen.AdminUpdateShipRequestStatusParams{
		Status:    "processing",
		UpdatedAt: now,
		ID:        id,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update status"))
	}
	updated, _ := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	return c.JSON(fiber.Map{"data": updated})
}

func warehouseShipQueueWeighed(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		ConsolidatedWeightLbs *float64 `json:"consolidated_weight_lbs"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.ConsolidatedWeightLbs == nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "consolidated_weight_lbs required"))
	}
	_, err := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	err = db.Queries().UpdateShipRequestConsolidatedWeight(c.Context(), gen.UpdateShipRequestConsolidatedWeightParams{
		ConsolidatedWeightLbs: sql.NullFloat64{Float64: *body.ConsolidatedWeightLbs, Valid: true},
		UpdatedAt:             now,
		ID:                    id,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update weight"))
	}
	updated, _ := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	return c.JSON(fiber.Map{"data": updated})
}

func warehouseShipQueueStaged(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		StagingBay *string `json:"staging_bay"`
		ManifestID *string `json:"manifest_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	_, err := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	arg := gen.UpdateShipRequestStagedParams{
		StagingBay: toNullString(body.StagingBay),
		ManifestID: toNullString(body.ManifestID),
		Status:     "staged",
		UpdatedAt:  now,
		ID:         id,
	}
	err = db.Queries().UpdateShipRequestStaged(c.Context(), arg)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update staged"))
	}
	updated, _ := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	return c.JSON(fiber.Map{"data": updated})
}

func warehouseBays(c *fiber.Ctx) error {
	list, err := db.Queries().ListWarehouseBays(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list bays"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func warehouseBaysMove(c *fiber.Ctx) error {
	var body struct {
		PackageIDs []string `json:"package_ids"`
		BayID      string   `json:"bay_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if len(body.PackageIDs) == 0 || body.BayID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "package_ids and bay_id required"))
	}
	_, err := db.Queries().GetWarehouseBayByID(c.Context(), body.BayID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Bay not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load bay"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	bayIDNull := sql.NullString{String: body.BayID, Valid: true}
	for _, pkgID := range body.PackageIDs {
		_, err := db.Queries().GetLockerPackageByIDOnly(c.Context(), pkgID)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Package not found: "+pkgID))
			}
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package"))
		}
		err = db.Queries().UpdateLockerPackageStorageBay(c.Context(), gen.UpdateLockerPackageStorageBayParams{
			StorageBay: bayIDNull,
			UpdatedAt:  now,
			ID:         pkgID,
		})
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to move package"))
		}
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"moved": len(body.PackageIDs), "bay_id": body.BayID}})
}

func warehouseManifestsList(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListWarehouseManifests(c.Context(), gen.ListWarehouseManifestsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list manifests"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func warehouseManifestsCreate(c *fiber.Ctx) error {
	var body struct {
		DestinationID  string   `json:"destination_id"`
		ShipRequestIDs []string `json:"ship_request_ids"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.DestinationID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "destination_id required"))
	}
	if len(body.ShipRequestIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "ship_request_ids required"))
	}
	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start transaction"))
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("warehouseManifestsCreate rollback failed: %v", rbErr)
		}
	}()
	qtx := db.Queries().WithTx(tx)

	now := time.Now().UTC().Format(time.RFC3339)
	manifestID := uuid.New().String()
	_, err = qtx.CreateWarehouseManifest(c.Context(), gen.CreateWarehouseManifestParams{
		ID:            manifestID,
		DestinationID: body.DestinationID,
		Status:        "draft",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create manifest"))
	}

	manifestIDNull := sql.NullString{String: manifestID, Valid: true}
	for _, srID := range body.ShipRequestIDs {
		if err := qtx.AddShipRequestToManifest(c.Context(), gen.AddShipRequestToManifestParams{
			ManifestID:    manifestID,
			ShipRequestID: srID,
		}); err != nil {
			if err == sql.ErrNoRows {
				return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found: "+srID))
			}
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to add ship request to manifest"))
		}
		// Mark ship request as shipped
		if err := qtx.AdminUpdateShipRequestStatus(c.Context(), gen.AdminUpdateShipRequestStatusParams{
			Status:    "shipped",
			UpdatedAt: now,
			ID:        srID,
		}); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update ship request status"))
		}
		if err := qtx.UpdateShipRequestManifestID(c.Context(), gen.UpdateShipRequestManifestIDParams{
			ManifestID: manifestIDNull,
			UpdatedAt:  now,
			ID:         srID,
		}); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to link manifest to ship request"))
		}

		// Mark locker packages as shipped
		items, err := qtx.ListShipRequestItemsByShipRequestID(c.Context(), srID)
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request items"))
		}
		for _, item := range items {
			if err := qtx.UpdateLockerPackageStatus(c.Context(), gen.UpdateLockerPackageStatusParams{
				Status:    "shipped",
				UpdatedAt: now,
				ID:        item.LockerPackageID,
			}); err != nil {
				return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update locker package status"))
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit transaction"))
	}

	m, err := db.Queries().GetWarehouseManifestByID(c.Context(), manifestID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to reload manifest"))
	}
	return c.Status(201).JSON(fiber.Map{"data": m})
}

func warehouseManifestsDocuments(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	_, err := db.Queries().GetWarehouseManifestByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Manifest not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load manifest"))
	}
	// Stub: return plain text manifest summary (minimal; PDF can be added later).
	doc := "Manifest " + id + "\nGenerated at " + time.Now().UTC().Format(time.RFC3339) + "\n"
	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(doc)
}

func warehouseExceptions(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListUnmatchedPackages(c.Context(), gen.ListUnmatchedPackagesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list exceptions"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func warehouseExceptionResolve(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Resolution *string `json:"resolution"`
		Action     *string `json:"action"`
		UserID     *string `json:"user_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	_, err := db.Queries().GetUnmatchedPackageByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Exception not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load exception"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status := "matched"
	if body.Action != nil && *body.Action != "" {
		switch *body.Action {
		case "match":
			status = "matched"
		case "return":
			status = "returned"
		case "dispose":
			status = "disposed"
		default:
			status = "matched"
		}
	}
	matchedUserID := sql.NullString{}
	if body.UserID != nil && *body.UserID != "" {
		matchedUserID = sql.NullString{String: *body.UserID, Valid: true}
	}
	notes := sql.NullString{}
	if body.Resolution != nil {
		notes = sql.NullString{String: *body.Resolution, Valid: true}
	}
	err = db.Queries().UpdateUnmatchedPackageStatus(c.Context(), gen.UpdateUnmatchedPackageStatusParams{
		Status:          status,
		MatchedUserID:   matchedUserID,
		ResolutionNotes: notes,
		ResolvedAt:      sql.NullString{String: now, Valid: true},
		ID:              id,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to resolve exception"))
	}
	updated, _ := db.Queries().GetUnmatchedPackageByID(c.Context(), id)
	return c.JSON(fiber.Map{"data": updated})
}

func toNullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func toNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

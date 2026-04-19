// Package api: warehouse API. PRD §6.10. Staff or admin only under /api/v1/warehouse.

package api

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Pass 2.5 MED-14 fix: warehouseReceiveFromBooking previously trusted
// body.package_count and pre-allocated + inserted that many locker_package
// rows in a single transaction. A misconfigured or hostile staff client
// could request millions of rows, blowing up memory and locking the DB.
// Cap the per-request fan-out at a sane operational ceiling. 100 is well
// above any real booking (dozens of cartons at most) but bounds worst-case
// allocation and tx duration.
const maxReceiveFromBookingPackages = 100

// Pass 2.5 MED-13 fix: warehouseServiceQueueUpdate previously accepted
// any string as the new status. The allowlist below mirrors the PRD
// service_requests lifecycle (pending|in_progress|completed|cancelled)
// since the schema lacks a CHECK constraint to enforce it at the DB
// layer. Keep this in sync with sql/schema/006_service_requests.sql if
// the PRD ever extends the lifecycle.
func isAllowedServiceRequestStatus(s string) bool {
	switch s {
	case "pending", "in_progress", "completed", "cancelled":
		return true
	default:
		return false
	}
}

// RegisterWarehouse mounts warehouse routes under /warehouse. All require auth + staff or admin. PRD §6.10.
//
// Pass 2 audit fix M-9: warehouse mutations are wrapped with
// IdempotencyMiddleware so a service-worker offline replay carrying a
// repeated Idempotency-Key returns the cached response instead of
// re-executing the handler. This pairs with the SW changes in
// internal/static/sw.js (H-6 + M-9).
func RegisterWarehouse(g fiber.Router) {
	wh := g.Group("/warehouse", middleware.RequireAuth, middleware.RequireStaffOrAdmin, middleware.IdempotencyMiddleware)
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
			// Pass 2.5 LOW-03: de-PII. user.ID + pkg.ID are not strictly
			// PII but are useful pivots if logs leak. The template name
			// + outcome is enough for ops without identifying the row.
			log.Printf("[warehouse] package_arrived email send failed: %v", err)
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
	// Pass 2.5 MED-13: reject statuses outside the lifecycle allowlist
	// so a misbehaving or hostile client cannot wedge a service request
	// into an arbitrary string the rest of the system doesn't recognise.
	if !isAllowedServiceRequestStatus(body.Status) {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "status must be one of: pending, in_progress, completed, cancelled"))
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
	if body.PackageCount < 1 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "package_count must be at least 1"))
	}
	if body.PackageCount > maxReceiveFromBookingPackages {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", fmt.Sprintf("package_count must be %d or fewer", maxReceiveFromBookingPackages)))
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
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("warehouseReceiveFromBooking rollback failed: %v", rbErr)
		}
	}()
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
			// Pass 2.5 LOW-03: de-PII. Drop booking + user IDs from the
			// log line so a leaked log doesn't link a notification
			// failure to a specific customer / booking row.
			log.Printf("[warehouse] booking_receive email send failed: %v", err)
		}
	}

	return c.Status(201).JSON(fiber.Map{"data": created})
}

func warehouseShipQueueProcess(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	// Pass 2.5 HIGH-04: client supplies expected_status so two staff
	// scanning the same package don't both succeed silently. If absent
	// or stale, return 409 and force a refresh.
	var body struct {
		ExpectedStatus string `json:"expected_status"`
	}
	_ = c.BodyParser(&body)
	if strings.TrimSpace(body.ExpectedStatus) == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "expected_status required"))
	}
	_, err := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := db.Queries().AdminUpdateShipRequestStatus(c.Context(), gen.AdminUpdateShipRequestStatusParams{
		Status:    "processing",
		UpdatedAt: now,
		ID:        id,
		Status_2:  body.ExpectedStatus,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update status"))
	}
	if rows == 0 {
		return c.Status(409).JSON(ErrorResponse{}.withCode("CONFLICT", "Ship request is no longer in status "+body.ExpectedStatus+"; refresh and retry"))
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

// warehouseBaysMove relocates one or more locker packages to a different
// staging bay. Pass 2.5 changes:
//
//   - HIGH-04: the request now MUST include `previous_bay`, the source
//     bay the caller saw when it built the move. The UPDATE is gated on
//     that value so two staff scanning the same package cannot both
//     succeed silently — the second writer gets a 409 Conflict.
//   - MED-12: the per-package UPDATEs run inside a single BeginTx so a
//     mid-batch failure (DB error or a single 409) rolls the entire
//     batch back rather than leaving packages split across bays.
//
// Wire-format change for the frontend: the request body fields are now
// {package_ids, to_bay, previous_bay}. The legacy {bay_id} key is no
// longer accepted. The matching JS update lives in
// internal/static/warehouse/scripts/staging.js.
func warehouseBaysMove(c *fiber.Ctx) error {
	var body struct {
		PackageIDs  []string `json:"package_ids"`
		ToBay       string   `json:"to_bay"`
		PreviousBay string   `json:"previous_bay"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	toBay := strings.TrimSpace(body.ToBay)
	prevBay := strings.TrimSpace(body.PreviousBay)
	if toBay == "" || prevBay == "" || len(body.PackageIDs) == 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "package_ids, to_bay, previous_bay required"))
	}

	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start transaction"))
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("warehouseBaysMove rollback: %v", rbErr)
		}
	}()
	qtx := db.Queries().WithTx(tx)

	now := time.Now().UTC().Format(time.RFC3339)
	toBayNull := sql.NullString{String: toBay, Valid: true}
	prevBayNull := sql.NullString{String: prevBay, Valid: true}
	for _, pkgID := range body.PackageIDs {
		rows, err := qtx.UpdateLockerPackageStorageBay(c.Context(), gen.UpdateLockerPackageStorageBayParams{
			StorageBay:   toBayNull,
			UpdatedAt:    now,
			ID:           pkgID,
			StorageBay_2: prevBayNull,
		})
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to move package"))
		}
		if rows == 0 {
			return c.Status(409).JSON(ErrorResponse{}.withCode("CONFLICT", "Package "+pkgID+" is no longer in bay "+prevBay+"; refresh and retry"))
		}
	}
	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit bay move"))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"moved": len(body.PackageIDs), "to_bay": toBay}})
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
		// Pass 2.5 HIGH-04: AdminUpdateShipRequestStatus now requires
		// the expected current status. Manifest creation is the
		// "shipped" transition, which can legitimately come from
		// several prior statuses (paid, processing, staged), so we
		// read the current status inside the same transaction and
		// pass it as the expected value. RowsAffected==0 here would
		// only happen if the row is concurrently mutated between the
		// read and the update, which we surface as a 409.
		current, err := qtx.GetShipRequestByIDOnly(c.Context(), srID)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Ship request not found: "+srID))
			}
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load ship request"))
		}
		rows, err := qtx.AdminUpdateShipRequestStatus(c.Context(), gen.AdminUpdateShipRequestStatusParams{
			Status:    "shipped",
			UpdatedAt: now,
			ID:        srID,
			Status_2:  current.Status,
		})
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update ship request status"))
		}
		if rows == 0 {
			return c.Status(409).JSON(ErrorResponse{}.withCode("CONFLICT", "Ship request "+srID+" was modified concurrently; refresh and retry"))
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
	// Pass 2.5 LOW-04: previously the switch silently coerced any
	// unknown action to "matched", which masked client bugs and let a
	// fuzzed payload quietly resolve an exception. Reject unknown
	// actions explicitly.
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
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "invalid action; must be 'match', 'return', or 'dispose'"))
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

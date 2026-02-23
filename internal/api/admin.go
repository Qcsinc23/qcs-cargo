package api

// Admin API: PRD §6.9, §11. All routes require JWT auth and role "admin" (middleware.RequireAdmin).
// To set a user as admin: UPDATE users SET role = 'admin', updated_at = ? WHERE id = ? (or by email).
// See README "Admin console" for sqlite3 one-liner.

import (
	"database/sql"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

const defaultLimit = 20
const maxLimit = 100

// RegisterAdmin mounts admin routes under /admin. All require auth + admin role. PRD §6.9, §11.
func RegisterAdmin(g fiber.Router) {
	admin := g.Group("/admin", middleware.RequireAuth, middleware.RequireAdmin)
	admin.Get("/dashboard", adminDashboard)
	admin.Get("/search", adminSearch)
	admin.Get("/notifications", adminNotifications)
	admin.Get("/activity", adminActivity)
	admin.Get("/bookings/today", adminBookingsToday)
	admin.Get("/locker-packages", adminLockerPackages)
	admin.Get("/ship-requests", adminShipRequests)
	admin.Patch("/ship-requests/:id/status", adminShipRequestUpdateStatus)
	admin.Get("/service-requests", adminServiceRequests)
	admin.Patch("/service-requests/:id", adminServiceRequestUpdate)
	admin.Get("/unmatched-packages", adminUnmatchedPackages)
	admin.Patch("/unmatched-packages/:id", adminUnmatchedPackageUpdate)
	admin.Get("/bookings", adminBookings)
	admin.Get("/users", adminUsersList)
	admin.Get("/users/:id", adminUserGet)
	admin.Patch("/users/:id", adminUserUpdate)
	admin.Get("/reports/storage", adminStorageReport)
	admin.Get("/reports/revenue", adminReportsRevenue)
	admin.Get("/reports/shipments", adminReportsShipments)
	admin.Get("/reports/customers", adminReportsCustomers)
}

func pagination(c *fiber.Ctx) (limit, offset int64) {
	limit = int64(c.QueryInt("limit", defaultLimit))
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	page := int64(c.QueryInt("page", 1))
	if page < 1 {
		page = 1
	}
	offset = (page - 1) * limit
	return limit, offset
}

func adminDashboard(c *fiber.Ctx) error {
	row, err := db.Queries().AdminDashboardCounts(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load dashboard"))
	}
	pendingShip, err := db.Queries().AdminDashboardPendingShipCount(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load pending shipments count"))
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"locker_packages_count": row.LockerPackagesCount,
			"ship_requests_count":   row.ShipRequestsCount,
			"bookings_count":        row.BookingsCount,
			"service_queue_count":   row.ServiceQueueCount,
			"unmatched_count":       row.UnmatchedCount,
			"pending_ship_count":    pendingShip,
		},
	})
}

// adminStorageReport returns storage report for admin reports UI.
func adminStorageReport(c *fiber.Ctx) error {
	row, err := db.Queries().AdminStorageReport(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load storage report"))
	}
	totalWeight, _ := toFloat64(row.TotalWeight)
	feesToday, _ := toFloat64(row.StorageFeesCollectedToday)
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"total_packages_stored":        row.TotalPackagesStored,
			"total_weight":                 totalWeight,
			"utilization_pct":              0,
			"packages_expiring_soon":       row.PackagesExpiringSoon,
			"storage_fees_collected_today": feesToday,
		},
	})
}

// toFloat64 is used by admin report helpers.
func toFloat64(v interface{}) (float64, bool) {
	if v == nil {
		return 0, true
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	default:
		return 0, false
	}
}

// adminReportsRevenue returns revenue report for admin reports UI.
func adminReportsRevenue(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	rev, err := db.Queries().AdminRevenueReport(c.Context(), gen.AdminRevenueReportParams{
		CreatedAt:   from,
		Column2:     from,
		CreatedAt_2: to,
		Column4:     to,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load revenue report"))
	}
	revenue, _ := toFloat64(rev)
	return c.JSON(fiber.Map{"data": fiber.Map{"revenue": revenue, "from": from, "to": to}})
}

// adminReportsShipments returns shipments count report for admin reports UI.
func adminReportsShipments(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	count, err := db.Queries().AdminShipmentsCountReport(c.Context(), gen.AdminShipmentsCountReportParams{
		CreatedAt:   from,
		Column2:     from,
		CreatedAt_2: to,
		Column4:     to,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load shipments report"))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"count": count, "from": from, "to": to}})
}

// adminReportsCustomers returns customers count for admin reports UI.
func adminReportsCustomers(c *fiber.Ctx) error {
	count, err := db.Queries().AdminCustomersCount(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load customers count"))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"count": count}})
}

func adminSearch(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q", ""))
	if q == "" {
		return c.JSON(fiber.Map{
			"users":           []fiber.Map{},
			"ship_requests":   []fiber.Map{},
			"locker_packages": []fiber.Map{},
		})
	}
	pattern := "%" + q + "%"
	users, _ := db.Queries().AdminSearchUsers(c.Context(), gen.AdminSearchUsersParams{
		Name:      pattern,
		Email:     pattern,
		SuiteCode: sql.NullString{String: pattern, Valid: true},
	})
	srs, _ := db.Queries().AdminSearchShipRequests(c.Context(), pattern)
	pkgs, _ := db.Queries().AdminSearchLockerPackages(c.Context(), gen.AdminSearchLockerPackagesParams{
		SuiteCode:  pattern,
		SenderName: sql.NullString{String: pattern, Valid: true},
	})
	userMaps := make([]fiber.Map, 0, len(users))
	for _, u := range users {
		userMaps = append(userMaps, userToMap(u))
	}
	srMaps := make([]fiber.Map, 0, len(srs))
	for _, row := range srs {
		sr := gen.ShipRequest{
			ID: row.ID, UserID: row.UserID, ConfirmationCode: row.ConfirmationCode, Status: row.Status,
			DestinationID: row.DestinationID, RecipientID: row.RecipientID, ServiceType: row.ServiceType,
			Consolidate: row.Consolidate, SpecialInstructions: row.SpecialInstructions,
			Subtotal: row.Subtotal, ServiceFees: row.ServiceFees, Insurance: row.Insurance, Discount: row.Discount, Total: row.Total,
			PaymentStatus: row.PaymentStatus, StripePaymentIntentID: row.StripePaymentIntentID, CustomsStatus: row.CustomsStatus,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		srMaps = append(srMaps, shipRequestToMap(sr))
	}
	pkgMaps := make([]fiber.Map, 0, len(pkgs))
	for _, row := range pkgs {
		p := gen.LockerPackage{
			ID: row.ID, UserID: row.UserID, SuiteCode: row.SuiteCode,
			TrackingInbound: row.TrackingInbound, CarrierInbound: row.CarrierInbound,
			SenderName: row.SenderName, SenderAddress: row.SenderAddress,
			WeightLbs: row.WeightLbs, LengthIn: row.LengthIn, WidthIn: row.WidthIn, HeightIn: row.HeightIn,
			ArrivalPhotoUrl: row.ArrivalPhotoUrl, Condition: row.Condition, StorageBay: row.StorageBay,
			Status: row.Status, ArrivedAt: row.ArrivedAt, FreeStorageExpiresAt: row.FreeStorageExpiresAt,
			DisposedAt: row.DisposedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		pkgMaps = append(pkgMaps, lockerPackageToMap(p))
	}
	return c.JSON(fiber.Map{
		"users":           userMaps,
		"ship_requests":   srMaps,
		"locker_packages": pkgMaps,
	})
}

func shipRequestToMap(sr gen.ShipRequest) fiber.Map {
	paymentStatus, piID, customs, recip := "", "", "", ""
	if sr.PaymentStatus.Valid {
		paymentStatus = sr.PaymentStatus.String
	}
	if sr.StripePaymentIntentID.Valid {
		piID = sr.StripePaymentIntentID.String
	}
	if sr.CustomsStatus.Valid {
		customs = sr.CustomsStatus.String
	}
	if sr.RecipientID.Valid {
		recip = sr.RecipientID.String
	}
	return fiber.Map{
		"id":                       sr.ID,
		"user_id":                  sr.UserID,
		"confirmation_code":        sr.ConfirmationCode,
		"status":                   sr.Status,
		"destination_id":           sr.DestinationID,
		"recipient_id":             recip,
		"service_type":             sr.ServiceType,
		"payment_status":           paymentStatus,
		"stripe_payment_intent_id": piID,
		"customs_status":           customs,
		"created_at":               sr.CreatedAt,
		"updated_at":               sr.UpdatedAt,
	}
}

func lockerPackageToMap(p gen.LockerPackage) fiber.Map {
	senderName, senderAddr, tracking, carrier := "", "", "", ""
	if p.SenderName.Valid {
		senderName = p.SenderName.String
	}
	if p.SenderAddress.Valid {
		senderAddr = p.SenderAddress.String
	}
	if p.TrackingInbound.Valid {
		tracking = p.TrackingInbound.String
	}
	if p.CarrierInbound.Valid {
		carrier = p.CarrierInbound.String
	}
	return fiber.Map{
		"id":               p.ID,
		"user_id":          p.UserID,
		"suite_code":       p.SuiteCode,
		"tracking_inbound": tracking,
		"carrier_inbound":  carrier,
		"sender_name":      senderName,
		"sender_address":   senderAddr,
		"status":           p.Status,
		"arrived_at":       p.ArrivedAt,
		"created_at":       p.CreatedAt,
		"updated_at":       p.UpdatedAt,
	}
}

func adminNotifications(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"data": []fiber.Map{}})
}

func adminLockerPackages(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	customerID := c.Query("customer_id", "")
	suiteCode := c.Query("suite_code", "")
	status := c.Query("status", "")
	arg := gen.AdminListLockerPackagesParams{
		Column1:   "",
		UserID:    customerID,
		Column3:   "",
		SuiteCode: suiteCode,
		Column5:   "",
		Status:    status,
		Limit:     limit,
		Offset:    offset,
	}
	if customerID != "" {
		arg.Column1 = customerID
	}
	if suiteCode != "" {
		arg.Column3 = suiteCode
	}
	if status != "" {
		arg.Column5 = status
	}
	list, err := db.Queries().AdminListLockerPackages(c.Context(), arg)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list locker packages"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func adminShipRequests(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	status := c.Query("status", "")
	arg := gen.AdminListShipRequestsParams{
		Column1: "",
		Status:  status,
		Limit:   limit,
		Offset:  offset,
	}
	if status != "" {
		arg.Column1 = status
	}
	list, err := db.Queries().AdminListShipRequests(c.Context(), arg)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list ship requests"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func adminShipRequestUpdateStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil || body.Status == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "status required"))
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
		Status:    body.Status,
		UpdatedAt: now,
		ID:        id,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update status"))
	}
	sr, _ := db.Queries().GetShipRequestByIDOnly(c.Context(), id)
	return c.JSON(fiber.Map{"data": sr})
}

func adminServiceRequests(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListServiceRequests(c.Context(), gen.ListServiceRequestsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list service requests"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func adminServiceRequestUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Action string `json:"action"` // "complete" or "cancel"
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	sr, err := db.Queries().GetServiceRequestByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Service request not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load service request"))
	}
	adminID := c.Locals(middleware.CtxUserID).(string)
	now := time.Now().UTC().Format(time.RFC3339)
	completedBy := sql.NullString{}
	completedAt := sql.NullString{}
	var status string
	switch body.Action {
	case "complete":
		status = "completed"
		completedBy = sql.NullString{String: adminID, Valid: true}
		completedAt = sql.NullString{String: now, Valid: true}
	case "cancel":
		status = "cancelled"
	default:
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "action must be complete or cancel"))
	}
	err = db.Queries().UpdateServiceRequestStatus(c.Context(), gen.UpdateServiceRequestStatusParams{
		Status:      status,
		CompletedBy: completedBy,
		CompletedAt: completedAt,
		Price:       sr.Price,
		ID:          id,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update service request"))
	}
	updated, _ := db.Queries().GetServiceRequestByID(c.Context(), id)
	return c.JSON(fiber.Map{"data": updated})
}

func adminUnmatchedPackages(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListUnmatchedPackages(c.Context(), gen.ListUnmatchedPackagesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list unmatched packages"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func adminUnmatchedPackageUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Action string  `json:"action"`  // "match", "return", "dispose"
		UserID *string `json:"user_id"` // for match
		Notes  *string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	_, err := db.Queries().GetUnmatchedPackageByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Unmatched package not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load package"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var status string
	matchedUserID := sql.NullString{}
	resolutionNotes := sql.NullString{}
	if body.UserID != nil && *body.UserID != "" {
		matchedUserID = sql.NullString{String: *body.UserID, Valid: true}
	}
	if body.Notes != nil {
		resolutionNotes = sql.NullString{String: *body.Notes, Valid: true}
	}
	switch body.Action {
	case "match":
		status = "matched"
	case "return":
		status = "returned"
	case "dispose":
		status = "disposed"
	default:
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "action must be match, return, or dispose"))
	}
	resolvedAt := sql.NullString{String: now, Valid: true}
	err = db.Queries().UpdateUnmatchedPackageStatus(c.Context(), gen.UpdateUnmatchedPackageStatusParams{
		Status:          status,
		MatchedUserID:   matchedUserID,
		ResolutionNotes: resolutionNotes,
		ResolvedAt:      resolvedAt,
		ID:              id,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update unmatched package"))
	}
	updated, _ := db.Queries().GetUnmatchedPackageByID(c.Context(), id)
	return c.JSON(fiber.Map{"data": updated})
}

func adminBookingsToday(c *fiber.Ctx) error {
	today := time.Now().UTC().Format("2006-01-02")
	list, err := db.Queries().AdminListBookingsToday(c.Context(), today)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list today's bookings"))
	}
	out := make([]fiber.Map, 0, len(list))
	for _, b := range list {
		out = append(out, bookingToMap(b))
	}
	return c.JSON(fiber.Map{"data": out})
}

func bookingToMap(b gen.Booking) fiber.Map {
	recip, payStatus, stripeID := "", "", ""
	if b.RecipientID.Valid {
		recip = b.RecipientID.String
	}
	if b.PaymentStatus.Valid {
		payStatus = b.PaymentStatus.String
	}
	if b.StripePaymentIntentID.Valid {
		stripeID = b.StripePaymentIntentID.String
	}
	return fiber.Map{
		"id":                       b.ID,
		"user_id":                  b.UserID,
		"confirmation_code":        b.ConfirmationCode,
		"status":                   b.Status,
		"service_type":             b.ServiceType,
		"recipient_id":             recip,
		"scheduled_date":           b.ScheduledDate,
		"time_slot":                b.TimeSlot,
		"payment_status":           payStatus,
		"stripe_payment_intent_id": stripeID,
		"created_at":               b.CreatedAt,
	}
}

func adminActivity(c *fiber.Ctx) error {
	limit := int64(c.QueryInt("limit", 50))
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	list, err := db.Queries().ListAdminActivity(c.Context(), limit)
	if err != nil {
		return c.JSON(fiber.Map{"data": []fiber.Map{}})
	}
	out := make([]fiber.Map, 0, len(list))
	for _, a := range list {
		entityID, details := "", ""
		if a.EntityID.Valid {
			entityID = a.EntityID.String
		}
		if a.Details.Valid {
			details = a.Details.String
		}
		out = append(out, fiber.Map{
			"id":          a.ID,
			"actor_id":    a.ActorID,
			"action":      a.Action,
			"entity_type": a.EntityType,
			"entity_id":   entityID,
			"details":     details,
			"created_at":  a.CreatedAt,
		})
	}
	return c.JSON(fiber.Map{"data": out})
}

func adminBookings(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().AdminListBookings(c.Context(), gen.AdminListBookingsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list bookings"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func adminUsersList(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListUsers(c.Context(), gen.ListUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list users"))
	}
	out := make([]fiber.Map, 0, len(list))
	for _, u := range list {
		out = append(out, userToMap(u))
	}
	return c.JSON(fiber.Map{"data": out})
}

func adminUserGet(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	u, err := db.Queries().GetUserByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

func adminUserUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	u, err := db.Queries().GetUserByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	role := u.Role
	status := u.Status
	if body.Role != nil && *body.Role != "" {
		role = *body.Role
	}
	if body.Status != nil && *body.Status != "" {
		status = *body.Status
	}
	if body.Role != nil || body.Status != nil {
		err = db.Queries().UpdateUserRoleAndStatus(c.Context(), gen.UpdateUserRoleAndStatusParams{
			Role:      role,
			Status:    status,
			UpdatedAt: now,
			ID:        id,
		})
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update user"))
		}
		u, _ = db.Queries().GetUserByID(c.Context(), id)
	}
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

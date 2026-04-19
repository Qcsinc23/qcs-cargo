package api

// Admin API: PRD §6.9, §11. All routes require JWT auth and role "admin" (middleware.RequireAdmin).
// To set a user as admin: UPDATE users SET role = 'admin', updated_at = ? WHERE id = ? (or by email).
// See README "Admin console" for sqlite3 one-liner.

import (
	"database/sql"
	"os"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

const defaultLimit = 20
const maxLimit = 100
const defaultInsightsWindowDays = 7
const minInsightsWindowDays = 1
const maxInsightsWindowDays = 90
const defaultInsightsSlowMS = 500
const minInsightsSlowMS = 50
const maxInsightsSlowMS = 10000
const defaultInsightsSlowLimit = 5
const maxInsightsSlowLimit = 20

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
	admin.Get("/destinations", adminDestinationsList)
	admin.Patch("/destinations/:id", adminDestinationUpdate)
	admin.Get("/system-health", adminSystemHealth)
	admin.Get("/insights", adminInsights)
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

// adminStorageReport, toFloat64, adminReportsRevenue,
// adminReportsShipments, adminReportsCustomers moved to
// admin_reports.go in Phase 3.3 (QAL-001).

func adminDestinationsList(c *fiber.Ctx) error {
	list, err := db.Queries().ListDestinationsAdmin(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load destinations"))
	}
	out := make([]fiber.Map, 0, len(list))
	for _, d := range list {
		out = append(out, fiber.Map{
			"id":               d.ID,
			"name":             d.Name,
			"code":             d.Code,
			"capital":          d.Capital,
			"usd_per_lb":       d.UsdPerLb,
			"transit_days_min": d.TransitDaysMin,
			"transit_days_max": d.TransitDaysMax,
			"is_active":        d.IsActive == 1,
			"sort_order":       d.SortOrder,
			"updated_at":       d.UpdatedAt,
		})
	}
	return c.JSON(fiber.Map{"data": out})
}

func adminDestinationUpdate(c *fiber.Ctx) error {
	id := strings.ToLower(strings.TrimSpace(c.Params("id")))
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Name           *string  `json:"name"`
		Code           *string  `json:"code"`
		Capital        *string  `json:"capital"`
		USDPerLb       *float64 `json:"usd_per_lb"`
		TransitDaysMin *int     `json:"transit_days_min"`
		TransitDaysMax *int     `json:"transit_days_max"`
		IsActive       *bool    `json:"is_active"`
		SortOrder      *int     `json:"sort_order"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	current, err := db.Queries().GetActiveDestinationByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			all, listErr := db.Queries().ListDestinationsAdmin(c.Context())
			if listErr != nil {
				return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load destination"))
			}
			var found bool
			for _, d := range all {
				if d.ID == id {
					current = d
					found = true
					break
				}
			}
			if !found {
				return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Destination not found"))
			}
		} else {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load destination"))
		}
	}

	name := current.Name
	if body.Name != nil && strings.TrimSpace(*body.Name) != "" {
		name = strings.TrimSpace(*body.Name)
	}
	code := current.Code
	if body.Code != nil && strings.TrimSpace(*body.Code) != "" {
		code = strings.ToUpper(strings.TrimSpace(*body.Code))
	}
	capital := current.Capital
	if body.Capital != nil && strings.TrimSpace(*body.Capital) != "" {
		capital = strings.TrimSpace(*body.Capital)
	}
	rate := current.UsdPerLb
	if body.USDPerLb != nil {
		rate = *body.USDPerLb
	}
	transitMin := current.TransitDaysMin
	if body.TransitDaysMin != nil {
		transitMin = *body.TransitDaysMin
	}
	transitMax := current.TransitDaysMax
	if body.TransitDaysMax != nil {
		transitMax = *body.TransitDaysMax
	}
	isActive := current.IsActive
	if body.IsActive != nil {
		isActive = boolToInt(*body.IsActive)
	}
	sortOrder := current.SortOrder
	if body.SortOrder != nil {
		sortOrder = *body.SortOrder
	}
	if transitMin <= 0 || transitMax <= 0 || transitMin > transitMax || rate <= 0 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "invalid destination fields"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.Queries().UpdateDestinationAdmin(c.Context(), gen.UpdateDestinationAdminParams{
		Name:           name,
		Code:           code,
		Capital:        capital,
		UsdPerLb:       rate,
		TransitDaysMin: transitMin,
		TransitDaysMax: transitMax,
		IsActive:       isActive,
		SortOrder:      sortOrder,
		UpdatedAt:      now,
		ID:             id,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update destination"))
	}
	invalidateCachedPublicDestinations(c.Context())
	adminID := c.Locals(middleware.CtxUserID).(string)
	recordActivity(c.Context(), adminID, "admin.destination.update", "destination", id, "")
	return c.JSON(fiber.Map{"data": fiber.Map{
		"id":               id,
		"name":             name,
		"code":             code,
		"capital":          capital,
		"usd_per_lb":       rate,
		"transit_days_min": transitMin,
		"transit_days_max": transitMax,
		"is_active":        isActive == 1,
		"sort_order":       sortOrder,
		"updated_at":       now,
	}})
}

// adminSystemHealth returns lightweight operational status for admin monitoring views.
func adminSystemHealth(c *fiber.Ctx) error {
	dbOK := db.Ping() == nil
	stripeConfigured := os.Getenv("STRIPE_SECRET_KEY") != ""
	resendConfigured := os.Getenv("RESEND_API_KEY") != ""

	counts, err := db.Queries().AdminSystemHealthSnapshot(c.Context())
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load system health"))
	}

	status := "operational"
	if !dbOK {
		status = "degraded"
	}

	// DEF-005: surface daily-job liveness so operators can see it in the
	// admin UI without having to scrape Prometheus.
	storageFeeLast := middleware.LastSuccessfulJobRun("storage_fee")
	expiryNotifierLast := middleware.LastSuccessfulJobRun("expiry_notifier")

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"status":             status,
			"db_ok":              dbOK,
			"stripe_configured":  stripeConfigured,
			"resend_configured":  resendConfigured,
			"metrics_endpoint":   "/metrics",
			"users":              counts.UsersCount,
			"locker_packages":    counts.LockerPackagesCount,
			"stored_packages":    counts.StoredPackagesCount,
			"service_queue":      counts.PendingServiceRequestsCount,
			"unmatched_packages": counts.PendingUnmatchedPackagesCount,
			"pending_ship_count": counts.PendingShipRequestsCount,
			"jobs": fiber.Map{
				"storage_fee_last_success_unix":     storageFeeLast,
				"expiry_notifier_last_success_unix": expiryNotifierLast,
			},
			"generated_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// adminInsights returns operational observability summaries for admin dashboards.
func adminInsights(c *fiber.Ctx) error {
	windowDays := c.QueryInt("window_days", c.QueryInt("window", defaultInsightsWindowDays))
	if windowDays < minInsightsWindowDays {
		windowDays = minInsightsWindowDays
	}
	if windowDays > maxInsightsWindowDays {
		windowDays = maxInsightsWindowDays
	}

	slowMS := c.QueryInt("slow_ms", defaultInsightsSlowMS)
	if slowMS < minInsightsSlowMS {
		slowMS = minInsightsSlowMS
	}
	if slowMS > maxInsightsSlowMS {
		slowMS = maxInsightsSlowMS
	}

	slowLimit := c.QueryInt("slow_limit", defaultInsightsSlowLimit)
	if slowLimit <= 0 {
		slowLimit = defaultInsightsSlowLimit
	}
	if slowLimit > maxInsightsSlowLimit {
		slowLimit = maxInsightsSlowLimit
	}

	now := time.Now().UTC()
	since := now.AddDate(0, 0, -windowDays).Format(time.RFC3339)
	until := now.Format(time.RFC3339)

	analytics, err := db.Queries().ObservabilityAnalyticsSummary(c.Context(), gen.ObservabilityAnalyticsSummaryParams{
		CreatedAt:   since,
		CreatedAt_2: until,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load analytics summary"))
	}

	performance, err := db.Queries().ObservabilityPerformanceSummary(c.Context(), gen.ObservabilityPerformanceSummaryParams{
		DurationMs:  sql.NullFloat64{Float64: float64(slowMS), Valid: true},
		CreatedAt:   since,
		CreatedAt_2: until,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load performance summary"))
	}

	topSlowRoutes, err := db.Queries().ObservabilityTopSlowRoutes(c.Context(), gen.ObservabilityTopSlowRoutesParams{
		CreatedAt:   since,
		CreatedAt_2: until,
		Limit:       int64(slowLimit),
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load top slow routes"))
	}

	errorSummary, err := db.Queries().ObservabilityErrorSummary(c.Context(), gen.ObservabilityErrorSummaryParams{
		CreatedAt:   since,
		CreatedAt_2: until,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load error summary"))
	}

	business, err := db.Queries().ObservabilityBusinessMetricsSummary(c.Context(), gen.ObservabilityBusinessMetricsSummaryParams{
		CreatedAt:   since,
		CreatedAt_2: until,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load business summary"))
	}

	topSlow := make([]fiber.Map, 0, len(topSlowRoutes))
	for _, row := range topSlowRoutes {
		if row.AvgDurationMs < float64(slowMS) {
			continue
		}
		topSlow = append(topSlow, fiber.Map{
			"path":            row.Path,
			"method":          row.Method,
			"request_count":   row.RequestCount,
			"avg_duration_ms": row.AvgDurationMs,
			"max_duration_ms": row.MaxDurationMs,
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"window_days":  windowDays,
			"generated_at": time.Now().UTC().Format(time.RFC3339),
			"analytics": fiber.Map{
				"total_events":        analytics.TotalEvents,
				"unique_users":        analytics.UniqueUsers,
				"unique_routes":       analytics.UniqueRoutes,
				"avg_events_per_user": analytics.AvgEventsPerUser,
			},
			"performance": fiber.Map{
				"slow_threshold_ms": slowMS,
				"avg_duration_ms":   performance.AvgDurationMs,
				"max_duration_ms":   performance.MaxDurationMs,
				"slow_requests":     performance.SlowRequests,
				"total_requests":    performance.TotalRequests,
				"top_slow_routes":   topSlow,
			},
			"errors": fiber.Map{
				"total_errors":      errorSummary.TotalErrors,
				"affected_requests": errorSummary.AffectedRequests,
				"server_errors":     errorSummary.ServerErrors,
				"client_errors":     errorSummary.ClientErrors,
			},
			"business": fiber.Map{
				"total_events": business.TotalEvents,
				"unique_users": business.UniqueUsers,
				"total_value":  business.TotalValue,
				"avg_value":    business.AvgValue,
			},
		},
	})
}

func adminSearch(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q", ""))
	limit, offset := pagination(c)
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}

	if q == "" {
		return c.JSON(fiber.Map{
			"users":           []fiber.Map{},
			"ship_requests":   []fiber.Map{},
			"locker_packages": []fiber.Map{},
			"page":            page,
			"limit":           limit,
		})
	}
	pattern := "%" + q + "%"
	users, err := db.Queries().AdminSearchUsers(c.Context(), gen.AdminSearchUsersParams{
		Name:      pattern,
		Email:     pattern,
		SuiteCode: sql.NullString{String: pattern, Valid: true},
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to search users"))
	}
	srs, err := db.Queries().AdminSearchShipRequests(c.Context(), gen.AdminSearchShipRequestsParams{
		ConfirmationCode: pattern,
		Limit:            limit,
		Offset:           offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to search ship requests"))
	}
	pkgs, err := db.Queries().AdminSearchLockerPackages(c.Context(), gen.AdminSearchLockerPackagesParams{
		SuiteCode:  pattern,
		SenderName: sql.NullString{String: pattern, Valid: true},
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to search locker packages"))
	}
	userMaps := make([]fiber.Map, 0, len(users))
	for _, u := range users {
		userMaps = append(userMaps, userToMap(u))
	}
	srMaps := make([]fiber.Map, 0, len(srs))
	for _, row := range srs {
		sr := gen.ShipRequest{
			ID:                    row.ID,
			UserID:                row.UserID,
			ConfirmationCode:      row.ConfirmationCode,
			Status:                row.Status,
			DestinationID:         row.DestinationID,
			RecipientID:           row.RecipientID,
			ServiceType:           row.ServiceType,
			Consolidate:           row.Consolidate,
			SpecialInstructions:   row.SpecialInstructions,
			Subtotal:              row.Subtotal,
			ServiceFees:           row.ServiceFees,
			Insurance:             row.Insurance,
			Discount:              row.Discount,
			Total:                 row.Total,
			PaymentStatus:         row.PaymentStatus,
			StripePaymentIntentID: row.StripePaymentIntentID,
			CustomsStatus:         row.CustomsStatus,
			CreatedAt:             row.CreatedAt,
			UpdatedAt:             row.UpdatedAt,
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
		"page":            page,
		"limit":           limit,
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
	adminID := c.Locals(middleware.CtxUserID).(string)
	recordActivity(c.Context(), adminID, "admin.ship_request.status_update", "ship_request", id, "status="+body.Status)
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
	recordActivity(c.Context(), adminID, "admin.service_request.update", "service_request", id, "action="+body.Action)
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
	adminID := c.Locals(middleware.CtxUserID).(string)
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
	recordActivity(c.Context(), adminID, "admin.unmatched_package.update", "unmatched_package", id, "action="+body.Action)
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

// adminUsersList, adminUserGet, adminUserUpdate, isAllowedUserRole, and
// isAllowedUserStatus moved to admin_users.go in Phase 3.3 (QAL-001).

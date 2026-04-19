package api

// admin_reports.go holds the /admin/reports/* + /admin/reports/storage
// handlers split out from admin.go in Phase 3.3 (QAL-001). Routes
// remain registered by RegisterAdmin in admin.go.

import (
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/gofiber/fiber/v2"
)

// adminStorageReport returns storage report for admin reports UI.
//
// INC-002 (backlog) note: utilization_pct is intentionally omitted from
// the response. The previous implementation returned a hard-coded zero,
// which was misleading — it implied the value was meaningful when no bay
// capacity model exists yet. When warehouse_bays grows a `capacity`
// column the field can return.
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
			"packages_expiring_soon":       row.PackagesExpiringSoon,
			"storage_fees_collected_today": feesToday,
		},
	})
}

// toFloat64 is shared by admin report helpers.
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

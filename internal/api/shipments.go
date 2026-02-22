package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterShipments mounts shipment routes. All require auth.
func RegisterShipments(g fiber.Router) {
	g.Get("/shipments", middleware.RequireAuth, shipmentList)
	g.Get("/shipments/:id", middleware.RequireAuth, shipmentGetByID)
}

func shipmentList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	list, err := db.Queries().ListShipmentsByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list shipments"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func shipmentGetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	s, err := db.Queries().GetShipmentByID(c.Context(), gen.GetShipmentByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Shipment not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load shipment"))
	}
	return c.JSON(fiber.Map{"data": shipmentToMap(s)})
}

func shipmentToMap(s gen.Shipment) fiber.Map {
	m := fiber.Map{
		"id":             s.ID,
		"destination_id": s.DestinationID,
		"status":         s.Status,
		"created_at":     s.CreatedAt,
		"updated_at":     s.UpdatedAt,
	}
	if s.ManifestID.Valid {
		m["manifest_id"] = s.ManifestID.String
	}
	if s.ShipRequestID.Valid {
		m["ship_request_id"] = s.ShipRequestID.String
	}
	if s.TrackingNumber.Valid {
		m["tracking_number"] = s.TrackingNumber.String
	}
	if s.TotalWeight.Valid {
		m["total_weight"] = s.TotalWeight.Float64
	}
	if s.PackageCount.Valid {
		m["package_count"] = s.PackageCount.Int64
	}
	if s.Carrier.Valid {
		m["carrier"] = s.Carrier.String
	}
	if s.EstimatedDelivery.Valid {
		m["estimated_delivery"] = s.EstimatedDelivery.String
	}
	if s.ActualDelivery.Valid {
		m["actual_delivery"] = s.ActualDelivery.String
	}
	return m
}

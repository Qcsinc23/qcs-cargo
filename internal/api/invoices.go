package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterInvoices mounts invoice routes. All require auth. Register :id/pdf before :id so it matches first.
func RegisterInvoices(g fiber.Router) {
	g.Get("/invoices", middleware.RequireAuth, invoiceList)
	g.Get("/invoices/:id/pdf", middleware.RequireAuth, invoicePDF)
	g.Get("/invoices/:id", middleware.RequireAuth, invoiceGetByID)
}

func invoiceList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	list, err := db.Queries().ListInvoicesByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list invoices"))
	}
	return c.JSON(fiber.Map{"data": list})
}

func invoiceGetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	inv, err := db.Queries().GetInvoiceByID(c.Context(), gen.GetInvoiceByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Invoice not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load invoice"))
	}
	items, err := db.Queries().ListInvoiceItemsByInvoiceID(c.Context(), inv.ID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load invoice items"))
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"invoice": invoiceToMap(inv),
			"items":   items,
		},
	})
}

// invoicePDF returns a stub: 404 or placeholder. PRD: download invoice PDF.
func invoicePDF(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	_, err := db.Queries().GetInvoiceByID(c.Context(), gen.GetInvoiceByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Invoice not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load invoice"))
	}
	// Stub: PDF generation not implemented; return 404 so client can show "not available"
	return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_IMPLEMENTED", "Invoice PDF not yet available"))
}

func invoiceToMap(inv gen.Invoice) fiber.Map {
	m := fiber.Map{
		"id":              inv.ID,
		"user_id":         inv.UserID,
		"invoice_number":  inv.InvoiceNumber,
		"subtotal":        inv.Subtotal,
		"tax":             inv.Tax,
		"total":           inv.Total,
		"status":          inv.Status,
		"created_at":      inv.CreatedAt,
	}
	if inv.BookingID.Valid {
		m["booking_id"] = inv.BookingID.String
	}
	if inv.ShipRequestID.Valid {
		m["ship_request_id"] = inv.ShipRequestID.String
	}
	if inv.DueDate.Valid {
		m["due_date"] = inv.DueDate.String
	}
	if inv.PaidAt.Valid {
		m["paid_at"] = inv.PaidAt.String
	}
	if inv.Notes.Valid {
		m["notes"] = inv.Notes.String
	}
	return m
}

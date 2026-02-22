package api

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
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

// invoicePDF generates a simple PDF for the invoice. PRD: GET /invoices/:id/pdf.
func invoicePDF(c *fiber.Ctx) error {
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
	pdfBuf, err := buildInvoicePDF(inv, items)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to generate PDF"))
	}
	safeName := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, inv.InvoiceNumber)
	if safeName == "" {
		safeName = inv.ID
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="invoice-`+safeName+`.pdf"`)
	return c.Send(pdfBuf)
}

func buildInvoicePDF(inv gen.Invoice, items []gen.InvoiceItem) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "Invoice", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Invoice #: "+inv.InvoiceNumber, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Date: "+inv.CreatedAt, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Status: "+inv.Status, "", 1, "L", false, 0, "")
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(80, 6, "Description", "1", 0, "L", false, 0, "")
	pdf.CellFormat(25, 6, "Qty", "1", 0, "R", false, 0, "")
	pdf.CellFormat(35, 6, "Unit Price", "1", 0, "R", false, 0, "")
	pdf.CellFormat(40, 6, "Total", "1", 1, "R", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	for _, it := range items {
		pdf.CellFormat(80, 6, it.Description, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6, formatInt(it.Quantity), "1", 0, "R", false, 0, "")
		pdf.CellFormat(35, 6, formatMoney(it.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(40, 6, formatMoney(it.Total), "1", 1, "R", false, 0, "")
	}
	pdf.Ln(4)
	pdf.CellFormat(140, 6, "Subtotal", "0", 0, "R", false, 0, "")
	pdf.CellFormat(40, 6, formatMoney(inv.Subtotal), "0", 1, "R", false, 0, "")
	pdf.CellFormat(140, 6, "Tax", "0", 0, "R", false, 0, "")
	pdf.CellFormat(40, 6, formatMoney(inv.Tax), "0", 1, "R", false, 0, "")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(140, 6, "Total", "0", 0, "R", false, 0, "")
	pdf.CellFormat(40, 6, formatMoney(inv.Total), "0", 1, "R", false, 0, "")
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	return buf.Bytes(), err
}

func formatInt(n int) string { return fmt.Sprint(n) }

func formatMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
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

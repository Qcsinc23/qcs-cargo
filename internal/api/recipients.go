package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Pass 2.5 MED-02: cap recipients pagination so a malicious or buggy
// caller cannot ask for an unbounded page size and force a large
// PII-bearing response.
const (
	defaultRecipientLimit = 50
	maxRecipientLimit     = 200
)

// Pass 2.5 MED-03 PII length caps for recipient address fields. Mirrors
// the rules used elsewhere (see services.ValidateName for names).
const (
	maxRecipientStreetLen        = 200
	maxRecipientCityLen          = 200
	maxRecipientAptLen           = 50
	maxRecipientPhoneLen         = 30
	maxRecipientDeliveryInstrLen = 500
)

// RegisterRecipients mounts recipient CRUD routes. All require auth.
func RegisterRecipients(g fiber.Router) {
	g.Get("/recipients", middleware.RequireAuth, recipientsList)
	g.Get("/recipients/:id", middleware.RequireAuth, recipientsGetByID)
	g.Post("/recipients", middleware.RequireAuth, recipientsCreate)
	g.Patch("/recipients/:id", middleware.RequireAuth, recipientsUpdate)
	g.Delete("/recipients/:id", middleware.RequireAuth, recipientsDelete)
}

// sanitizeRecipientField trims, length-caps, and rejects HTML
// metacharacters from a generic free-text PII field (street, city, apt,
// delivery_instructions). Pass 2.5 MED-03.
func sanitizeRecipientField(s string, maxLen int, allowAngle bool) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return "", fmt.Errorf("value exceeds max length %d", maxLen)
	}
	if !allowAngle && strings.ContainsAny(s, "<>") {
		return "", errors.New("invalid characters")
	}
	return s, nil
}

// sanitizeRecipientPhone applies a phone-specific length cap and rejects
// control characters. Format validation is delegated to
// services.ValidatePhone for non-empty values. Pass 2.5 MED-03.
func sanitizeRecipientPhone(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > maxRecipientPhoneLen {
		return "", fmt.Errorf("phone exceeds max length %d", maxRecipientPhoneLen)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", errors.New("phone contains invalid characters")
		}
	}
	return s, nil
}

func recipientsList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	// Pass 3 HIGH-07: paginate at the SQL layer using the per-endpoint
	// caps from MED-02 (default 50, max 200) so a malicious caller
	// cannot widen the page beyond what tests assert.
	limit, offset := paginationWithCaps(c, defaultRecipientLimit, maxRecipientLimit)
	page := pageFromOffset(offset, limit)
	total, err := db.Queries().CountRecipientsByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list recipients"))
	}
	list, err := db.Queries().ListRecipientsByUser(c.Context(), gen.ListRecipientsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list recipients"))
	}
	return c.JSON(fiber.Map{
		"data":  list,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}

func recipientsGetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	rec, err := db.Queries().GetRecipientByID(c.Context(), gen.GetRecipientByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Recipient not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load recipient"))
	}
	return c.JSON(fiber.Map{"data": rec})
}

func recipientsCreate(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		Name                 string `json:"name"`
		Phone                string `json:"phone"`
		DestinationID        string `json:"destination_id"`
		Street               string `json:"street"`
		Apt                  string `json:"apt"`
		City                 string `json:"city"`
		DeliveryInstructions string `json:"delivery_instructions"`
		IsDefault            *int   `json:"is_default"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Name == "" || body.DestinationID == "" || body.Street == "" || body.City == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name, destination_id, street, and city required"))
	}
	// Pass 2.5 MED-03: enforce per-field length + character validation
	// on every PII field so we cannot persist HTML/JS that would later
	// render in admin/warehouse UIs.
	cleanName, err := services.ValidateName(body.Name)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	body.Name = cleanName
	cleanStreet, err := sanitizeRecipientField(body.Street, maxRecipientStreetLen, false)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "street: "+err.Error()))
	}
	if cleanStreet == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "street is required"))
	}
	body.Street = cleanStreet
	cleanCity, err := sanitizeRecipientField(body.City, maxRecipientCityLen, false)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "city: "+err.Error()))
	}
	if cleanCity == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "city is required"))
	}
	body.City = cleanCity
	cleanApt, err := sanitizeRecipientField(body.Apt, maxRecipientAptLen, false)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "apt: "+err.Error()))
	}
	body.Apt = cleanApt
	cleanInstr, err := sanitizeRecipientField(body.DeliveryInstructions, maxRecipientDeliveryInstrLen, false)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "delivery_instructions: "+err.Error()))
	}
	body.DeliveryInstructions = cleanInstr
	cleanPhone, err := sanitizeRecipientPhone(body.Phone)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	body.Phone = cleanPhone
	if err := services.ValidatePhone(body.Phone); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	if err := services.ValidateDestination(body.DestinationID); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	isDefault := 0
	if body.IsDefault != nil && *body.IsDefault != 0 {
		isDefault = 1
	}

	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start transaction"))
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("recipientsCreate rollback failed: %v", rbErr)
		}
	}()
	qtx := db.Queries().WithTx(tx)

	if isDefault == 1 {
		if err := qtx.UnsetDefaultRecipients(c.Context(), userID); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to unset existing defaults"))
		}
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = qtx.CreateRecipient(c.Context(), gen.CreateRecipientParams{
		ID:                   id,
		UserID:               userID,
		Name:                 body.Name,
		Phone:                nullString(body.Phone),
		DestinationID:        body.DestinationID,
		Street:               body.Street,
		Apt:                  nullString(body.Apt),
		City:                 body.City,
		DeliveryInstructions: nullString(body.DeliveryInstructions),
		IsDefault:            isDefault,
		UseCount:             0,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create recipient"))
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit transaction"))
	}

	return c.Status(201).JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": id}})
}

func recipientsUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	rec, err := db.Queries().GetRecipientByID(c.Context(), gen.GetRecipientByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Recipient not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load recipient"))
	}
	var body struct {
		Name                 *string `json:"name"`
		Phone                *string `json:"phone"`
		DestinationID        *string `json:"destination_id"`
		Street               *string `json:"street"`
		Apt                  *string `json:"apt"`
		City                 *string `json:"city"`
		DeliveryInstructions *string `json:"delivery_instructions"`
		IsDefault            *int    `json:"is_default"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	// Pass 2.5 MED-03 + MED-04: validate every body field that the
	// caller actually supplied, then verify the *merged* row still has
	// the NOT NULL fields populated (a PATCH must never blank out a
	// required column by sending an empty string).
	name := rec.Name
	if body.Name != nil {
		cleanName, err := services.ValidateName(*body.Name)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
		}
		name = cleanName
	}
	street := rec.Street
	if body.Street != nil {
		clean, err := sanitizeRecipientField(*body.Street, maxRecipientStreetLen, false)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "street: "+err.Error()))
		}
		street = clean
	}
	city := rec.City
	if body.City != nil {
		clean, err := sanitizeRecipientField(*body.City, maxRecipientCityLen, false)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "city: "+err.Error()))
		}
		city = clean
	}
	destID := rec.DestinationID
	if body.DestinationID != nil && *body.DestinationID != "" {
		destID = strings.TrimSpace(*body.DestinationID)
	}
	if err := services.ValidateDestination(destID); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	// Pass 2.5 MED-04: post-merge required-field check. The recipients
	// schema (sql/schema/007_recipients.sql) declares name, street, city
	// and destination_id NOT NULL; if a PATCH cleared any of them by
	// passing "" the row would still satisfy SQLite's affinity but
	// violate our application contract.
	if name == "" || street == "" || city == "" || destID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name, destination_id, street, and city must be non-empty"))
	}
	var phone, apt, deliveryInstructions sql.NullString
	if body.Phone != nil {
		cleanPhone, err := sanitizeRecipientPhone(*body.Phone)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
		}
		if err := services.ValidatePhone(cleanPhone); err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
		}
		phone = nullString(cleanPhone)
	} else {
		phone = rec.Phone
	}
	if body.Apt != nil {
		clean, err := sanitizeRecipientField(*body.Apt, maxRecipientAptLen, false)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "apt: "+err.Error()))
		}
		apt = nullString(clean)
	} else {
		apt = rec.Apt
	}
	if body.DeliveryInstructions != nil {
		clean, err := sanitizeRecipientField(*body.DeliveryInstructions, maxRecipientDeliveryInstrLen, false)
		if err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "delivery_instructions: "+err.Error()))
		}
		deliveryInstructions = nullString(clean)
	} else {
		deliveryInstructions = rec.DeliveryInstructions
	}
	isDefault := rec.IsDefault
	if body.IsDefault != nil {
		if *body.IsDefault != 0 {
			isDefault = 1
		} else {
			isDefault = 0
		}
	}

	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to start transaction"))
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("recipientsUpdate rollback failed: %v", rbErr)
		}
	}()
	qtx := db.Queries().WithTx(tx)

	if isDefault == 1 {
		if err := qtx.UnsetDefaultRecipients(c.Context(), userID); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to unset existing defaults"))
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	updatedID, err := qtx.UpdateRecipient(c.Context(), gen.UpdateRecipientParams{
		Name:                 name,
		Phone:                phone,
		DestinationID:        destID,
		Street:               street,
		Apt:                  apt,
		City:                 city,
		DeliveryInstructions: deliveryInstructions,
		IsDefault:            isDefault,
		UpdatedAt:            now,
		ID:                   id,
		UserID:               userID,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update recipient"))
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to commit transaction"))
	}

	return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"id": updatedID}})
}

func recipientsDelete(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	// Pass 2.5 LOW-01: preflight existence check. Without this the
	// underlying DELETE silently succeeds with 0 rows affected for any
	// id the caller does not own (or that does not exist), which makes
	// the endpoint look like it accepts arbitrary ids and gives 200/OK
	// for no-op deletes. Probe with the existing user-scoped read so a
	// missing/foreign id returns 404.
	if _, err := db.Queries().GetRecipientByID(c.Context(), gen.GetRecipientByIDParams{ID: id, UserID: userID}); err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Recipient not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load recipient"))
	}
	err := db.Queries().DeleteRecipient(c.Context(), gen.DeleteRecipientParams{ID: id, UserID: userID})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to delete recipient"))
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

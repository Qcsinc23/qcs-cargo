package api

import (
	"database/sql"
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

// RegisterRecipients mounts recipient CRUD routes. All require auth.
func RegisterRecipients(g fiber.Router) {
	g.Get("/recipients", middleware.RequireAuth, recipientsList)
	g.Get("/recipients/:id", middleware.RequireAuth, recipientsGetByID)
	g.Post("/recipients", middleware.RequireAuth, recipientsCreate)
	g.Patch("/recipients/:id", middleware.RequireAuth, recipientsUpdate)
	g.Delete("/recipients/:id", middleware.RequireAuth, recipientsDelete)
}

func recipientsList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	list, err := db.Queries().ListRecipientsByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list recipients"))
	}
	return c.JSON(fiber.Map{"data": list})
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
	name := rec.Name
	if body.Name != nil && *body.Name != "" {
		name = *body.Name
	}
	street := rec.Street
	if body.Street != nil {
		street = *body.Street
	}
	city := rec.City
	if body.City != nil {
		city = *body.City
	}
	destID := rec.DestinationID
	if body.DestinationID != nil && *body.DestinationID != "" {
		destID = strings.TrimSpace(*body.DestinationID)
	}
	if err := services.ValidateDestination(destID); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	var phone, apt, deliveryInstructions sql.NullString
	if body.Phone != nil {
		if err := services.ValidatePhone(*body.Phone); err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
		}
		phone = nullString(*body.Phone)
	} else {
		phone = rec.Phone
	}
	if body.Apt != nil {
		apt = nullString(*body.Apt)
	} else {
		apt = rec.Apt
	}
	if body.DeliveryInstructions != nil {
		deliveryInstructions = nullString(*body.DeliveryInstructions)
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

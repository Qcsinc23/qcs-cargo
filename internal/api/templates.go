package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterTemplates mounts template routes. All require auth.
func RegisterTemplates(g fiber.Router) {
	g.Get("/templates", middleware.RequireAuth, templateList)
	g.Post("/templates", middleware.RequireAuth, templateCreate)
	g.Patch("/templates/:id", middleware.RequireAuth, templateUpdate)
	g.Delete("/templates/:id", middleware.RequireAuth, templateDelete)
}

func templateList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	list, err := db.Queries().ListTemplatesByUser(c.Context(), userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list templates"))
	}
	return c.JSON(fiber.Map{"data": list})
}

type createTemplateBody struct {
	Name          string  `json:"name"`
	ServiceType   string  `json:"service_type"`
	DestinationID string  `json:"destination_id"`
	RecipientID   *string `json:"recipient_id"`
}

func templateCreate(c *fiber.Ctx) error {
	var body createTemplateBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Name == "" || body.ServiceType == "" || body.DestinationID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name, service_type, and destination_id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	var recipID sql.NullString
	if body.RecipientID != nil && *body.RecipientID != "" {
		recipID = sql.NullString{String: *body.RecipientID, Valid: true}
	}
	created, err := db.Queries().CreateTemplate(c.Context(), gen.CreateTemplateParams{
		ID:            uuid.New().String(),
		UserID:        userID,
		Name:          body.Name,
		ServiceType:   body.ServiceType,
		DestinationID: body.DestinationID,
		RecipientID:   recipID,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create template"))
	}
	return c.Status(201).JSON(fiber.Map{"data": created})
}

type updateTemplateBody struct {
	Name          *string `json:"name"`
	ServiceType   *string `json:"service_type"`
	DestinationID *string `json:"destination_id"`
	RecipientID   *string `json:"recipient_id"`
}

func templateUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	existing, err := db.Queries().GetTemplateByID(c.Context(), gen.GetTemplateByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Template not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load template"))
	}
	var body updateTemplateBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	name := existing.Name
	if body.Name != nil {
		name = *body.Name
	}
	serviceType := existing.ServiceType
	if body.ServiceType != nil {
		serviceType = *body.ServiceType
	}
	destinationID := existing.DestinationID
	if body.DestinationID != nil {
		destinationID = *body.DestinationID
	}
	var recipID sql.NullString
	if body.RecipientID != nil {
		if *body.RecipientID == "" {
			recipID = sql.NullString{}
		} else {
			recipID = sql.NullString{String: *body.RecipientID, Valid: true}
		}
	} else {
		recipID = existing.RecipientID
	}
	if err := db.Queries().UpdateTemplate(c.Context(), gen.UpdateTemplateParams{
		Name: name, ServiceType: serviceType, DestinationID: destinationID, RecipientID: recipID, ID: id, UserID: userID,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update template"))
	}
	updated, _ := db.Queries().GetTemplateByID(c.Context(), gen.GetTemplateByIDParams{ID: id, UserID: userID})
	return c.JSON(fiber.Map{"data": updated})
}

func templateDelete(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	_, err := db.Queries().GetTemplateByID(c.Context(), gen.GetTemplateByIDParams{ID: id, UserID: userID})
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Template not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load template"))
	}
	if err := db.Queries().DeleteTemplate(c.Context(), gen.DeleteTemplateParams{ID: id, UserID: userID}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to delete template"))
	}
	return c.SendStatus(204)
}

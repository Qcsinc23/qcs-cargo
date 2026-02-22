package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterTemplates mounts template routes. All require auth.
func RegisterTemplates(g fiber.Router) {
	g.Get("/templates", middleware.RequireAuth, templateList)
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

package api

import (
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

// RegisterAccount mounts POST /account/delete. Requires auth.
func RegisterAccount(g fiber.Router) {
	g.Post("/account/delete", middleware.RequireAuth, accountDelete)
}

// accountDelete marks the user as deleted (status = 'deleted'). PRD 2.14.
func accountDelete(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	now := time.Now().UTC().Format(time.RFC3339)
	err := db.Queries().UpdateUserStatus(c.Context(), gen.UpdateUserStatusParams{
		Status:    "deleted",
		UpdatedAt: now,
		ID:        userID,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to delete account"))
	}
	// Optionally revoke all sessions so they are logged out
	_ = db.Queries().DeleteSessionsByUser(c.Context(), userID)
	recordActivity(c.Context(), userID, "auth.account.delete", "user", userID, "status=deleted")
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Account marked for deletion."}})
}

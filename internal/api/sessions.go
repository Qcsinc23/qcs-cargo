package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterSessions mounts GET /sessions, DELETE /sessions/:id, DELETE /sessions. All require auth.
func RegisterSessions(g fiber.Router) {
	g.Get("/sessions", middleware.RequireAuth, sessionsList)
	g.Delete("/sessions/:id", middleware.RequireAuth, sessionsRevokeOne)
	g.Delete("/sessions", middleware.RequireAuth, sessionsRevokeAll)
}

func sessionsList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	now := time.Now().UTC().Format(time.RFC3339)
	list, err := db.Queries().ListSessionsByUser(c.Context(), gen.ListSessionsByUserParams{UserID: userID, ExpiresAt: now})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list sessions"))
	}
	// Convert to JSON-friendly maps (no refresh_token_hash)
	out := make([]fiber.Map, 0, len(list))
	for _, row := range list {
		m := fiber.Map{
			"id":         row.ID,
			"user_id":    row.UserID,
			"expires_at": row.ExpiresAt,
			"created_at": row.CreatedAt,
		}
		if row.IpAddress.Valid {
			m["ip_address"] = row.IpAddress.String
		}
		if row.UserAgent.Valid {
			m["user_agent"] = row.UserAgent.String
		}
		out = append(out, m)
	}
	return c.JSON(fiber.Map{"data": out})
}

func sessionsRevokeOne(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	userID := c.Locals(middleware.CtxUserID).(string)
	// List to verify session belongs to user
	now := time.Now().UTC().Format(time.RFC3339)
	list, err := db.Queries().ListSessionsByUser(c.Context(), gen.ListSessionsByUserParams{UserID: userID, ExpiresAt: now})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list sessions"))
	}
	var found bool
	for _, row := range list {
		if row.ID == id {
			found = true
			break
		}
	}
	if !found {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Session not found"))
	}
	if err := db.Queries().DeleteSession(c.Context(), id); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to revoke session"))
	}
	return c.SendStatus(204)
}

func sessionsRevokeAll(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		KeepSessionID string `json:"keep_session_id"`
	}
	_ = c.BodyParser(&body)
	if body.KeepSessionID != "" {
		if err := db.Queries().DeleteSessionsByUserExcept(c.Context(), gen.DeleteSessionsByUserExceptParams{UserID: userID, ID: body.KeepSessionID}); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to revoke sessions"))
		}
	} else {
		if err := db.Queries().DeleteSessionsByUser(c.Context(), userID); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to revoke sessions"))
		}
	}
	return c.SendStatus(204)
}

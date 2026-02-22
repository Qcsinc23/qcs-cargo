package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
)

// CtxUserID is the key for user ID in Locals.
const CtxUserID = "user_id"
const CtxUserEmail = "user_email"
const CtxUserRole = "user_role"

// RequireAuth parses Bearer token and sets user_id, user_email, user_role in Locals. Returns 401 if missing/invalid.
func RequireAuth(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Authorization required"},
		})
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Invalid authorization header"},
		})
	}
	token := strings.TrimSpace(auth[len(prefix):])
	userID, email, role, err := services.ValidateAccessToken(token)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Invalid or expired token"},
		})
	}
	c.Locals(CtxUserID, userID)
	c.Locals(CtxUserEmail, email)
	c.Locals(CtxUserRole, role)
	return c.Next()
}

package middleware

import (
	"database/sql"
	"log"
	"strings"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
)

// CtxUserID is the key for user ID in Locals.
const CtxUserID = "user_id"
const CtxUserEmail = "user_email"
const CtxUserRole = "user_role"

// RequireAuth parses Bearer token and sets user_id, user_email, user_role in Locals. Returns 401 if missing/invalid.
func RequireAuth(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" {
		log.Printf("[auth] %s %s RequireAuth: no Authorization header", c.Method(), c.Path())
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Authorization required"},
		})
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		log.Printf("[auth] %s %s RequireAuth: invalid Authorization format", c.Method(), c.Path())
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Invalid authorization header"},
		})
	}
	token := strings.TrimSpace(auth[len(prefix):])
	claims, err := services.ValidateAccessTokenClaims(token)
	if err != nil {
		log.Printf("[auth] %s %s RequireAuth: token invalid or expired (%v)", c.Method(), c.Path(), err)
		return c.Status(401).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Invalid or expired token"},
		})
	}

	// Check token revocation blacklist.
	if claims.ID != "" {
		blacklisted, err := services.IsTokenBlacklisted(c.Context(), claims.ID)
		if err != nil {
			log.Printf("[auth] %s %s RequireAuth: blacklist check error (%v)", c.Method(), c.Path(), err)
			return c.Status(503).JSON(fiber.Map{
				"error": fiber.Map{"code": "AUTH_CHECK_UNAVAILABLE", "message": "Authentication temporarily unavailable"},
			})
		}
		if blacklisted {
			return c.Status(401).JSON(fiber.Map{
				"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Token has been revoked"},
			})
		}
	}

	userID := claims.UserID
	user, err := db.Queries().GetUserByID(c.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(401).JSON(fiber.Map{
				"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "User not found"},
			})
		}
		log.Printf("[auth] %s %s RequireAuth: user status lookup failed (%v)", c.Method(), c.Path(), err)
		return c.Status(503).JSON(fiber.Map{
			"error": fiber.Map{"code": "AUTH_CHECK_UNAVAILABLE", "message": "Authentication temporarily unavailable"},
		})
	}
	if !strings.EqualFold(strings.TrimSpace(user.Status), "active") {
		return c.Status(403).JSON(fiber.Map{
			"error": fiber.Map{"code": "ACCOUNT_INACTIVE", "message": "Account is inactive"},
		})
	}

	email := claims.Email
	role := claims.Role
	log.Printf("[auth] %s %s RequireAuth: ok user_id=%s", c.Method(), c.Path(), userID)
	c.Locals(CtxUserID, userID)
	c.Locals(CtxUserEmail, email)
	c.Locals(CtxUserRole, role)
	return c.Next()
}

// RequireAdmin must be used after RequireAuth. Returns 403 if user role is not "admin".
func RequireAdmin(c *fiber.Ctx) error {
	role, _ := c.Locals(CtxUserRole).(string)
	if role != "admin" {
		return c.Status(403).JSON(fiber.Map{
			"error": fiber.Map{"code": "FORBIDDEN", "message": "Admin access required"},
		})
	}
	return c.Next()
}

// RequireStaffOrAdmin must be used after RequireAuth. Returns 403 if user role is not "staff" or "admin".
func RequireStaffOrAdmin(c *fiber.Ctx) error {
	role, _ := c.Locals(CtxUserRole).(string)
	if role != "staff" && role != "admin" {
		return c.Status(403).JSON(fiber.Map{
			"error": fiber.Map{"code": "FORBIDDEN", "message": "Staff or admin access required"},
		})
	}
	return c.Next()
}

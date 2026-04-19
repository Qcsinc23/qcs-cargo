package api

// admin_users.go holds /admin/users{,/:id} handlers split out from
// admin.go in Phase 3.3 (QAL-001). Routes remain registered by
// RegisterAdmin in admin.go; the handler bodies live here so the user
// management surface can be reasoned about in isolation.

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
)

func adminUsersList(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	list, err := db.Queries().ListUsers(c.Context(), gen.ListUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to list users"))
	}
	out := make([]fiber.Map, 0, len(list))
	for _, u := range list {
		out = append(out, userToMap(u))
	}
	return c.JSON(fiber.Map{"data": out})
}

func adminUserGet(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	u, err := db.Queries().GetUserByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

// adminUserUpdate updates an existing user's role and/or status.
//
// Pass 2 audit fixes:
//   - H-8: validate role/status against a closed enum (no typos that silently
//     deactivate accounts), prevent demoting the last remaining admin, and
//     prevent an admin from accidentally locking themselves out.
//   - L-3: write a structured audit-log entry capturing both the old and new
//     values so changes are always reconstructable.
func adminUserUpdate(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	u, err := db.Queries().GetUserByID(c.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	role := strings.ToLower(strings.TrimSpace(u.Role))
	status := strings.ToLower(strings.TrimSpace(u.Status))
	originalRole := role
	originalStatus := status

	if body.Role != nil {
		role = strings.ToLower(strings.TrimSpace(*body.Role))
		if !isAllowedUserRole(role) {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "role must be one of: customer, staff, admin"))
		}
	}
	if body.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*body.Status))
		if !isAllowedUserStatus(status) {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "status must be one of: active, inactive, banned"))
		}
	}
	adminID := c.Locals(middleware.CtxUserID).(string)
	if originalRole == "admin" && role != "admin" {
		// Last-admin guard — count remaining admins after the would-be demotion.
		var remaining int
		if err := db.DB().QueryRowContext(c.Context(), `SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> ? AND status = 'active'`, id).Scan(&remaining); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to validate admin count"))
		}
		if remaining < 1 {
			return c.Status(409).JSON(ErrorResponse{}.withCode("LAST_ADMIN", "cannot demote the last active admin"))
		}
	}
	if id == adminID && (role != "admin" || (status != "" && status != "active")) {
		return c.Status(409).JSON(ErrorResponse{}.withCode("SELF_LOCKOUT", "you cannot demote or deactivate your own admin account"))
	}

	if body.Role != nil || body.Status != nil {
		err = db.Queries().UpdateUserRoleAndStatus(c.Context(), gen.UpdateUserRoleAndStatusParams{
			Role:      role,
			Status:    status,
			UpdatedAt: now,
			ID:        id,
		})
		if err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update user"))
		}
		// If the user was deactivated, also revoke their active sessions so
		// they cannot continue using the app on previously-issued cookies.
		if status != "active" {
			_ = services.InvalidateAllUserSessions(c.Context(), id)
		}
		u, _ = db.Queries().GetUserByID(c.Context(), id)
	}
	detail := fmt.Sprintf("role:%s->%s,status:%s->%s", originalRole, role, originalStatus, status)
	recordActivity(c.Context(), adminID, "admin.user.update", "user", id, detail)
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

func isAllowedUserRole(r string) bool {
	switch r {
	case "customer", "staff", "admin":
		return true
	}
	return false
}

func isAllowedUserStatus(s string) bool {
	switch s {
	case "active", "inactive", "banned":
		return true
	}
	return false
}

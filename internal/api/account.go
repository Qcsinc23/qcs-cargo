package api

import (
	"log"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
)

const deletedUserName = "Deleted User"

// RegisterAccount mounts account lifecycle routes. Requires auth.
func RegisterAccount(g fiber.Router) {
	g.Post("/account/deactivate", middleware.RequireAuth, accountDeactivate)
	g.Post("/account/delete", middleware.RequireAuth, accountDelete)
}

func accountDeactivate(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.DB().BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to deactivate account"))
	}
	defer func() { _ = tx.Rollback() }()

	qtx := db.Queries().WithTx(tx)
	if err := qtx.UpdateUserStatus(c.Context(), gen.UpdateUserStatusParams{
		Status:    "inactive",
		UpdatedAt: now,
		ID:        userID,
	}); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to deactivate account"))
	}
	if err := qtx.DeleteSessionsByUser(c.Context(), userID); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to deactivate account"))
	}
	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to deactivate account"))
	}

	blacklistCurrentAccessToken(c)
	clearRefreshCookie(c)
	recordActivity(c.Context(), userID, "auth.account.deactivate", "user", userID, "status=inactive")
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Account deactivated. You can contact support to reactivate."}})
}

// accountDelete anonymizes user PII and marks account deleted (GDPR-style soft deletion).
//
// Pass 2.5 CRIT-03 fix: the previous implementation only called
// AnonymizeUserForDeletion, which touched only the users row — every
// other PII-bearing table (recipients, locker_packages, ship_requests,
// bookings, customs docs, signatures, photos, etc.) was left intact,
// making the customer-facing "personal data anonymized" message
// materially false. We now delegate to services.AnonymizeUserData,
// which scrubs/deletes all user-scoped PII in a single transaction.
func accountDelete(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)

	if err := services.AnonymizeUserData(c.Context(), userID, deletedUserName, deletedEmailForUser(userID)); err != nil {
		log.Printf("[account delete] AnonymizeUserData user=%s: %v", userID, err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to delete account"))
	}

	blacklistCurrentAccessToken(c)
	clearRefreshCookie(c)
	recordActivity(c.Context(), userID, "auth.account.delete", "user", userID, "status=deleted,anonymized=true")
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Account deleted and personal data anonymized."}})
}

func deletedEmailForUser(userID string) string {
	return "deleted+" + strings.TrimSpace(userID) + "@qcs.invalid"
}

func blacklistCurrentAccessToken(c *fiber.Ctx) {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	claims, err := services.ValidateAccessTokenClaims(token)
	if err != nil {
		return
	}
	if claims.ID == "" || claims.ExpiresAt == nil {
		return
	}
	_ = services.BlacklistToken(c.Context(), claims.ID, claims.ExpiresAt.Time)
}

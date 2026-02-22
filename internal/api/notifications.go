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

// RegisterNotifications mounts GET and PUT /notifications/preferences. All require auth.
func RegisterNotifications(g fiber.Router) {
	g.Get("/notifications/preferences", middleware.RequireAuth, notificationsGetPrefs)
	g.Put("/notifications/preferences", middleware.RequireAuth, notificationsPutPrefs)
}

func notificationsGetPrefs(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	prefs, err := db.Queries().GetNotificationPrefsByUser(c.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(fiber.Map{"data": defaultNotificationPrefsMap(userID)})
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load preferences"))
	}
	return c.JSON(fiber.Map{"data": notificationPrefsToMap(prefs)})
}

func notificationsPutPrefs(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		EmailEnabled     *int    `json:"email_enabled"`
		SmsEnabled       *int    `json:"sms_enabled"`
		PushEnabled      *int    `json:"push_enabled"`
		OnPackageArrived *int    `json:"on_package_arrived"`
		OnStorageExpiry  *int    `json:"on_storage_expiry"`
		OnShipUpdates    *int    `json:"on_ship_updates"`
		OnInboundUpdates *int    `json:"on_inbound_updates"`
		DailyDigest      *string `json:"daily_digest"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	prefs, err := db.Queries().GetNotificationPrefsByUser(c.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			em, sms, push := 1, 0, 1
			pkg, storage, ship, inbound := 1, 1, 1, 1
			digest := "off"
			if body.EmailEnabled != nil {
				em = *body.EmailEnabled
			}
			if body.SmsEnabled != nil {
				sms = *body.SmsEnabled
			}
			if body.PushEnabled != nil {
				push = *body.PushEnabled
			}
			if body.OnPackageArrived != nil {
				pkg = *body.OnPackageArrived
			}
			if body.OnStorageExpiry != nil {
				storage = *body.OnStorageExpiry
			}
			if body.OnShipUpdates != nil {
				ship = *body.OnShipUpdates
			}
			if body.OnInboundUpdates != nil {
				inbound = *body.OnInboundUpdates
			}
			if body.DailyDigest != nil {
				digest = *body.DailyDigest
			}
			_, err = db.Queries().CreateNotificationPrefs(c.Context(), gen.CreateNotificationPrefsParams{
				ID:               uuid.New().String(),
				UserID:           userID,
				EmailEnabled:     em,
				SmsEnabled:       sms,
				PushEnabled:      push,
				OnPackageArrived: pkg,
				OnStorageExpiry:  storage,
				OnShipUpdates:    ship,
				OnInboundUpdates: inbound,
				DailyDigest:      digest,
				CreatedAt:        now,
				UpdatedAt:        now,
			})
			if err != nil {
				return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to save preferences"))
			}
			prefs, _ = db.Queries().GetNotificationPrefsByUser(c.Context(), userID)
			return c.JSON(fiber.Map{"data": notificationPrefsToMap(prefs)})
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load preferences"))
	}
	em, sms, push := prefs.EmailEnabled, prefs.SmsEnabled, prefs.PushEnabled
	pkg, storage, ship, inbound := prefs.OnPackageArrived, prefs.OnStorageExpiry, prefs.OnShipUpdates, prefs.OnInboundUpdates
	digest := prefs.DailyDigest
	if body.EmailEnabled != nil {
		em = *body.EmailEnabled
	}
	if body.SmsEnabled != nil {
		sms = *body.SmsEnabled
	}
	if body.PushEnabled != nil {
		push = *body.PushEnabled
	}
	if body.OnPackageArrived != nil {
		pkg = *body.OnPackageArrived
	}
	if body.OnStorageExpiry != nil {
		storage = *body.OnStorageExpiry
	}
	if body.OnShipUpdates != nil {
		ship = *body.OnShipUpdates
	}
	if body.OnInboundUpdates != nil {
		inbound = *body.OnInboundUpdates
	}
	if body.DailyDigest != nil {
		digest = *body.DailyDigest
	}
	err = db.Queries().UpdateNotificationPrefs(c.Context(), gen.UpdateNotificationPrefsParams{
		EmailEnabled:     em,
		SmsEnabled:       sms,
		PushEnabled:      push,
		OnPackageArrived: pkg,
		OnStorageExpiry:  storage,
		OnShipUpdates:    ship,
		OnInboundUpdates: inbound,
		DailyDigest:      digest,
		UpdatedAt:        now,
		UserID:           userID,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update preferences"))
	}
	prefs, _ = db.Queries().GetNotificationPrefsByUser(c.Context(), userID)
	return c.JSON(fiber.Map{"data": notificationPrefsToMap(prefs)})
}

func notificationPrefsToMap(p gen.NotificationPref) fiber.Map {
	return fiber.Map{
		"id":                 p.ID,
		"user_id":            p.UserID,
		"email_enabled":      p.EmailEnabled != 0,
		"sms_enabled":        p.SmsEnabled != 0,
		"push_enabled":       p.PushEnabled != 0,
		"on_package_arrived": p.OnPackageArrived != 0,
		"on_storage_expiry":  p.OnStorageExpiry != 0,
		"on_ship_updates":    p.OnShipUpdates != 0,
		"on_inbound_updates": p.OnInboundUpdates != 0,
		"daily_digest":       p.DailyDigest,
		"created_at":         p.CreatedAt,
		"updated_at":         p.UpdatedAt,
	}
}

func defaultNotificationPrefsMap(userID string) fiber.Map {
	return fiber.Map{
		"user_id":            userID,
		"email_enabled":      true,
		"sms_enabled":        false,
		"push_enabled":       true,
		"on_package_arrived": true,
		"on_storage_expiry":  true,
		"on_ship_updates":    true,
		"on_inbound_updates": true,
		"daily_digest":       "off",
	}
}

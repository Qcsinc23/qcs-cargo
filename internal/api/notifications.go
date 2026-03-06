package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RegisterNotifications mounts GET and PUT /notifications/preferences. All require auth.
func RegisterNotifications(g fiber.Router) {
	g.Get("/notifications", middleware.RequireAuth, notificationsList)
	g.Post("/notifications/:id/read", middleware.RequireAuth, notificationsMarkRead)
	g.Get("/notifications/preferences", middleware.RequireAuth, notificationsGetPrefs)
	g.Put("/notifications/preferences", middleware.RequireAuth, notificationsPutPrefs)
	g.Get("/notifications/stream", notificationsStream)
	g.Post("/notifications/push/subscribe", middleware.RequireAuth, notificationsPushSubscribe)
}

func authenticateNotificationStream(c *fiber.Ctx) (string, int, string, string) {
	auth := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		claims, err := services.ValidateAccessTokenClaims(token)
		if err != nil {
			return "", 401, "UNAUTHENTICATED", "Invalid or expired token"
		}
		if claims.ID != "" {
			blacklisted, err := services.IsTokenBlacklisted(c.Context(), claims.ID)
			if err != nil {
				return "", 503, "AUTH_CHECK_UNAVAILABLE", "Authentication temporarily unavailable"
			}
			if blacklisted {
				return "", 401, "UNAUTHENTICATED", "Token has been revoked"
			}
		}
		user, err := db.Queries().GetUserByID(c.Context(), claims.UserID)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", 401, "UNAUTHENTICATED", "User not found"
			}
			return "", 503, "AUTH_CHECK_UNAVAILABLE", "Authentication temporarily unavailable"
		}
		if !strings.EqualFold(strings.TrimSpace(user.Status), "active") {
			return "", 403, "ACCOUNT_INACTIVE", "Account is inactive"
		}
		return user.ID, 0, "", ""
	}

	refreshToken := strings.TrimSpace(c.Cookies(refreshCookieName))
	if refreshToken == "" {
		return "", 401, "UNAUTHENTICATED", "Authorization required"
	}
	user, _, err := services.RefreshSession(c.Context(), refreshToken)
	if err != nil {
		if err == services.ErrAccountInactive {
			return "", 403, "ACCOUNT_INACTIVE", "Account is inactive"
		}
		return "", 401, "UNAUTHENTICATED", "Invalid or expired session"
	}
	return user.ID, 0, "", ""
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
	_ = createUserNotification(c.Context(), userID, "Preferences updated", "Your notification preferences were saved.", "info", "/dashboard/settings/notifications")
	return c.JSON(fiber.Map{"data": notificationPrefsToMap(prefs)})
}

func notificationsList(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	if err := ensureNotificationSeed(c.Context(), userID); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load notifications"))
	}
	rows, err := db.DB().QueryContext(c.Context(), `
SELECT id, title, body, level, link_url, read_at, created_at
FROM in_app_notifications
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT 50
`, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load notifications"))
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id, title, body, level, createdAt string
		var linkURL, readAt sql.NullString
		if err := rows.Scan(&id, &title, &body, &level, &linkURL, &readAt, &createdAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to parse notifications"))
		}
		items = append(items, fiber.Map{
			"id":         id,
			"title":      title,
			"body":       body,
			"level":      level,
			"link_url":   nullStringValue(linkURL),
			"read_at":    nullStringValue(readAt),
			"created_at": createdAt,
			"unread":     !readAt.Valid || strings.TrimSpace(readAt.String) == "",
		})
	}
	return c.JSON(fiber.Map{"data": items})
}

func notificationsMarkRead(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.DB().ExecContext(c.Context(), `
UPDATE in_app_notifications
SET read_at = ?
WHERE id = ? AND user_id = ? AND read_at IS NULL
`, now, id, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update notification"))
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Notification not found"))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"id": id, "read_at": now}})
}

func notificationsStream(c *fiber.Ctx) error {
	userID, status, code, message := authenticateNotificationStream(c)
	if status != 0 {
		return c.Status(status).JSON(ErrorResponse{}.withCode(code, message))
	}
	if err := ensureNotificationSeed(c.Context(), userID); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to open notification stream"))
	}
	rows, err := db.DB().QueryContext(c.Context(), `
SELECT id, title, body, level, link_url, read_at, created_at
FROM in_app_notifications
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT 10
`, userID)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to open notification stream"))
	}
	defer rows.Close()
	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id, title, body, level, createdAt string
		var linkURL, readAt sql.NullString
		if err := rows.Scan(&id, &title, &body, &level, &linkURL, &readAt, &createdAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to open notification stream"))
		}
		items = append(items, fiber.Map{
			"id":         id,
			"title":      title,
			"body":       body,
			"level":      level,
			"link_url":   nullStringValue(linkURL),
			"read_at":    nullStringValue(readAt),
			"created_at": createdAt,
		})
	}
	payload, _ := json.Marshal(fiber.Map{
		"type":          "notification_snapshot",
		"notifications": items,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
	})
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set("Connection", "keep-alive")
	return c.SendString(fmt.Sprintf("event: snapshot\ndata: %s\n\n", payload))
}

func notificationsPushSubscribe(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	var body struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	body.Endpoint = strings.TrimSpace(body.Endpoint)
	body.Keys.P256dh = strings.TrimSpace(body.Keys.P256dh)
	body.Keys.Auth = strings.TrimSpace(body.Keys.Auth)
	if body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "endpoint and keys are required"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.DB().ExecContext(c.Context(), `
INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, endpoint) DO UPDATE SET
    p256dh = excluded.p256dh,
    auth = excluded.auth,
    updated_at = excluded.updated_at
`, uuid.NewString(), userID, body.Endpoint, body.Keys.P256dh, body.Keys.Auth, now, now)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to save push subscription"))
	}
	_ = createUserNotification(c.Context(), userID, "Push enabled", "This device is now registered for push notifications.", "info", "/dashboard/settings/notifications")
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"endpoint": body.Endpoint, "updated_at": now}})
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

func ensureNotificationSeed(ctx interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(any) any
}, userID string) error {
	var count int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM in_app_notifications WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return createUserNotification(ctx, userID, "Notification center ready", "Realtime and in-app notifications are now available on your dashboard.", "info", "/dashboard/settings/notifications")
}

func createUserNotification(ctx interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(any) any
}, userID, title, body, level, linkURL string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return nil
	}
	_, err := db.DB().ExecContext(ctx, `
INSERT INTO in_app_notifications (id, user_id, title, body, level, link_url, read_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, NULL, ?)
`, uuid.NewString(), userID, title, body, firstNonEmpty(strings.TrimSpace(level), "info"), nullIfEmpty(strings.TrimSpace(linkURL)), time.Now().UTC().Format(time.RFC3339))
	return err
}

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var platformCache = services.NewCacheFromEnv()

const publicDestinationsCacheKey = "public:destinations:v1"
const publicDestinationsCacheTTL = 5 * time.Minute

// RegisterPlatformOps mounts platform/ops routes.
func RegisterPlatformOps(g fiber.Router) {
	g.Get("/platform/readiness", platformReadiness)
	g.Get("/platform/runtime", platformRuntime)

	admin := g.Group("/admin", middleware.RequireAuth, middleware.RequireAdmin)
	admin.Get("/moderation", adminModerationList)
	admin.Post("/moderation", adminModerationCreate)
	admin.Patch("/moderation/:id", adminModerationUpdate)
}

func platformReadiness(c *fiber.Ctx) error {
	dbOK := db.Ping() == nil
	cacheOK := platformCache.Ping(c.Context()) == nil
	status := "ready"
	if !dbOK {
		status = "degraded"
	}
	if !dbOK || !cacheOK {
		status = "degraded"
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"status":        status,
			"db_ok":         dbOK,
			"cache_ok":      cacheOK,
			"cache_backend": platformCache.Backend(),
			"cdn_base_url":  strings.TrimSpace(os.Getenv("CDN_BASE_URL")),
			"app_url":       strings.TrimSpace(os.Getenv("APP_URL")),
			"ready_at":      time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func platformRuntime(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"cache_backend":        platformCache.Backend(),
			"cdn_base_url":         strings.TrimSpace(os.Getenv("CDN_BASE_URL")),
			"horizontal_scaling":   true,
			"stateless_api":        true,
			"shared_cache_enabled": strings.TrimSpace(os.Getenv("REDIS_URL")) != "",
			"asset_cache_policy":   "public, max-age=31536000, immutable for versioned assets",
		},
	})
}

func adminModerationList(c *fiber.Ctx) error {
	status := strings.TrimSpace(c.Query("status"))
	query := `
SELECT id, resource_type, resource_id, status, notes, created_by, created_at, updated_at
FROM moderation_items
`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := db.DB().QueryContext(c.Context(), query, args...)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load moderation queue"))
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id, resourceType, resourceID, rowStatus, createdAt, updatedAt string
		var notes, createdBy sql.NullString
		if err := rows.Scan(&id, &resourceType, &resourceID, &rowStatus, &notes, &createdBy, &createdAt, &updatedAt); err != nil {
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to parse moderation queue"))
		}
		items = append(items, fiber.Map{
			"id":            id,
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"status":        rowStatus,
			"notes":         nullStringValue(notes),
			"created_by":    nullStringValue(createdBy),
			"created_at":    createdAt,
			"updated_at":    updatedAt,
		})
	}
	return c.JSON(fiber.Map{"data": items})
}

func adminModerationCreate(c *fiber.Ctx) error {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		Notes        string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	body.ResourceType = strings.TrimSpace(body.ResourceType)
	body.ResourceID = strings.TrimSpace(body.ResourceID)
	if body.ResourceType == "" || body.ResourceID == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "resource_type and resource_id required"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.NewString()
	userID := currentUserID(c)
	_, err := db.DB().ExecContext(c.Context(), `
INSERT INTO moderation_items (id, resource_type, resource_id, status, notes, created_by, created_at, updated_at)
VALUES (?, ?, ?, 'pending', ?, ?, ?, ?)
`, id, body.ResourceType, body.ResourceID, nullIfEmpty(strings.TrimSpace(body.Notes)), nullIfEmpty(userID), now, now)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create moderation item"))
	}
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"id": id, "status": "pending"}})
}

func adminModerationUpdate(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "id required"))
	}
	var body struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	body.Status = strings.TrimSpace(body.Status)
	if body.Status == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "status required"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.DB().ExecContext(c.Context(), `
UPDATE moderation_items
SET status = ?, notes = ?, updated_at = ?
WHERE id = ?
`, body.Status, nullIfEmpty(strings.TrimSpace(body.Notes)), now, id)
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update moderation item"))
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "Moderation item not found"))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"id": id, "status": body.Status, "updated_at": now}})
}

func getCachedJSON(ctx context.Context, key string, dest any) bool {
	raw, ok, err := platformCache.Get(ctx, key)
	if err != nil || !ok || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

func setCachedJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = platformCache.Set(ctx, key, raw, ttl)
}

func invalidateCachedPublicDestinations(ctx context.Context) {
	_ = platformCache.Delete(ctx, publicDestinationsCacheKey)
}

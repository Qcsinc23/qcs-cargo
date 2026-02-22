package api

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

const maxAvatarSize = 5 << 20 // 5MB

// RegisterMe mounts PATCH /me (profile update) and POST /me/avatar. GET /me is registered in main.
func RegisterMe(g fiber.Router) {
	g.Patch("/me", middleware.RequireAuth, MeUpdate)
	g.Post("/me/avatar", middleware.RequireAuth, MeAvatarUpload)
}

// MeUpdate handles PATCH /me — update profile (name, phone, address). PRD 2.14.
func MeUpdate(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	u, err := db.Queries().GetUserByID(c.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	var body struct {
		Name          *string `json:"name"`
		Phone         *string `json:"phone"`
		AddressStreet *string `json:"address_street"`
		AddressCity   *string `json:"address_city"`
		AddressState  *string `json:"address_state"`
		AddressZip    *string `json:"address_zip"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	name := u.Name
	if body.Name != nil && *body.Name != "" {
		name = *body.Name
	}
	phone := u.Phone
	if body.Phone != nil {
		phone = sql.NullString{String: *body.Phone, Valid: *body.Phone != ""}
	}
	addrStreet := u.AddressStreet
	if body.AddressStreet != nil {
		addrStreet = sql.NullString{String: *body.AddressStreet, Valid: *body.AddressStreet != ""}
	}
	addrCity := u.AddressCity
	if body.AddressCity != nil {
		addrCity = sql.NullString{String: *body.AddressCity, Valid: *body.AddressCity != ""}
	}
	addrState := u.AddressState
	if body.AddressState != nil {
		addrState = sql.NullString{String: *body.AddressState, Valid: *body.AddressState != ""}
	}
	addrZip := u.AddressZip
	if body.AddressZip != nil {
		addrZip = sql.NullString{String: *body.AddressZip, Valid: *body.AddressZip != ""}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	err = db.Queries().UpdateUserProfile(c.Context(), gen.UpdateUserProfileParams{
		Name:          name,
		Phone:         phone,
		AddressStreet: addrStreet,
		AddressCity:   addrCity,
		AddressState:  addrState,
		AddressZip:    addrZip,
		UpdatedAt:     now,
		ID:            userID,
	})
	if err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update profile"))
	}
	u, _ = db.Queries().GetUserByID(c.Context(), userID)
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

// MeAvatarUpload handles POST /me/avatar — multipart file (image, max 5MB). Saves to UPLOAD_DIR/avatars, updates user.avatar_url. PRD 6.6.
func MeAvatarUpload(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID).(string)
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "file required"))
	}
	if file.Size > maxAvatarSize {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "file too large (max 5MB)"))
	}
	ct := file.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "image/") {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "only image/* allowed"))
	}
	ext := imageExtFromContentType(ct)
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	avatarsDir := filepath.Join(uploadDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to create upload directory"))
	}
	filename := userID + "_" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339), ":", "-") + ext
	savePath := filepath.Join(avatarsDir, filename)
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to save file"))
	}
	avatarURL := "/uploads/avatars/" + filename
	now := time.Now().UTC().Format(time.RFC3339)
	err = db.Queries().UpdateUserAvatar(c.Context(), gen.UpdateUserAvatarParams{
		AvatarUrl: sql.NullString{String: avatarURL, Valid: true},
		UpdatedAt: now,
		ID:        userID,
	})
	if err != nil {
		_ = os.Remove(savePath)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update profile"))
	}
	u, _ := db.Queries().GetUserByID(c.Context(), userID)
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

func imageExtFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	default:
		return ".jpg"
	}
}

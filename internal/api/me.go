package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
)

// RegisterMe mounts PATCH /me (profile update). GET /me is registered in main.
func RegisterMe(g fiber.Router) {
	g.Patch("/me", middleware.RequireAuth, MeUpdate)
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

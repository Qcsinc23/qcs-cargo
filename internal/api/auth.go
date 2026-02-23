package api

import (
	"database/sql"
	"log"
	"os"
	"strings"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
)

const refreshCookieName = "qcs_refresh"
const refreshCookieMaxAge = 7 * 24 * 3600 // 7 days in seconds

// RegisterAuth mounts auth routes on the given group. Pass the same group to RegisterMe with auth middleware applied.
func RegisterAuth(g fiber.Router) {
	g.Post("/auth/register", authRegister)
	g.Post("/auth/magic-link/request", authMagicLinkRequest)
	g.Post("/auth/magic-link/verify", authMagicLinkVerify)
	g.Post("/auth/refresh", authRefresh)
	g.Post("/auth/logout", authLogout)
	// Password reset (PRD 6.1)
	g.Post("/auth/password/forgot", authForgotPassword)
	g.Post("/auth/password/reset", authResetPassword)
	g.Patch("/auth/password/change", middleware.RequireAuth, authPasswordChange)
}

// authPasswordChange is PATCH /auth/password/change (authenticated). PRD 6.1.
func authPasswordChange(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID)
	if userID == nil {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Not authenticated"))
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.NewPassword == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "new_password required"))
	}
	if err := services.ChangePassword(c.Context(), userID.(string), body.CurrentPassword, body.NewPassword); err != nil {
		if err.Error() == "current password is incorrect" || err.Error() == "current password required" {
			return c.Status(400).JSON(ErrorResponse{}.withCode("INVALID_PASSWORD", err.Error()))
		}
		if err.Error() == "password must be at least 8 characters" {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to update password"))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Password updated."}})
}

func authRegister(c *fiber.Ctx) error {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Name == "" || body.Email == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name and email required"))
	}
	user, err := services.Register(c.Context(), body.Name, body.Email, body.Phone, body.Password)
	if err != nil {
		if err.Error() == "email already registered" {
			return c.Status(409).JSON(ErrorResponse{}.withCode("EMAIL_EXISTS", err.Error()))
		}
		log.Printf("auth register: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Registration failed"))
	}
	return c.Status(201).JSON(fiber.Map{
		"data": userToMap(user),
	})
}

func authMagicLinkRequest(c *fiber.Ctx) error {
	var body struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		RedirectTo string `json:"redirectTo"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Email == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "email required"))
	}
	// Always return the same response regardless of whether the email exists.
	// This prevents user enumeration (PRD 3.2.1).
	const enumSafeMsg = "If an account with that email exists, you will receive a sign-in link shortly."
	q := db.Queries()
	user, err := q.GetUserByEmail(c.Context(), body.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// Do not reveal non-existence — return 200 with same message.
			return c.JSON(fiber.Map{"data": fiber.Map{"message": enumSafeMsg}})
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}
	rawToken, err := services.RequestMagicLink(c.Context(), user.ID, body.RedirectTo)
	if err != nil {
		log.Printf("magic link request: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	link := appURL + "/verify?token=" + rawToken
	if body.RedirectTo != "" {
		link += "&redirectTo=" + body.RedirectTo
	}
	if os.Getenv("RESEND_API_KEY") != "" {
		if err := services.SendMagicLink(body.Email, link); err != nil {
			log.Printf("[Resend] magic link email send failed for %s: %v", body.Email, err)
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
		}
	} else {
		log.Printf("[Resend] not configured — magic link not sent. For %s use link (valid 10 min): %s", body.Email, link)
	}
	data := fiber.Map{"message": enumSafeMsg}
	// In local dev, expose link on login page so you don't need to check server logs
	if os.Getenv("APP_ENV") == "dev" || appURL == "http://localhost:8080" {
		data["magic_link"] = link
	}
	return c.JSON(fiber.Map{"data": data})
}

// authForgotPassword requests a password-reset token. Always returns 200 (no enumeration).
func authForgotPassword(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil || body.Email == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "email required"))
	}
	const msg = "If an account with that email exists, you will receive a password reset link."
	q := db.Queries()
	user, err := q.GetUserByEmail(c.Context(), body.Email)
	if err != nil {
		// Return 200 regardless — prevent enumeration
		return c.JSON(fiber.Map{"data": fiber.Map{"message": msg}})
	}
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	_, link, err := services.RequestPasswordReset(c.Context(), user.ID, appURL)
	if err != nil {
		log.Printf("forgot password: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}
	if os.Getenv("RESEND_API_KEY") != "" {
		if err := services.SendPasswordResetLink(body.Email, link); err != nil {
			log.Printf("password reset email send: %v", err)
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
		}
	} else {
		log.Printf("[DEV] Password reset link for %s: %s", body.Email, link)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"message": msg}})
}

// authResetPassword consumes a reset token and updates the user's password.
func authResetPassword(c *fiber.Ctx) error {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Token == "" || body.Password == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "token and password required"))
	}
	if len(body.Password) < 8 {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "password must be at least 8 characters"))
	}
	if err := services.ResetPassword(c.Context(), body.Token, body.Password); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("INVALID_TOKEN", err.Error()))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Password updated. You can now sign in."}})
}

func authMagicLinkVerify(c *fiber.Ctx) error {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "token required"))
	}
	user, accessToken, refreshToken, err := services.VerifyMagicLink(c.Context(), body.Token)
	if err != nil {
		log.Printf("[auth] magic-link verify failed: %v", err)
		return c.Status(401).JSON(ErrorResponse{}.withCode("INVALID_LINK", err.Error()))
	}
	log.Printf("[auth] magic-link verify success user_id=%s", user.ID)
	setRefreshCookie(c, refreshToken)
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"user":         userToMap(user),
			"access_token": accessToken,
			"expires_in":   int(services.AccessExpiry.Seconds()),
		},
	})
}

func authRefresh(c *fiber.Ctx) error {
	refreshToken := c.Cookies(refreshCookieName)
	if refreshToken == "" {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "No refresh token"))
	}
	user, accessToken, err := services.RefreshSession(c.Context(), refreshToken)
	if err != nil {
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", err.Error()))
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"user":         userToMap(user),
			"access_token": accessToken,
			"expires_in":   int(services.AccessExpiry.Seconds()),
		},
	})
}

func authLogout(c *fiber.Ctx) error {
	refreshToken := c.Cookies(refreshCookieName)
	_ = services.Logout(c.Context(), refreshToken)
	clearRefreshCookie(c)
	return c.SendStatus(204)
}

func setRefreshCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   refreshCookieMaxAge,
		HTTPOnly: true,
		Secure:   os.Getenv("APP_URL") != "" && strings.HasPrefix(os.Getenv("APP_URL"), "https"),
		SameSite: "Lax",
	})
}

func clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
	})
}

// Me returns the current user. Use with middleware.RequireAuth.
func Me(c *fiber.Ctx) error {
	userID := c.Locals(middleware.CtxUserID)
	if userID == nil {
		log.Printf("[auth] GET /me: no user_id in context")
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Not authenticated"))
	}
	u, err := db.Queries().GetUserByID(c.Context(), userID.(string))
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[auth] GET /me: user not found id=%s", userID)
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		log.Printf("[auth] GET /me: db error %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	log.Printf("[auth] GET /me: success user_id=%s", u.ID)
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

func userToMap(u gen.User) fiber.Map {
	suiteCode := ""
	if u.SuiteCode.Valid {
		suiteCode = u.SuiteCode.String
	}
	phone := ""
	if u.Phone.Valid {
		phone = u.Phone.String
	}
	addressStreet, addressCity, addressState, addressZip := "", "", "", ""
	if u.AddressStreet.Valid {
		addressStreet = u.AddressStreet.String
	}
	if u.AddressCity.Valid {
		addressCity = u.AddressCity.String
	}
	if u.AddressState.Valid {
		addressState = u.AddressState.String
	}
	if u.AddressZip.Valid {
		addressZip = u.AddressZip.String
	}
	avatarURL := ""
	if u.AvatarUrl.Valid {
		avatarURL = u.AvatarUrl.String
	}
	return fiber.Map{
		"id":                u.ID,
		"name":              u.Name,
		"email":             u.Email,
		"phone":             phone,
		"role":              u.Role,
		"avatar_url":        avatarURL,
		"suite_code":        suiteCode,
		"address_street":    addressStreet,
		"address_city":      addressCity,
		"address_state":     addressState,
		"address_zip":       addressZip,
		"storage_plan":      u.StoragePlan,
		"free_storage_days": u.FreeStorageDays,
		"email_verified":    u.EmailVerified != 0,
		"status":            u.Status,
		"created_at":        u.CreatedAt,
		"updated_at":        u.UpdatedAt,
	}
}

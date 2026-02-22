package api

import (
	"database/sql"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
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
	// Password change requires auth — registered in main.go with RequireAuth middleware
}

func authRegister(c *fiber.Ctx) error {
	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body"))
	}
	if body.Name == "" || body.Email == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name and email required"))
	}
	user, err := services.Register(c.Context(), body.Name, body.Email)
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
	// TODO: send email via Resend with link containing rawToken. For now log it.
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:8080"
	}
	link := appURL + "/verify?token=" + rawToken
	if body.RedirectTo != "" {
		link += "&redirectTo=" + body.RedirectTo
	}
	log.Printf("[DEV] Magic link for %s: %s", body.Email, link)
	return c.JSON(fiber.Map{"data": fiber.Map{"message": enumSafeMsg}})
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
	rawToken, link, err := services.RequestPasswordReset(c.Context(), user.ID, appURL)
	if err != nil {
		log.Printf("forgot password: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}
	log.Printf("[DEV] Password reset link for %s: %s (token=%s)", body.Email, link, rawToken)
	// TODO: send via Resend when RESEND_API_KEY is set
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
		return c.Status(401).JSON(ErrorResponse{}.withCode("INVALID_LINK", err.Error()))
	}
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
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", "Not authenticated"))
	}
	u, err := db.Queries().GetUserByID(c.Context(), userID.(string))
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(ErrorResponse{}.withCode("NOT_FOUND", "User not found"))
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Failed to load user"))
	}
	return c.JSON(fiber.Map{"data": userToMap(u)})
}

func userToMap(u gen.User) fiber.Map {
	suiteCode := ""
	if u.SuiteCode.Valid {
		suiteCode = u.SuiteCode.String
	}
	return fiber.Map{
		"id":                u.ID,
		"name":              u.Name,
		"email":             u.Email,
		"role":              u.Role,
		"suite_code":        suiteCode,
		"storage_plan":      u.StoragePlan,
		"free_storage_days": u.FreeStorageDays,
		"email_verified":    u.EmailVerified != 0,
		"status":            u.Status,
		"created_at":        u.CreatedAt,
		"updated_at":        u.UpdatedAt,
	}
}

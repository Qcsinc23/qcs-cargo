package api

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
)

const refreshCookieName = "qcs_refresh"
const refreshCookieMaxAge = 7 * 24 * 3600 // 7 days in seconds

func normalizeAuthEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func currentAppURL() string {
	appURL := strings.TrimSpace(os.Getenv("APP_URL"))
	if appURL == "" {
		return "http://localhost:8080"
	}
	return appURL
}

func logSensitiveAuthArtifact(format string, args ...any) {
	if services.AllowDebugAuthArtifacts() {
		log.Printf(format, args...)
	}
}

func sendVerificationEmail(ctx context.Context, user gen.User, logPrefix string) error {
	rawToken, err := services.RequestEmailVerification(ctx, user.ID)
	if err != nil {
		return err
	}

	link := currentAppURL() + "/verify-email?token=" + rawToken
	if os.Getenv("RESEND_API_KEY") != "" {
		if err := services.SendVerificationEmail(user.Email, link); err != nil {
			return err
		}
	} else {
		log.Printf("%s: verification transport not configured for %s", logPrefix, user.Email)
		logSensitiveAuthArtifact("[DEV] Verification link for %s: %s", user.Email, link)
	}
	return nil
}

// RegisterAuth mounts auth routes on the given group. Pass the same group to RegisterMe with auth middleware applied.
func RegisterAuth(g fiber.Router) {
	g.Post("/auth/register", authRegister)
	g.Post("/auth/verify-email", authVerifyEmail)
	g.Post("/auth/resend-verification", authResendVerification)
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
	recordActivity(c.Context(), userID.(string), "auth.password.change", "auth", userID.(string), "")
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
	normalizedEmail := normalizeAuthEmail(body.Email)
	if body.Name == "" || normalizedEmail == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "name and email required"))
	}
	// Validate email format
	if err := services.ValidateEmail(normalizedEmail); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	// Pass 2 audit fix C-1 (server-side): bound the display name and reject
	// HTML metacharacters so a malicious name cannot become stored XSS in the
	// admin or warehouse UIs that render this field.
	cleanName, err := services.ValidateName(body.Name)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	body.Name = cleanName
	// Validate password complexity if provided
	if body.Password != "" {
		if err := services.ValidatePassword(body.Password); err != nil {
			return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
		}
	}
	user, err := services.Register(c.Context(), body.Name, normalizedEmail, body.Phone, body.Password)
	if err != nil {
		if errors.Is(err, services.ErrEmailAlreadyRegistered) {
			existing, lookupErr := db.Queries().GetUserByEmail(c.Context(), normalizedEmail)
			if lookupErr == nil && existing.EmailVerified == 0 {
				if sendErr := sendVerificationEmail(c.Context(), existing, "auth register"); sendErr != nil {
					log.Printf("auth register: failed to create/send verification token for existing user: %v", sendErr)
				}
				return c.JSON(fiber.Map{
					"data": fiber.Map{
						"user":    userToMap(existing),
						"message": "Account already exists. Please check your email to verify your account.",
					},
				})
			}
			return c.Status(409).JSON(ErrorResponse{}.withCode("EMAIL_EXISTS", err.Error()))
		}
		log.Printf("auth register: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Registration failed"))
	}

	// Send verification email for newly registered users.
	if err := sendVerificationEmail(c.Context(), user, "auth register"); err != nil {
		log.Printf("auth register: failed to create/send verification token: %v", err)
	}
	recordActivity(c.Context(), user.ID, "auth.register", "user", user.ID, "email="+user.Email)

	return c.Status(201).JSON(fiber.Map{
		"data": fiber.Map{
			"user":    userToMap(user),
			"message": "Account created. Please check your email to verify your account.",
		},
	})
}

func authVerifyEmail(c *fiber.Ctx) error {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "token required"))
	}
	if err := services.VerifyEmail(c.Context(), body.Token); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("INVALID_TOKEN", err.Error()))
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Email verified successfully"}})
}

func authResendVerification(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "email required"))
	}
	normalizedEmail := normalizeAuthEmail(body.Email)
	if normalizedEmail == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "email required"))
	}
	if err := services.ValidateEmail(normalizedEmail); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}

	// Pass 2 audit fix M-4: per-account + per-IP throttle so an attacker
	// cannot bomb a victim's inbox by replaying this endpoint.
	if err := services.CheckAndRecordAuthRequest(c.Context(), "resend_verification:"+normalizedEmail, 3, 10*time.Minute); err != nil {
		return c.Status(429).JSON(ErrorResponse{}.withCode("RATE_LIMITED", "Too many verification emails requested. Please wait before trying again."))
	}
	if err := services.CheckAndRecordAuthRequest(c.Context(), "resend_verification_ip:"+c.IP(), 20, 10*time.Minute); err != nil {
		return c.Status(429).JSON(ErrorResponse{}.withCode("RATE_LIMITED", "Too many requests from this network. Please wait before trying again."))
	}

	const msg = "If an account with that email exists, a verification email has been sent."
	q := db.Queries()
	user, err := q.GetUserByEmail(c.Context(), normalizedEmail)
	if err != nil {
		return c.JSON(fiber.Map{"data": fiber.Map{"message": msg}})
	}
	if user.EmailVerified != 0 {
		return c.JSON(fiber.Map{"data": fiber.Map{"message": msg}})
	}

	if err := sendVerificationEmail(c.Context(), user, "auth resend verification"); err != nil {
		log.Printf("auth resend verification: token create/send failed: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}

	return c.JSON(fiber.Map{"data": fiber.Map{"message": msg}})
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
	normalizedEmail := normalizeAuthEmail(body.Email)
	if normalizedEmail == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "email required"))
	}
	// Validate email format
	if err := services.ValidateEmail(normalizedEmail); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	// Pass 2 audit fix M-4: per-account + per-IP throttle. Bounds:
	// 3 requests per 5 minutes per email, 20 per 5 minutes per source IP.
	// Returns the same enum-safe message envelope on throttle so we do not
	// inadvertently reveal account existence via the throttle response code.
	if err := services.CheckAndRecordAuthRequest(c.Context(), "magic_link:"+normalizedEmail, 3, 5*time.Minute); err != nil {
		return c.Status(429).JSON(ErrorResponse{}.withCode("RATE_LIMITED", "Too many sign-in links requested. Please wait before trying again."))
	}
	if err := services.CheckAndRecordAuthRequest(c.Context(), "magic_link_ip:"+c.IP(), 20, 5*time.Minute); err != nil {
		return c.Status(429).JSON(ErrorResponse{}.withCode("RATE_LIMITED", "Too many requests from this network. Please wait before trying again."))
	}
	// Always return the same response regardless of whether the email exists.
	// This prevents user enumeration (PRD 3.2.1).
	const enumSafeMsg = "If an account with that email exists, you will receive a sign-in link shortly."
	q := db.Queries()
	user, err := q.GetUserByEmail(c.Context(), normalizedEmail)
	if err != nil {
		if err == sql.ErrNoRows {
			// Do not reveal non-existence — return 200 with same message.
			return c.JSON(fiber.Map{"data": fiber.Map{"message": enumSafeMsg}})
		}
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}
	appURL := currentAppURL()
	if user.EmailVerified == 0 {
		rawToken, reqErr := services.RequestEmailVerification(c.Context(), user.ID)
		if reqErr != nil {
			log.Printf("magic link request: verification token create failed: %v", reqErr)
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
		}
		link := appURL + "/verify-email?token=" + rawToken
		if os.Getenv("RESEND_API_KEY") != "" {
			if err := services.SendVerificationEmail(user.Email, link); err != nil {
				log.Printf("magic link request: verification email send failed for %s: %v", normalizedEmail, err)
				return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
			}
		} else {
			log.Printf("magic link request: verification transport not configured for %s", normalizedEmail)
			logSensitiveAuthArtifact("[DEV] Verification link for %s: %s", normalizedEmail, link)
		}
		recordActivity(c.Context(), user.ID, "auth.magic_link.request", "user", user.ID, "rerouted=verify_email")
		return c.JSON(fiber.Map{"data": fiber.Map{"message": enumSafeMsg}})
	}
	rawToken, err := services.RequestMagicLink(c.Context(), user.ID, body.RedirectTo)
	if err != nil {
		log.Printf("magic link request: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}
	link := appURL + "/verify?token=" + rawToken
	if body.RedirectTo != "" {
		link += "&redirectTo=" + body.RedirectTo
	}
	if os.Getenv("RESEND_API_KEY") != "" {
		if err := services.SendMagicLink(user.Email, link); err != nil {
			log.Printf("[Resend] magic link email send failed for %s: %v", normalizedEmail, err)
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
		}
	} else {
		log.Printf("magic link request: mail transport not configured for %s", normalizedEmail)
		logSensitiveAuthArtifact("[DEV] Magic link for %s: %s", normalizedEmail, link)
	}
	recordActivity(c.Context(), user.ID, "auth.magic_link.request", "user", user.ID, "email="+user.Email)
	return c.JSON(fiber.Map{"data": fiber.Map{"message": enumSafeMsg}})
}

// authForgotPassword requests a password-reset token. Always returns 200 (no enumeration).
func authForgotPassword(c *fiber.Ctx) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "email required"))
	}
	normalizedEmail := normalizeAuthEmail(body.Email)
	if normalizedEmail == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "email required"))
	}
	// Validate email format
	if err := services.ValidateEmail(normalizedEmail); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	// Pass 2 audit fix M-4: per-account + per-IP throttle.
	if err := services.CheckAndRecordAuthRequest(c.Context(), "forgot_password:"+normalizedEmail, 3, 15*time.Minute); err != nil {
		return c.Status(429).JSON(ErrorResponse{}.withCode("RATE_LIMITED", "Too many password reset requests. Please wait before trying again."))
	}
	if err := services.CheckAndRecordAuthRequest(c.Context(), "forgot_password_ip:"+c.IP(), 20, 15*time.Minute); err != nil {
		return c.Status(429).JSON(ErrorResponse{}.withCode("RATE_LIMITED", "Too many requests from this network. Please wait before trying again."))
	}
	const msg = "If an account with that email exists, you will receive a password reset link."
	q := db.Queries()
	user, err := q.GetUserByEmail(c.Context(), normalizedEmail)
	if err != nil {
		// Return 200 regardless — prevent enumeration
		return c.JSON(fiber.Map{"data": fiber.Map{"message": msg}})
	}
	appURL := currentAppURL()
	_, link, err := services.RequestPasswordReset(c.Context(), user.ID, appURL)
	if err != nil {
		log.Printf("forgot password: %v", err)
		return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
	}
	if os.Getenv("RESEND_API_KEY") != "" {
		if err := services.SendPasswordResetLink(user.Email, link); err != nil {
			log.Printf("password reset email send: %v", err)
			return c.Status(500).JSON(ErrorResponse{}.withCode("INTERNAL_ERROR", "Request failed"))
		}
	} else {
		log.Printf("forgot password: mail transport not configured for %s", normalizedEmail)
		logSensitiveAuthArtifact("[DEV] Password reset link for %s: %s", normalizedEmail, link)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"message": msg}})
}

// authResetPassword consumes a reset token and updates the user's password.
func authResetPassword(c *fiber.Ctx) error {
	lockKey := "auth:password_reset:" + c.IP()
	if locked, until := middleware.CheckAuthAttemptLockout(lockKey); locked {
		return c.Status(429).JSON(ErrorResponse{}.withCode(
			"RATE_LIMITED",
			"Too many failed attempts. Try again after "+until.UTC().Format(time.RFC3339),
		))
	}

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
	// Validate password complexity
	if err := services.ValidatePassword(body.Password); err != nil {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", err.Error()))
	}
	if err := services.ResetPassword(c.Context(), body.Token, body.Password); err != nil {
		middleware.RecordAuthAttemptFailure(lockKey)
		return c.Status(400).JSON(ErrorResponse{}.withCode("INVALID_TOKEN", err.Error()))
	}
	middleware.ClearAuthAttemptFailures(lockKey)
	return c.JSON(fiber.Map{"data": fiber.Map{"message": "Password updated. You can now sign in."}})
}

func authMagicLinkVerify(c *fiber.Ctx) error {
	lockKey := "auth:magic_link_verify:" + c.IP()
	if locked, until := middleware.CheckAuthAttemptLockout(lockKey); locked {
		return c.Status(429).JSON(ErrorResponse{}.withCode(
			"RATE_LIMITED",
			"Too many failed attempts. Try again after "+until.UTC().Format(time.RFC3339),
		))
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil || body.Token == "" {
		return c.Status(400).JSON(ErrorResponse{}.withCode("VALIDATION_ERROR", "token required"))
	}
	user, accessToken, refreshToken, err := services.VerifyMagicLink(c.Context(), body.Token)
	if err != nil {
		middleware.RecordAuthAttemptFailure(lockKey)
		if errors.Is(err, services.ErrEmailNotVerified) {
			return c.Status(403).JSON(ErrorResponse{}.withCode("EMAIL_NOT_VERIFIED", "Please verify your email before signing in."))
		}
		if errors.Is(err, services.ErrAccountInactive) {
			return c.Status(403).JSON(ErrorResponse{}.withCode("ACCOUNT_INACTIVE", "Account is inactive"))
		}
		log.Printf("[auth] magic-link verify failed: %v", err)
		return c.Status(401).JSON(ErrorResponse{}.withCode("INVALID_LINK", err.Error()))
	}
	log.Printf("[auth] magic-link verify success user_id=%s", user.ID)
	middleware.ClearAuthAttemptFailures(lockKey)
	recordActivity(c.Context(), user.ID, "auth.magic_link.verify", "user", user.ID, "")
	setRefreshCookie(c, refreshToken)
	sessionID, _ := services.ValidateRefreshToken(refreshToken)
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"user":         userToMap(user),
			"access_token": accessToken,
			"session_id":   sessionID,
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
		if errors.Is(err, services.ErrAccountInactive) {
			return c.Status(403).JSON(ErrorResponse{}.withCode("ACCOUNT_INACTIVE", "Account is inactive"))
		}
		return c.Status(401).JSON(ErrorResponse{}.withCode("UNAUTHENTICATED", err.Error()))
	}
	sessionID, _ := services.ValidateRefreshToken(refreshToken)
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"user":         userToMap(user),
			"access_token": accessToken,
			"session_id":   sessionID,
			"expires_in":   int(services.AccessExpiry.Seconds()),
		},
	})
}

func authLogout(c *fiber.Ctx) error {
	refreshToken := c.Cookies(refreshCookieName)
	authUserID := ""

	// Best-effort access token revocation (blacklist by JTI)
	auth := c.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		if claims, err := services.ValidateAccessTokenClaims(strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))); err == nil {
			authUserID = claims.UserID
			if claims.ID != "" && claims.ExpiresAt != nil {
				_ = services.BlacklistToken(c.Context(), claims.ID, claims.ExpiresAt.Time)
			}
		}
	}

	if err := services.Logout(c.Context(), refreshToken); err != nil {
		log.Printf("auth logout: %v", err)
	}
	if authUserID != "" {
		recordActivity(c.Context(), authUserID, "auth.logout", "user", authUserID, "")
	}
	clearRefreshCookie(c)
	return c.SendStatus(204)
}

func setRefreshCookie(c *fiber.Ctx, token string) {
	appURL := currentAppURL()
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   refreshCookieMaxAge,
		HTTPOnly: true,
		Secure:   services.IsProductionRuntime() || strings.HasPrefix(appURL, "https://"),
		SameSite: "Strict",
	})
}

func clearRefreshCookie(c *fiber.Ctx) {
	appURL := currentAppURL()
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   services.IsProductionRuntime() || strings.HasPrefix(appURL, "https://"),
		SameSite: "Strict",
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

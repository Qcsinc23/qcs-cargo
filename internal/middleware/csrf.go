package middleware

import (
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// csrfExemptAuthRoutes lists the unauthenticated /auth/* routes that legitimately
// run without a CSRF check (registration, login, refresh, logout, password
// recovery, email verification, magic link). Any future authenticated route
// under /auth/* will NOT be exempted, closing the latent footgun called out by
// Pass 2 audit fix H-4.
var csrfExemptAuthRoutes = map[string]struct{}{
	"/api/v1/auth/register":             {},
	"/api/v1/auth/verify-email":         {},
	"/api/v1/auth/resend-verification":  {},
	"/api/v1/auth/magic-link/request":   {},
	"/api/v1/auth/magic-link/verify":    {},
	"/api/v1/auth/refresh":              {},
	"/api/v1/auth/logout":               {},
	"/api/v1/auth/password/forgot":      {},
	"/api/v1/auth/password/reset":       {},
}

// CSRFProtection enforces same-origin checks for state-changing requests that carry
// the refresh-token cookie. Bearer-token and machine-to-machine calls (no cookie) pass through.
func CSRFProtection(c *fiber.Ctx) error {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return c.Next()
	}
	if c.Cookies("qcs_refresh") == "" {
		return c.Next()
	}
	if strings.HasPrefix(c.Path(), "/api/webhooks/") {
		return c.Next()
	}
	// Pass 2 audit fix H-4: explicit allowlist instead of /api/v1/auth/* prefix.
	if _, ok := csrfExemptAuthRoutes[c.Path()]; ok {
		return c.Next()
	}

	origin := strings.TrimSpace(c.Get("Origin"))
	if origin != "" {
		if isAllowedOrigin(origin) {
			return c.Next()
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fiber.Map{"code": "CSRF_FAILED", "message": "Origin not allowed"},
		})
	}

	referer := strings.TrimSpace(c.Get("Referer"))
	if referer != "" {
		u, err := url.Parse(referer)
		if err == nil && u.Scheme != "" && u.Host != "" {
			if isAllowedOrigin(u.Scheme + "://" + u.Host) {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fiber.Map{"code": "CSRF_FAILED", "message": "Referer not allowed"},
		})
	}

	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": fiber.Map{"code": "CSRF_FAILED", "message": "Missing origin/referrer"},
	})
}

func isAllowedOrigin(origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	if appURL := strings.TrimSpace(os.Getenv("APP_URL")); appURL != "" && sameOrigin(origin, appURL) {
		return true
	}
	for _, allowed := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if sameOrigin(origin, allowed) {
			return true
		}
	}
	return false
}

func sameOrigin(a, b string) bool {
	ua, errA := url.Parse(strings.TrimSpace(a))
	ub, errB := url.Parse(strings.TrimSpace(b))
	if errA != nil || errB != nil || ua.Scheme == "" || ub.Scheme == "" || ua.Host == "" || ub.Host == "" {
		return false
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) && strings.EqualFold(ua.Host, ub.Host)
}


package middleware

import (
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

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
	if strings.HasPrefix(c.Path(), "/api/v1/auth/") {
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


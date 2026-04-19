package middleware

import "github.com/gofiber/fiber/v2"

// SecurityHeaders adds security-related headers to all responses.
//
// Phase 2.1 (DEF-004): CSP no longer whitelists cdn.tailwindcss.com or
// cdn.jsdelivr.net; the app ships a precompiled /css/tailwind.css.
//
// Phase 3.1 (SEC-001): script-src has dropped 'unsafe-inline'. All
// previously-inline <script> blocks were extracted into per-page
// external files in internal/static/{,dashboard,admin,warehouse}/scripts/
// during Phase 2.4 / Phase 3.1a, so the policy can now refuse arbitrary
// inline scripts. style-src keeps 'unsafe-inline' for now because
// inline <style> blocks remain on several pages; that is tracked as a
// follow-up cleanup.
func SecurityHeaders(c *fiber.Ctx) error {
	c.Set("X-Frame-Options", "DENY")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-XSS-Protection", "1; mode=block")
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Set(
		"Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' https://js.stripe.com; "+
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
			"font-src 'self' https://fonts.gstatic.com; "+
			"img-src 'self' data: https:; "+
			"frame-src https://js.stripe.com https://hooks.stripe.com; "+
			"connect-src 'self' https://api.stripe.com",
	)
	if c.Protocol() == "https" {
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	return c.Next()
}

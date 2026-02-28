package middleware

import "github.com/gofiber/fiber/v2"

// SecurityHeaders adds security-related headers to all responses
func SecurityHeaders(c *fiber.Ctx) error {
	// Prevent clickjacking
	c.Set("X-Frame-Options", "DENY")
	// Prevent MIME type sniffing
	c.Set("X-Content-Type-Options", "nosniff")
	// Enable XSS filter
	c.Set("X-XSS-Protection", "1; mode=block")
	// Referrer policy
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	// Content Security Policy - allows Stripe integration
	c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://js.stripe.com https://cdn.tailwindcss.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:; frame-src https://js.stripe.com https://hooks.stripe.com; connect-src 'self' https://api.stripe.com")
	// HSTS (only in production with HTTPS)
	if c.Protocol() == "https" {
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	// Permissions Policy
	c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	return c.Next()
}

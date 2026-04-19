package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// TestSecurityHeaders_CSPHasNoUnsafeInlineScripts is the SEC-001
// regression test. Phase 2.4 / 3.1 extracted every inline <script> block
// into external files so that script-src can refuse 'unsafe-inline'.
// This test fails loudly if a future change accidentally re-introduces
// 'unsafe-inline' to script-src, which would silently neutralize CSP as
// an XSS defense.
func TestSecurityHeaders_CSPHasNoUnsafeInlineScripts(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders)
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatalf("Content-Security-Policy header missing")
	}
	scriptSrc := extractDirective(csp, "script-src")
	if scriptSrc == "" {
		t.Fatalf("script-src directive missing from CSP: %q", csp)
	}
	if strings.Contains(scriptSrc, "'unsafe-inline'") {
		t.Fatalf("script-src must not allow 'unsafe-inline'; got %q", scriptSrc)
	}
	if strings.Contains(scriptSrc, "cdn.tailwindcss.com") {
		t.Fatalf("script-src must not whitelist cdn.tailwindcss.com (Tailwind shipped locally); got %q", scriptSrc)
	}
	if strings.Contains(scriptSrc, "cdn.jsdelivr.net") {
		t.Fatalf("script-src must not whitelist cdn.jsdelivr.net; got %q", scriptSrc)
	}
}

func extractDirective(csp, directive string) string {
	for _, part := range strings.Split(csp, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, directive+" ") {
			return part
		}
	}
	return ""
}

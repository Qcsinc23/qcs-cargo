package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSRFProtection_RejectsCrossOriginForCookieMutation(t *testing.T) {
	t.Setenv("APP_URL", "https://qcs-cargo.com")
	app := fiber.New()
	app.Use(CSRFProtection)
	app.Post("/api/v1/test", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(&http.Cookie{Name: "qcs_refresh", Value: "token"})

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCSRFProtection_AllowsSameOriginForCookieMutation(t *testing.T) {
	t.Setenv("APP_URL", "https://qcs-cargo.com")
	app := fiber.New()
	app.Use(CSRFProtection)
	app.Post("/api/v1/test", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://qcs-cargo.com")
	req.AddCookie(&http.Cookie{Name: "qcs_refresh", Value: "token"})

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCSRFProtection_SkipsWhenNoRefreshCookie(t *testing.T) {
	t.Setenv("APP_URL", "https://qcs-cargo.com")
	app := fiber.New()
	app.Use(CSRFProtection)
	app.Post("/api/v1/test", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", nil)
	req.Header.Set("Origin", "https://evil.example")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

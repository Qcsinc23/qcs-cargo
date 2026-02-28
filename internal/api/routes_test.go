package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/api"
	"github.com/Qcsinc23/qcs-cargo/internal/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRoutesTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: api.ErrorHandler,
	})
	api.RegisterAPIRoutes(app)
	return app
}

func TestAuthRateLimit_AppliesOnlyOnAuthPrefix(t *testing.T) {
	middleware.ResetAuthRateLimitersForTest()
	app := setupRoutesTestApp()

	nonAuthIP := "198.51.100.10:1234"
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
		req.RemoteAddr = nonAuthIP
		resp, err := app.Test(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	}

	authIP := "198.51.100.11:1234"
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/not-found", nil)
		req.RemoteAddr = authIP
		resp, err := app.Test(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/not-found", nil)
	req.RemoteAddr = authIP
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestRequestID_IsPropagatedToResponsesWhenPresent(t *testing.T) {
	app := setupRoutesTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/not-found", nil)
	req.Header.Set("X-Request-ID", "req-123")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "req-123", resp.Header.Get("X-Request-ID"))
}

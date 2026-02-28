package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsEndpoint_ExposesRequestMetrics(t *testing.T) {
	app := fiber.New()
	app.Use(MetricsMiddleware)
	app.Get("/hello/:id", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusCreated)
	})
	app.Get("/metrics", MetricsHandler)

	req := httptest.NewRequest(http.MethodGet, "/hello/123", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	_ = resp.Body.Close()

	req = httptest.NewRequest(http.MethodGet, "/hello/456", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	_ = resp.Body.Close()

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	text := string(body)

	assert.Contains(t, text, "qcs_http_requests_total")
	assert.True(t, strings.Contains(text, `route="/hello/:id"`) || strings.Contains(text, `path="/hello/:id"`))
	assert.True(t, strings.Contains(text, `status_class="2xx"`) || strings.Contains(text, `status_family="2xx"`))
}

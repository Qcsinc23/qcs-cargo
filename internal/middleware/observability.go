package middleware

import (
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/gofiber/fiber/v2"
)

// APIObservability records API request analytics and performance events asynchronously.
func APIObservability(c *fiber.Ctx) error {
	start := time.Now()
	err := c.Next()

	statusCode := c.Response().StatusCode()
	if statusCode == 0 {
		statusCode = fiber.StatusOK
	}
	durationMS := float64(time.Since(start).Microseconds()) / 1000.0

	requestID := requestIDFromContext(c)
	userID, _ := c.Locals(CtxUserID).(string)
	path := observabilityPath(c)
	method := strings.TrimSpace(c.Method())

	services.Observability().Record(services.ObservabilityEvent{
		Category:   "analytics",
		EventName:  "api.request",
		UserID:     userID,
		RequestID:  requestID,
		Path:       path,
		Method:     method,
		StatusCode: &statusCode,
		DurationMS: &durationMS,
	})

	services.Observability().Record(services.ObservabilityEvent{
		Category:   "performance",
		EventName:  "api.request.duration",
		UserID:     userID,
		RequestID:  requestID,
		Path:       path,
		Method:     method,
		StatusCode: &statusCode,
		DurationMS: &durationMS,
	})

	return err
}

func requestIDFromContext(c *fiber.Ctx) string {
	if v := strings.TrimSpace(c.GetRespHeader(fiber.HeaderXRequestID)); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.Get(fiber.HeaderXRequestID)); v != "" {
		return v
	}
	if localValue, ok := c.Locals(requestIDLocalKey).(string); ok {
		return strings.TrimSpace(localValue)
	}
	return ""
}

func observabilityPath(c *fiber.Ctx) string {
	if route := c.Route(); route != nil {
		if routePath := strings.TrimSpace(route.Path); routePath != "" {
			return routePath
		}
	}
	return strings.TrimSpace(c.Path())
}

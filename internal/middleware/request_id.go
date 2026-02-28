package middleware

import "github.com/gofiber/fiber/v2"

const requestIDLocalKey = "requestid"

// PropagateRequestID ensures responses include X-Request-ID when one is available.
func PropagateRequestID(c *fiber.Ctx) error {
	err := c.Next()

	requestID := c.GetRespHeader(fiber.HeaderXRequestID)
	if requestID == "" {
		requestID = c.Get(fiber.HeaderXRequestID)
	}
	if requestID == "" {
		if localValue, ok := c.Locals(requestIDLocalKey).(string); ok {
			requestID = localValue
		}
	}
	if requestID != "" {
		c.Set(fiber.HeaderXRequestID, requestID)
	}

	return err
}

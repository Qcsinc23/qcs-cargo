package api

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Validation limits for API input (contact form and similar text fields).
// Use these when validating request bodies to reject oversized content.
const (
	MaxContactMessageLength = 5000 // max length for contact form message body
)

// ErrorResponse matches PRD 3.6.1: { "error": { "code", "message", "details" } }
type ErrorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}

func (e ErrorResponse) withCode(code, message string) ErrorResponse {
	e.Error.Code = code
	e.Error.Message = message
	return e
}

// ErrorHandler is Fiber's custom error handler.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	er := ErrorResponse{}
	er.Error.Code = "INTERNAL_ERROR"
	er.Error.Message = "An unexpected error occurred"

	if e := new(fiber.Error); errors.As(err, &e) {
		code = e.Code
		er.Error.Message = e.Message
		er.Error.Code = httpStatusToCode(code)
	}

	if code >= 500 {
		log.Printf("[%s] %s: %v", c.Get("X-Request-ID"), er.Error.Code, err)
	}
	return c.Status(code).JSON(er)
}

func httpStatusToCode(status int) string {
	switch status {
	case 400:
		return "VALIDATION_ERROR"
	case 401:
		return "UNAUTHENTICATED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 409:
		return "CONFLICT"
	case 422:
		return "BUSINESS_RULE_VIOLATION"
	case 429:
		return "RATE_LIMITED"
	default:
		return "INTERNAL_ERROR"
	}
}

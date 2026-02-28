package api

import (
	"errors"
	"log"
	"net/http"

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

func newErrorResponse(code, message string) ErrorResponse {
	var resp ErrorResponse
	resp.Error.Code = code
	resp.Error.Message = message
	return resp
}

func (e ErrorResponse) withCode(code, message string) ErrorResponse {
	return newErrorResponse(code, message)
}

// ErrorHandler is Fiber's custom error handler.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	er := ErrorResponse{}

	if e := new(fiber.Error); errors.As(err, &e) {
		code = e.Code
	}

	// Client errors get a safe status-derived message.
	if code >= 400 && code < 500 {
		er = newErrorResponse(httpStatusToCode(code), clientMessageForStatus(code))
		return c.Status(code).JSON(er)
	}

	// Non-client errors are treated as server failures and are always generic.
	if code < 500 || code > 599 {
		code = fiber.StatusInternalServerError
	}
	er = newErrorResponse("INTERNAL_ERROR", "An unexpected error occurred")

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
	case 405:
		return "METHOD_NOT_ALLOWED"
	case 409:
		return "CONFLICT"
	case 415:
		return "UNSUPPORTED_MEDIA_TYPE"
	case 422:
		return "BUSINESS_RULE_VIOLATION"
	case 429:
		return "RATE_LIMITED"
	default:
		if status >= 400 && status < 500 {
			return "BAD_REQUEST"
		}
		return "INTERNAL_ERROR"
	}
}

func clientMessageForStatus(status int) string {
	if msg := http.StatusText(status); msg != "" {
		return msg
	}
	return "Bad Request"
}

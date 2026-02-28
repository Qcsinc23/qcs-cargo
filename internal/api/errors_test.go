package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorHandler_NonFiberError_IsGeneric500(t *testing.T) {
	body := callErrorHandler(t, errors.New("sql: connection refused"))

	assert.Equal(t, http.StatusInternalServerError, body.status)
	assert.Equal(t, "INTERNAL_ERROR", body.parsed.Error.Code)
	assert.Equal(t, "An unexpected error occurred", body.parsed.Error.Message)
	assert.NotContains(t, strings.ToLower(body.raw), "sql")
}

func TestErrorHandler_ClientErrors_AreSanitized(t *testing.T) {
	body := callErrorHandler(t, fiber.NewError(http.StatusBadRequest, "db constraint violation"))

	assert.Equal(t, http.StatusBadRequest, body.status)
	assert.Equal(t, "VALIDATION_ERROR", body.parsed.Error.Code)
	assert.Equal(t, "Bad Request", body.parsed.Error.Message)
	assert.NotContains(t, strings.ToLower(body.raw), "constraint")
}

func TestErrorHandler_ServerErrors_AreSanitized(t *testing.T) {
	body := callErrorHandler(t, fiber.NewError(http.StatusServiceUnavailable, "database down"))

	assert.Equal(t, http.StatusServiceUnavailable, body.status)
	assert.Equal(t, "INTERNAL_ERROR", body.parsed.Error.Code)
	assert.Equal(t, "An unexpected error occurred", body.parsed.Error.Message)
	assert.NotContains(t, strings.ToLower(body.raw), "database")
}

func TestErrorHandler_UnknownClientStatus_UsesBadRequestCode(t *testing.T) {
	body := callErrorHandler(t, fiber.NewError(http.StatusTeapot, "sensitive info"))

	assert.Equal(t, http.StatusTeapot, body.status)
	assert.Equal(t, "BAD_REQUEST", body.parsed.Error.Code)
	assert.Equal(t, "I'm a teapot", body.parsed.Error.Message)
	assert.NotContains(t, strings.ToLower(body.raw), "sensitive")
}

func TestErrorHandler_ResponseShape_RemainsStable(t *testing.T) {
	body := callErrorHandler(t, fiber.NewError(http.StatusNotFound, "route internals"))

	var envelope map[string]any
	err := json.Unmarshal([]byte(body.raw), &envelope)
	require.NoError(t, err)
	require.Contains(t, envelope, "error")
	assert.Len(t, envelope, 1)

	errorObj, ok := envelope["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "NOT_FOUND", errorObj["code"])
	assert.Equal(t, "Not Found", errorObj["message"])
	assert.Len(t, errorObj, 2)
}

func TestErrorResponse_WithCode_UsesConstructorPattern(t *testing.T) {
	resp := ErrorResponse{}.withCode("VALIDATION_ERROR", "Invalid body")

	assert.Equal(t, "VALIDATION_ERROR", resp.Error.Code)
	assert.Equal(t, "Invalid body", resp.Error.Message)
	assert.Nil(t, resp.Error.Details)
}

type handlerResult struct {
	status int
	parsed ErrorResponse
	raw    string
}

func callErrorHandler(t *testing.T, errToReturn error) handlerResult {
	t.Helper()

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})
	app.Get("/error", func(c *fiber.Ctx) error {
		return errToReturn
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var parsed ErrorResponse
	err = json.Unmarshal(rawBytes, &parsed)
	require.NoError(t, err)

	return handlerResult{
		status: resp.StatusCode,
		parsed: parsed,
		raw:    string(rawBytes),
	}
}

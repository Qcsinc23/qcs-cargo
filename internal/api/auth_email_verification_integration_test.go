//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRegister_DuplicateUnverifiedEmailReturnsVerificationMessage(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	app := setupTestApp(t)

	payload := []byte(`{
		"name":"Trevor Example",
		"email":"duplicate-register@example.com",
		"phone":"+15551234567",
		"password":"StrongPass1!"
	}`)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(payload))
	firstReq.Header.Set("Content-Type", "application/json")
	firstResp, err := app.Test(firstReq)
	require.NoError(t, err)
	defer firstResp.Body.Close()
	require.Equal(t, http.StatusCreated, firstResp.StatusCode)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(payload))
	secondReq.Header.Set("Content-Type", "application/json")
	secondResp, err := app.Test(secondReq)
	require.NoError(t, err)
	defer secondResp.Body.Close()
	require.Equal(t, http.StatusOK, secondResp.StatusCode)

	var body struct {
		Data struct {
			Message string `json:"message"`
			User    struct {
				Email         string `json:"email"`
				EmailVerified bool   `json:"email_verified"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(secondResp.Body).Decode(&body))
	assert.Contains(t, body.Data.Message, "Please check your email")
	assert.Equal(t, "duplicate-register@example.com", body.Data.User.Email)
	assert.False(t, body.Data.User.EmailVerified)

	var tokenCount int
	err = db.DB().QueryRow(`
		SELECT COUNT(*)
		FROM email_verification_tokens
		WHERE user_id = (SELECT id FROM users WHERE email = ?)
	`, body.Data.User.Email).Scan(&tokenCount)
	require.NoError(t, err)
	assert.Equal(t, 2, tokenCount)
}

func TestAuthVerifyEmail_OriginalTokenStillWorksAfterResend(t *testing.T) {
	app := setupTestApp(t)

	user, err := services.Register(context.Background(), "Verify Flow", "verify-flow@example.com", "+15551234567", "StrongPass1!")
	require.NoError(t, err)

	firstToken, err := services.RequestEmailVerification(context.Background(), user.ID)
	require.NoError(t, err)
	_, err = services.RequestEmailVerification(context.Background(), user.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewReader([]byte(`{"token":"`+firstToken+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

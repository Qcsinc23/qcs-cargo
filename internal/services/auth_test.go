package services_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSuiteCode(t *testing.T) {
	// Format: QCS- + 6 alphanumeric (PRD 8.10)
	prefix := "QCS-"
	re := regexp.MustCompile(`^QCS-[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{6}$`)

	for i := 0; i < 20; i++ {
		code, err := services.GenerateSuiteCode()
		require.NoError(t, err)
		assert.Len(t, code, len(prefix)+6, "suite code should be QCS- + 6 chars")
		assert.Regexp(t, re, code, "suite code should match QCS-XXXXXX format")
		assert.True(t, strings.HasPrefix(code, prefix))
	}
}

func TestValidateAccessToken_Boundaries(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	t.Setenv("JWT_SECRET", secret)
	t.Setenv("APP_ENV", "test")

	validClaims := services.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Minute)),
			Subject:   "user-123",
			ID:        "jti-valid",
		},
		UserID: "user-123",
		Email:  "user@example.com",
		Role:   "customer",
	}
	validToken := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)
	validRaw, err := validToken.SignedString([]byte(secret)[:32])
	require.NoError(t, err)

	userID, email, role, err := services.ValidateAccessToken(validRaw)
	require.NoError(t, err)
	assert.Equal(t, "user-123", userID)
	assert.Equal(t, "user@example.com", email)
	assert.Equal(t, "customer", role)

	expiredClaims := services.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			Subject:   "user-123",
			ID:        "jti-expired",
		},
		UserID: "user-123",
		Email:  "user@example.com",
		Role:   "customer",
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredRaw, err := expiredToken.SignedString([]byte(secret)[:32])
	require.NoError(t, err)

	_, _, _, err = services.ValidateAccessToken(expiredRaw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestValidateRefreshToken_Boundaries(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	t.Setenv("JWT_SECRET", secret)
	t.Setenv("APP_ENV", "test")

	validClaims := services.RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Minute)),
		},
		SessionID: "sess-123",
	}
	validToken := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)
	validRaw, err := validToken.SignedString([]byte(secret)[:32])
	require.NoError(t, err)

	sessionID, err := services.ValidateRefreshToken(validRaw)
	require.NoError(t, err)
	assert.Equal(t, "sess-123", sessionID)

	expiredClaims := services.RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
		},
		SessionID: "sess-expired",
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredRaw, err := expiredToken.SignedString([]byte(secret)[:32])
	require.NoError(t, err)

	_, err = services.ValidateRefreshToken(expiredRaw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid refresh token")
}

func TestLogout_InvalidRefreshTokenReturnsValidationError(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")

	err := services.Logout(context.Background(), "not-a-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "logout validate refresh token")
}

func TestRefreshSession_ExpiredSessionReturnsError(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")
	t.Setenv("RESEND_API_KEY", "re_test_fake")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	raw, err := services.RequestMagicLink(context.Background(), testdata.CustomerAliceID, "")
	require.NoError(t, err)
	_, _, refreshToken, err := services.VerifyMagicLink(context.Background(), raw)
	require.NoError(t, err)

	sessionID, err := services.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	_, err = conn.Exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`, time.Now().Add(-10*time.Minute).UTC().Format(time.RFC3339), sessionID)
	require.NoError(t, err)

	_, _, err = services.RefreshSession(context.Background(), refreshToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session expired or invalid")
}

func TestRefreshSession_RejectsRefreshHashMismatch(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	raw, err := services.RequestMagicLink(context.Background(), testdata.CustomerAliceID, "")
	require.NoError(t, err)
	_, _, refreshToken, err := services.VerifyMagicLink(context.Background(), raw)
	require.NoError(t, err)

	sessionID, err := services.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	_, err = conn.Exec(`UPDATE sessions SET refresh_token_hash = ? WHERE id = ?`, "tampered-hash", sessionID)
	require.NoError(t, err)

	_, _, err = services.RefreshSession(context.Background(), refreshToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session expired or invalid")

	var count int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, sessionID).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestRefreshSession_ReturnsCurrentUserRowFields(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")
	t.Setenv("RESEND_API_KEY", "re_test_fake")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	_, err := conn.Exec(`
		UPDATE users
		SET avatar_url = ?,
		    address_street = ?,
		    address_city = ?,
		    address_state = ?,
		    address_zip = ?,
		    storage_plan = ?,
		    free_storage_days = ?,
		    status = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		"https://cdn.example.com/avatar.png",
		"101 Main St",
		"Georgetown",
		"Region 4",
		"00000",
		"premium",
		45,
		"active",
		time.Now().UTC().Format(time.RFC3339),
		testdata.CustomerAliceID,
	)
	require.NoError(t, err)

	raw, err := services.RequestMagicLink(context.Background(), testdata.CustomerAliceID, "")
	require.NoError(t, err)
	_, _, refreshToken, err := services.VerifyMagicLink(context.Background(), raw)
	require.NoError(t, err)

	user, accessToken, err := services.RefreshSession(context.Background(), refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.Equal(t, testdata.CustomerAliceID, user.ID)
	assert.Equal(t, "premium", user.StoragePlan)
	assert.Equal(t, 45, user.FreeStorageDays)
	assert.Equal(t, "active", user.Status)
	assert.True(t, user.AvatarUrl.Valid)
	assert.Equal(t, "https://cdn.example.com/avatar.png", user.AvatarUrl.String)
	assert.True(t, user.AddressStreet.Valid)
	assert.Equal(t, "101 Main St", user.AddressStreet.String)
	assert.True(t, user.AddressCity.Valid)
	assert.Equal(t, "Georgetown", user.AddressCity.String)
}

func TestLogout_ValidRefreshTokenDeletesSession(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	raw, err := services.RequestMagicLink(context.Background(), testdata.CustomerAliceID, "")
	require.NoError(t, err)
	_, _, refreshToken, err := services.VerifyMagicLink(context.Background(), raw)
	require.NoError(t, err)

	sessionID, err := services.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	err = services.Logout(context.Background(), refreshToken)
	require.NoError(t, err)

	var count int
	err = conn.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, sessionID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestEmailVerification_OlderTokensRemainValidAfterReissue(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	user, err := services.Register(context.Background(), "Verify User", "verify-user@example.com", "+15551234567", "StrongPass1!")
	require.NoError(t, err)

	firstToken, err := services.RequestEmailVerification(context.Background(), user.ID)
	require.NoError(t, err)
	secondToken, err := services.RequestEmailVerification(context.Background(), user.ID)
	require.NoError(t, err)

	err = services.VerifyEmail(context.Background(), firstToken)
	require.NoError(t, err)

	verifiedUser, err := db.Queries().GetUserByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, verifiedUser.EmailVerified)

	err = services.VerifyEmail(context.Background(), secondToken)
	require.NoError(t, err)

	var usedCount int
	err = conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ? AND used = 1`, user.ID).Scan(&usedCount)
	require.NoError(t, err)
	assert.Equal(t, 2, usedCount)
}

func TestEmailVerification_UsedTokenIsIdempotentForVerifiedUser(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	user, err := services.Register(context.Background(), "Verify Again", "verify-again@example.com", "+15551234567", "StrongPass1!")
	require.NoError(t, err)

	token, err := services.RequestEmailVerification(context.Background(), user.ID)
	require.NoError(t, err)

	err = services.VerifyEmail(context.Background(), token)
	require.NoError(t, err)

	err = services.VerifyEmail(context.Background(), token)
	require.NoError(t, err)
}

func TestLogout_RejectsRefreshHashMismatch(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	raw, err := services.RequestMagicLink(context.Background(), testdata.CustomerAliceID, "")
	require.NoError(t, err)
	_, _, refreshToken, err := services.VerifyMagicLink(context.Background(), raw)
	require.NoError(t, err)

	sessionID, err := services.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)

	_, err = conn.Exec(`UPDATE sessions SET refresh_token_hash = ? WHERE id = ?`, "tampered-hash", sessionID)
	require.NoError(t, err)

	err = services.Logout(context.Background(), refreshToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session expired or invalid")

	var count int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, sessionID).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestVerifyMagicLink_RejectsUnverifiedUser(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_ENV", "test")

	conn := testdata.NewSeededDB(t)
	db.SetConnForTest(conn)

	_, err := conn.Exec(`UPDATE users SET email_verified = 0 WHERE id = ?`, testdata.CustomerAliceID)
	require.NoError(t, err)

	raw, err := services.RequestMagicLink(context.Background(), testdata.CustomerAliceID, "")
	require.NoError(t, err)

	_, _, _, err = services.VerifyMagicLink(context.Background(), raw)
	require.ErrorIs(t, err, services.ErrEmailNotVerified)

	var sessions int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id = ?`, testdata.CustomerAliceID).Scan(&sessions))
	assert.Equal(t, 0, sessions)
}

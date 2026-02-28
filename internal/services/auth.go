package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
)

func isProductionEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func init() {
	// Validate JWT_SECRET at startup in production
	if isProductionEnv() {
		s := os.Getenv("JWT_SECRET")
		if len(s) < 32 {
			log.Fatal("JWT_SECRET must be at least 32 characters in production")
		}
	}
}

const (
	SuiteCodePrefix = "QCS-"
	SuiteCodeLength = 6
	MagicLinkExpiry = 10 * time.Minute
	AccessExpiry    = 15 * time.Minute
	RefreshExpiry   = 7 * 24 * time.Hour
)

var alphanum = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

// ErrEmailNotVerified indicates an auth attempt for an account that has not completed email verification.
var ErrEmailNotVerified = errors.New("email not verified")

// GenerateSuiteCode returns QCS-{6 alphanumeric} per PRD 8.10.
func GenerateSuiteCode() (string, error) {
	b := make([]byte, SuiteCodeLength)
	max := big.NewInt(int64(len(alphanum)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphanum[n.Int64()]
	}
	return SuiteCodePrefix + string(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Register creates a user with a new suite code. Returns user or error if email exists.
func Register(ctx context.Context, name, email, phone, password string) (gen.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	q := db.Queries()
	_, err := q.GetUserByEmail(ctx, email)
	if err == nil {
		return gen.User{}, fmt.Errorf("email already registered")
	}
	if err != sql.ErrNoRows {
		return gen.User{}, err
	}
	suiteCode, err := GenerateSuiteCode()
	if err != nil {
		return gen.User{}, err
	}

	// Hash the password if provided (cost 12 per PRD 13.5)
	var passwordHash sql.NullString
	if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return gen.User{}, fmt.Errorf("Register: bcrypt: %w", err)
		}
		passwordHash = sql.NullString{String: string(hashed), Valid: true}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	return q.CreateUser(ctx, gen.CreateUserParams{
		ID:           id,
		Name:         name,
		Email:        email,
		Phone:        sql.NullString{String: phone, Valid: phone != ""},
		PasswordHash: passwordHash,
		SuiteCode:    sql.NullString{String: suiteCode, Valid: true},
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

// RequestEmailVerification creates/stores a verification token for the user and returns the raw token.
func RequestEmailVerification(ctx context.Context, userID string) (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	rawToken := hex.EncodeToString(token)
	hash := hashToken(rawToken)
	now := time.Now().UTC().Format(time.RFC3339)

	err := db.Queries().SetEmailVerificationToken(ctx, gen.SetEmailVerificationTokenParams{
		EmailVerificationToken:  sql.NullString{String: hash, Valid: true},
		EmailVerificationSentAt: sql.NullString{String: now, Valid: true},
		UpdatedAt:               now,
		ID:                      userID,
	})
	if err != nil {
		return "", err
	}

	return rawToken, nil
}

// VerifyEmail validates and consumes an email verification token.
func VerifyEmail(ctx context.Context, rawToken string) error {
	hash := hashToken(rawToken)
	user, err := db.Queries().GetUserByEmailVerificationToken(ctx, sql.NullString{String: hash, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid or expired verification token")
		}
		return err
	}

	if !user.EmailVerificationSentAt.Valid {
		return fmt.Errorf("invalid or expired verification token")
	}
	sentAt, err := time.Parse(time.RFC3339, user.EmailVerificationSentAt.String)
	if err != nil {
		return fmt.Errorf("invalid verification token timestamp")
	}
	if time.Since(sentAt) > 24*time.Hour {
		return fmt.Errorf("verification token expired")
	}

	return db.Queries().SetEmailVerified(ctx, gen.SetEmailVerifiedParams{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		ID:        user.ID,
	})
}

// RequestMagicLink creates a magic link record and returns the raw token (to send in email).
func RequestMagicLink(ctx context.Context, userID, redirectTo string) (rawToken string, err error) {
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return "", err
	}
	rawToken = hex.EncodeToString(tok)
	hash := hashToken(rawToken)
	expires := time.Now().Add(MagicLinkExpiry).UTC().Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)
	q := db.Queries()
	_, err = q.CreateMagicLink(ctx, gen.CreateMagicLinkParams{
		ID:         uuid.New().String(),
		UserID:     userID,
		TokenHash:  hash,
		RedirectTo: sql.NullString{String: redirectTo, Valid: redirectTo != ""},
		ExpiresAt:  expires,
		CreatedAt:  now,
	})
	if err != nil {
		return "", err
	}
	return rawToken, nil
}

// VerifyMagicLink consumes the token, creates a session, returns user and session tokens.
func VerifyMagicLink(ctx context.Context, rawToken string) (user gen.User, accessToken, refreshToken string, err error) {
	hash := hashToken(rawToken)
	now := time.Now().UTC().Format(time.RFC3339)
	q := db.Queries()
	link, err := q.GetMagicLinkByTokenHash(ctx, gen.GetMagicLinkByTokenHashParams{TokenHash: hash, ExpiresAt: now})
	if err != nil {
		if err == sql.ErrNoRows {
			return gen.User{}, "", "", fmt.Errorf("invalid or expired link")
		}
		return gen.User{}, "", "", err
	}
	user, err = q.GetUserByID(ctx, link.UserID)
	if err != nil {
		return gen.User{}, "", "", err
	}
	if user.EmailVerified == 0 {
		return gen.User{}, "", "", ErrEmailNotVerified
	}
	if err := q.MarkMagicLinkUsed(ctx, link.ID); err != nil {
		return gen.User{}, "", "", err
	}
	sessionID := uuid.New().String()
	refreshToken, err = issueRefreshToken(sessionID)
	if err != nil {
		return gen.User{}, "", "", err
	}
	refreshHash := hashToken(refreshToken)
	expiresAt := time.Now().Add(RefreshExpiry).UTC().Format(time.RFC3339)
	_, err = q.CreateSession(ctx, gen.CreateSessionParams{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: refreshHash,
		ExpiresAt:        expiresAt,
		CreatedAt:        now,
	})
	if err != nil {
		return gen.User{}, "", "", err
	}
	accessToken, err = issueAccessToken(user.ID, user.Email, user.Role)
	if err != nil {
		return gen.User{}, "", "", err
	}
	return user, accessToken, refreshToken, nil
}

// AccessClaims for JWT (15 min).
type AccessClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// RefreshClaims for refresh JWT (7 days), stored in httpOnly cookie.
type RefreshClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"session_id"`
}

func getJWTSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if len(s) < 32 {
		// In development mode, allow fallback with warning
		if !isProductionEnv() {
			log.Println("WARNING: Using development JWT secret. Set JWT_SECRET in production!")
			return []byte("qcs-dev-secret-change-in-production-32bytes!!")
		}
		panic("JWT_SECRET environment variable must be at least 32 characters")
	}
	return []byte(s)[:32]
}

func issueAccessToken(userID, email, role string) (string, error) {
	jti := uuid.New().String()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessExpiry)),
			Subject:   userID,
			ID:        jti,
		},
		UserID: userID,
		Email:  email,
		Role:   role,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(getJWTSecret())
}

func issueRefreshToken(sessionID string) (string, error) {
	claims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshExpiry)),
		},
		SessionID: sessionID,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(getJWTSecret())
}

// ValidateAccessToken returns user_id, email, role or error.
func ValidateAccessToken(tokenString string) (userID, email, role string, err error) {
	var claims AccessClaims
	t, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (interface{}, error) { return getJWTSecret(), nil })
	if err != nil || !t.Valid {
		return "", "", "", fmt.Errorf("invalid token")
	}
	return claims.UserID, claims.Email, claims.Role, nil
}

// ValidateAccessTokenClaims returns parsed access claims, including JTI for token revocation checks.
func ValidateAccessTokenClaims(tokenString string) (AccessClaims, error) {
	var claims AccessClaims
	t, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (interface{}, error) { return getJWTSecret(), nil })
	if err != nil || !t.Valid {
		return AccessClaims{}, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// ValidateRefreshToken returns session_id or error.
func ValidateRefreshToken(tokenString string) (sessionID string, err error) {
	var claims RefreshClaims
	t, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (interface{}, error) { return getJWTSecret(), nil })
	if err != nil || !t.Valid {
		return "", fmt.Errorf("invalid refresh token")
	}
	return claims.SessionID, nil
}

// RefreshSession validates the refresh token (JWT), loads session from DB, returns new access token and user.
func RefreshSession(ctx context.Context, refreshTokenString string) (user gen.User, accessToken string, err error) {
	sessionID, err := ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return gen.User{}, "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	q := db.Queries()
	sess, err := q.GetSessionByID(ctx, gen.GetSessionByIDParams{ID: sessionID, ExpiresAt: now})
	if err != nil {
		if err == sql.ErrNoRows {
			return gen.User{}, "", fmt.Errorf("session expired or invalid")
		}
		return gen.User{}, "", err
	}
	user, err = q.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return gen.User{}, "", err
	}
	accessToken, err = issueAccessToken(user.ID, user.Email, user.Role)
	if err != nil {
		return gen.User{}, "", err
	}
	return user, accessToken, nil
}

// Logout deletes the session (call with session_id from refresh token).
func Logout(ctx context.Context, refreshTokenString string) error {
	sessionID, err := ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return fmt.Errorf("logout validate refresh token: %w", err)
	}
	return db.Queries().DeleteSession(ctx, sessionID)
}

// BlacklistToken revokes an access token (by JTI) until its expiration.
func BlacklistToken(ctx context.Context, jti string, expiresAt time.Time) error {
	if jti == "" {
		return nil
	}
	return db.Queries().CreateTokenBlacklist(ctx, gen.CreateTokenBlacklistParams{
		ID:        uuid.New().String(),
		TokenJti:  jti,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// IsTokenBlacklisted checks whether a token JTI has been revoked and is still active.
func IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	count, err := db.Queries().CountTokenBlacklistByJti(ctx, gen.CountTokenBlacklistByJtiParams{
		TokenJti:  jti,
		ExpiresAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CleanupExpiredBlacklistedTokens removes expired blacklist entries.
func CleanupExpiredBlacklistedTokens(ctx context.Context) error {
	return db.Queries().DeleteExpiredTokenBlacklist(ctx, time.Now().UTC().Format(time.RFC3339))
}

// PasswordResetExpiry is the lifetime of a password-reset token (PRD 3.2.2).
const PasswordResetExpiry = 1 * time.Hour

// RequestPasswordReset inserts a reset token and returns the rawToken and the full reset link.
// The caller is responsible for sending the link to the user via email.
func RequestPasswordReset(ctx context.Context, userID, appURL string) (rawToken, link string, err error) {
	tok := make([]byte, 32)
	if _, err = rand.Read(tok); err != nil {
		return "", "", fmt.Errorf("RequestPasswordReset: generate token: %w", err)
	}
	rawToken = hex.EncodeToString(tok)
	hash := hashToken(rawToken)
	now := time.Now().UTC().Format(time.RFC3339)
	expires := time.Now().Add(PasswordResetExpiry).UTC().Format(time.RFC3339)

	sqlDB := db.DB()
	_, err = sqlDB.ExecContext(ctx,
		`INSERT INTO password_resets (id, user_id, token_hash, used, expires_at, created_at)
		 VALUES (?, ?, ?, 0, ?, ?)`,
		uuid.New().String(), userID, hash, expires, now,
	)
	if err != nil {
		return "", "", fmt.Errorf("RequestPasswordReset: insert: %w", err)
	}
	link = appURL + "/reset-password?token=" + rawToken
	return rawToken, link, nil
}

// ResetPassword validates the reset token and updates the user's password.
// Uses bcrypt at cost 12 per PRD 13.5.
func ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	hash := hashToken(rawToken)
	now := time.Now().UTC().Format(time.RFC3339)
	sqlDB := db.DB()

	row := sqlDB.QueryRowContext(ctx,
		`SELECT id, user_id FROM password_resets WHERE token_hash = ? AND used = 0 AND expires_at > ?`,
		hash, now,
	)
	var resetID, userID string
	if err := row.Scan(&resetID, &userID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid or expired reset link")
		}
		return fmt.Errorf("ResetPassword: lookup: %w", err)
	}

	// Hash the new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return fmt.Errorf("ResetPassword: bcrypt: %w", err)
	}

	// Update the user's password and mark token used — in a transaction
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ResetPassword: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err = tx.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		string(hashed), now, userID,
	); err != nil {
		return fmt.Errorf("ResetPassword: update user: %w", err)
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE password_resets SET used = 1 WHERE id = ?`, resetID,
	); err != nil {
		return fmt.Errorf("ResetPassword: mark used: %w", err)
	}
	return tx.Commit()
}

// ChangePassword updates the authenticated user's password (PRD 6.1 PATCH /auth/password/change).
// If the user has an existing password, currentPassword must match; otherwise currentPassword can be empty.
func ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	var existingHash sql.NullString
	err := db.DB().QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&existingHash)
	if err != nil {
		return err
	}
	if existingHash.Valid && existingHash.String != "" {
		if currentPassword == "" {
			return fmt.Errorf("current password required")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(existingHash.String), []byte(currentPassword)); err != nil {
			return fmt.Errorf("current password is incorrect")
		}
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.DB().ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, string(hashed), now, userID)
	return err
}

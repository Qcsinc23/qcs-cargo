package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
)

const (
	SuiteCodePrefix = "QCS-"
	SuiteCodeLength = 6
	MagicLinkExpiry = 10 * time.Minute
	AccessExpiry    = 15 * time.Minute
	RefreshExpiry   = 7 * 24 * time.Hour
)

var alphanum = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

// GenerateSuiteCode returns QCS-{6 alphanumeric} per PRD 8.10.
func GenerateSuiteCode() (string, error) {
	b := make([]byte, SuiteCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphanum[int(b[i])%len(alphanum)]
	}
	return SuiteCodePrefix + string(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Register creates a user with a new suite code. Returns user or error if email exists.
func Register(ctx context.Context, name, email string) (gen.User, error) {
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
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	return q.CreateUser(ctx, gen.CreateUserParams{
		ID:        id,
		Name:      name,
		Email:     email,
		SuiteCode: sql.NullString{String: suiteCode, Valid: true},
		CreatedAt: now,
		UpdatedAt: now,
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
		ID:        uuid.New().String(),
		UserID:    userID,
		TokenHash: hash,
		RedirectTo: sql.NullString{String: redirectTo, Valid: redirectTo != ""},
		ExpiresAt: expires,
		CreatedAt: now,
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
	if err := q.MarkMagicLinkUsed(ctx, link.ID); err != nil {
		return gen.User{}, "", "", err
	}
	user, err = q.GetUserByID(ctx, link.UserID)
	if err != nil {
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
	if s := os.Getenv("JWT_SECRET"); len(s) >= 32 {
		return []byte(s)[:32]
	}
	return []byte("qcs-dev-secret-change-in-production-32bytes!!")
}

func issueAccessToken(userID, email, role string) (string, error) {
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessExpiry)),
			Subject:   userID,
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
		return nil // no-op if token invalid
	}
	return db.Queries().DeleteSession(ctx, sessionID)
}

package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net"
	"strings"
	"time"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/gofiber/fiber/v2"
)

const CtxAPIKeyID = "api_key_id"
const CtxAPIKeyUserID = "api_key_user_id"

// EnforceAPIKeyIPAccess enforces allow/deny rules for machine calls using X-API-Key.
// Safe default: when no matching rules exist, request is allowed.
func EnforceAPIKeyIPAccess(c *fiber.Ctx) error {
	rawAPIKey := strings.TrimSpace(c.Get("X-API-Key"))
	if rawAPIKey == "" {
		return c.Next()
	}

	hashed := hashAPIKey(rawAPIKey)
	var apiKeyID, userID string
	var expiresAt, revokedAt sql.NullString
	err := db.DB().QueryRowContext(c.Context(), `
SELECT id, user_id, expires_at, revoked_at
FROM api_keys
WHERE key_hash = ?
LIMIT 1
`, hashed).Scan(&apiKeyID, &userID, &expiresAt, &revokedAt)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "Invalid API key"},
		})
	}
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": fiber.Map{"code": "AUTH_CHECK_UNAVAILABLE", "message": "Authentication temporarily unavailable"},
		})
	}

	if revokedAt.Valid && strings.TrimSpace(revokedAt.String) != "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "API key revoked"},
		})
	}
	if isExpired(expiresAt) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "UNAUTHENTICATED", "message": "API key expired"},
		})
	}

	c.Locals(CtxAPIKeyID, apiKeyID)
	c.Locals(CtxAPIKeyUserID, userID)

	rules, err := loadIPRules(c, apiKeyID, userID)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": fiber.Map{"code": "AUTH_CHECK_UNAVAILABLE", "message": "IP rules unavailable"},
		})
	}
	if len(rules) == 0 {
		return c.Next()
	}

	clientIP := clientIPFromRequest(c)
	hasAllow := false
	allowMatch := false
	denyMatch := false

	for _, rule := range rules {
		action := strings.ToLower(strings.TrimSpace(rule.action))
		if action == "allow" {
			hasAllow = true
		}
		if !ipMatches(clientIP, rule.cidr) {
			continue
		}
		if action == "deny" {
			denyMatch = true
		}
		if action == "allow" {
			allowMatch = true
		}
	}

	if denyMatch {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fiber.Map{"code": "FORBIDDEN", "message": "IP not allowed for this API key"},
		})
	}
	if hasAllow && !allowMatch {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fiber.Map{"code": "FORBIDDEN", "message": "IP not allowlisted for this API key"},
		})
	}

	return c.Next()
}

type ipAccessRule struct {
	cidr   string
	action string
}

func loadIPRules(c *fiber.Ctx, apiKeyID, userID string) ([]ipAccessRule, error) {
	rows, err := db.DB().QueryContext(c.Context(), `
SELECT cidr, action
FROM ip_access_rules
WHERE enabled = 1 AND (
    api_key_id = ?
    OR (api_key_id IS NULL AND user_id = ?)
)
`, apiKeyID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]ipAccessRule, 0)
	for rows.Next() {
		var rule ipAccessRule
		if err := rows.Scan(&rule.cidr, &rule.action); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func ipMatches(clientIP, ruleCIDR string) bool {
	clientIP = strings.TrimSpace(clientIP)
	ruleCIDR = strings.TrimSpace(ruleCIDR)
	if clientIP == "" || ruleCIDR == "" {
		return false
	}

	if strings.Contains(ruleCIDR, "/") {
		_, network, err := net.ParseCIDR(ruleCIDR)
		if err != nil {
			return false
		}
		ip := net.ParseIP(clientIP)
		if ip == nil {
			return false
		}
		return network.Contains(ip)
	}

	client := net.ParseIP(clientIP)
	rule := net.ParseIP(ruleCIDR)
	if client != nil && rule != nil {
		return client.Equal(rule)
	}
	return clientIP == ruleCIDR
}

func clientIPFromRequest(c *fiber.Ctx) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		raw := strings.TrimSpace(c.Get(header))
		if raw == "" {
			continue
		}
		if header == "X-Forwarded-For" && strings.Contains(raw, ",") {
			raw = strings.TrimSpace(strings.Split(raw, ",")[0])
		}
		if raw != "" {
			return raw
		}
	}
	return strings.TrimSpace(c.IP())
}

func isExpired(expiresAt sql.NullString) bool {
	if !expiresAt.Valid {
		return false
	}
	expires := strings.TrimSpace(expiresAt.String)
	if expires == "" {
		return false
	}
	ts, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return false
	}
	return time.Now().UTC().After(ts)
}

func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

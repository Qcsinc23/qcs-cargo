package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Pass 2 audit fix H-3: at-rest encryption for sensitive secrets that the
// application must be able to read back (TOTP shared secrets, API key
// material we may add later, etc.). Uses AES-256-GCM with a key derived from
// SECRET_ENCRYPTION_KEY (preferred) or, in non-production environments, a
// deterministic-per-process fallback derived from JWT_SECRET so existing dev
// flows keep working without operational changes.
//
// Stored ciphertext format:    "enc:v1:" + base64(nonce || ciphertext_with_tag)
// Anything not starting with "enc:v1:" is treated as legacy plaintext, decrypted
// transparently, and re-encrypted on next write.

const secretEncryptionPrefix = "enc:v1:"

var (
	secretEncryptionOnce sync.Once
	secretEncryptionKey  []byte
	secretEncryptionErr  error
)

func loadSecretEncryptionKey() ([]byte, error) {
	secretEncryptionOnce.Do(func() {
		raw := strings.TrimSpace(os.Getenv("SECRET_ENCRYPTION_KEY"))
		if raw == "" {
			if IsProductionRuntime() {
				secretEncryptionErr = errors.New("SECRET_ENCRYPTION_KEY must be set in production")
				return
			}
			// Fall back to a deterministic per-process key in dev so existing
			// flows keep working. Tied to the dev JWT secret so a misconfigured
			// production never accidentally hits this path.
			h := sha256.Sum256(devJWTSecret)
			secretEncryptionKey = h[:]
			return
		}
		// Accept hex (64 chars), base64 (44 chars), or any string at least 32
		// bytes long. We always produce a 32-byte key via SHA-256 so operators
		// cannot accidentally pick a too-short key.
		h := sha256.Sum256([]byte(raw))
		secretEncryptionKey = h[:]
	})
	if secretEncryptionErr != nil {
		return nil, secretEncryptionErr
	}
	return secretEncryptionKey, nil
}

// EncryptSecret encrypts a plaintext string for at-rest storage.
func EncryptSecret(plaintext string) (string, error) {
	key, err := loadSecretEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("EncryptSecret: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("EncryptSecret: cipher.NewGCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("EncryptSecret: rand.Read: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return secretEncryptionPrefix + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret reverses EncryptSecret. Strings without the encryption prefix
// are returned as-is so that legacy plaintext rows continue to work; callers
// should re-encrypt such values on the next write.
func DecryptSecret(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if !strings.HasPrefix(stored, secretEncryptionPrefix) {
		return stored, nil
	}
	encoded := strings.TrimPrefix(stored, secretEncryptionPrefix)
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("DecryptSecret: base64: %w", err)
	}
	key, err := loadSecretEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("DecryptSecret: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("DecryptSecret: cipher.NewGCM: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("DecryptSecret: ciphertext too short")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("DecryptSecret: gcm.Open: %w", err)
	}
	return string(plaintext), nil
}

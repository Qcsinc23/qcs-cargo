package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
)

var (
	ErrPasswordTooShort  = errors.New("password must be at least 8 characters")
	ErrPasswordNoUpper   = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLower   = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoNumber  = errors.New("password must contain at least one number")
	ErrPasswordNoSpecial = errors.New("password must contain at least one special character")
	ErrEmailRequired     = errors.New("email is required")
	ErrEmailTooLong      = errors.New("email is too long")
	ErrEmailInvalid      = errors.New("invalid email format")
	ErrPhoneInvalid      = errors.New("invalid phone format")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
var phoneRegex = regexp.MustCompile(`^\+?[1-9][0-9]{7,14}$`)

// ValidatePassword checks password complexity requirements
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return ErrPasswordNoUpper
	}
	if !hasLower {
		return ErrPasswordNoLower
	}
	if !hasNumber {
		return ErrPasswordNoNumber
	}
	if !hasSpecial {
		return ErrPasswordNoSpecial
	}

	return nil
}

// ValidateEmail checks email format validity
func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmailRequired
	}
	if len(email) > 254 {
		return ErrEmailTooLong
	}
	if !emailRegex.MatchString(email) {
		return ErrEmailInvalid
	}
	return nil
}

// ValidatePhone checks E.164-ish phone format, allowing optional leading "+".
func ValidatePhone(phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil
	}
	if !phoneRegex.MatchString(phone) {
		return ErrPhoneInvalid
	}
	return nil
}

// ValidDestinations contains the list of valid destination IDs
var ValidDestinations = map[string]bool{
	"guyana":   true,
	"jamaica":  true,
	"trinidad": true,
	"barbados": true,
	"suriname": true,
}

// ValidateDestination checks if a destination ID is valid
func ValidateDestination(destID string) error {
	destID = strings.ToLower(strings.TrimSpace(destID))
	if destID == "" {
		return errors.New("destination_id is required")
	}
	if ok, err := destinationExistsInDB(destID); err == nil {
		if ok {
			return nil
		}
		return errors.New("invalid destination_id: " + destID)
	}
	if !ValidDestinations[destID] {
		return errors.New("invalid destination_id: " + destID + " (valid: guyana, jamaica, trinidad, barbados, suriname)")
	}
	return nil
}

// ValidateName checks a user display name against length and character class
// rules so that registration / profile updates cannot inject HTML/JS that
// later renders in admin or warehouse UIs. Pass 2 audit fix C-1 (server-side).
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("name is required")
	}
	if len(name) > 80 {
		return "", errors.New("name must be 80 characters or fewer")
	}
	for _, r := range name {
		// Permit letters, marks, numbers, spaces, hyphen, apostrophe, period,
		// comma. Reject angle brackets, quotes, control chars, etc.
		switch r {
		case ' ', '-', '\'', '.', ',':
			continue
		}
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			continue
		}
		return "", errors.New("name contains invalid characters")
	}
	return name, nil
}

// ValidateUploadedFileURL constrains a user-supplied file_url to safe schemes
// and hosts. Pass 2 audit fix H-5.
//
// Rules:
//   - must parse as a URL
//   - scheme must be https
//   - host must be in UPLOAD_HOST_ALLOWLIST (comma-separated env var) when set;
//     otherwise we accept the configured CDN_BASE_URL host or the app host.
//   - length capped at 2048 bytes
func ValidateUploadedFileURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("file_url is required")
	}
	if len(raw) > 2048 {
		return errors.New("file_url is too long")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return errors.New("file_url must be a valid URL")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return errors.New("file_url must use https")
	}
	if parsed.Host == "" {
		return errors.New("file_url is missing a host")
	}
	if isUploadHostAllowed(parsed.Host) {
		return nil
	}
	return errors.New("file_url host is not in the upload allowlist")
}

func isUploadHostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	allowlist := strings.TrimSpace(os.Getenv("UPLOAD_HOST_ALLOWLIST"))
	if allowlist != "" {
		for _, h := range strings.Split(allowlist, ",") {
			if strings.EqualFold(strings.TrimSpace(h), host) {
				return true
			}
		}
		return false
	}
	for _, envName := range []string{"CDN_BASE_URL", "APP_URL"} {
		if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
			if u, err := url.Parse(v); err == nil && strings.EqualFold(u.Host, host) {
				return true
			}
		}
	}
	return false
}

var fileNameRegex = regexp.MustCompile(`^[A-Za-z0-9._\- ()]+$`)

// ValidateFileName ensures user-provided filenames do not contain HTML
// metacharacters or path separators. Pass 2 audit fix H-5.
func ValidateFileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("file_name is required")
	}
	if len(name) > 255 {
		return errors.New("file_name is too long")
	}
	if !fileNameRegex.MatchString(name) {
		return errors.New("file_name contains invalid characters")
	}
	if strings.Contains(name, "..") {
		return errors.New("file_name cannot contain '..'")
	}
	return nil
}

func destinationExistsInDB(destID string) (exists bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			exists = false
			err = fmt.Errorf("destination lookup panic: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = db.Queries().GetActiveDestinationByID(ctx, destID)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

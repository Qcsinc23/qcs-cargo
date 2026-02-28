package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

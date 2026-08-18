package validator

import (
	"errors"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ValidateEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return errors.New("email is required")
	}
	if !emailRegex.MatchString(trimmed) {
		return errors.New("invalid email address format")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 6 {
		return errors.New("password must be at least 6 characters")
	}
	return nil
}

func ValidateRequired(fieldName, value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(fieldName + " is required")
	}
	return nil
}

func ValidatePositive(fieldName string, value int) error {
	if value <= 0 {
		return errors.New(fieldName + " must be greater than 0")
	}
	return nil
}

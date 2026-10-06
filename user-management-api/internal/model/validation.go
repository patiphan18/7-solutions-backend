package model

import (
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
)

func InvalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254 && strings.Contains(email, "@")
}

func ValidateUserID(id string) error {
	if len(id) != 24 {
		return InvalidInput("invalid user ID")
	}

	if _, err := hex.DecodeString(id); err != nil {
		return InvalidInput("invalid user ID")
	}

	return nil
}

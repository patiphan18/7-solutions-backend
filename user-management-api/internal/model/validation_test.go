package model

import (
	"errors"
	"strings"
	"testing"
)

func TestEmailValidation(t *testing.T) {
	if NormalizeEmail(" ALICE@example.com ") != "alice@example.com" {
		t.Fatal("email normalization failed")
	}

	for _, email := range []string{"", "invalid", "Alice <alice@example.com>", "alice@example.com extra", strings.Repeat("a", 255) + "@example.com"} {
		if ValidEmail(email) {
			t.Fatalf("invalid email accepted: %q", email)
		}
	}

	if !ValidEmail("alice@example.com") {
		t.Fatal("valid email rejected")
	}
}

func TestIDValidation(t *testing.T) {
	for _, id := range []string{"", "507f1f77bcf86cd79943901", "zz7f1f77bcf86cd799439011"} {
		if err := ValidateUserID(id); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid ID %q: %v", id, err)
		}
	}

	if err := ValidateUserID("507F1F77BCF86CD799439011"); err != nil {
		t.Fatal(err)
	}
}

package security

import (
	"strings"
	"testing"
)

func TestBcrypt(t *testing.T) {
	hasher := Bcrypt{}

	hash, err := hasher.Hash("password123")
	if err != nil {
		t.Fatal(err)
	}

	if hash == "password123" || !hasher.Compare(hash, "password123") {
		t.Fatal("hashing or verification failed")
	}

	if hasher.Compare(hash, "wrong") || hasher.Compare("invalid hash", "password123") || hasher.Compare("", "missing-account-dummy-password") || hasher.Compare(hash, strings.Repeat("x", 73)) {
		t.Fatal("invalid credentials accepted")
	}

	if _, err := hasher.Hash(strings.Repeat("x", 73)); err == nil {
		t.Fatal("oversized password accepted")
	}
}

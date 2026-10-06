package internal

import (
	"context"
	"errors"
	"testing"
)

func TestNewRejectsInvalidConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := New(); err == nil {
		t.Fatal("invalid configuration accepted")
	}
}

func TestStartServerHonorsCanceledContext(t *testing.T) {
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	application, err := New()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := application.StartServer(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("StartServer() = %v, want context.Canceled", err)
	}
}

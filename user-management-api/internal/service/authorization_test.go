package service

import (
	"context"
	"errors"
	"testing"

	"user-management-api/internal/model"
)

func TestAuthorization(t *testing.T) {
	for _, tc := range []struct {
		role      model.Role
		self      bool
		adminOnly bool
		allowed   bool
	}{
		{model.RoleAdmin, false, true, true},
		{model.RoleUser, true, false, true},
		{model.RoleUser, false, false, false},
		{model.RoleUser, true, true, false},
		{"unknown", true, false, false},
	} {
		ctx := model.WithIdentity(context.Background(), model.Identity{UserID: "self", Role: tc.role})

		id := "other"
		if tc.self {
			id = "self"
		}

		err := authorize(ctx, id, tc.adminOnly)
		if tc.allowed && err != nil {
			t.Fatalf("allowed principal rejected: %v", err)
		}

		if !tc.allowed && !errors.Is(err, model.ErrForbidden) {
			t.Fatalf("unauthorized principal allowed: %v", err)
		}
	}

	if err := authorize(context.Background(), "self", false); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatal("missing identity allowed")
	}
}

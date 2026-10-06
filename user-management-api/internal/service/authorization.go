package service

import (
	"context"
	"strings"

	"user-management-api/internal/model"
	"user-management-api/internal/port/driving"
)

func authorize(ctx context.Context, userID string, adminOnly bool) error {
	identity, ok := model.IdentityFromContext(ctx)
	if !ok {
		return model.ErrUnauthorized
	}

	if identity.Role == model.RoleAdmin {
		return nil
	}

	if identity.Role != model.RoleUser || adminOnly || !strings.EqualFold(identity.UserID, userID) {
		return model.ErrForbidden
	}

	return nil
}

func (s *auth) Create(ctx context.Context, command driving.RegisterCommand) (model.User, error) {
	if err := authorize(ctx, "", true); err != nil {
		return model.User{}, err
	}

	return s.Register(ctx, command)
}

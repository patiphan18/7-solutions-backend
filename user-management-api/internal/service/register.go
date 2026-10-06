package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"user-management-api/internal/model"
	"user-management-api/internal/port/driving"
)

func (s *auth) Register(ctx context.Context, command driving.RegisterCommand) (model.User, error) {
	name, email := strings.TrimSpace(command.Name), model.NormalizeEmail(command.Email)
	if name == "" || len(name) > 100 || !model.ValidEmail(email) || len(command.Password) < 8 || len(command.Password) > 72 {
		return model.User{}, model.InvalidInput("name (1-100 bytes), valid email and password (8-72 bytes) required")
	}

	hash, err := s.passwords.Hash(command.Password)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}

	u := model.User{
		Role:      model.RoleUser,
		Name:      name,
		Email:     email,
		Password:  hash,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repository.Create(ctx, &u); err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	return u, nil
}

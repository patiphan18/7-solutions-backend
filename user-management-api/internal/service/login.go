package service

import (
	"context"
	"errors"
	"fmt"

	"user-management-api/internal/model"
	"user-management-api/internal/port/driving"
)

func (s *auth) Login(ctx context.Context, command driving.LoginCommand) (driving.LoginResult, error) {
	if len(command.Password) < 8 || len(command.Password) > 72 || !model.ValidEmail(model.NormalizeEmail(command.Email)) {
		return driving.LoginResult{}, model.ErrInvalidCredentials
	}

	u, err := s.repository.ByEmail(ctx, model.NormalizeEmail(command.Email))
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return driving.LoginResult{}, fmt.Errorf("find user for login: %w", err)
	}

	validPassword := s.passwords.Compare(u.Password, command.Password)
	if err != nil || !validPassword {
		return driving.LoginResult{}, model.ErrInvalidCredentials
	}

	token, err := s.tokens.Issue(u.ID)
	if err != nil {
		return driving.LoginResult{}, fmt.Errorf("issue access token: %w", err)
	}

	return driving.LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}, nil
}

func (s *auth) Authenticate(ctx context.Context, token string) (model.Identity, error) {
	id, err := s.tokens.Validate(token)
	if err != nil {
		return model.Identity{}, model.ErrUnauthorized
	}

	if err := model.ValidateUserID(id); err != nil {
		return model.Identity{}, model.ErrUnauthorized
	}

	user, err := s.repository.Get(ctx, id)
	if errors.Is(err, model.ErrNotFound) {
		return model.Identity{}, model.ErrUnauthorized
	}

	if err != nil {
		return model.Identity{}, err
	}

	role := user.Role
	if role == "" {
		role = model.RoleUser
	}

	if role != model.RoleUser && role != model.RoleAdmin {
		return model.Identity{}, model.ErrUnauthorized
	}

	return model.Identity{UserID: user.ID, Role: role}, nil
}

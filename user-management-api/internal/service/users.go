package service

import (
	"context"
	"strings"

	"user-management-api/internal/model"
	"user-management-api/internal/port/driving"
)

func (s *auth) Get(ctx context.Context, id string) (model.User, error) {
	if err := model.ValidateUserID(id); err != nil {
		return model.User{}, err
	}

	if err := authorize(ctx, id, false); err != nil {
		return model.User{}, err
	}

	return s.repository.Get(ctx, id)
}

func (s *auth) List(ctx context.Context) ([]model.User, error) {
	if err := authorize(ctx, "", true); err != nil {
		return nil, err
	}

	return s.repository.List(ctx)
}

func (s *auth) Update(ctx context.Context, id string, command driving.UpdateUserCommand) (model.User, error) {
	if err := model.ValidateUserID(id); err != nil {
		return model.User{}, err
	}

	if err := authorize(ctx, id, false); err != nil {
		return model.User{}, err
	}

	name, email := "", ""

	if command.Name == nil && command.Email == nil {
		return model.User{}, model.InvalidInput("name or email required")
	}

	if command.Name != nil {
		name = strings.TrimSpace(*command.Name)
		if name == "" || len(name) > 100 {
			return model.User{}, model.InvalidInput("invalid name")
		}
	}

	if command.Email != nil {
		email = model.NormalizeEmail(*command.Email)
		if !model.ValidEmail(email) {
			return model.User{}, model.InvalidInput("invalid email")
		}
	}

	return s.repository.Update(ctx, id, name, email)
}

func (s *auth) Delete(ctx context.Context, id string) error {
	if err := model.ValidateUserID(id); err != nil {
		return err
	}

	if err := authorize(ctx, id, false); err != nil {
		return err
	}

	return s.repository.Delete(ctx, id)
}

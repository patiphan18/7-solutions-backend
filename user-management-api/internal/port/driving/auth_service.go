package driving

import (
	"context"

	"user-management-api/internal/model"
)

type AuthService interface {
	Register(context.Context, RegisterCommand) (model.User, error)
	Login(context.Context, LoginCommand) (LoginResult, error)
	Authenticate(context.Context, string) (model.Identity, error)
}

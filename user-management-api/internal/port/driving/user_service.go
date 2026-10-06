package driving

import (
	"context"

	"user-management-api/internal/model"
)

type UserService interface {
	Create(context.Context, RegisterCommand) (model.User, error)
	Get(context.Context, string) (model.User, error)
	List(context.Context) ([]model.User, error)
	Update(context.Context, string, UpdateUserCommand) (model.User, error)
	Delete(context.Context, string) error
}

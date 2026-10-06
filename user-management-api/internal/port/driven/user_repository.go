package driven

import (
	"context"

	"user-management-api/internal/model"
)

type UserRepository interface {
	Create(context.Context, *model.User) error
	Get(context.Context, string) (model.User, error)
	ByEmail(context.Context, string) (model.User, error)
	List(context.Context) ([]model.User, error)
	Update(context.Context, string, string, string) (model.User, error)
	Delete(context.Context, string) error
	Count(context.Context) (int64, error)
}

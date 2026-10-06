package model

import "context"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type Identity struct {
	UserID string
	Role   Role
}

type identityKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok
}

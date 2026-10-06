package service

import (
	"user-management-api/internal/port/driven"
	"user-management-api/internal/port/driving"
)

type auth struct {
	repository driven.UserRepository
	tokens     driven.TokenProvider
	passwords  driven.PasswordHasher
}

var _ driving.AuthService = (*auth)(nil)
var _ driving.UserService = (*auth)(nil)

type AuthDeps struct {
	Repository driven.UserRepository
	Tokens     driven.TokenProvider
	Passwords  driven.PasswordHasher
}

func NewAuth(deps AuthDeps) *auth {
	return &auth{
		repository: deps.Repository,
		tokens:     deps.Tokens,
		passwords:  deps.Passwords,
	}
}

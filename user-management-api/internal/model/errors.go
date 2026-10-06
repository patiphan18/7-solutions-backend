package model

import "errors"

var (
	ErrNotFound           = errors.New("user not found")
	ErrConflict           = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("invalid or expired token")
	ErrForbidden          = errors.New("permission denied")
)

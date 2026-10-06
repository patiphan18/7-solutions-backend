package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"user-management-api/internal/model"
	"user-management-api/internal/port/driven"
)

type JWT struct {
	Secret []byte
}

var _ driven.TokenProvider = (*JWT)(nil)

func (j *JWT) Issue(subject string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    "user-management-api",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.Secret)
}

func (j *JWT) Validate(raw string) (string, error) {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(*jwt.Token) (any, error) {
			return j.Secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer("user-management-api"),
	)
	if err != nil || !token.Valid || claims.Subject == "" {
		return "", model.ErrUnauthorized
	}

	return claims.Subject, nil
}

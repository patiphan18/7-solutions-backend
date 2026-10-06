package service

import (
	"context"
	"errors"
	"testing"

	"user-management-api/internal/model"
	"user-management-api/internal/port/driven"
	"user-management-api/internal/port/driving"
)

type repositoryStub struct {
	driven.UserRepository
	user model.User
	err  error
}

func (r *repositoryStub) Create(_ context.Context, u *model.User) error {
	if r.err != nil {
		return r.err
	}

	u.ID = "507f1f77bcf86cd799439011"
	r.user = *u

	return nil
}

func (r *repositoryStub) ByEmail(_ context.Context, email string) (model.User, error) {
	if email != r.user.Email && r.err == nil {
		return model.User{}, model.ErrNotFound
	}

	return r.user, r.err
}

func (r *repositoryStub) Get(context.Context, string) (model.User, error) {
	return r.user, r.err
}

type passwordStub struct {
	err error
}

func (p passwordStub) Hash(password string) (string, error) {
	return "hashed:" + password, p.err
}

func (passwordStub) Compare(hash, password string) bool {
	return hash == "hashed:"+password
}

type tokenStub struct {
	subject string
	err     error
}

func (t *tokenStub) Issue(subject string) (string, error) {
	t.subject = subject
	return "token", t.err
}

func (t *tokenStub) Validate(string) (string, error) {
	return t.subject, t.err
}

func TestRegisterAndLoginThroughPorts(t *testing.T) {
	repo, tokens := &repositoryStub{}, &tokenStub{subject: "507f1f77bcf86cd799439011"}
	s := &auth{repository: repo, tokens: tokens, passwords: passwordStub{}}
	ctx := model.WithIdentity(context.Background(), model.Identity{UserID: "507f1f77bcf86cd799439011", Role: model.RoleAdmin})

	u, err := s.Register(ctx, driving.RegisterCommand{
		Name:     " Alice ",
		Email:    "ALICE@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}

	if u.Name != "Alice" || u.Email != "alice@example.com" || u.Password != "hashed:password123" || u.CreatedAt.IsZero() {
		t.Fatalf("unexpected user: %+v", u)
	}

	result, err := s.Login(ctx, driving.LoginCommand{
		Email:    " ALICE@example.com ",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.AccessToken != "token" || result.ExpiresIn != 3600 || tokens.subject != u.ID {
		t.Fatalf("unexpected login result: %+v", result)
	}

	if _, err := s.Authenticate(ctx, "token"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Login(ctx, driving.LoginCommand{
		Email:    u.Email,
		Password: "wrong",
	}); !errors.Is(err, model.ErrInvalidCredentials) {
		t.Fatalf("wrong password error: %v", err)
	}

	repo.err = model.ErrNotFound
	if _, err := s.Authenticate(ctx, "token"); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatalf("deleted user error: %v", err)
	}
}

func TestDependencyFailuresArePropagated(t *testing.T) {
	failure := errors.New("dependency unavailable")
	ctx := model.WithIdentity(context.Background(), model.Identity{UserID: "507f1f77bcf86cd799439011", Role: model.RoleAdmin})
	command := driving.RegisterCommand{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "password123",
	}
	repo, tokens := &repositoryStub{}, &tokenStub{subject: "507f1f77bcf86cd799439011"}

	s := &auth{repository: repo, tokens: tokens, passwords: passwordStub{err: failure}}
	if _, err := s.Register(ctx, command); !errors.Is(err, failure) {
		t.Fatalf("hash error: %v", err)
	}

	s.passwords = passwordStub{}

	repo.err = failure
	if _, err := s.Register(ctx, command); !errors.Is(err, failure) {
		t.Fatalf("repository error: %v", err)
	}

	if _, err := s.Login(ctx, driving.LoginCommand{
		Email:    command.Email,
		Password: command.Password,
	}); !errors.Is(err, failure) {
		t.Fatalf("login repository error: %v", err)
	}

	if _, err := s.Authenticate(ctx, "token"); !errors.Is(err, failure) {
		t.Fatalf("authentication repository error: %v", err)
	}

	repo.err = nil
	if _, err := s.Register(ctx, command); err != nil {
		t.Fatal(err)
	}

	tokens.err = failure
	if _, err := s.Login(ctx, driving.LoginCommand{
		Email:    command.Email,
		Password: command.Password,
	}); !errors.Is(err, failure) {
		t.Fatalf("token issue error: %v", err)
	}

	if _, err := s.Authenticate(ctx, "token"); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatalf("token validation error: %v", err)
	}
}

func TestInvalidInputDoesNotCallRepository(t *testing.T) {
	// A nil repository makes accidental persistence calls fail immediately.
	s := &auth{}

	ctx := model.WithIdentity(context.Background(), model.Identity{UserID: "507f1f77bcf86cd799439011", Role: model.RoleAdmin})
	if _, err := s.Register(ctx, driving.RegisterCommand{}); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("register error: %v", err)
	}

	if _, err := s.Get(ctx, "invalid"); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("get error: %v", err)
	}

	if err := s.Delete(ctx, "invalid"); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("delete error: %v", err)
	}

	if _, err := s.Update(ctx, "507f1f77bcf86cd799439011", driving.UpdateUserCommand{}); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("update error: %v", err)
	}
}

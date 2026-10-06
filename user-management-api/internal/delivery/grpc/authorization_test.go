package grpcadapter

import (
	"context"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"user-management-api/internal/lib/security"
	"user-management-api/internal/model"
	"user-management-api/internal/port/driven"
	"user-management-api/internal/service"
	pb "user-management-api/pkg/proto/usermanagement/v1"
)

const ownerID = "507f1f77bcf86cd799439011"
const adminID = "507f1f77bcf86cd799439012"

type authorizationRepository struct {
	driven.UserRepository
	mu    sync.Mutex
	users map[string]model.User
}

func (r *authorizationRepository) Get(_ context.Context, id string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[id]
	if !ok {
		return model.User{}, model.ErrNotFound
	}

	return user, nil
}

func (r *authorizationRepository) List(context.Context) ([]model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	users := make([]model.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}

	return users, nil
}

func TestGRPCOwnerAndAdminAuthorization(t *testing.T) {
	repository := &authorizationRepository{users: map[string]model.User{
		ownerID: {ID: ownerID, Role: model.RoleUser},
		adminID: {ID: adminID, Role: model.RoleAdmin},
	}}
	tokens := &security.JWT{Secret: []byte("01234567890123456789012345678901")}
	application := service.NewAuth(service.AuthDeps{Repository: repository, Tokens: tokens, Passwords: security.Bcrypt{}})
	client := newClient(t, application)

	token, err := tokens.Issue(ownerID)
	if err != nil {
		t.Fatal(err)
	}

	owner := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token))

	own, err := client.GetUser(owner, &pb.GetUserRequest{Id: ownerID})
	if err != nil || own.GetUser().GetRole() != "user" {
		t.Fatalf("own user: %v %v", own, err)
	}

	for name, call := range map[string]func() error{
		"list":         func() error { _, err := client.ListUsers(owner, &pb.ListUsersRequest{}); return err },
		"create":       func() error { _, err := client.CreateUser(owner, &pb.CreateUserRequest{Name: "Injected"}); return err },
		"read other":   func() error { _, err := client.GetUser(owner, &pb.GetUserRequest{Id: adminID}); return err },
		"update other": func() error { _, err := client.UpdateUser(owner, &pb.UpdateUserRequest{Id: adminID}); return err },
		"delete other": func() error { _, err := client.DeleteUser(owner, &pb.DeleteUserRequest{Id: adminID}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); status.Code(err) != codes.PermissionDenied {
				t.Fatalf("error: %v", err)
			}
		})
	}

	token, err = tokens.Issue(adminID)
	if err != nil {
		t.Fatal(err)
	}

	admin := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+token))
	if _, err := client.ListUsers(admin, &pb.ListUsersRequest{}); err != nil {
		t.Fatal(err)
	}

	repository.mu.Lock()
	repository.users[adminID] = model.User{ID: adminID, Role: model.RoleUser}
	repository.mu.Unlock()

	if _, err := client.ListUsers(admin, &pb.ListUsersRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("demoted admin: %v", err)
	}

	repository.mu.Lock()
	repository.users[adminID] = model.User{ID: adminID, Role: "unknown"}
	repository.mu.Unlock()

	if _, err := client.ListUsers(admin, &pb.ListUsersRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unknown role: %v", err)
	}
}

package grpcadapter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"user-management-api/internal/model"
	"user-management-api/internal/port/driving"
	pb "user-management-api/pkg/proto/usermanagement/v1"
)

type serviceStub struct {
	driving.AuthService
	driving.UserService
	user    model.User
	failure error
}

func (s *serviceStub) Register(_ context.Context, command driving.RegisterCommand) (model.User, error) {
	if command.Name == "" {
		return model.User{}, model.ErrInvalidInput
	}

	s.user = model.User{ID: "507f1f77bcf86cd799439011", Name: command.Name, Email: command.Email, CreatedAt: time.Now().UTC()}

	return s.user, nil
}

func (s *serviceStub) Login(context.Context, driving.LoginCommand) (driving.LoginResult, error) {
	return driving.LoginResult{AccessToken: "good", TokenType: "Bearer", ExpiresIn: 3600}, nil
}

func (s *serviceStub) Authenticate(_ context.Context, token string) (model.Identity, error) {
	if token != "good" {
		return model.Identity{}, model.ErrUnauthorized
	}

	return model.Identity{UserID: s.user.ID, Role: model.RoleAdmin}, nil
}

func (s *serviceStub) Get(_ context.Context, id string) (model.User, error) {
	if s.failure != nil {
		return model.User{}, s.failure
	}

	if id != s.user.ID {
		return model.User{}, model.ErrNotFound
	}

	return s.user, nil
}

func (s *serviceStub) List(context.Context) ([]model.User, error) { return []model.User{s.user}, nil }

func (s *serviceStub) Update(_ context.Context, _ string, command driving.UpdateUserCommand) (model.User, error) {
	if command.Name != nil {
		s.user.Name = *command.Name
	}

	if command.Email != nil {
		s.user.Email = *command.Email
	}

	return s.user, nil
}

func (s *serviceStub) Delete(context.Context, string) error { s.user = model.User{}; return nil }

func newClient(t *testing.T, application interface {
	driving.AuthService
	driving.UserService
}) pb.UserManagementServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)

	server := NewServer(application, application, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = server.Serve(listener) }()

	connection, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = connection.Close(); server.Stop(); _ = listener.Close() })

	return pb.NewUserManagementServiceClient(connection)
}

func TestGRPCTransportAndAuthentication(t *testing.T) {
	application := &serviceStub{}
	client := newClient(t, application)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	registered, err := client.Register(ctx, &pb.RegisterRequest{Name: "Alice", Email: "alice@example.com", Password: "password123"})
	if err != nil || registered.GetUser().GetId() == "" {
		t.Fatalf("Register(): %v, %v", registered, err)
	}

	login, err := client.Login(ctx, &pb.LoginRequest{Email: "alice@example.com", Password: "password123"})
	if err != nil || login.GetAccessToken() != "good" {
		t.Fatalf("Login(): %v, %v", login, err)
	}

	if _, err := client.GetUser(ctx, &pb.GetUserRequest{Id: registered.User.Id}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing metadata: %v", err)
	}

	bad := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer wrong"))
	if _, err := client.ListUsers(bad, &pb.ListUsersRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("invalid token: %v", err)
	}

	authorized := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer good"))
	if _, err := client.CreateUser(authorized, &pb.CreateUserRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid input: %v", err)
	}

	if _, err := client.CreateUser(authorized, &pb.CreateUserRequest{Name: "Bob", Email: "bob@example.com", Password: "password123"}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.GetUser(authorized, &pb.GetUserRequest{Id: registered.User.Id}); err != nil {
		t.Fatal(err)
	}

	listed, err := client.ListUsers(authorized, &pb.ListUsersRequest{})
	if err != nil || len(listed.GetUsers()) != 1 {
		t.Fatalf("ListUsers(): %v, %v", listed, err)
	}

	name := "Updated"

	updated, err := client.UpdateUser(authorized, &pb.UpdateUserRequest{Id: registered.User.Id, Name: &name})
	if err != nil || updated.GetUser().GetName() != name {
		t.Fatalf("UpdateUser(): %v, %v", updated, err)
	}

	if _, err := client.DeleteUser(authorized, &pb.DeleteUserRequest{Id: registered.User.Id}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.GetUser(authorized, &pb.GetUserRequest{Id: registered.User.Id}); status.Code(err) != codes.NotFound {
		t.Fatalf("missing user: %v", err)
	}

	application.failure = errors.New("private database detail")

	if _, err := client.GetUser(authorized, &pb.GetUserRequest{}); status.Code(err) != codes.Internal || strings.Contains(err.Error(), "private") {
		t.Fatalf("internal error exposed: %v", err)
	}

	if _, err := client.Register(ctx, &pb.RegisterRequest{Name: strings.Repeat("x", 9000)}); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("message limit: %v", err)
	}
}

func (s *serviceStub) Create(ctx context.Context, command driving.RegisterCommand) (model.User, error) {
	return s.Register(ctx, command)
}

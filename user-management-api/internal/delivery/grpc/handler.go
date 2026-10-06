package grpcadapter

import (
	"context"

	"user-management-api/internal/port/driving"
	pb "user-management-api/pkg/proto/usermanagement/v1"
)

type handler struct {
	pb.UnimplementedUserManagementServiceServer
	auth  driving.AuthService
	users driving.UserService
}

func (h *handler) Register(ctx context.Context, request *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	user, err := h.auth.Register(ctx, driving.RegisterCommand{
		Name:     request.GetName(),
		Email:    request.GetEmail(),
		Password: request.GetPassword(),
	})
	if err != nil {
		return nil, err
	}

	return &pb.RegisterResponse{User: mapUser(user)}, nil
}

func (h *handler) Login(ctx context.Context, request *pb.LoginRequest) (*pb.LoginResponse, error) {
	result, err := h.auth.Login(ctx, driving.LoginCommand{
		Email:    request.GetEmail(),
		Password: request.GetPassword(),
	})
	if err != nil {
		return nil, err
	}

	return &pb.LoginResponse{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		ExpiresIn:   result.ExpiresIn,
	}, nil
}

func (h *handler) CreateUser(ctx context.Context, request *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	user, err := h.users.Create(ctx, driving.RegisterCommand{
		Name:     request.GetName(),
		Email:    request.GetEmail(),
		Password: request.GetPassword(),
	})
	if err != nil {
		return nil, err
	}

	return &pb.CreateUserResponse{User: mapUser(user)}, nil
}

func (h *handler) GetUser(ctx context.Context, request *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := h.users.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	return &pb.GetUserResponse{User: mapUser(user)}, nil
}

func (h *handler) ListUsers(ctx context.Context, _ *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users, err := h.users.List(ctx)
	if err != nil {
		return nil, err
	}

	response := &pb.ListUsersResponse{Users: make([]*pb.User, 0, len(users))}
	for _, user := range users {
		response.Users = append(response.Users, mapUser(user))
	}

	return response, nil
}

func (h *handler) UpdateUser(ctx context.Context, request *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	user, err := h.users.Update(ctx, request.GetId(), driving.UpdateUserCommand{
		Name:  request.Name,
		Email: request.Email,
	})
	if err != nil {
		return nil, err
	}

	return &pb.UpdateUserResponse{User: mapUser(user)}, nil
}

func (h *handler) DeleteUser(ctx context.Context, request *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	if err := h.users.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}

	return &pb.DeleteUserResponse{}, nil
}

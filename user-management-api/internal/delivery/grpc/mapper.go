package grpcadapter

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"user-management-api/internal/model"
	pb "user-management-api/pkg/proto/usermanagement/v1"
)

func mapUser(user model.User) *pb.User {
	return &pb.User{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: timestamppb.New(user.CreatedAt),
		Role:      string(user.Role),
	}
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, model.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, model.ErrInvalidCredentials), errors.Is(err, model.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, "invalid credentials or token")
	case errors.Is(err, model.ErrNotFound):
		return status.Error(codes.NotFound, "user not found")
	case errors.Is(err, model.ErrConflict):
		return status.Error(codes.AlreadyExists, "email already exists")
	case errors.Is(err, model.ErrForbidden):
		return status.Error(codes.PermissionDenied, "permission denied")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

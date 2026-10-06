package grpcadapter

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"user-management-api/internal/model"
	"user-management-api/internal/port/driving"
	pb "user-management-api/pkg/proto/usermanagement/v1"
)

func NewServer(auth driving.AuthService, users driving.UserService, logger *slog.Logger) *grpc.Server {
	interceptor := func(ctx context.Context, request any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (response any, result error) {
		start := time.Now()
		defer func() {
			logger.Info("grpc request", "method", info.FullMethod, "code", status.Code(result).String(), "duration", time.Since(start))
		}()

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if info.FullMethod != pb.UserManagementService_Register_FullMethodName && info.FullMethod != pb.UserManagementService_Login_FullMethodName {
			incoming, _ := metadata.FromIncomingContext(ctx)

			values := incoming.Get("authorization")
			if len(values) != 1 {
				return nil, status.Error(codes.Unauthenticated, "Bearer token required")
			}

			parts := strings.Fields(values[0])
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return nil, status.Error(codes.Unauthenticated, "Bearer token required")
			}

			identity, err := auth.Authenticate(ctx, parts[1])
			if err != nil {
				return nil, mapError(err)
			}

			ctx = model.WithIdentity(ctx, identity)
		}

		response, err := next(ctx, request)

		result = mapError(err)
		if status.Code(result) == codes.Internal {
			logger.Error("grpc service operation failed", "error", err)
		}

		return response, result
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(interceptor), grpc.MaxRecvMsgSize(8192), grpc.MaxHeaderListSize(16*1024))
	pb.RegisterUserManagementServiceServer(server, &handler{auth: auth, users: users})
	reflection.Register(server)

	return server
}

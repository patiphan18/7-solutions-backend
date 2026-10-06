package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/grpc"
	"user-management-api/internal/config"
	grpcadapter "user-management-api/internal/delivery/grpc"
	httpadapter "user-management-api/internal/delivery/http"
	"user-management-api/internal/infra/repo"
	"user-management-api/internal/lib/security"
	"user-management-api/internal/port/driven"
	"user-management-api/internal/service"
)

type Dependencies struct {
	mongoClient *mongo.Client
	Repository  driven.UserRepository
	Handler     http.Handler
	GRPCServer  *grpc.Server
}

// Close must be called to release the owned MongoDB connection.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Dependencies, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		disconnect(client)
		return nil, err
	}

	repository := &repo.Mongo{Collection: client.Database(cfg.MongoDatabase).Collection("users")}
	if err := repository.EnsureIndexes(ctx); err != nil {
		disconnect(client)
		return nil, err
	}

	application := service.NewAuth(service.AuthDeps{
		Repository: repository,
		Tokens:     &security.JWT{Secret: []byte(cfg.JWTSecret)},
		Passwords:  security.Bcrypt{},
	})
	delivery := &httpadapter.API{
		Auth:   application,
		Users:  application,
		Logger: logger,
	}

	return &Dependencies{
		mongoClient: client,
		Repository:  repository,
		Handler:     delivery.Handler(),
		GRPCServer:  grpcadapter.NewServer(application, application, logger),
	}, nil
}

func disconnect(client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = client.Disconnect(ctx)
}

func (b *Dependencies) Close(ctx context.Context) error {
	return b.mongoClient.Disconnect(ctx)
}

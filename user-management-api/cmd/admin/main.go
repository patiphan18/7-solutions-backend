package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"user-management-api/internal/config"
	"user-management-api/internal/infra/repo"
	"user-management-api/internal/lib/security"
	"user-management-api/internal/model"
)

func main() {
	if err := run(); err != nil {
		slog.Error("failed to create admin", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	name := strings.TrimSpace(os.Getenv("ADMIN_NAME"))
	email := model.NormalizeEmail(os.Getenv("ADMIN_EMAIL"))

	password := os.Getenv("ADMIN_PASSWORD")
	if name == "" || len(name) > 100 || !model.ValidEmail(email) || len(password) < 8 || len(password) > 72 {
		return model.InvalidInput("set ADMIN_NAME, ADMIN_EMAIL and ADMIN_PASSWORD (8-72 bytes)")
	}

	hash, err := (security.Bcrypt{}).Hash(password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = client.Disconnect(cleanup)
	}()

	if err := client.Ping(ctx, nil); err != nil {
		return err
	}

	repository := &repo.Mongo{Collection: client.Database(cfg.MongoDatabase).Collection("users")}
	if err := repository.EnsureIndexes(ctx); err != nil {
		return err
	}

	user := model.User{Role: model.RoleAdmin, Name: name, Email: email, Password: hash, CreatedAt: time.Now().UTC()}
	if err := repository.Create(ctx, &user); err != nil {
		return err
	}

	slog.Info("admin created", "id", user.ID)

	return nil
}

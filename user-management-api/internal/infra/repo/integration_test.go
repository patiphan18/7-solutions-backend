package repo

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"user-management-api/internal/model"
)

func TestMongoIntegration(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("set MONGO_TEST_URI to run the real MongoDB integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}

	database := client.Database("user_management_test_" + bson.NewObjectID().Hex())

	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := database.Drop(cleanup); err != nil {
			t.Errorf("drop isolated test database: %v", err)
		}

		if err := client.Disconnect(cleanup); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	})

	if err := client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}

	repository := &Mongo{Collection: database.Collection("users")}
	if err := repository.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}

	if err := repository.EnsureIndexes(ctx); err != nil {
		t.Fatalf("index migration not idempotent: %v", err)
	}

	u := model.User{
		Name:      "Alice",
		Email:     "alice@example.com",
		Password:  "hashed",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := repository.Create(ctx, &u); err != nil {
		t.Fatal(err)
	}

	if err := model.ValidateUserID(u.ID); err != nil {
		t.Fatal(err)
	}

	duplicate := u
	if err := repository.Create(ctx, &duplicate); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("duplicate email: %v", err)
	}

	found, err := repository.Get(ctx, u.ID)
	if err != nil || found != u {
		t.Fatalf("Get() = %+v, %v, want %+v", found, err, u)
	}

	found, err = repository.ByEmail(ctx, u.Email)
	if err != nil || found.ID != u.ID {
		t.Fatalf("ByEmail() = %+v, %v", found, err)
	}

	updated, err := repository.Update(ctx, u.ID, "Updated", "updated@example.com")
	if err != nil || updated.Name != "Updated" || updated.Email != "updated@example.com" {
		t.Fatalf("Update() = %+v, %v", updated, err)
	}

	users, err := repository.List(ctx)
	if err != nil || len(users) != 1 {
		t.Fatalf("List() = %+v, %v", users, err)
	}

	count, err := repository.Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("Count() = %d, %v", count, err)
	}

	if err := repository.Delete(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.Get(ctx, u.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Get deleted user: %v", err)
	}

	if err := repository.Delete(ctx, u.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("Delete missing user: %v", err)
	}
}

package repo

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"user-management-api/internal/model"
)

func TestTranslate(t *testing.T) {
	unexpected := errors.New("connection failure")
	for _, tc := range []struct {
		name        string
		input, want error
	}{
		{"success", nil, nil},
		{"not found", mongo.ErrNoDocuments, model.ErrNotFound},
		{"duplicate email", mongo.WriteException{WriteErrors: mongo.WriteErrors{{Code: 11000}}}, model.ErrConflict},
		{"unexpected", unexpected, unexpected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := translate(tc.input); !errors.Is(got, tc.want) {
				t.Fatalf("translate(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestLegacyDocumentDefaultsToOrdinaryUser(t *testing.T) {
	if got := (document{}).user().Role; got != model.RoleUser {
		t.Fatalf("legacy role = %q", got)
	}
}

func TestCreateRejectsUnknownRoleBeforeDatabase(t *testing.T) {
	repository := &Mongo{}

	user := model.User{Role: "superuser"}
	if err := repository.Create(t.Context(), &user); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("unknown role error: %v", err)
	}
}

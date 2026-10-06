package repo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"user-management-api/internal/migration"
	"user-management-api/internal/model"
	"user-management-api/internal/port/driven"
)

type document struct {
	Role      model.Role    `bson:"role"`
	ID        bson.ObjectID `bson:"_id"`
	Name      string        `bson:"name"`
	Email     string        `bson:"email"`
	Password  string        `bson:"password"`
	CreatedAt time.Time     `bson:"created_at"`
}

func (d document) user() model.User {
	role := d.Role
	if role == "" {
		role = model.RoleUser
	}

	return model.User{
		Role:      role,
		ID:        d.ID.Hex(),
		Name:      d.Name,
		Email:     d.Email,
		Password:  d.Password,
		CreatedAt: d.CreatedAt,
	}
}

var _ driven.UserRepository = (*Mongo)(nil)

type Mongo struct {
	Collection *mongo.Collection
}

func (m *Mongo) EnsureIndexes(ctx context.Context) error {
	return migration.EnsureUsersIndexes(ctx, m.Collection)
}

func translate(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.ErrNotFound
	}

	if mongo.IsDuplicateKeyError(err) {
		return model.ErrConflict
	}

	return err
}

func (m *Mongo) Create(ctx context.Context, u *model.User) error {
	if u.Role == "" {
		u.Role = model.RoleUser
	}

	if u.Role != model.RoleUser && u.Role != model.RoleAdmin {
		return model.ErrInvalidInput
	}

	doc := document{
		Role:      u.Role,
		ID:        bson.NewObjectID(),
		Name:      u.Name,
		Email:     u.Email,
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
	}

	_, err := m.Collection.InsertOne(ctx, doc)
	if err == nil {
		u.ID = doc.ID.Hex()
	}

	return translate(err)
}

func (m *Mongo) Get(ctx context.Context, id string) (model.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return model.User{}, model.ErrInvalidInput
	}

	var u document

	err = m.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&u)
	if err != nil {
		return model.User{}, translate(err)
	}

	return u.user(), nil
}

func (m *Mongo) ByEmail(ctx context.Context, email string) (model.User, error) {
	var u document

	err := m.Collection.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if err != nil {
		return model.User{}, translate(err)
	}

	return u.user(), nil
}

func (m *Mongo) List(ctx context.Context) ([]model.User, error) {
	cursor, err := m.Collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	docs := []document{}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	users := make([]model.User, 0, len(docs))
	for _, doc := range docs {
		users = append(users, doc.user())
	}

	return users, nil
}

func (m *Mongo) Update(ctx context.Context, id string, name, email string) (model.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return model.User{}, model.ErrInvalidInput
	}

	fields := bson.M{}
	if name != "" {
		fields["name"] = name
	}

	if email != "" {
		fields["email"] = email
	}

	var u document

	err = m.Collection.FindOneAndUpdate(ctx, bson.M{"_id": objectID}, bson.M{"$set": fields}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&u)
	if err != nil {
		return model.User{}, translate(err)
	}

	return u.user(), nil
}

func (m *Mongo) Delete(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return model.ErrInvalidInput
	}

	r, err := m.Collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return err
	}

	if r.DeletedCount == 0 {
		return model.ErrNotFound
	}

	return nil
}

func (m *Mongo) Count(ctx context.Context) (int64, error) {
	return m.Collection.CountDocuments(ctx, bson.M{})
}

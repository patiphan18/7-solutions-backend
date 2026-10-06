package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
	"user-management-api/internal/lib/security"
	"user-management-api/internal/model"
	"user-management-api/internal/service"
)

type memory struct {
	users   map[string]model.User
	failure error
}

func (m *memory) Create(_ context.Context, u *model.User) error {
	if m.failure != nil {
		return m.failure
	}

	for _, v := range m.users {
		if v.Email == u.Email {
			return model.ErrConflict
		}
	}

	u.ID = bson.NewObjectID().Hex()
	m.users[u.ID] = *u

	return nil
}

func (m *memory) Get(_ context.Context, id string) (model.User, error) {
	if m.failure != nil {
		return model.User{}, m.failure
	}

	v, ok := m.users[id]
	if !ok {
		return v, model.ErrNotFound
	}

	return v, nil
}

func (m *memory) ByEmail(_ context.Context, email string) (model.User, error) {
	if m.failure != nil {
		return model.User{}, m.failure
	}

	for _, v := range m.users {
		if v.Email == email {
			return v, nil
		}
	}

	return model.User{}, model.ErrNotFound
}

func (m *memory) List(context.Context) ([]model.User, error) {
	if m.failure != nil {
		return nil, m.failure
	}

	v := []model.User{}
	for _, u := range m.users {
		v = append(v, u)
	}

	return v, nil
}

func (m *memory) Update(ctx context.Context, id string, name, email string) (model.User, error) {
	u, e := m.Get(ctx, id)
	if e != nil {
		return u, e
	}

	for other, v := range m.users {
		if other != id && email != "" && v.Email == email {
			return u, model.ErrConflict
		}
	}

	if name != "" {
		u.Name = name
	}

	if email != "" {
		u.Email = email
	}

	m.users[id] = u

	return u, nil
}

func (m *memory) Delete(ctx context.Context, id string) error {
	if _, e := m.Get(ctx, id); e != nil {
		return e
	}

	delete(m.users, id)

	return nil
}

func (m *memory) Count(context.Context) (int64, error) {
	return int64(len(m.users)), m.failure
}

func setup() (*security.JWT, *memory, http.Handler) {
	m := &memory{users: map[string]model.User{}}
	tokens := &security.JWT{Secret: []byte(strings.Repeat("s", 32))}
	service := service.NewAuth(service.AuthDeps{
		Repository: m,
		Tokens:     tokens,
		Passwords:  security.Bcrypt{},
	})
	a := &API{
		Auth:   service,
		Users:  service,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	return tokens, m, a.Handler()
}

func request(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	return w
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()

	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
	}
}

func registerLogin(t *testing.T, h http.Handler, repositories ...*memory) (model.User, string) {
	t.Helper()

	w := request(h, "POST", "/auth/register", `{"name":" Alice ","email":"ALICE@example.com","password":"password123"}`, "")
	expect(t, w, 201)

	var u model.User
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}

	w = request(h, "POST", "/auth/login", `{"email":"alice@example.com","password":"password123"}`, "")
	expect(t, w, 200)

	var response struct {
		Token string `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if len(repositories) > 0 {
		admin := repositories[0].users[u.ID]
		admin.Role = model.RoleAdmin
		repositories[0].users[u.ID] = admin
	}

	return u, response.Token
}

func TestLifecycle(t *testing.T) {
	_, m, h := setup()

	u, token := registerLogin(t, h, m)
	if u.Name != "Alice" || u.Email != "alice@example.com" || u.CreatedAt.IsZero() {
		t.Fatal("normalization or timestamp missing")
	}

	if bcrypt.CompareHashAndPassword([]byte(m.users[u.ID].Password), []byte("password123")) != nil {
		t.Fatal("password not hashed")
	}

	path := "/users/" + u.ID
	w := request(h, "GET", path, "", token)
	expect(t, w, 200)

	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("password leaked")
	}

	expect(t, request(h, "GET", "/users", "", token), 200)
	expect(t, request(h, "PATCH", path, `{"name":"Bob","email":"bob@example.com"}`, token), 200)
	expect(t, request(h, "POST", "/users", `{"name":"Other","email":"other@example.com","password":"password123"}`, token), 201)
	expect(t, request(h, "PATCH", path, `{"email":"other@example.com"}`, token), 409)
	expect(t, request(h, "GET", "/users/"+bson.NewObjectID().Hex(), "", token), 404)
	expect(t, request(h, "DELETE", path, "", token), 204)
	expect(t, request(h, "GET", "/users", "", token), 401)
}

func TestValidationAndCredentials(t *testing.T) {
	_, m, h := setup()

	_, token := registerLogin(t, h, m)
	for _, body := range []string{`{`, `null`, `{}`, `{"name":"x","email":"bad","password":"password123"}`, `{"name":"x","email":"x@example.com","password":"short"}`, `{"name":"x","email":"x@example.com","password":"password123","extra":1}`, `{} {}`} {
		expect(t, request(h, "POST", "/auth/register", body, ""), 400)
	}

	expect(t, request(h, "POST", "/auth/register", `{"name":"x","email":"alice@example.com","password":"password123"}`, ""), 409)

	for _, body := range []string{`{"email":"alice@example.com","password":"wrong"}`, `{"email":"missing@example.com","password":"password123"}`} {
		expect(t, request(h, "POST", "/auth/login", body, ""), 401)
	}

	expect(t, request(h, "GET", "/users/bad", "", token), 400)

	for _, body := range []string{`{}`, `{"name":""}`, `{"email":"bad"}`, `{"password":"password123"}`} {
		expect(t, request(h, "PATCH", "/users/"+bson.NewObjectID().Hex(), body, token), 400)
	}

	expect(t, request(h, "POST", "/auth/register", strings.Repeat("x", 9000), ""), 400)
}

func TestTokenValidation(t *testing.T) {
	a, m, h := setup()

	u, _ := registerLogin(t, h, m)
	for _, token := range []string{"", "bad.token", "garbage"} {
		expect(t, request(h, "GET", "/users", "", token), 401)
	}

	cases := []struct {
		method jwt.SigningMethod
		secret []byte
		claims jwt.RegisteredClaims
	}{
		{jwt.SigningMethodHS256, a.Secret, jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    "user-management-api",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		}},
		{jwt.SigningMethodHS384, a.Secret, jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    "user-management-api",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}},
		{jwt.SigningMethodHS256, []byte("wrong"), jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    "user-management-api",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}},
		{jwt.SigningMethodHS256, a.Secret, jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    "wrong",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}},
		{jwt.SigningMethodHS256, a.Secret, jwt.RegisteredClaims{
			Subject: u.ID,
			Issuer:  "user-management-api",
		}},
		{jwt.SigningMethodHS256, a.Secret, jwt.RegisteredClaims{
			Subject:   "bad",
			Issuer:    "user-management-api",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}},
	}
	for _, tc := range cases {
		token, e := jwt.NewWithClaims(tc.method, tc.claims).SignedString(tc.secret)
		if e != nil {
			t.Fatal(e)
		}

		expect(t, request(h, "GET", "/users", "", token), 401)
	}
}

func TestDatabaseFailure(t *testing.T) {
	_, m, h := setup()
	_, token := registerLogin(t, h, m)
	m.failure = errors.New("private database detail")

	for _, tc := range []struct{ method, path, body string }{{"GET", "/users", ""}, {"POST", "/auth/login", `{"email":"alice@example.com","password":"password123"}`}, {"POST", "/auth/register", `{"name":"x","email":"x@example.com","password":"password123"}`}} {
		w := request(h, tc.method, tc.path, tc.body, token)
		expect(t, w, 500)

		if strings.Contains(w.Body.String(), "private") {
			t.Fatal("internal details leaked")
		}
	}
}

package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"user-management-api/internal/model"
	"user-management-api/internal/port/driving"
)

type API struct {
	Auth   driving.AuthService
	Users  driving.UserService
	Logger *slog.Logger
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)

	var body json.RawMessage
	if err := d.Decode(&body); err != nil {
		fail(w, 400, "invalid JSON body")
		return false
	}

	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		fail(w, 400, "expected a JSON object")
		return false
	}

	if d.Decode(&struct{}{}) != io.EOF {
		fail(w, 400, "expected one JSON object")
		return false
	}

	payload := json.NewDecoder(bytes.NewReader(body))
	payload.DisallowUnknownFields()

	if err := payload.Decode(v); err != nil {
		fail(w, 400, "invalid JSON body")
		return false
	}

	return true
}

func (a *API) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrInvalidInput):
		fail(w, 400, err.Error())
	case errors.Is(err, model.ErrInvalidCredentials), errors.Is(err, model.ErrUnauthorized):
		fail(w, 401, err.Error())
	case errors.Is(err, model.ErrNotFound):
		fail(w, 404, "user not found")
	case errors.Is(err, model.ErrForbidden):
		fail(w, http.StatusForbidden, "permission denied")
	case errors.Is(err, model.ErrConflict):
		fail(w, 409, "email already exists")
	default:
		a.Logger.Error("service operation failed", "error", err)
		fail(w, 500, "internal server error")
	}
}

func (a *API) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /auth/register", a.register)
	m.HandleFunc("POST /auth/login", a.login)
	m.Handle("POST /users", a.auth(http.HandlerFunc(a.create)))
	m.Handle("GET /users", a.auth(http.HandlerFunc(a.list)))
	m.Handle("GET /users/{id}", a.auth(http.HandlerFunc(a.get)))
	m.Handle("PATCH /users/{id}", a.auth(http.HandlerFunc(a.update)))
	m.Handle("DELETE /users/{id}", a.auth(http.HandlerFunc(a.delete)))
	m.HandleFunc("/", notFound)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		response := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		m.ServeHTTP(response, r.WithContext(ctx))
		a.Logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", response.status, "duration", time.Since(start))
	})
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	a.createUser(w, r, true)
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	a.createUser(w, r, false)
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request, protected bool) {
	var request registerRequest
	if !decode(w, r, &request) {
		return
	}

	command := driving.RegisterCommand{
		Name:     request.Name,
		Email:    request.Email,
		Password: request.Password,
	}

	var (
		user model.User
		err  error
	)
	if protected {
		user, err = a.Users.Create(r.Context(), command)
	} else {
		user, err = a.Auth.Register(r.Context(), command)
	}

	if err != nil {
		a.handleError(w, err)
		return
	}

	respond(w, http.StatusCreated, mapUser(user))
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !decode(w, r, &request) {
		return
	}

	result, err := a.Auth.Login(r.Context(), driving.LoginCommand{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		a.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, loginResponse{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		ExpiresIn:   result.ExpiresIn,
	})
}

func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("Authorization")
		if len(values) != 1 {
			fail(w, 401, "Bearer token required")
			return
		}

		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			fail(w, 401, "Bearer token required")
			return
		}

		identity, err := a.Auth.Authenticate(r.Context(), parts[1])
		if err != nil {
			a.handleError(w, err)
			return
		}

		next.ServeHTTP(w, r.WithContext(model.WithIdentity(r.Context(), identity)))
	})
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	user, err := a.Users.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		a.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, mapUser(user))
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	users, err := a.Users.List(r.Context())
	if err != nil {
		a.handleError(w, err)
		return
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, mapUser(user))
	}

	respond(w, http.StatusOK, response)
}

func (a *API) update(w http.ResponseWriter, r *http.Request) {
	var request updateRequest
	if !decode(w, r, &request) {
		return
	}

	user, err := a.Users.Update(r.Context(), r.PathValue("id"), driving.UpdateUserCommand{
		Name:  request.Name,
		Email: request.Email,
	})
	if err != nil {
		a.handleError(w, err)
		return
	}

	respond(w, http.StatusOK, mapUser(user))
}

func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	if err := a.Users.Delete(r.Context(), r.PathValue("id")); err != nil {
		a.handleError(w, err)
		return
	}

	w.WriteHeader(204)
}

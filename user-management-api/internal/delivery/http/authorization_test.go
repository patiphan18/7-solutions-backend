package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"user-management-api/internal/model"
)

func TestUserCanManageOnlySelf(t *testing.T) {
	_, repository, handler := setup()

	u, token := registerLogin(t, handler)
	if repository.users[u.ID].Role != model.RoleUser {
		t.Fatal("public registration elevated the role")
	}

	other := model.User{ID: "507f1f77bcf86cd799439011", Role: model.RoleUser}
	repository.users[other.ID] = other
	expect(t, request(handler, "GET", "/users/"+u.ID, "", token), 200)
	expect(t, request(handler, "PATCH", "/users/"+u.ID, `{"name":"Updated"}`, token), 200)

	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/users", ""},
		{"POST", "/users", `{"name":"Another","email":"another@example.com","password":"password123"}`},
		{"GET", "/users/" + other.ID, ""},
		{"PATCH", "/users/" + other.ID, `{"name":"Attack"}`},
		{"DELETE", "/users/" + other.ID, ""},
	} {
		expect(t, request(handler, tc.method, tc.path, tc.body, token), 403)
	}

	expect(t, request(handler, "POST", "/auth/register", `{"name":"Attack","email":"attack@example.com","password":"password123","role":"admin"}`, ""), 400)
	expect(t, request(handler, "PATCH", "/users/"+u.ID, `{"role":"admin"}`, token), 400)
	expect(t, request(handler, "DELETE", "/users/"+u.ID, "", token), 204)
}

func TestRoleChangesTakeEffectWithoutReissuingJWT(t *testing.T) {
	_, repository, handler := setup()
	u, token := registerLogin(t, handler, repository)
	expect(t, request(handler, "GET", "/users", "", token), 200)

	user := repository.users[u.ID]
	user.Role = model.RoleUser
	repository.users[u.ID] = user

	expect(t, request(handler, "GET", "/users", "", token), 403)
}

func TestDuplicateAuthorizationHeadersRejected(t *testing.T) {
	_, _, handler := setup()
	_, token := registerLogin(t, handler)
	request := httptest.NewRequest(http.MethodGet, "/users", nil)
	request.Header.Add("Authorization", "Bearer "+token)
	request.Header.Add("Authorization", "Bearer invalid")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	expect(t, response, http.StatusUnauthorized)
}

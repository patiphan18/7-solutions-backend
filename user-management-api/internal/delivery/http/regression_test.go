package httpadapter

import (
	"strings"
	"testing"
)

func TestJSONObjectsAndRoutingErrors(t *testing.T) {
	_, _, handler := setup()
	for _, body := range []string{"null", "[]", "true", `"text"`} {
		expect(t, request(handler, "POST", "/auth/login", body, ""), 400)
	}

	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/missing", 404},
		{"GET", "/auth/login", 405},
		{"PUT", "/users", 405},
		{"POST", "/users/507f1f77bcf86cd799439011", 405},
	} {
		response := request(handler, tc.method, tc.path, "", "")
		expect(t, response, tc.status)

		if response.Header().Get("Content-Type") != "application/json" || !strings.Contains(response.Body.String(), `"error"`) {
			t.Fatalf("routing error is not JSON: %s", response.Body.String())
		}

		if tc.status == 405 && response.Header().Get("Allow") == "" {
			t.Fatal("missing Allow header")
		}
	}
}

func TestLoginRejectsPasswordBeyondBcryptLimit(t *testing.T) {
	_, _, handler := setup()
	password := strings.Repeat("p", 72)
	expect(t, request(handler, "POST", "/auth/register", `{"name":"Alice","email":"alice@example.com","password":"`+password+`"}`, ""), 201)
	expect(t, request(handler, "POST", "/auth/login", `{"email":"alice@example.com","password":"`+password+`extra"}`, ""), 401)
}

package httpadapter

import (
	"net/http"
	"strings"
)

type responseWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *responseWriter) WriteHeader(status int) {
	if w.written {
		return
	}

	w.status = status
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(body)
}

func notFound(w http.ResponseWriter, r *http.Request) {
	allow := ""

	switch r.URL.Path {
	case "/auth/register", "/auth/login":
		allow = "POST"
	case "/users":
		allow = "GET, POST"
	default:
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		if id != r.URL.Path && id != "" && !strings.Contains(id, "/") {
			allow = "GET, PATCH, DELETE"
		}
	}

	if allow != "" {
		w.Header().Set("Allow", allow)
		fail(w, http.StatusMethodNotAllowed, "method not allowed")

		return
	}

	fail(w, http.StatusNotFound, "endpoint not found")
}

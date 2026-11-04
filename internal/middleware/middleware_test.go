package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPanicRecovery(t *testing.T) {
	test := struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		"Middleware recovers server from panic",
		http.MethodGet,
		"/panic",
		500,
		"Internal Server Error",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("intentional test panic")
	})

	handler := RecoveryMiddleware(mux)

	t.Run(test.name, func(t *testing.T) {
		req := httptest.NewRequest(test.method, test.path, nil)
		resp := httptest.NewRecorder()

		handler.ServeHTTP(resp, req)

		if resp.Code != test.wantStatus {
			t.Fatalf("Status code doesn't match: want %d, got %d", test.wantStatus, resp.Code)
		}

		result := resp.Body.String()

		if strings.TrimSpace(result) != test.wantBody {
			t.Fatalf("Body mismatch: want %q, got %q", test.wantBody, result)
		}
	})
}

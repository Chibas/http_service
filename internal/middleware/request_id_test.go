package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	mux := http.NewServeMux()
	var downstreamID string

	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		downstreamID = RequestIDFromContext(r.Context())
		w.Write([]byte("OK"))
	})

	handler := RequestIDMiddleware(mux)

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	reqID := res.Header().Get("X-Request-ID")

	if reqID == "" {
		t.Errorf("Expeted non empty X-Request-ID value, got empty string")
	}

	if downstreamID != reqID {
		t.Errorf("Expeted downstream handler to receive same X-Request-ID %q, got %q", reqID, downstreamID)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	secondRes := httptest.NewRecorder()

	handler.ServeHTTP(secondRes, secondReq)

	secondReqID := secondRes.Header().Get("X-Request-ID")

	if secondReqID == "" {
		t.Errorf("Expected non empty second X-Request-ID value, got empty string")
	}

	if reqID == secondReqID {
		t.Errorf("Second request ID is equal to first")
	}

}

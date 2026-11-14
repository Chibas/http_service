package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsMiddleware(t *testing.T) {
	countBefore := testutil.ToFloat64(counter.WithLabelValues("POST", "201"))
	mux := http.NewServeMux()

	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := MetricsMiddleware(mux)

	req := httptest.NewRequest(http.MethodPost, "/tasks", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)
	countAfter := testutil.ToFloat64(counter.WithLabelValues("POST", "201"))

	if resp.Code != http.StatusCreated {
		t.Fatalf("Metrics middleware failed, expected %d, got %d", http.StatusCreated, resp.Code)
	}

	if countAfter != countBefore+1 {
		t.Fatalf(
			"expected counter to increase by 1: before=%f, after=%f",
			countBefore,
			countAfter,
		)
	}
}

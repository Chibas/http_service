package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
)

type requestIDKey struct{}

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuid := rand.Text()

		ctx := context.WithValue(r.Context(), requestIDKey{}, uuid)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", uuid)
		next.ServeHTTP(w, r)
	})
}

func RequestIDFromContext(ctx context.Context) string {
	id := ctx.Value(requestIDKey{})
	requestID, ok := id.(string)
	if !ok {
		return ""
	}
	return requestID
}

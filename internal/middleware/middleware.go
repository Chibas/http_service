package middleware

import (
	"log"
	"log/slog"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		if r.URL.Path == "/healthz" && recorder.statusCode == http.StatusOK {
			return
		}

		duration := time.Since(start)
		level := slog.LevelInfo
		sc := recorder.statusCode

		switch {
		case sc >= 500:
			level = slog.LevelError
		case sc >= 400:
			level = slog.LevelWarn
		}

		slog.Log(
			r.Context(),
			level,
			"HTTP request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.statusCode,
			"duration_ms", duration.Seconds()*1000,
		)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic %+v", err)
				http.Error(w, http.StatusText(500), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

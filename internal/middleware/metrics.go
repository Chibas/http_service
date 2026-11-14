package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var counter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
}, []string{"method", "status"})
var histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"method"})

func init() {
	prometheus.MustRegister(counter, histogram)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()

		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)

		counter.WithLabelValues(r.Method, strconv.Itoa(recorder.statusCode)).Inc()
		histogram.WithLabelValues(r.Method).Observe(duration.Seconds())
	})
}

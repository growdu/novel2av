package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/novel2av/backend/internal/infra/observability"
)

// statusRecorder lets us capture the http.ResponseWriter status code (which
// the std lib discards by default) and the bytes written so the metrics
// middleware can label both dimensions.
type statusRecorder struct {
	http.ResponseWriter
	status   int
	written  int64
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.written += int64(n)
	return n, err
}

// MetricsMiddleware records http_requests_total + http_request_duration_seconds
// for every served request. Labels are method, route (the chi route template
// like /api/v1/projects/{id}, falling back to the raw URL path), and status.
// /metrics itself is skipped to avoid recursive scrapes doubling their own
// counters.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == observability.MetricsPath {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 0}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		route := routePattern(r)
		method := r.Method
		statusLabel := httpStatusLabel(status)
		// Defense in depth: a test binary can mount this middleware without
		// first calling observability.SetupMetrics(). Skip cleanly so the
		// rest of the chain still serves.
		if observability.HTTPRequestsTotal != nil && observability.HTTPRequests != nil {
			observability.HTTPRequestsTotal.WithLabelValues(method, route, statusLabel).Inc()
			observability.HTTPRequests.WithLabelValues(method, route, statusLabel).Observe(time.Since(start).Seconds())
		}
	})
}

// routePattern returns the chi route template (e.g. "/api/v1/projects/{id}")
// or "/" when no template was matched (404 fallthrough). Unmatched routes are
// labelled "/" so cardinality stays bounded even under random scan traffic.
func routePattern(r *http.Request) string {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return "/"
	}
	if pattern := rc.RoutePattern(); pattern != "" {
		return pattern
	}
	return "/"
}

// httpStatusLabel renders the numeric status as a string label with the
// matching sentinel for unknown (chi writes nothing for status 0 means OK).
func httpStatusLabel(code int) string {
	switch {
	case code >= 100 && code < 600:
		return itoa(code)
	}
	return "0"
}

// itoa is the same int-to-string helper used in the existing repo package
// (we cannot import it from a sibling package without expanding visibility).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(b[pos:])
}

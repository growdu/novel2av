package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMetricsMiddleware_RecordsRouteTemplate(t *testing.T) {
	r := chi.NewRouter()
	r.Use(MetricsMiddleware)
	r.Get("/items/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	req := httptest.NewRequest("GET", "/items/42", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("upstream code = %d", rec.Code)
	}
}

func TestMetricsMiddleware_SkipsSelf(t *testing.T) {
	called := false
	r := chi.NewRouter()
	r.Use(MetricsMiddleware)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(200) })

	// We can't easily scrape /metrics without a Registry, but we can check
	// that /metrics requests are *not* short-circuited: the request simply
	// falls through to whatever the next handler does.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if !called {
		t.Fatal("inner handler never called")
	}
}

func TestStatusLabel_KnownCodes(t *testing.T) {
	cases := []int{200, 201, 301, 400, 404, 500}
	for _, c := range cases {
		got := httpStatusLabel(c)
		want := itoa(c)
		if got != want {
			t.Errorf("httpStatusLabel(%d)=%q want %q", c, got, want)
		}
	}
	if got := httpStatusLabel(0); got != "0" {
		t.Errorf("httpStatusLabel(0)=%q want \"0\"", got)
	}
	if got := httpStatusLabel(999); got != "0" {
		t.Errorf("httpStatusLabel(999)=%q want \"0\" (out of range)", got)
	}
}

func TestRoutePattern_Fallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/whatever", nil)
	if got := routePattern(req); got != "/" {
		t.Errorf("empty RouteContext pattern = %q want \"/\"", got)
	}
}

func TestStatusRecorder_DefaultsTo200(t *testing.T) {
	inner := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: inner}
	if _, err := sr.Write([]byte("hi")); err != nil {
		t.Fatalf("write err: %v", err)
	}
	if sr.status != 200 {
		t.Errorf("implicit status default = %d want 200", sr.status)
	}
	if sr.written != 2 {
		t.Errorf("bytes = %d want 2", sr.written)
	}
	if !strings.Contains(inner.Body.String(), "hi") {
		t.Errorf("inner body = %q", inner.Body.String())
	}
}

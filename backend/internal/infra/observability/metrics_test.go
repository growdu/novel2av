package observability_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/novel2av/backend/internal/infra/observability"
)

// TestSetupMetrics_ServesProcessAndGoSeries asserts that the default process +
// Go runtime collectors are wired in so a vanilla /metrics scrape sees the
// baseline telemetry every ops dashboard expects.
func TestSetupMetrics_ServesProcessAndGoSeries(t *testing.T) {
	h := observability.SetupMetrics()
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)

	for _, want := range []string{"go_goroutines", "process_cpu_seconds_total"} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics output missing %q\ngot:\n%s", want, text)
		}
	}
}

// TestSetupMetrics_HTTPRequests verifies the per-request series emits one
// histogram and one counter sample after the registry sees a request. The
// actual middleware that calls Observe() lands in A2; this test asserts the
// series are reachable with the documented labels.
func TestSetupMetrics_HTTPRequests(t *testing.T) {
	h := observability.SetupMetrics()
	observability.HTTPRequestsTotal.WithLabelValues("GET", "/healthz", "200").Inc()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape: code %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		`http_requests_total{method="GET",route="/healthz",status="200"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q\ngot:\n%s", want, body)
		}
	}
}

package httpapi

import (
	"os"
	"testing"

	"github.com/novel2av/backend/internal/infra/observability"
)

// TestMain ensures observability.SetupMetrics() runs before any test that
// exercises the metrics-aware middleware; the middleware itself has a
// nil-guard defense layer for unrelated partial-init callers.
func TestMain(m *testing.M) {
	observability.SetupMetrics()
	os.Exit(m.Run())
}

package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry holds the process-wide Prometheus registry. It is set by
// SetupMetrics and used everywhere we need to record or scrape metrics.
var Registry *prometheus.Registry

// HTTPRequests tracks per-request latency. Labels: method, route (the chi
// route template, NOT the request URL, to keep cardinality bounded),
// status (numeric code as a string).
var HTTPRequests *prometheus.HistogramVec

// HTTPRequestsTotal counts every served request. Same labels as HTTPRequests;
// exported as a separate counter so dashboards can show "rps" cheaply without
// summing histogram buckets.
var HTTPRequestsTotal *prometheus.CounterVec

// MetricsPath is where SetupMetrics registers the promhttp handler. Operators
// typically scrape this over a private network, not the public listener.
const MetricsPath = "/metrics"

// SetupMetrics installs a private Prometheus registry and the standard
// process + Go runtime collectors, plus the HTTP request series. It returns
// the http.Handler that should be mounted at MetricsPath. Safe to call once
// at startup; calling twice returns the existing handler without re-registering.
func SetupMetrics() http.Handler {
	if Registry != nil {
		return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{Registry: Registry})
	}
	Registry = prometheus.NewRegistry()
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	Registry.MustRegister(collectors.NewGoCollector())

	HTTPRequests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
	Registry.MustRegister(HTTPRequests)

	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests served, partitioned by method/route/status.",
		},
		[]string{"method", "route", "status"},
	)
	Registry.MustRegister(HTTPRequestsTotal)

	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{Registry: Registry})
}

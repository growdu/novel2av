package service

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/novel2av/backend/internal/infra/observability"
)

// PipelineJobsEnqueued tracks every task handed to asynq. Label: step.
var PipelineJobsEnqueued *prometheus.CounterVec

// PipelineJobsCompleted tracks the result of the internal-callback ingest
// path. outcome is one of: success | failure | ignored_task_failed.
var PipelineJobsCompleted *prometheus.CounterVec

// PipelineIngestDuration measures how long IngestTaskResult spent in the
// fast in-memory switch. NOTE: this is NOT end-to-end enqueue->ingest
// latency; that would require carrying a queued_at timestamp through the
// asynq payload (TODO M8).
var PipelineIngestDuration *prometheus.HistogramVec

// PipelineProvidersFailures increments whenever the internal callback
// arrives with errMsg set. kind is "llm" / "image" / "tts" / "unknown";
// the worker payload will need an additional "kind" field for finer labels.
// TODO(M8): ai-engine notify should include provider + kind for precise labels.
var PipelineProvidersFailures *prometheus.CounterVec

// WSConnectionsActive is the live count of open WebSocket clients.
var WSConnectionsActive prometheus.Gauge

// RedisPingDuration measures Services.Ping queue.RedisClient round-trip.
var RedisPingDuration prometheus.Histogram

// EnsureSvcMetrics allocates the service-layer metrics and registers them
// with observability.Registry. Idempotent. Safe to call before tests; nil
// guards protect code paths that observe metrics when ensure has not run.
func EnsureSvcMetrics() {
	if PipelineJobsEnqueued != nil {
		return
	}
	r := observability.Registry
	if r == nil {
		// Standalone (test) mode: allocate a private registry so the
		// collectors still exist and can be inspected by httptest
		// callers in this package.
		r = prometheus.NewRegistry()
	}
	PipelineJobsEnqueued = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "n2av_jobs_enqueued_total", Help: "Tasks handed to asynq."},
		[]string{"step"},
	)
	PipelineJobsCompleted = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "n2av_jobs_completed_total", Help: "Internal-callback ingest outcomes."},
		[]string{"step", "outcome"},
	)
	PipelineIngestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "n2av_ingest_duration_seconds",
			Help:    "Time spent in IngestTaskResult, by step.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"step"},
	)
	PipelineProvidersFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "n2av_providers_failures_total", Help: "Worker-reported failures by kind."},
		[]string{"kind"},
	)
	WSConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "n2av_ws_connections_active",
		Help: "Open WebSocket connections to /api/v1/ws/projects/{id}.",
	})
	RedisPingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "n2av_redis_ping_seconds",
		Help:    "Services.Ping Redis round-trip latency.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	})

	for _, c := range []prometheus.Collector{
		PipelineJobsEnqueued,
		PipelineJobsCompleted,
		PipelineIngestDuration,
		PipelineProvidersFailures,
		WSConnectionsActive,
		RedisPingDuration,
	} {
		r.MustRegister(c)
	}
}

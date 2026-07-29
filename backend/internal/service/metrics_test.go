package service

import "testing"

// TestEnsureSvcMetrics_Idempotent asserts EnsureSvcMetrics can be called
// twice without panicking and without replacing the metric vars on the
// second call (double-register would otherwise crash).
func TestEnsureSvcMetrics_Idempotent(t *testing.T) {
	EnsureSvcMetrics()
	original := PipelineJobsEnqueued
	EnsureSvcMetrics()
	if PipelineJobsEnqueued != original {
		t.Fatalf("second EnsureSvcMetrics replaced existing collectors")
	}
}

// TestPipelineMetrics_BumpSafe walks every metric var and confirms it can
// carry a sample without panicking. The actual labels are arbitrary; what
// matters is the var was allocated and is reachable.
func TestPipelineMetrics_BumpSafe(t *testing.T) {
	EnsureSvcMetrics()

	if PipelineJobsEnqueued == nil {
		t.Fatal("PipelineJobsEnqueued not allocated")
	}
	if PipelineJobsCompleted == nil {
		t.Fatal("PipelineJobsCompleted not allocated")
	}
	if PipelineIngestDuration == nil {
		t.Fatal("PipelineIngestDuration not allocated")
	}
	if PipelineProvidersFailures == nil {
		t.Fatal("PipelineProvidersFailures not allocated")
	}
	if WSConnectionsActive == nil {
		t.Fatal("WSConnectionsActive not allocated")
	}
	if RedisPingDuration == nil {
		t.Fatal("RedisPingDuration not allocated")
	}

	PipelineJobsEnqueued.WithLabelValues("ai:split_chapters").Inc()
	PipelineJobsCompleted.WithLabelValues("ai:split_chapters", "success").Inc()
	PipelineJobsCompleted.WithLabelValues("ai:compose_full", "failure").Inc()
	PipelineProvidersFailures.WithLabelValues("unknown").Inc()
	PipelineIngestDuration.WithLabelValues("ai:extract_characters").Observe(0.04)
	WSConnectionsActive.Set(3)
	WSConnectionsActive.Inc()
	WSConnectionsActive.Dec()
	RedisPingDuration.Observe(0.005)
}

package inference

import "testing"

func TestMetricsTrackRequestLifecycle(t *testing.T) {
	metrics := NewMetrics()
	finish := metrics.Begin("worker-a")
	finish(200)
	finish(500) // completion is idempotent
	snapshot := metrics.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("workers=%d want 1", len(snapshot))
	}
	got := snapshot[0]
	if got.RequestsTotal != 1 || got.Successes != 1 || got.Failures != 0 || got.InFlight != 0 {
		t.Fatalf("unexpected metrics: %+v", got)
	}
	if got.AverageLatencyMS < 0 {
		t.Fatalf("negative latency: %f", got.AverageLatencyMS)
	}
}

func TestMetricsTrackConcurrentInflightRequests(t *testing.T) {
	metrics := NewMetrics()
	first := metrics.Begin("worker-a")
	second := metrics.Begin("worker-a")
	if got := metrics.Snapshot()[0].InFlight; got != 2 {
		t.Fatalf("in_flight=%d want 2", got)
	}
	first(200)
	second(502)
	got := metrics.Snapshot()[0]
	if got.RequestsTotal != 2 || got.Successes != 1 || got.Failures != 1 || got.InFlight != 0 {
		t.Fatalf("unexpected metrics: %+v", got)
	}
}

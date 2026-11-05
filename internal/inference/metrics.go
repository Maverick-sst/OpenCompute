package inference

import (
	"sort"
	"sync"
	"time"
)

type WorkerMetrics struct {
	WorkerID        string    `json:"worker_id"`
	RequestsTotal   uint64    `json:"requests_total"`
	Successes       uint64    `json:"successes"`
	Failures        uint64    `json:"failures"`
	InFlight        int64     `json:"in_flight"`
	AverageLatencyMS float64   `json:"average_latency_ms"`
	LastStatus      int       `json:"last_status,omitempty"`
	LastRequestAt   time.Time `json:"last_request_at,omitempty"`
	totalLatencyMS  float64
}

type Metrics struct {
	mu      sync.Mutex
	workers map[string]*WorkerMetrics
}

func NewMetrics() *Metrics {
	return &Metrics{workers: make(map[string]*WorkerMetrics)}
}

func (m *Metrics) Begin(workerID string) func(int) {
	m.mu.Lock()
	entry := m.workers[workerID]
	if entry == nil {
		entry = &WorkerMetrics{WorkerID: workerID}
		m.workers[workerID] = entry
	}
	entry.RequestsTotal++
	entry.InFlight++
	entry.LastRequestAt = time.Now().UTC()
	m.mu.Unlock()

	started := time.Now()
	var once sync.Once
	return func(status int) {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			entry.InFlight--
			entry.LastStatus = status
			entry.totalLatencyMS += float64(time.Since(started).Microseconds()) / 1000
			entry.AverageLatencyMS = entry.totalLatencyMS / float64(entry.RequestsTotal)
			if status >= 200 && status < 400 {
				entry.Successes++
			} else {
				entry.Failures++
			}
		})
	}
}

func (m *Metrics) Snapshot() []WorkerMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]WorkerMetrics, 0, len(m.workers))
	for _, entry := range m.workers {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorkerID < out[j].WorkerID })
	return out
}

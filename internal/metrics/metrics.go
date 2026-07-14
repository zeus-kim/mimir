package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds server metrics
type Metrics struct {
	startTime time.Time

	// Counters
	totalRequests   atomic.Int64
	totalErrors     atomic.Int64
	totalFetches    atomic.Int64
	totalDocuments  atomic.Int64

	// Per-tool counters
	toolCalls map[string]*atomic.Int64
	toolMu    sync.RWMutex

	// Per-API counters
	apiCalls   map[string]*atomic.Int64
	apiErrors  map[string]*atomic.Int64
	apiMu      sync.RWMutex

	// Latency tracking
	latencies   map[string]*LatencyTracker
	latencyMu   sync.RWMutex
}

// LatencyTracker tracks latency statistics
type LatencyTracker struct {
	count atomic.Int64
	sum   atomic.Int64 // nanoseconds
	min   atomic.Int64
	max   atomic.Int64
}

// Global metrics instance
var global *Metrics
var once sync.Once

// Global returns the global metrics instance
func Global() *Metrics {
	once.Do(func() {
		global = New()
	})
	return global
}

// New creates a new Metrics instance
func New() *Metrics {
	return &Metrics{
		startTime:  time.Now(),
		toolCalls:  make(map[string]*atomic.Int64),
		apiCalls:   make(map[string]*atomic.Int64),
		apiErrors:  make(map[string]*atomic.Int64),
		latencies:  make(map[string]*LatencyTracker),
	}
}

// IncRequests increments the total request counter
func (m *Metrics) IncRequests() {
	m.totalRequests.Add(1)
}

// IncErrors increments the total error counter
func (m *Metrics) IncErrors() {
	m.totalErrors.Add(1)
}

// IncFetches increments the total fetch counter
func (m *Metrics) IncFetches() {
	m.totalFetches.Add(1)
}

// AddDocuments adds to the total document counter
func (m *Metrics) AddDocuments(n int64) {
	m.totalDocuments.Add(n)
}

// IncToolCall increments the call counter for a specific tool
func (m *Metrics) IncToolCall(tool string) {
	m.toolMu.Lock()
	if _, ok := m.toolCalls[tool]; !ok {
		m.toolCalls[tool] = &atomic.Int64{}
	}
	m.toolMu.Unlock()

	m.toolMu.RLock()
	m.toolCalls[tool].Add(1)
	m.toolMu.RUnlock()
}

// IncAPICall increments the call counter for a specific API
func (m *Metrics) IncAPICall(api string) {
	m.apiMu.Lock()
	if _, ok := m.apiCalls[api]; !ok {
		m.apiCalls[api] = &atomic.Int64{}
	}
	m.apiMu.Unlock()

	m.apiMu.RLock()
	m.apiCalls[api].Add(1)
	m.apiMu.RUnlock()
}

// IncAPIError increments the error counter for a specific API
func (m *Metrics) IncAPIError(api string) {
	m.apiMu.Lock()
	if _, ok := m.apiErrors[api]; !ok {
		m.apiErrors[api] = &atomic.Int64{}
	}
	m.apiMu.Unlock()

	m.apiMu.RLock()
	m.apiErrors[api].Add(1)
	m.apiMu.RUnlock()
}

// RecordLatency records a latency measurement
func (m *Metrics) RecordLatency(name string, d time.Duration) {
	ns := d.Nanoseconds()

	m.latencyMu.Lock()
	if _, ok := m.latencies[name]; !ok {
		m.latencies[name] = &LatencyTracker{}
		m.latencies[name].min.Store(ns)
		m.latencies[name].max.Store(ns)
	}
	m.latencyMu.Unlock()

	m.latencyMu.RLock()
	tracker := m.latencies[name]
	m.latencyMu.RUnlock()

	tracker.count.Add(1)
	tracker.sum.Add(ns)

	// Update min
	for {
		old := tracker.min.Load()
		if ns >= old || tracker.min.CompareAndSwap(old, ns) {
			break
		}
	}

	// Update max
	for {
		old := tracker.max.Load()
		if ns <= old || tracker.max.CompareAndSwap(old, ns) {
			break
		}
	}
}

// Timer returns a function that records latency when called
func (m *Metrics) Timer(name string) func() {
	start := time.Now()
	return func() {
		m.RecordLatency(name, time.Since(start))
	}
}

// Snapshot returns a snapshot of all metrics
func (m *Metrics) Snapshot() map[string]interface{} {
	snapshot := map[string]interface{}{
		"uptime_seconds":   time.Since(m.startTime).Seconds(),
		"start_time":       m.startTime.Format(time.RFC3339),
		"total_requests":   m.totalRequests.Load(),
		"total_errors":     m.totalErrors.Load(),
		"total_fetches":    m.totalFetches.Load(),
		"total_documents":  m.totalDocuments.Load(),
	}

	// Tool calls
	toolCalls := make(map[string]int64)
	m.toolMu.RLock()
	for k, v := range m.toolCalls {
		toolCalls[k] = v.Load()
	}
	m.toolMu.RUnlock()
	snapshot["tool_calls"] = toolCalls

	// API calls
	apiCalls := make(map[string]int64)
	m.apiMu.RLock()
	for k, v := range m.apiCalls {
		apiCalls[k] = v.Load()
	}
	m.apiMu.RUnlock()
	snapshot["api_calls"] = apiCalls

	// API errors
	apiErrors := make(map[string]int64)
	m.apiMu.RLock()
	for k, v := range m.apiErrors {
		apiErrors[k] = v.Load()
	}
	m.apiMu.RUnlock()
	snapshot["api_errors"] = apiErrors

	// Latencies
	latencies := make(map[string]map[string]interface{})
	m.latencyMu.RLock()
	for k, v := range m.latencies {
		count := v.count.Load()
		if count == 0 {
			continue
		}
		latencies[k] = map[string]interface{}{
			"count":    count,
			"avg_ms":   float64(v.sum.Load()) / float64(count) / 1e6,
			"min_ms":   float64(v.min.Load()) / 1e6,
			"max_ms":   float64(v.max.Load()) / 1e6,
		}
	}
	m.latencyMu.RUnlock()
	snapshot["latencies"] = latencies

	return snapshot
}

// Reset resets all metrics (useful for testing)
func (m *Metrics) Reset() {
	m.totalRequests.Store(0)
	m.totalErrors.Store(0)
	m.totalFetches.Store(0)
	m.totalDocuments.Store(0)

	m.toolMu.Lock()
	m.toolCalls = make(map[string]*atomic.Int64)
	m.toolMu.Unlock()

	m.apiMu.Lock()
	m.apiCalls = make(map[string]*atomic.Int64)
	m.apiErrors = make(map[string]*atomic.Int64)
	m.apiMu.Unlock()

	m.latencyMu.Lock()
	m.latencies = make(map[string]*LatencyTracker)
	m.latencyMu.Unlock()
}

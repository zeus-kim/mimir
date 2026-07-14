package metrics

import (
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}
}

func TestMetricsCounters(t *testing.T) {
	m := New()

	m.IncRequests()
	m.IncRequests()
	m.IncErrors()
	m.IncFetches()
	m.AddDocuments(10)

	snapshot := m.Snapshot()

	if snapshot["total_requests"].(int64) != 2 {
		t.Errorf("expected 2 requests, got %d", snapshot["total_requests"])
	}
	if snapshot["total_errors"].(int64) != 1 {
		t.Errorf("expected 1 error, got %d", snapshot["total_errors"])
	}
	if snapshot["total_fetches"].(int64) != 1 {
		t.Errorf("expected 1 fetch, got %d", snapshot["total_fetches"])
	}
	if snapshot["total_documents"].(int64) != 10 {
		t.Errorf("expected 10 documents, got %d", snapshot["total_documents"])
	}
}

func TestMetricsToolCalls(t *testing.T) {
	m := New()

	m.IncToolCall("fetch_pharma")
	m.IncToolCall("fetch_pharma")
	m.IncToolCall("create_vertical")

	snapshot := m.Snapshot()
	toolCalls := snapshot["tool_calls"].(map[string]int64)

	if toolCalls["fetch_pharma"] != 2 {
		t.Errorf("expected 2 fetch_pharma calls, got %d", toolCalls["fetch_pharma"])
	}
	if toolCalls["create_vertical"] != 1 {
		t.Errorf("expected 1 create_vertical call, got %d", toolCalls["create_vertical"])
	}
}

func TestMetricsAPICalls(t *testing.T) {
	m := New()

	m.IncAPICall("pubmed")
	m.IncAPICall("pubmed")
	m.IncAPICall("arxiv")
	m.IncAPIError("pubmed")

	snapshot := m.Snapshot()
	apiCalls := snapshot["api_calls"].(map[string]int64)
	apiErrors := snapshot["api_errors"].(map[string]int64)

	if apiCalls["pubmed"] != 2 {
		t.Errorf("expected 2 pubmed calls, got %d", apiCalls["pubmed"])
	}
	if apiCalls["arxiv"] != 1 {
		t.Errorf("expected 1 arxiv call, got %d", apiCalls["arxiv"])
	}
	if apiErrors["pubmed"] != 1 {
		t.Errorf("expected 1 pubmed error, got %d", apiErrors["pubmed"])
	}
}

func TestMetricsLatency(t *testing.T) {
	m := New()

	m.RecordLatency("fetch_pubmed", 100*time.Millisecond)
	m.RecordLatency("fetch_pubmed", 200*time.Millisecond)
	m.RecordLatency("fetch_pubmed", 150*time.Millisecond)

	snapshot := m.Snapshot()
	latencies := snapshot["latencies"].(map[string]map[string]interface{})

	pubmed := latencies["fetch_pubmed"]
	if pubmed["count"].(int64) != 3 {
		t.Errorf("expected 3 latency records, got %d", pubmed["count"])
	}
	if pubmed["min_ms"].(float64) < 99 || pubmed["min_ms"].(float64) > 101 {
		t.Errorf("expected min ~100ms, got %f", pubmed["min_ms"])
	}
	if pubmed["max_ms"].(float64) < 199 || pubmed["max_ms"].(float64) > 201 {
		t.Errorf("expected max ~200ms, got %f", pubmed["max_ms"])
	}
}

func TestMetricsTimer(t *testing.T) {
	m := New()

	done := m.Timer("test_operation")
	time.Sleep(50 * time.Millisecond)
	done()

	snapshot := m.Snapshot()
	latencies := snapshot["latencies"].(map[string]map[string]interface{})

	if _, ok := latencies["test_operation"]; !ok {
		t.Error("expected test_operation latency to be recorded")
	}
}

func TestMetricsReset(t *testing.T) {
	m := New()

	m.IncRequests()
	m.IncToolCall("test")
	m.IncAPICall("test")
	m.RecordLatency("test", time.Second)

	m.Reset()

	snapshot := m.Snapshot()
	if snapshot["total_requests"].(int64) != 0 {
		t.Error("expected 0 requests after reset")
	}
	if len(snapshot["tool_calls"].(map[string]int64)) != 0 {
		t.Error("expected empty tool_calls after reset")
	}
	if len(snapshot["api_calls"].(map[string]int64)) != 0 {
		t.Error("expected empty api_calls after reset")
	}
	if len(snapshot["latencies"].(map[string]map[string]interface{})) != 0 {
		t.Error("expected empty latencies after reset")
	}
}

func TestMetricsUptime(t *testing.T) {
	m := New()
	time.Sleep(10 * time.Millisecond)

	snapshot := m.Snapshot()
	uptime := snapshot["uptime_seconds"].(float64)

	if uptime < 0.01 {
		t.Errorf("expected uptime > 0.01s, got %f", uptime)
	}
}

func TestGlobalMetrics(t *testing.T) {
	g1 := Global()
	g2 := Global()

	if g1 != g2 {
		t.Error("Global() should return same instance")
	}
}

func TestMetricsConcurrency(t *testing.T) {
	m := New()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.IncRequests()
				m.IncToolCall("test")
				m.IncAPICall("test")
				m.RecordLatency("test", time.Millisecond)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	snapshot := m.Snapshot()
	if snapshot["total_requests"].(int64) != 1000 {
		t.Errorf("expected 1000 requests, got %d", snapshot["total_requests"])
	}
}

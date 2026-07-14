package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.userAgent != "Mimir/1.0" {
		t.Errorf("expected default userAgent 'Mimir/1.0', got '%s'", c.userAgent)
	}
}

func TestClientOptions(t *testing.T) {
	c := New(
		WithTimeout(5*time.Second),
		WithUserAgent("TestAgent/1.0"),
		WithRateLimit(10.0, 5),
		WithRetry(RetryConfig{
			MaxRetries:  5,
			InitialWait: 100 * time.Millisecond,
			MaxWait:     1 * time.Second,
			Multiplier:  1.5,
		}),
	)

	if c.userAgent != "TestAgent/1.0" {
		t.Errorf("expected userAgent 'TestAgent/1.0', got '%s'", c.userAgent)
	}
	if c.retryConfig.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", c.retryConfig.MaxRetries)
	}
	if c.rateLimiter == nil {
		t.Error("expected rateLimiter to be set")
	}
}

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := New()
	resp, err := c.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestClientRetry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(
		WithRetry(RetryConfig{
			MaxRetries:  3,
			InitialWait: 10 * time.Millisecond,
			MaxWait:     100 * time.Millisecond,
			Multiplier:  2.0,
		}),
	)

	resp, err := c.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	defer resp.Body.Close()

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestClientRetryExhausted(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := New(
		WithRetry(RetryConfig{
			MaxRetries:  2,
			InitialWait: 10 * time.Millisecond,
			MaxWait:     100 * time.Millisecond,
			Multiplier:  2.0,
		}),
	)

	_, err := c.Get(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}

	// Initial + 2 retries = 3 attempts
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClientRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10.0, 2) // 10 req/sec, burst 2

	ctx := context.Background()

	// First two should be immediate (burst)
	start := time.Now()
	for i := 0; i < 2; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait() error: %v", err)
		}
	}
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Errorf("burst requests took too long: %v", elapsed)
	}

	// Third should wait
	start = time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
	elapsed = time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("rate-limited request was too fast: %v", elapsed)
	}
}

func TestClientRateLimiterContextCancel(t *testing.T) {
	rl := NewRateLimiter(0.5, 1) // Very slow: 0.5 req/sec

	// Use up the burst
	ctx := context.Background()
	rl.Wait(ctx)

	// Now cancel before the wait completes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected context timeout error")
	}
}

func TestClientUserAgent(t *testing.T) {
	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithUserAgent("CustomAgent/2.0"))
	resp, err := c.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	resp.Body.Close()

	if receivedUA != "CustomAgent/2.0" {
		t.Errorf("expected User-Agent 'CustomAgent/2.0', got '%s'", receivedUA)
	}
}

func TestClient429Retry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(
		WithRetry(RetryConfig{
			MaxRetries:  3,
			InitialWait: 10 * time.Millisecond,
			MaxWait:     100 * time.Millisecond,
			Multiplier:  2.0,
		}),
	)

	resp, err := c.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts for 429 retry, got %d", attempts)
	}
}

func TestDefaultClient(t *testing.T) {
	if DefaultClient == nil {
		t.Error("DefaultClient is nil")
	}
}

func TestPredefinedClients(t *testing.T) {
	if SemanticScholarClient == nil {
		t.Error("SemanticScholarClient is nil")
	}
	if ArxivClient == nil {
		t.Error("ArxivClient is nil")
	}
	if HuggingFaceClient == nil {
		t.Error("HuggingFaceClient is nil")
	}
}

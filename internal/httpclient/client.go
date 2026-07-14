package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/zeus-kim/mimir/internal/logger"
)

// Client is an HTTP client with retry, rate limiting, and timeout support
type Client struct {
	client      *http.Client
	rateLimiter *RateLimiter
	retryConfig RetryConfig
	userAgent   string
	log         *logger.Logger
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mu          sync.Mutex
	tokens      float64
	maxTokens   float64
	refillRate  float64 // tokens per second
	lastRefill  time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: requestsPerSecond,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now

	if rl.tokens >= 1 {
		rl.tokens--
		return nil
	}

	// Calculate wait time
	waitTime := time.Duration((1 - rl.tokens) / rl.refillRate * float64(time.Second))

	select {
	case <-time.After(waitTime):
		rl.tokens = 0
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ClientOption configures the client
type ClientOption func(*Client)

// WithTimeout sets the request timeout
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.client.Timeout = d
	}
}

// WithRetry configures retry behavior
func WithRetry(cfg RetryConfig) ClientOption {
	return func(c *Client) {
		c.retryConfig = cfg
	}
}

// WithRateLimit configures rate limiting
func WithRateLimit(requestsPerSecond float64, burst int) ClientOption {
	return func(c *Client) {
		c.rateLimiter = NewRateLimiter(requestsPerSecond, burst)
	}
}

// WithUserAgent sets the User-Agent header
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// New creates a new HTTP client
func New(opts ...ClientOption) *Client {
	c := &Client{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryConfig: RetryConfig{
			MaxRetries:  3,
			InitialWait: 1 * time.Second,
			MaxWait:     30 * time.Second,
			Multiplier:  2.0,
		},
		userAgent: "Mimir/1.0",
		log:       logger.Default().WithField("component", "httpclient"),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Do executes an HTTP request with retry and rate limiting
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req)
}

// DoWithContext executes an HTTP request with context
func (c *Client) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Set User-Agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	// Rate limit
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	var lastErr error
	wait := c.retryConfig.InitialWait

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			c.log.Debug("retrying request (attempt %d/%d): %s %s",
				attempt+1, c.retryConfig.MaxRetries+1, req.Method, req.URL)

			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			wait = time.Duration(float64(wait) * c.retryConfig.Multiplier)
			if wait > c.retryConfig.MaxWait {
				wait = c.retryConfig.MaxWait
			}
		}

		resp, err := c.client.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = err
			continue
		}

		// Retry on 429 (rate limit) or 5xx errors
		if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// GetJSON performs a GET request and expects JSON response
func (c *Client) GetJSON(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return c.Do(req)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// DefaultClient is the default HTTP client
var DefaultClient = New(
	WithTimeout(30*time.Second),
	WithRetry(RetryConfig{
		MaxRetries:  3,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}),
)

// Predefined rate-limited clients for specific APIs
var (
	// SemanticScholarClient has 100 requests per 5 minutes
	SemanticScholarClient = New(
		WithRateLimit(100.0/300.0, 10), // ~0.33 req/sec, burst 10
		WithUserAgent("Mimir/1.0 (mailto:contact@mimir.local)"),
	)

	// ArxivClient has no strict limit but we're polite
	ArxivClient = New(
		WithRateLimit(1.0, 5), // 1 req/sec, burst 5
		WithUserAgent("Mimir/1.0"),
	)

	// HuggingFaceClient
	HuggingFaceClient = New(
		WithRateLimit(10.0, 20),
		WithUserAgent("Mimir/1.0"),
	)
)

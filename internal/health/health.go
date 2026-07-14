package health

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// Status represents health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents a single health check
type Check struct {
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Message string        `json:"message,omitempty"`
	Latency time.Duration `json:"latency_ms"`
}

// Result represents the overall health check result
type Result struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Checks    []Check   `json:"checks"`
}

// Checker performs health checks
type Checker struct {
	version   string
	startTime time.Time
	db        *sql.DB
	checks    []CheckFunc
	mu        sync.RWMutex
}

// CheckFunc is a function that performs a health check
type CheckFunc func(ctx context.Context) Check

// NewChecker creates a new health checker
func NewChecker(version string) *Checker {
	return &Checker{
		version:   version,
		startTime: time.Now(),
		checks:    make([]CheckFunc, 0),
	}
}

// SetDB sets the database for health checks
func (c *Checker) SetDB(db *sql.DB) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.db = db
}

// AddCheck adds a custom health check
func (c *Checker) AddCheck(check CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, check)
}

// Check performs all health checks
func (c *Checker) Check(ctx context.Context) Result {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := Result{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Version:   c.version,
		Uptime:    time.Since(c.startTime).Round(time.Second).String(),
		Checks:    make([]Check, 0),
	}

	// Database check
	if c.db != nil {
		check := c.checkDB(ctx)
		result.Checks = append(result.Checks, check)
		if check.Status != StatusHealthy {
			result.Status = StatusDegraded
		}
	}

	// Custom checks
	for _, checkFunc := range c.checks {
		check := checkFunc(ctx)
		result.Checks = append(result.Checks, check)
		if check.Status == StatusUnhealthy {
			result.Status = StatusUnhealthy
		} else if check.Status == StatusDegraded && result.Status == StatusHealthy {
			result.Status = StatusDegraded
		}
	}

	return result
}

// checkDB checks database connectivity
func (c *Checker) checkDB(ctx context.Context) Check {
	start := time.Now()
	check := Check{
		Name:   "database",
		Status: StatusHealthy,
	}

	// Try a simple query
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := c.db.PingContext(ctx)
	check.Latency = time.Since(start)

	if err != nil {
		check.Status = StatusUnhealthy
		check.Message = err.Error()
		return check
	}

	// Check if we can query
	var count int
	err = c.db.QueryRowContext(ctx, "SELECT 1").Scan(&count)
	if err != nil {
		check.Status = StatusDegraded
		check.Message = "query failed: " + err.Error()
		return check
	}

	return check
}

// Liveness returns a simple liveness check (for Kubernetes)
func (c *Checker) Liveness() bool {
	return true
}

// Readiness returns whether the service is ready to accept traffic
func (c *Checker) Readiness(ctx context.Context) bool {
	result := c.Check(ctx)
	return result.Status != StatusUnhealthy
}

// Global checker instance
var globalChecker *Checker
var globalOnce sync.Once

// Global returns the global health checker
func Global() *Checker {
	globalOnce.Do(func() {
		globalChecker = NewChecker("dev")
	})
	return globalChecker
}

// SetGlobal sets the global health checker
func SetGlobal(c *Checker) {
	globalChecker = c
}

package health

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestNewChecker(t *testing.T) {
	c := NewChecker("1.0.0")
	if c == nil {
		t.Fatal("NewChecker returned nil")
	}
	if c.version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", c.version)
	}
}

func TestCheckerLiveness(t *testing.T) {
	c := NewChecker("1.0.0")
	if !c.Liveness() {
		t.Error("Liveness() should return true")
	}
}

func TestCheckerBasicCheck(t *testing.T) {
	c := NewChecker("1.0.0")
	ctx := context.Background()

	result := c.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("expected healthy status, got %s", result.Status)
	}
	if result.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", result.Version)
	}
	if result.Uptime == "" {
		t.Error("expected uptime to be set")
	}
}

func TestCheckerReadiness(t *testing.T) {
	c := NewChecker("1.0.0")
	ctx := context.Background()

	if !c.Readiness(ctx) {
		t.Error("Readiness() should return true for healthy checker")
	}
}

func TestCheckerCustomCheck(t *testing.T) {
	c := NewChecker("1.0.0")

	c.AddCheck(func(ctx context.Context) Check {
		return Check{
			Name:    "custom",
			Status:  StatusHealthy,
			Message: "all good",
			Latency: 10 * time.Millisecond,
		}
	})

	ctx := context.Background()
	result := c.Check(ctx)

	if len(result.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(result.Checks))
	}
	if result.Checks[0].Name != "custom" {
		t.Errorf("expected check name 'custom', got '%s'", result.Checks[0].Name)
	}
}

func TestCheckerDegradedStatus(t *testing.T) {
	c := NewChecker("1.0.0")

	c.AddCheck(func(ctx context.Context) Check {
		return Check{
			Name:   "degraded_service",
			Status: StatusDegraded,
		}
	})

	ctx := context.Background()
	result := c.Check(ctx)

	if result.Status != StatusDegraded {
		t.Errorf("expected degraded status, got %s", result.Status)
	}
}

func TestCheckerUnhealthyStatus(t *testing.T) {
	c := NewChecker("1.0.0")

	c.AddCheck(func(ctx context.Context) Check {
		return Check{
			Name:   "critical_service",
			Status: StatusUnhealthy,
		}
	})

	ctx := context.Background()
	result := c.Check(ctx)

	if result.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy status, got %s", result.Status)
	}

	if c.Readiness(ctx) {
		t.Error("Readiness() should return false when unhealthy")
	}
}

func TestCheckerMultipleChecks(t *testing.T) {
	c := NewChecker("1.0.0")

	c.AddCheck(func(ctx context.Context) Check {
		return Check{Name: "check1", Status: StatusHealthy}
	})
	c.AddCheck(func(ctx context.Context) Check {
		return Check{Name: "check2", Status: StatusHealthy}
	})
	c.AddCheck(func(ctx context.Context) Check {
		return Check{Name: "check3", Status: StatusHealthy}
	})

	ctx := context.Background()
	result := c.Check(ctx)

	if len(result.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(result.Checks))
	}
}

func TestCheckerWithDB(t *testing.T) {
	// Open in-memory SQLite
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skip("SQLite not available")
	}
	defer db.Close()

	c := NewChecker("1.0.0")
	c.SetDB(db)

	ctx := context.Background()
	result := c.Check(ctx)

	// Should have database check
	found := false
	for _, check := range result.Checks {
		if check.Name == "database" {
			found = true
			if check.Status != StatusHealthy {
				t.Errorf("expected database check healthy, got %s: %s", check.Status, check.Message)
			}
		}
	}

	if !found {
		t.Error("expected database check in results")
	}
}

func TestGlobalChecker(t *testing.T) {
	g1 := Global()
	g2 := Global()

	if g1 != g2 {
		t.Error("Global() should return same instance")
	}
}

func TestSetGlobal(t *testing.T) {
	c := NewChecker("2.0.0")
	SetGlobal(c)

	// Note: This modifies global state
	// In real tests, you might want to restore it
}

func TestCheckerUptime(t *testing.T) {
	c := NewChecker("1.0.0")
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	result := c.Check(ctx)

	// Uptime should be non-empty (might be "0s" for very fast tests, that's ok)
	if result.Uptime == "" {
		t.Error("expected uptime to be set")
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusHealthy != "healthy" {
		t.Errorf("expected 'healthy', got '%s'", StatusHealthy)
	}
	if StatusDegraded != "degraded" {
		t.Errorf("expected 'degraded', got '%s'", StatusDegraded)
	}
	if StatusUnhealthy != "unhealthy" {
		t.Errorf("expected 'unhealthy', got '%s'", StatusUnhealthy)
	}
}

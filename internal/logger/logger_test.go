package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"info", INFO},
		{"INFO", INFO},
		{"warn", WARN},
		{"WARN", WARN},
		{"warning", WARN},
		{"error", ERROR},
		{"ERROR", ERROR},
		{"unknown", INFO}, // Default
		{"", INFO},
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.expected {
			t.Errorf("ParseLevel(%q): got %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.expected {
			t.Errorf("Level(%d).String(): got %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", DEBUG, "text", &buf)

	log.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "test:") {
		t.Errorf("expected 'test:' in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected 'test message' in output, got: %s", output)
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", DEBUG, "json", &buf)

	log.Info("test message")

	output := buf.String()

	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON log: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
	if entry.Logger != "test" {
		t.Errorf("expected logger 'test', got %s", entry.Logger)
	}
	if entry.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", entry.Message)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", WARN, "text", &buf)

	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	output := buf.String()

	if strings.Contains(output, "debug message") {
		t.Error("debug message should be filtered")
	}
	if strings.Contains(output, "info message") {
		t.Error("info message should be filtered")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("error message should be present")
	}
}

func TestLoggerWithField(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", DEBUG, "json", &buf)

	log.WithField("user", "alice").Info("login")

	output := buf.String()

	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON log: %v", err)
	}

	if entry.Fields["user"] != "alice" {
		t.Errorf("expected field user=alice, got %v", entry.Fields["user"])
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", DEBUG, "json", &buf)

	log.WithFields(map[string]interface{}{
		"user":   "bob",
		"action": "upload",
		"size":   1024,
	}).Info("file uploaded")

	output := buf.String()

	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON log: %v", err)
	}

	if entry.Fields["user"] != "bob" {
		t.Errorf("expected field user=bob, got %v", entry.Fields["user"])
	}
	if entry.Fields["action"] != "upload" {
		t.Errorf("expected field action=upload, got %v", entry.Fields["action"])
	}
}

func TestLoggerFormatting(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", DEBUG, "text", &buf)

	log.Info("count: %d, name: %s", 42, "test")

	output := buf.String()
	if !strings.Contains(output, "count: 42, name: test") {
		t.Errorf("expected formatted message, got: %s", output)
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", ERROR, "text", &buf)

	log.Info("should not appear")
	if buf.Len() > 0 {
		t.Error("info should be filtered at ERROR level")
	}

	log.SetLevel(INFO)
	log.Info("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Error("info should appear after SetLevel(INFO)")
	}
}

func TestDefaultLogger(t *testing.T) {
	var buf bytes.Buffer
	log := New("default-test", INFO, "text", &buf)
	SetDefault(log)

	Info("default logger test")

	output := buf.String()
	if !strings.Contains(output, "default logger test") {
		t.Errorf("expected message in default logger output, got: %s", output)
	}
}

func TestWithFieldChaining(t *testing.T) {
	var buf bytes.Buffer
	log := New("test", DEBUG, "json", &buf)

	// Chain multiple WithField calls
	log.WithField("a", 1).WithField("b", 2).Info("chained")

	output := buf.String()

	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON log: %v", err)
	}

	// Both fields should be present
	if entry.Fields["a"] != float64(1) { // JSON numbers are float64
		t.Errorf("expected field a=1, got %v", entry.Fields["a"])
	}
	if entry.Fields["b"] != float64(2) {
		t.Errorf("expected field b=2, got %v", entry.Fields["b"])
	}
}

package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses log level from string
func ParseLevel(s string) Level {
	switch s {
	case "debug", "DEBUG":
		return DEBUG
	case "info", "INFO":
		return INFO
	case "warn", "WARN", "warning", "WARNING":
		return WARN
	case "error", "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// Logger provides structured logging
type Logger struct {
	mu       sync.Mutex
	level    Level
	format   string // json or text
	output   io.Writer
	fields   map[string]interface{}
	name     string
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Logger    string                 `json:"logger,omitempty"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

var defaultLogger = New("mimir", INFO, "text", os.Stderr)

// New creates a new logger
func New(name string, level Level, format string, output io.Writer) *Logger {
	return &Logger{
		name:   name,
		level:  level,
		format: format,
		output: output,
		fields: make(map[string]interface{}),
	}
}

// Default returns the default logger
func Default() *Logger {
	return defaultLogger
}

// SetDefault sets the default logger
func SetDefault(l *Logger) {
	defaultLogger = l
}

// WithField returns a new logger with the given field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		name:   l.name,
		level:  l.level,
		format: l.format,
		output: l.output,
		fields: make(map[string]interface{}),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithFields returns a new logger with the given fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := &Logger{
		name:   l.name,
		level:  l.level,
		format: l.format,
		output: l.output,
		fields: make(map[string]interface{}),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Logger:    l.name,
		Message:   fmt.Sprintf(msg, args...),
		Fields:    l.fields,
	}

	if l.format == "json" {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.output, string(data))
	} else {
		// Text format
		fieldsStr := ""
		if len(entry.Fields) > 0 {
			data, _ := json.Marshal(entry.Fields)
			fieldsStr = " " + string(data)
		}
		fmt.Fprintf(l.output, "%s [%s] %s: %s%s\n",
			entry.Timestamp, entry.Level, entry.Logger, entry.Message, fieldsStr)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, msg, args...)
}

// Package-level convenience functions
func Debug(msg string, args ...interface{}) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...interface{})  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...interface{})  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...interface{}) { defaultLogger.Error(msg, args...) }

func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

func WithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}

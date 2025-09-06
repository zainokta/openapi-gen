package logger

import "log/slog"

// Logger interface allows users to integrate with any logging framework
//
// This interface provides a common logging API that can be implemented by
// any logging framework (slog, logrus, zap, zerolog, etc.)
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// SlogAdapter wraps slog.Logger to implement the Logger interface
//
// This adapter allows seamless integration with Go's standard slog package
// while maintaining compatibility with the generic Logger interface.
//
// Example usage:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	adapter := logger.NewSlogAdapter(logger)
//	
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithLogger(adapter),
//	)
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new SlogAdapter wrapping the provided slog.Logger
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: logger}
}

// Info logs an info message with optional key-value pairs
func (s *SlogAdapter) Info(msg string, args ...any) {
	s.logger.Info(msg, args...)
}

// Warn logs a warning message with optional key-value pairs
func (s *SlogAdapter) Warn(msg string, args ...any) {
	s.logger.Warn(msg, args...)
}

// Error logs an error message with optional key-value pairs
func (s *SlogAdapter) Error(msg string, args ...any) {
	s.logger.Error(msg, args...)
}

// Debug logs a debug message with optional key-value pairs
func (s *SlogAdapter) Debug(msg string, args ...any) {
	s.logger.Debug(msg, args...)
}

// NoOpLogger is a logger that discards all log messages
//
// Useful for testing or when logging is not desired.
//
// Example usage:
//
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithLogger(&logger.NoOpLogger{}),
//	)
type NoOpLogger struct{}

// Info discards the log message
func (n *NoOpLogger) Info(msg string, args ...any) {}

// Warn discards the log message
func (n *NoOpLogger) Warn(msg string, args ...any) {}

// Error discards the log message
func (n *NoOpLogger) Error(msg string, args ...any) {}

// Debug discards the log message
func (n *NoOpLogger) Debug(msg string, args ...any) {}

// TestLogger is a simple test logger that captures log messages for testing
// This is useful for verifying logging behavior in tests
type TestLogger struct {
	InfoCalls  []LogCall
	WarnCalls  []LogCall
	ErrorCalls []LogCall
	DebugCalls []LogCall
}

// LogCall represents a captured log call with message and arguments
type LogCall struct {
	Message string
	Args    []any
}

// Info captures an info log call
func (t *TestLogger) Info(msg string, args ...any) {
	t.InfoCalls = append(t.InfoCalls, LogCall{Message: msg, Args: args})
}

// Warn captures a warn log call
func (t *TestLogger) Warn(msg string, args ...any) {
	t.WarnCalls = append(t.WarnCalls, LogCall{Message: msg, Args: args})
}

// Error captures an error log call
func (t *TestLogger) Error(msg string, args ...any) {
	t.ErrorCalls = append(t.ErrorCalls, LogCall{Message: msg, Args: args})
}

// Debug captures a debug log call
func (t *TestLogger) Debug(msg string, args ...any) {
	t.DebugCalls = append(t.DebugCalls, LogCall{Message: msg, Args: args})
}

// Clear clears all captured log calls
func (t *TestLogger) Clear() {
	t.InfoCalls = nil
	t.WarnCalls = nil
	t.ErrorCalls = nil
	t.DebugCalls = nil
}
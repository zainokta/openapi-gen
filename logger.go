package openapi

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
//	adapter := openapi.NewSlogAdapter(logger)
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
//		openapi.WithLogger(&openapi.NoOpLogger{}),
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
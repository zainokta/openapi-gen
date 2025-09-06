package openapi

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLogger is a simple test logger that captures log messages
type TestLogger struct {
	InfoCalls  []LogCall
	WarnCalls  []LogCall
	ErrorCalls []LogCall
	DebugCalls []LogCall
}

type LogCall struct {
	Message string
	Args    []any
}

func (t *TestLogger) Info(msg string, args ...any) {
	t.InfoCalls = append(t.InfoCalls, LogCall{Message: msg, Args: args})
}

func (t *TestLogger) Warn(msg string, args ...any) {
	t.WarnCalls = append(t.WarnCalls, LogCall{Message: msg, Args: args})
}

func (t *TestLogger) Error(msg string, args ...any) {
	t.ErrorCalls = append(t.ErrorCalls, LogCall{Message: msg, Args: args})
}

func (t *TestLogger) Debug(msg string, args ...any) {
	t.DebugCalls = append(t.DebugCalls, LogCall{Message: msg, Args: args})
}

func TestLoggerInterface(t *testing.T) {
	t.Run("TestLogger implements Logger interface", func(t *testing.T) {
		var logger Logger = &TestLogger{}
		
		// Test that TestLogger can be used as Logger interface
		logger.Info("test info", "key", "value")
		logger.Warn("test warn")
		logger.Error("test error", "error", "something went wrong")
		logger.Debug("test debug")
		
		// Cast back to TestLogger to verify calls
		testLogger := logger.(*TestLogger)
		assert.Len(t, testLogger.InfoCalls, 1)
		assert.Equal(t, "test info", testLogger.InfoCalls[0].Message)
		assert.Equal(t, []any{"key", "value"}, testLogger.InfoCalls[0].Args)
		
		assert.Len(t, testLogger.WarnCalls, 1)
		assert.Equal(t, "test warn", testLogger.WarnCalls[0].Message)
		
		assert.Len(t, testLogger.ErrorCalls, 1)
		assert.Equal(t, "test error", testLogger.ErrorCalls[0].Message)
		
		assert.Len(t, testLogger.DebugCalls, 1)
		assert.Equal(t, "test debug", testLogger.DebugCalls[0].Message)
	})
}

func TestSlogAdapter(t *testing.T) {
	t.Run("SlogAdapter implements Logger interface", func(t *testing.T) {
		// Create a slog logger that writes to a string builder
		var buf strings.Builder
		slogLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		
		adapter := NewSlogAdapter(slogLogger)
		
		// Test that SlogAdapter can be used as Logger interface
		var logger Logger = adapter
		logger.Info("test message", "key", "value")
		
		// Verify output contains our message
		output := buf.String()
		assert.Contains(t, output, "test message")
		assert.Contains(t, output, "key=value")
	})
}

func TestNoOpLogger(t *testing.T) {
	t.Run("NoOpLogger implements Logger interface", func(t *testing.T) {
		var logger Logger = &NoOpLogger{}
		
		// Should not panic and do nothing
		logger.Info("test info")
		logger.Warn("test warn")
		logger.Error("test error")
		logger.Debug("test debug")
		
		// Test passes if no panic occurs
	})
}

func TestWithLoggerOption(t *testing.T) {
	t.Run("WithLogger option works correctly", func(t *testing.T) {
		testLogger := &TestLogger{}
		
		opts := processOptions(WithLogger(testLogger))
		
		assert.Equal(t, testLogger, opts.logger)
	})
}

func TestWithSlogLoggerOption(t *testing.T) {
	t.Run("WithSlogLogger option works correctly", func(t *testing.T) {
		slogLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		
		opts := processOptions(WithSlogLogger(slogLogger))
		
		// Should be wrapped in SlogAdapter
		adapter, ok := opts.logger.(*SlogAdapter)
		assert.True(t, ok, "Expected logger to be SlogAdapter")
		assert.Equal(t, slogLogger, adapter.logger)
	})
}

func TestDefaultLogger(t *testing.T) {
	t.Run("Default logger is SlogAdapter with slog.Default", func(t *testing.T) {
		opts := processOptions()
		
		// Should default to SlogAdapter
		adapter, ok := opts.logger.(*SlogAdapter)
		assert.True(t, ok, "Expected default logger to be SlogAdapter")
		assert.NotNil(t, adapter.logger)
	})
}
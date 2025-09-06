package common

import (
	"reflect"
	"runtime"
	"strings"
)

// HandlerNameExtractor provides utilities for extracting handler names from function values
type HandlerNameExtractor struct{}

// NewHandlerNameExtractor creates a new HandlerNameExtractor
func NewHandlerNameExtractor() *HandlerNameExtractor {
	return &HandlerNameExtractor{}
}

// GetOriginalHandlerName attempts to extract the original handler name from runtime info for external modules
func (e *HandlerNameExtractor) GetOriginalHandlerName(handlerValue reflect.Value) string {
	// Get the function pointer
	pc := handlerValue.Pointer()
	if pc == 0 {
		return ""
	}
	// Get function info
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}
	// Get the full function name
	fullName := fn.Name()
	// Enhanced parsing for external modules
	return e.ParseHandlerNameFromFunction(fullName)
}

// ParseHandlerNameFromFunction parses handler name from various function name patterns
func (e *HandlerNameExtractor) ParseHandlerNameFromFunction(fullName string) string {
	// Handle different patterns from external modules:
	// 1. external-app/internal/handlers.(*UserHandler).CreateUser-fm
	// 2. github.com/user/app/pkg/api.(*Controller).Method-fm
	// 3. some.domain/path/handlers.(*Handler).Method.func1
	// 4. app/handlers.Function
	// Pattern 1 & 2: Method receivers (*Type).Method
	if strings.Contains(fullName, "(*") && strings.Contains(fullName, ").") {
		return e.ExtractMethodFromReceiver(fullName)
	}
	// Pattern 3: Function calls (may include .func1, .func2 suffixes)
	if strings.Contains(fullName, ".") {
		return e.ExtractFunctionName(fullName)
	}
	// Pattern 4: Simple function names
	return fullName
}

// ExtractMethodFromReceiver extracts method name from receiver pattern
func (e *HandlerNameExtractor) ExtractMethodFromReceiver(fullName string) string {
	// Find the last occurrence of ).MethodName
	parenIdx := strings.LastIndex(fullName, ").")
	if parenIdx == -1 {
		return ""
	}
	// Extract everything after ).
	methodPart := fullName[parenIdx+2:]
	// Remove common suffixes
	methodPart = strings.TrimSuffix(methodPart, "-fm")
	methodPart = strings.TrimSuffix(methodPart, ".func1")
	methodPart = strings.TrimSuffix(methodPart, ".func2")
	// Extract just the method name (in case there are more dots)
	if dotIdx := strings.Index(methodPart, "."); dotIdx != -1 {
		methodPart = methodPart[:dotIdx]
	}
	return methodPart
}

// ExtractFunctionName extracts function name from various dot-separated patterns
func (e *HandlerNameExtractor) ExtractFunctionName(fullName string) string {
	// Split by dots and take the last meaningful part
	parts := strings.Split(fullName, ".")
	if len(parts) == 0 {
		return ""
	}
	// Work backwards to find the actual function name
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		// Skip common suffixes
		if part == "func1" || part == "func2" || part == "func3" ||
			strings.HasSuffix(part, "-fm") {
			continue
		}

		// Skip receiver types (surrounded by parentheses patterns)
		if strings.HasPrefix(part, "(*") || strings.HasSuffix(part, ")") {
			continue
		}

		// This should be our function name
		if part != "" && !strings.Contains(part, "/") {
			return strings.TrimSuffix(part, "-fm")
		}
	}
	// Fallback: return the last part
	lastPart := parts[len(parts)-1]
	return strings.TrimSuffix(lastPart, "-fm")
}

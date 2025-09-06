package integration

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
)

// TestHertzHandlerAnalyzer_NewAnalyzer tests the analyzer creation
func TestHertzHandlerAnalyzer_NewAnalyzer(t *testing.T) {
	analyzer := NewHertzHandlerAnalyzer()
	assert.NotNil(t, analyzer, "Analyzer should not be nil")
	assert.Equal(t, "CloudWeGo Hertz", analyzer.GetFrameworkName(), "Framework name should be correct")
}

// Sample handler function for testing
func sampleHandler(ctx context.Context, c *app.RequestContext) {
	// Simple handler that does nothing
}

// TestHertzHandlerAnalyzer_ExtractTypes tests type extraction
func TestHertzHandlerAnalyzer_ExtractTypes(t *testing.T) {
	analyzer := NewHertzHandlerAnalyzer()

	// Test with valid handler
	reqType, respType, err := analyzer.ExtractTypes(sampleHandler)
	assert.NoError(t, err, "Should not error with valid handler")
	// For now, we expect nil types since the simple handler doesn't have BindAndValidate calls
	assert.Nil(t, reqType, "Request type should be nil for simple handler")
	assert.Nil(t, respType, "Response type should be nil for simple handler")

	// Test with nil handler
	_, _, err = analyzer.ExtractTypes(nil)
	assert.Error(t, err, "Should error with nil handler")
	assert.Contains(t, err.Error(), "handler is nil", "Error should mention nil handler")

	// Test with non-function
	_, _, err = analyzer.ExtractTypes("not a function")
	assert.Error(t, err, "Should error with non-function")
	assert.Contains(t, err.Error(), "not a function", "Error should mention invalid type")
}

// TestHertzHandlerAnalyzer_AnalyzeHandler tests handler analysis
func TestHertzHandlerAnalyzer_AnalyzeHandler(t *testing.T) {
	analyzer := NewHertzHandlerAnalyzer()

	// Test with valid handler
	schema := analyzer.AnalyzeHandler(sampleHandler)

	// The schema should exist but might be empty if no types are extracted
	assert.NotNil(t, schema, "Schema should not be nil")
	// For a simple handler, we expect empty schemas
	assert.Empty(t, schema.RequestSchema.Type, "Request schema should be empty for simple handler")
	assert.Empty(t, schema.ResponseSchema.Type, "Response schema should be empty for simple handler")
}

// TestHertzHandlerAnalyzer_ValidateSignature tests signature validation
func TestHertzHandlerAnalyzer_ValidateSignature(t *testing.T) {
	analyzer := NewHertzHandlerAnalyzer()

	// Test with valid Hertz handler signature
	handlerType := reflect.TypeOf(sampleHandler)
	err := analyzer.validateHertzSignature(handlerType)
	assert.NoError(t, err, "Should not error with valid Hertz handler signature")

	// Test with invalid signature (wrong number of parameters)
	invalidHandler := func() {}
	invalidType := reflect.TypeOf(invalidHandler)
	err = analyzer.validateHertzSignature(invalidType)
	assert.Error(t, err, "Should error with invalid signature")
	assert.Contains(t, err.Error(), "expected 2 parameters", "Error should mention parameter count")
}

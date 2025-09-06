package integration

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestGinHandlerAnalyzer_NewAnalyzer tests the analyzer creation
func TestGinHandlerAnalyzer_NewAnalyzer(t *testing.T) {
	analyzer := NewGinHandlerAnalyzer()
	assert.NotNil(t, analyzer, "Analyzer should not be nil")
	assert.Equal(t, "Gin", analyzer.GetFrameworkName(), "Framework name should be correct")
}

// Sample Gin handler function for testing
func sampleGinHandler(c *gin.Context) {
	// Simple handler that does nothing
}

// TestGinHandlerAnalyzer_ExtractTypes tests type extraction
func TestGinHandlerAnalyzer_ExtractTypes(t *testing.T) {
	analyzer := NewGinHandlerAnalyzer()

	// Test with valid handler
	reqType, respType, err := analyzer.ExtractTypes(sampleGinHandler)
	assert.NoError(t, err, "Should not error with valid handler")
	// For now, we expect nil types since the simple handler doesn't have ShouldBind calls
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

// TestGinHandlerAnalyzer_AnalyzeHandler tests handler analysis
func TestGinHandlerAnalyzer_AnalyzeHandler(t *testing.T) {
	analyzer := NewGinHandlerAnalyzer()

	// Test with valid handler
	schema := analyzer.AnalyzeHandler(sampleGinHandler)

	// The schema should exist but might be empty if no types are extracted
	assert.NotNil(t, schema, "Schema should not be nil")
	// For a simple handler, we expect empty schemas
	assert.Equal(t, schema.RequestSchema.Type, "object")
	assert.Equal(t, schema.ResponseSchema.Type, "object")
}

// TestGinHandlerAnalyzer_ValidateSignature tests signature validation
func TestGinHandlerAnalyzer_ValidateSignature(t *testing.T) {
	analyzer := NewGinHandlerAnalyzer()

	// Test with valid Gin handler signature
	handlerType := reflect.TypeOf(sampleGinHandler)
	err := analyzer.validateGinSignature(handlerType)
	assert.NoError(t, err, "Should not error with valid Gin handler signature")

	// Test with invalid signature (wrong number of parameters)
	invalidHandler := func() {}
	invalidType := reflect.TypeOf(invalidHandler)
	err = analyzer.validateGinSignature(invalidType)
	assert.Error(t, err, "Should error with invalid signature")
	assert.Contains(t, err.Error(), "expected 1 parameter", "Error should mention parameter count")
}

// TestGinRouteDiscoverer tests route discovery
func TestGinRouteDiscoverer(t *testing.T) {
	// Create a Gin engine
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	
	// Add some test routes
	engine.GET("/test", sampleGinHandler)
	engine.POST("/users", sampleGinHandler)
	engine.PUT("/users/:id", sampleGinHandler)

	// Create discoverer
	discoverer := NewGinRouteDiscoverer(engine)
	assert.NotNil(t, discoverer, "Discoverer should not be nil")
	assert.Equal(t, "Gin", discoverer.GetFrameworkName(), "Framework name should be correct")

	// Discover routes
	routes, err := discoverer.DiscoverRoutes()
	assert.NoError(t, err, "Should not error when discovering routes")
	assert.Len(t, routes, 3, "Should discover 3 routes")

	// Check route details
	expectedRoutes := map[string]string{
		"GET":  "/test",
		"POST": "/users",
		"PUT":  "/users/:id",
	}

	for _, route := range routes {
		expectedPath, exists := expectedRoutes[route.Method]
		assert.True(t, exists, "Method %s should be expected", route.Method)
		assert.Equal(t, expectedPath, route.Path, "Path should match for method %s", route.Method)
		assert.NotEmpty(t, route.HandlerName, "Handler name should not be empty")
	}
}

// TestGinServerAdapter tests the server adapter
func TestGinServerAdapter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	
	adapter := NewGinServerAdapter(engine)
	assert.NotNil(t, adapter, "Adapter should not be nil")

	// Test adding a route through the adapter
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("test response"))
	}

	// This should not panic
	assert.NotPanics(t, func() {
		adapter.GET("/test-adapter", testHandler)
	}, "Should not panic when adding route")

	// Verify the route was added to the engine
	routes := engine.Routes()
	found := false
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/test-adapter" {
			found = true
			break
		}
	}
	assert.True(t, found, "Route should be added to the engine")
}

// TestAutoDiscoverer_Gin tests the auto discoverer with Gin
func TestAutoDiscoverer_Gin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/test", sampleGinHandler)

	// Test auto discovery
	discoverer, err := NewAutoDiscoverer(engine)
	assert.NoError(t, err, "Should not error when creating auto discoverer")
	assert.NotNil(t, discoverer, "Discoverer should not be nil")
	assert.Equal(t, "Gin", discoverer.GetFrameworkName(), "Should detect Gin framework")

	// Test route discovery
	routes, err := discoverer.DiscoverRoutes()
	assert.NoError(t, err, "Should not error when discovering routes")
	assert.Len(t, routes, 1, "Should discover 1 route")
	assert.Equal(t, "GET", routes[0].Method, "Method should be GET")
	assert.Equal(t, "/test", routes[0].Path, "Path should be /test")
}
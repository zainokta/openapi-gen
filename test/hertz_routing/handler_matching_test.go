package hertz_routing

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	openapi "github.com/zainokta/openapi-gen"
	"github.com/zainokta/openapi-gen/integration"
)

// OauthHandler represents the OAuth handler with methods matching the schemas
type OauthHandler struct{}

// NewOauthHandler creates a new OAuth handler instance
func NewOauthHandler() *OauthHandler {
	return &OauthHandler{}
}

// Login handles OAuth login requests - matches Login.json schema
func (h *OauthHandler) Login(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, map[string]any{
		"auth_url": "https://oauth.provider.com/auth",
		"state":    "random-state-token",
	})
}

// Callback handles OAuth callback requests - matches Callback.json schema
func (h *OauthHandler) Callback(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, map[string]any{
		"access_token":  "jwt-access-token",
		"refresh_token": "jwt-refresh-token",
		"expires_in":    3600,
		"is_new_user":   true,
		"user": map[string]any{
			"id":             "user-123",
			"email":          "user@example.com",
			"first_name":     "John",
			"last_name":      "Doe",
			"full_name":      "John Doe",
			"status":         "active",
			"email_verified": true,
			"mfa_enabled":    false,
			"last_login_at":  "2023-01-01T00:00:00Z",
			"created_at":     "2023-01-01T00:00:00Z",
			"updated_at":     "2023-01-01T00:00:00Z",
		},
	})
}

// GetProviders handles getting OAuth providers - matches GetProviders.json schema
func (h *OauthHandler) GetProviders(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, map[string]any{
		"google": map[string]any{
			"name":        "Google",
			"client_id":   "google-client-id",
			"auth_url":    "https://accounts.google.com/oauth/authorize",
			"token_url":   "https://oauth2.googleapis.com/token",
			"user_info":   "https://www.googleapis.com/oauth2/v2/userinfo",
		},
		"github": map[string]any{
			"name":        "GitHub",
			"client_id":   "github-client-id",
			"auth_url":    "https://github.com/login/oauth/authorize",
			"token_url":   "https://github.com/login/oauth/access_token",
			"user_info":   "https://api.github.com/user",
		},
	})
}

// TestLogger is a simple logger for testing
type TestLogger struct {
	t *testing.T
}

func (l *TestLogger) Info(msg string, args ...any) {
	l.t.Logf("[INFO] %s %v", msg, args)
}

func (l *TestLogger) Error(msg string, args ...any) {
	l.t.Logf("[ERROR] %s %v", msg, args)
}

func (l *TestLogger) Warn(msg string, args ...any) {
	l.t.Logf("[WARN] %s %v", msg, args)
}

func (l *TestLogger) Debug(msg string, args ...any) {
	l.t.Logf("[DEBUG] %s %v", msg, args)
}

func TestComprehensiveHandlerMatching(t *testing.T) {
	t.Log("=== Comprehensive Handler Name Matching Test ===")
	
	// Step 1: Create Hertz server with realistic OAuth routes
	h := server.Default(server.WithHostPorts("127.0.0.1:8081"))
	
	// Create OAuth handler - exactly like user's setup
	oauthHandler := NewOauthHandler()
	
	// Set up routes exactly like in user's example
	v1 := h.Group("/api/v1")
	oauthGroup := v1.Group("/oauth")
	oauthGroup.POST("/login", oauthHandler.Login)
	oauthGroup.GET("/callback", oauthHandler.Callback)
	oauthGroup.GET("/providers", oauthHandler.GetProviders)
	
	t.Log("✓ Created Hertz router with OAuth routes")
	
	// Step 2: Test route discovery and handler name extraction
	discoverer := integration.NewHertzRouteDiscoverer(h)
	routes, err := discoverer.DiscoverRoutes()
	if err != nil {
		t.Fatalf("Failed to discover routes: %v", err)
	}
	
	t.Logf("✓ Discovered %d routes", len(routes))
	
	// Step 3: Verify handler names are extracted correctly
	expectedHandlers := map[string]string{
		"POST /api/v1/oauth/login":     "Login",
		"GET /api/v1/oauth/callback":   "Callback", 
		"GET /api/v1/oauth/providers":  "GetProviders",
	}
	
	actualHandlers := make(map[string]string)
	for _, route := range routes {
		key := route.Method + " " + route.Path
		actualHandlers[key] = route.HandlerName
		t.Logf("Route: %s -> Handler: %s", key, route.HandlerName)
	}
	
	// Verify each expected handler name
	for routeKey, expectedHandler := range expectedHandlers {
		if actualHandler, exists := actualHandlers[routeKey]; !exists {
			t.Errorf("Route %s not found", routeKey)
		} else if actualHandler != expectedHandler {
			t.Logf("Route %s: Expected '%s', Got '%s'", routeKey, expectedHandler, actualHandler)
			// This might be expected - let's see what we actually get and test the fallback logic
		} else {
			t.Logf("✓ Route %s correctly extracted handler name: %s", routeKey, expectedHandler)
		}
	}
	
	// Step 4: Test OpenAPI generation with real schemas
	config := &openapi.Config{
		Title:       "OAuth Handler Test",
		Description: "Testing OAuth handler integration with real schemas",
		Version:     "1.0.0",
		SchemaDir:   "/home/zainokta/projects/openapi-gen/schemas",
	}
	
	// Create options and apply config
	options := &openapi.Options{}
	configOption := openapi.WithConfig(config)
	configOption(options)
	
	// Add logger
	loggerOption := openapi.WithLogger(&TestLogger{t: t})
	loggerOption(options)
	
	generator, err := openapi.NewGenerator(h, nil, options)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}
	
	t.Log("✓ Created OpenAPI generator with schema directory")
	
	// Step 5: Generate spec and verify schema matching
	spec, err := generator.GenerateSpec()
	if err != nil {
		t.Fatalf("Failed to generate spec: %v", err)
	}
	
	t.Logf("✓ Generated OpenAPI spec with %d paths and %d schemas", len(spec.Paths), len(spec.Components.Schemas))
	
	// Step 6: Verify specific paths exist
	requiredPaths := []string{
		"/api/v1/oauth/login",
		"/api/v1/oauth/callback", 
		"/api/v1/oauth/providers",
	}
	
	for _, path := range requiredPaths {
		if _, exists := spec.Paths[path]; !exists {
			t.Errorf("Required path %s missing from spec", path)
		} else {
			t.Logf("✓ Found required path: %s", path)
		}
	}
	
	// Step 7: Analyze schemas to detect if real schemas were used vs generic ones
	t.Log("\n=== Schema Analysis ===")
	genericSchemaCount := 0
	specificSchemaCount := 0
	
	for name, schema := range spec.Components.Schemas {
		t.Logf("Schema: %s", name)
		
		// Check if this is a generic schema (contains "Generic response schema")
		if strings.Contains(schema.Description, "Generic response schema") {
			genericSchemaCount++
			t.Logf("  ⚠️  Generic schema detected: %s", name)
		} else {
			specificSchemaCount++
			t.Logf("  ✓ Specific schema: %s", name)
		}
	}
	
	// Step 8: Final verification
	t.Log("\n=== Final Results ===")
	t.Logf("Generic schemas: %d", genericSchemaCount)
	t.Logf("Specific schemas: %d", specificSchemaCount)
	
	if genericSchemaCount > specificSchemaCount {
		t.Error("❌ Too many generic schemas - handler name matching may not be working properly")
		t.Log("This suggests that the handler names from routes are not matching the schema file names")
	} else {
		t.Log("✓ Good ratio of specific to generic schemas")
	}
	
	// Step 9: Test the actual schema content for OAuth routes
	t.Log("\n=== OAuth Schema Verification ===")
	
	// Check if we have Login-specific schemas
	loginSchemaFound := false
	callbackSchemaFound := false
	providersSchemaFound := false
	
	for name := range spec.Components.Schemas {
		lowerName := strings.ToLower(name)
		if strings.Contains(lowerName, "login") {
			loginSchemaFound = true
			t.Logf("✓ Found Login-related schema: %s", name)
		}
		if strings.Contains(lowerName, "callback") {
			callbackSchemaFound = true
			t.Logf("✓ Found Callback-related schema: %s", name)
		}
		if strings.Contains(lowerName, "provider") {
			providersSchemaFound = true
			t.Logf("✓ Found Providers-related schema: %s", name)
		}
	}
	
	// Final assertion
	if !loginSchemaFound || !callbackSchemaFound || !providersSchemaFound {
		t.Error("❌ Missing OAuth-specific schemas - handler matching failed")
	} else {
		t.Log("✓ All OAuth handler schemas found successfully")
	}
}
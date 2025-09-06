package openapi

import (
	"testing"

	"github.com/zainokta/openapi-gen/parser"

	"github.com/stretchr/testify/assert"
)

func TestPathParser(t *testing.T) {
	parser := parser.NewPathParser()

	tests := []struct {
		name            string
		method          string
		path            string
		expectedTag     string
		expectedSummary string
	}{
		{
			name:            "Auth login endpoint",
			method:          "POST",
			path:            "/api/v1/auth/login",
			expectedTag:     "auth",
			expectedSummary: "Create Auth Login",
		},
		{
			name:            "OAuth providers endpoint",
			method:          "GET",
			path:            "/api/v1/oauth/providers",
			expectedTag:     "oauth",
			expectedSummary: "Get Oauth Providers",
		},
		{
			name:            "Health check endpoint",
			method:          "GET",
			path:            "/health",
			expectedTag:     "health",
			expectedSummary: "Get Health",
		},
		{
			name:            "MFA setup endpoint",
			method:          "POST",
			path:            "/api/v1/user/mfa/setup",
			expectedTag:     "user",
			expectedSummary: "Create User Mfa Setup",
		},
		{
			name:            "Root endpoint",
			method:          "GET",
			path:            "/",
			expectedTag:     "root",
			expectedSummary: "Get Root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := parser.ParseRoute(tt.method, tt.path)

			assert.Equal(t, tt.expectedTag, parsed.Tag)
			assert.Equal(t, tt.expectedSummary, parsed.Summary)
			assert.Contains(t, parsed.Description, "operation")
		})
	}
}

func TestOverrideManager(t *testing.T) {
	om := NewOverrideManager()
	parser := parser.NewPathParser()

	// Test exact path override
	om.Override("POST", "/api/v1/auth/login", RouteMetadata{
		Tags:        "authentication",
		Summary:     "User Authentication",
		Description: "Authenticate user and return tokens",
	})

	// Parse route algorithmically
	parsed := parser.ParseRoute("POST", "/api/v1/auth/login")

	// Get metadata with overrides
	metadata := om.GetMetadata("POST", "/api/v1/auth/login", parsed)

	assert.Equal(t, "authentication", metadata.Tags)
	assert.Equal(t, "User Authentication", metadata.Summary)
	assert.Equal(t, "Authenticate user and return tokens", metadata.Description)
}

func TestPatternOverride(t *testing.T) {
	om := NewOverrideManager()
	parser := parser.NewPathParser()

	// Test pattern-based override
	err := om.OverridePattern("POST */login", RouteMetadata{
		Summary:     "Login Operation",
		Description: "Generic login operation",
	})
	assert.NoError(t, err)

	// Test different login endpoints
	loginPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/oauth/login",
		"/admin/login",
	}

	for _, path := range loginPaths {
		parsed := parser.ParseRoute("POST", path)
		metadata := om.GetMetadata("POST", path, parsed)

		// Pattern override should work now
		assert.Equal(t, "Login Operation", metadata.Summary)
		assert.Equal(t, "Generic login operation", metadata.Description)
	}
}

func TestTagOverrides(t *testing.T) {
	om := NewOverrideManager()
	parser := parser.NewPathParser()

	// Override auth tag
	om.OverrideTags("auth", "authentication")

	parsed := parser.ParseRoute("POST", "/api/v1/auth/login")
	metadata := om.GetMetadata("POST", "/api/v1/auth/login", parsed)

	assert.Equal(t, "authentication", metadata.Tags)
}

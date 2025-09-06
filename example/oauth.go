package example

import "github.com/zainokta/openapi-gen"

// CustomizeOAuth provides comprehensive OAuth endpoint customization
//
// This function enhances the automatically generated documentation for OAuth-related
// endpoints with detailed provider information, flow descriptions, and security considerations.
//
// Applied to endpoints:
// - POST /api/v1/oauth/login
// - GET /api/v1/oauth/callback
// - GET /api/v1/oauth/providers
//
// Expected Results:
//
// Before customization:
//
//	GET /api/v1/oauth/providers
//	- Tags: ["oauth"]
//	- Summary: "Get Oauth Providers"
//	- Description: "Get Oauth Providers operation"
//
// After customization:
//
//	GET /api/v1/oauth/providers
//	- Tags: ["oauth2"]
//	- Summary: "List Available OAuth Providers"
//	- Description: "Get list of configured OAuth providers (Google, Microsoft) with their authorization URLs..."
//
// Usage:
//
//	openapi.EnableDocsWithCustomization(server, config, logger, example.CustomizeOAuth)
func CustomizeOAuth(generator *openapi.Generator) error {
	om := generator.GetOverrideManager()

	// Enhanced OAuth tags for better categorization
	om.OverrideTags("oauth", "oauth2")

	// Enhanced OAuth login initiation endpoint
	om.Override("POST", "/api/v1/oauth/login", openapi.RouteMetadata{
		Tags:        "oauth2",
		Summary:     "Initiate OAuth Authentication",
		Description: "Start OAuth 2.0 authentication flow with external provider (Google, Microsoft). Returns authorization URL for user redirection and state parameter for CSRF protection. User should be redirected to the returned URL to complete authentication.",
	})

	// Enhanced OAuth callback endpoint
	om.Override("GET", "/api/v1/oauth/callback", openapi.RouteMetadata{
		Tags:        "oauth2",
		Summary:     "OAuth Provider Callback",
		Description: "Handle authentication callback from OAuth providers after user authorization. Processes authorization code, exchanges it for access token, creates or links user account, and returns JWT tokens. Automatically creates new user if email doesn't exist.",
	})

	// Enhanced OAuth providers list endpoint
	om.Override("GET", "/api/v1/oauth/providers", openapi.RouteMetadata{
		Tags:        "oauth2",
		Summary:     "List Available OAuth Providers",
		Description: "Get list of configured OAuth 2.0 providers with their authorization URLs, redirect URLs, and supported scopes. Use this endpoint to dynamically build OAuth login buttons in your frontend application.",
	})

	// Pattern-based override for all OAuth endpoints
	err := om.OverridePattern("*/oauth/*", openapi.RouteMetadata{
		Tags: "oauth2",
	})
	if err != nil {
		return err
	}

	return nil
}

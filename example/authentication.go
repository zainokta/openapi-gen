package example

import "github.com/zainokta/openapi-gen"

// CustomizeAuthentication provides comprehensive authentication endpoint customization
//
// This function enhances the automatically generated documentation for authentication-related
// endpoints with detailed descriptions, proper tags, and comprehensive information.
//
// Applied to endpoints:
// - POST /api/v1/auth/login
// - POST /api/v1/auth/register
// - POST /api/v1/auth/refresh-token
// - POST /api/v1/auth/logout
//
// Expected Results:
//
// Before customization:
//
//	POST /api/v1/auth/login
//	- Tags: ["auth"]
//	- Summary: "Create Auth Login"
//	- Description: "Create Auth Login operation"
//
// After customization:
//
//	POST /api/v1/auth/login
//	- Tags: ["authentication"]
//	- Summary: "User Authentication"
//	- Description: "Authenticate user with email and password. Returns JWT access token and refresh token for session management."
//
// Usage:
//
//	openapi.EnableDocsWithCustomization(server, config, logger, example.CustomizeAuthentication)
func CustomizeAuthentication(generator *openapi.Generator) error {
	om := generator.GetOverrideManager()

	// Enhanced login endpoint
	om.Override("POST", "/api/v1/auth/login", openapi.RouteMetadata{
		Tags:        "authentication",
		Summary:     "User Authentication",
		Description: "Authenticate user with email and password. Returns JWT access token and refresh token for session management. If MFA is enabled, returns a challenge token instead of access token.",
	})

	// Enhanced registration endpoint
	om.Override("POST", "/api/v1/auth/register", openapi.RouteMetadata{
		Tags:        "authentication",
		Summary:     "User Registration",
		Description: "Create a new user account with email, password, and profile information. Account requires email verification before activation. Returns user details and confirmation message.",
	})

	// Enhanced refresh token endpoint
	om.Override("POST", "/api/v1/auth/refresh-token", openapi.RouteMetadata{
		Tags:        "authentication",
		Summary:     "Refresh Access Token",
		Description: "Generate a new access token using a valid refresh token. Extends session without requiring re-authentication. Old access token is invalidated.",
	})

	// Enhanced logout endpoint
	om.Override("POST", "/api/v1/auth/logout", openapi.RouteMetadata{
		Tags:        "authentication",
		Summary:     "User Logout",
		Description: "Logout user and invalidate authentication tokens. Can optionally logout from all sessions across all devices.",
	})

	// Enhanced MFA verification endpoint
	om.Override("POST", "/api/v1/auth/verify-mfa", openapi.RouteMetadata{
		Tags:        "authentication",
		Summary:     "Verify MFA Token",
		Description: "Complete authentication process by verifying MFA token. Requires valid challenge ID from initial login attempt. Returns access and refresh tokens on successful verification.",
	})

	// Apply tag-level improvements for all auth endpoints
	om.OverrideTags("auth", "authentication")

	return nil
}

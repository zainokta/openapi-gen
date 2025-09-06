package example

import "github.com/zainokta/openapi-gen"

// CustomizeWithPatterns demonstrates pattern-based customization techniques
//
// This function shows how to use pattern matching to apply customizations to multiple
// endpoints at once, reducing duplication and ensuring consistency across similar routes.
//
// Pattern Types Demonstrated:
// - Method + Path patterns: "POST */login"
// - Path-only patterns: "*/health"
// - Wildcard patterns: "*/password-reset/*"
//
// Expected Results:
//
// Pattern "POST */login" matches:
//   - POST /api/v1/auth/login
//   - POST /api/v1/oauth/login
//   - POST /admin/login
//
// All matched endpoints get:
//   - Summary: "Authentication Login"
//   - Description: "Authenticate user via login endpoint"
//
// Usage:
//
//	openapi.EnableDocsWithCustomization(server, config, logger, example.CustomizeWithPatterns)
func CustomizeWithPatterns(generator *openapi.Generator) error {
	om := generator.GetOverrideManager()

	// Pattern 1: All login endpoints (regardless of path)
	err := om.OverridePattern("POST */login", openapi.RouteMetadata{
		Tags:        "authentication",
		Summary:     "Authentication Login",
		Description: "Authenticate user via login endpoint with credentials validation and session creation",
	})
	if err != nil {
		return err
	}

	// Pattern 2: All logout endpoints
	err = om.OverridePattern("POST */logout", openapi.RouteMetadata{
		Tags:        "authentication",
		Summary:     "User Logout",
		Description: "Terminate user session and invalidate authentication tokens",
	})
	if err != nil {
		return err
	}

	// Pattern 3: All health/monitoring endpoints (any HTTP method)
	err = om.OverridePattern("*/health", openapi.RouteMetadata{
		Tags:        "monitoring",
		Summary:     "System Health Check",
		Description: "Get comprehensive system health status including dependencies and performance metrics",
	})
	if err != nil {
		return err
	}

	// Pattern 4: All password reset related endpoints
	err = om.OverridePattern("*/password-reset/*", openapi.RouteMetadata{
		Tags:        "password-reset",
		Summary:     "Password Reset Operation",
		Description: "Password reset functionality for account recovery",
	})
	if err != nil {
		return err
	}

	// Pattern 5: All MFA related endpoints
	err = om.OverridePattern("*/mfa/*", openapi.RouteMetadata{
		Tags:        "multi-factor-auth",
		Summary:     "Multi-Factor Authentication",
		Description: "MFA security operations for enhanced account protection",
	})
	if err != nil {
		return err
	}

	// Pattern 6: All admin endpoints
	err = om.OverridePattern("*/admin/*", openapi.RouteMetadata{
		Tags:        "admin",
		Summary:     "Administrative Operation",
		Description: "Administrative functionality requiring elevated privileges",
	})
	if err != nil {
		return err
	}

	// Pattern 7: Method-specific patterns for different operations
	err = om.OverridePattern("GET */list", openapi.RouteMetadata{
		Summary:     "List Resources",
		Description: "Retrieve list of resources with optional filtering and pagination",
	})
	if err != nil {
		return err
	}

	err = om.OverridePattern("POST */create", openapi.RouteMetadata{
		Summary:     "Create Resource",
		Description: "Create new resource with provided data and validation",
	})
	if err != nil {
		return err
	}

	err = om.OverridePattern("DELETE */delete", openapi.RouteMetadata{
		Summary:     "Delete Resource",
		Description: "Permanently remove resource after authorization checks",
	})
	if err != nil {
		return err
	}

	return nil
}

// CustomizeByEnvironment demonstrates environment-specific customizations
//
// This function shows how to apply different customizations based on the environment
// (development, staging, production) to provide appropriate documentation.
//
// Usage:
//
//	openapi.EnableDocsWithCustomization(server, config, logger,
//	    func(g *openapi.Generator) error {
//	        return example.CustomizeByEnvironment(g, config.Environment)
//	    })
func CustomizeByEnvironment(generator *openapi.Generator, environment string) error {
	om := generator.GetOverrideManager()

	switch environment {
	case "development":
		// Add debug endpoints documentation in development
		om.Override("GET", "/debug/routes", openapi.RouteMetadata{
			Tags:        "debug",
			Summary:     "Debug Route Information",
			Description: "Development-only endpoint showing all registered routes and handlers",
		})

		om.Override("GET", "/debug/config", openapi.RouteMetadata{
			Tags:        "debug",
			Summary:     "Debug Configuration",
			Description: "Development-only endpoint showing current application configuration",
		})

	case "staging":
		// Add testing-related documentation in staging
		om.Override("POST", "/test/reset-db", openapi.RouteMetadata{
			Tags:        "testing",
			Summary:     "Reset Test Database",
			Description: "Staging-only endpoint to reset database to known state for testing",
		})

	case "production":
		// Add production-specific security notes
		err := om.OverridePattern("*/admin/*", openapi.RouteMetadata{
			Tags:        "admin",
			Summary:     "Administrative Operation",
			Description: "⚠️ PRODUCTION: Administrative functionality. Requires special authorization and audit logging.",
		})
		if err != nil {
			return err
		}
	}

	return nil
}

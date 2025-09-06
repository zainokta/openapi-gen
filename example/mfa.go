package example

import "github.com/zainokta/openapi-gen"

// CustomizeMFA provides comprehensive Multi-Factor Authentication endpoint customization
//
// This function enhances the automatically generated documentation for MFA-related
// endpoints with detailed security information, setup instructions, and usage examples.
//
// Applied to endpoints:
// - POST /api/v1/user/mfa/setup
// - POST /api/v1/user/mfa/enable
// - POST /api/v1/user/mfa/disable
// - POST /api/v1/user/mfa/backup-codes
//
// Expected Results:
//
// Before customization:
//
//	POST /api/v1/user/mfa/setup
//	- Tags: ["user"]
//	- Summary: "Create User Mfa Setup"
//	- Description: "Create User Mfa Setup operation"
//
// After customization:
//
//	POST /api/v1/user/mfa/setup
//	- Tags: ["multi-factor-auth"]
//	- Summary: "Initialize MFA Setup"
//	- Description: "Initialize MFA setup process. Generates TOTP secret, QR code, and backup codes for user..."
//
// Usage:
//
//	openapi.EnableDocsWithCustomization(server, config, logger, example.CustomizeMFA)
func CustomizeMFA(generator *openapi.Generator) error {
	om := generator.GetOverrideManager()

	// Enhanced MFA tags for better categorization
	om.OverrideTags("mfa", "multi-factor-auth")
	om.OverrideTags("user", "user-management") // When MFA endpoints are under /user

	// Enhanced MFA setup endpoint
	om.Override("POST", "/api/v1/user/mfa/setup", openapi.RouteMetadata{
		Tags:        "multi-factor-auth",
		Summary:     "Initialize MFA Setup",
		Description: "Initialize MFA setup process. Generates TOTP secret, QR code URL for authenticator apps, and backup codes. User must scan QR code with authenticator app (Google Authenticator, Authy, etc.) before enabling MFA.",
	})

	// Enhanced MFA enable endpoint
	om.Override("POST", "/api/v1/user/mfa/enable", openapi.RouteMetadata{
		Tags:        "multi-factor-auth",
		Summary:     "Enable MFA Protection",
		Description: "Enable MFA for user account after successful TOTP verification. Requires valid TOTP token from authenticator app to confirm setup. Once enabled, user will need TOTP codes for all future logins.",
	})

	// Enhanced MFA disable endpoint
	om.Override("POST", "/api/v1/user/mfa/disable", openapi.RouteMetadata{
		Tags:        "multi-factor-auth",
		Summary:     "Disable MFA Protection",
		Description: "Disable MFA for user account with proper authentication. Requires current valid TOTP token or backup code. Warning: This reduces account security and should be done carefully.",
	})

	// Enhanced backup codes endpoint
	om.Override("POST", "/api/v1/user/mfa/backup-codes", openapi.RouteMetadata{
		Tags:        "multi-factor-auth",
		Summary:     "Generate MFA Backup Codes",
		Description: "Generate new set of single-use backup codes for MFA recovery. Invalidates all previous backup codes. Store these codes securely - they can be used instead of TOTP when authenticator is unavailable.",
	})

	// Pattern-based override for all MFA endpoints
	err := om.OverridePattern("*/mfa/*", openapi.RouteMetadata{
		Tags: "multi-factor-auth",
	})
	if err != nil {
		return err
	}

	return nil
}

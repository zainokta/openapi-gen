# OpenAPI Customization Examples

This directory contains examples of how to customize the automatically generated OpenAPI documentation for your auth service.

## Overview

The OpenAPI generator provides three levels of customization:

1. **Algorithm**: Pure algorithmic generation from route paths
2. **Presets**: Common patterns applied automatically  
3. **Custom**: User-defined overrides for specific needs

## Basic Usage

### Simple Integration (Recommended)
```go
// In your main.go
import "github.com/zainokta/openapi-gen"

// Create config
cfg := openapi.NewConfig()
cfg.Title = "Your API"
cfg.Description = "Your API description"
cfg.Version = "1.0.0"

// One line to enable docs
err := openapi.EnableDocs(framework, httpServer, cfg, logger)
if err != nil {
    // handle error
}
```

**Result**: Access documentation at:
- **Swagger UI**: `http://localhost:8080/docs`
- **OpenAPI Spec**: `http://localhost:8080/openapi.json`

### Advanced Integration with Customization
```go
import (
    "github.com/zainokta/openapi-gen"
    "github.com/zainokta/openapi-gen/example"
)

// Enable with custom configuration
err := openapi.EnableDocs(framework, httpServer, cfg, logger, 
    func(generator *openapi.Generator) error {
        // Apply multiple customizations
        if err := example.CustomizeAuthentication(generator); err != nil {
            return err
        }
        if err := example.CustomizeMFA(generator); err != nil {
            return err
        }
        return example.CustomizeOAuth(generator)
    })
if err != nil {
    // handle error
}
```

## What Gets Generated

### Pure Algorithm Results
The system automatically generates documentation from your routes:

| Route | Generated Tag | Generated Summary | Generated Description |
|-------|---------------|-------------------|----------------------|
| `POST /api/v1/auth/login` | `auth` | `Create Auth Login` | `Create Auth Login operation` |
| `GET /api/v1/oauth/providers` | `oauth` | `Get Oauth Providers` | `Get Oauth Providers operation` |
| `POST /api/v1/user/mfa/setup` | `user` | `Create User Mfa Setup` | `Create User Mfa Setup operation` |
| `GET /health` | `health` | `Get Health` | `Get Health operation` |

### With Presets Applied
Presets improve the algorithmic results:

| Route | Enhanced Tags | Enhanced Summary | Enhanced Description |
|-------|---------------|------------------|----------------------|
| `POST /api/v1/auth/login` | `authentication` | `User Authentication` | `Authenticate user and return authentication tokens` |
| `GET /health` | `system, monitoring` | `Service Health Check` | `Get comprehensive service health status...` |

### With Custom Overrides
Your custom functions can provide perfect documentation:

| Route | Custom Tags | Custom Summary | Custom Description |
|-------|-------------|----------------|-------------------|
| `POST /api/v1/auth/login` | `authentication` | `User Authentication` | `Authenticate user with email and password. Returns JWT access token and refresh token for session management.` |

## Available Customization Functions

See the example files in this directory:

- **`authentication.go`** - Authentication endpoints customization
- **`mfa.go`** - Multi-factor authentication customization  
- **`oauth.go`** - OAuth provider customization
- **`patterns.go`** - Pattern-based customization examples

## Customization Types

### 1. Exact Path Override
```go
om.Override("POST", "/api/v1/auth/login", openapi.RouteMetadata{
    Tags:        "authentication",
    Summary:     "User Authentication",
    Description: "Authenticate user with email and password",
})
```

### 2. Pattern-Based Override
```go
om.OverridePattern("POST */login", openapi.RouteMetadata{
    Summary:     "Login Operation",
    Description: "Authenticate user via login endpoint",
})
```

### 3. Tag-Level Override
```go
om.OverrideTags("auth", "authentication")
```

## Performance Notes

- **Zero Runtime Cost**: Documentation is generated once on startup
- **Development Mode**: Spec regenerated on each request for live updates
- **Production Mode**: Spec cached after first generation
- **Memory Usage**: ~100KB for typical auth service with 20+ endpoints

## Framework Support

Currently supported:
- ✅ **CloudWeGo Hertz** (auto-detection via framework interface{})

Framework detection is automatic - just pass your framework instance:
```go
// Works with any framework implementing the discoverer interface
err := openapi.EnableDocs(hertzFramework, httpServer, cfg, logger)
```

## Troubleshooting

### Common Issues

1. **No routes found**: Ensure `EnableDocs` is called after route registration
2. **Pattern not matching**: Use debug logging to check regex patterns
3. **Missing schemas**: DTO types need to be exported structs

### Debug Mode
```go
// Enable debug logging
om := generator.GetOverrideManager()
overrides := om.ListOverrides()       // See active overrides
stats := om.GetOverrideStats()       // Get override statistics
```

### Testing
```bash
# Test your customizations
go test ./...

# Check generated spec
curl http://localhost:8080/openapi.json | jq .
```
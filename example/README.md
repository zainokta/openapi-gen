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

// Minimal setup with defaults
err := openapi.EnableDocs(framework, httpServer)
if err != nil {
    // handle error
}
```

**Result**: Access documentation at:
- **Swagger UI**: `http://localhost:8080/docs`
- **OpenAPI Spec**: `http://localhost:8080/openapi.json`

### Custom Configuration
```go
import (
    "log/slog"
    "os"
    "github.com/zainokta/openapi-gen"
)

// Custom config and logger
cfg := openapi.NewConfig()
cfg.Title = "Your API"
cfg.Description = "Your API description" 
cfg.Version = "1.0.0"
cfg.SchemaDir = "./schemas"  // Directory for static schema files

// Option 1: Use slog (recommended, convenience function)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),
    openapi.WithSlogLogger(logger),  // Convenience function for slog
)

// Option 2: Use any custom logger implementing Logger interface
type MyLogger struct{}
func (l *MyLogger) Info(msg string, args ...any) { /* your implementation */ }
func (l *MyLogger) Warn(msg string, args ...any) { /* your implementation */ }
func (l *MyLogger) Error(msg string, args ...any) { /* your implementation */ }
func (l *MyLogger) Debug(msg string, args ...any) { /* your implementation */ }

err = openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),
    openapi.WithLogger(&MyLogger{}),  // Any logger implementing the interface
)

if err != nil {
    // handle error
}
```

### Advanced Integration with Customization
```go
import (
    "github.com/zainokta/openapi-gen"
    "github.com/zainokta/openapi-gen/example"
)

// Multiple customizations with options pattern
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),
    openapi.WithLogger(logger),
    openapi.WithCustomizer(example.CustomizeAuthentication),
    openapi.WithCustomizer(example.CustomizeMFA),
    openapi.WithCustomizer(example.CustomizeOAuth),
)
if err != nil {
    // handle error
}
```

### Custom Framework Integration
```go
import "github.com/zainokta/openapi-gen"

// Implement custom route discoverer for your framework
type MyFrameworkDiscoverer struct {
    framework *MyFramework
}

func (d *MyFrameworkDiscoverer) DiscoverRoutes() ([]spec.RouteInfo, error) {
    // Your custom route discovery logic
    return routes, nil
}

func (d *MyFrameworkDiscoverer) GetFrameworkName() string {
    return "MyFramework"
}

// Use with custom discoverer
discoverer := &MyFrameworkDiscoverer{framework: myFramework}
err := openapi.EnableDocs(myFramework, httpServer,
    openapi.WithRouteDiscoverer(discoverer),
    openapi.WithCustomizer(example.CustomizeAuthentication),
)
```

## Schema Generation with go:generate

For production environments where source code is not available, use `go:generate` annotations to create static schema files at build time.

### Adding go:generate Annotations

Add annotations to your handlers to specify request and response types:

```go
//go:generate openapi-gen -request dto.LoginRequest -response dto.AuthResponse
func (c *authController) Login(ctx context.Context, c *app.RequestContext) {
    var req dto.LoginRequest
    if err := c.BindAndValidate(&req); err != nil {
        c.JSON(400, map[string]interface{}{"error": err.Error()})
        return
    }
    
    // Handler implementation
    resp := authService.Login(&req)
    c.JSON(200, resp)
}

//go:generate openapi-gen -response dto.UserResponse
func (c *userController) GetUser(ctx context.Context, c *app.RequestContext) {
    userID := c.Param("id")
    
    // Handler implementation
    user := userService.GetUser(userID)
    c.JSON(200, user)
}

//go:generate openapi-gen -request dto.CreateUserRequest
func (c *userController) CreateUser(ctx context.Context, c *app.RequestContext) {
    var req dto.CreateUserRequest
    if err := c.BindAndValidate(&req); err != nil {
        c.JSON(400, map[string]interface{}{"error": err.Error()})
        return
    }
    
    // Handler implementation
    user := userService.CreateUser(&req)
    c.JSON(201, user)
}
```

### Generating Schema Files

Generate static schema files using the CLI tool:

```bash
# Generate schemas for all annotated handlers
go generate ./...

# Or specify files explicitly
go run github.com/zainokta/openapi-gen/cmd/gen-schemas -output ./schemas handlers/*.go
```

This creates JSON schema files in the specified directory (default: `./schemas`).

### Using Static Schemas in Production

Configure the generator to use static schema files:

```go
func main() {
    h := server.Default()
    
    // Add your routes here
    h.POST("/auth/login", authController.Login)
    h.GET("/users/:id", userController.GetUser)
    h.POST("/users", userController.CreateUser)
    
    // Enable OpenAPI documentation with static schemas
    err := openapi.EnableDocs(h, integration.NewHertzServerAdapter(h),
        openapi.WithSchemaDir("./schemas"), // Directory containing generated schema files
    )
    if err != nil {
        panic(err)
    }
    
    h.Spin()
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
| `GET /health` | `system` | `Service Health Check` | `Get comprehensive service health status...` |

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

## Logging Options

The OpenAPI generator supports flexible logging through a generic Logger interface, allowing integration with any logging framework.

### Built-in Logger Support

```go
// Default behavior - uses slog.Default()
err := openapi.EnableDocs(framework, httpServer)

// Custom slog logger (convenience function)
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithSlogLogger(logger),
)

// No-op logger (silent operation)
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithLogger(&openapi.NoOpLogger{}),
)
```

### Custom Logger Integration

Implement the Logger interface for any logging framework:

```go
// Example with Logrus (hypothetical adapter)
type LogrusAdapter struct {
    logger *logrus.Logger
}

func (l *LogrusAdapter) Info(msg string, args ...any) {
    l.logger.WithFields(argsToFields(args)).Info(msg)
}
// ... implement Warn, Error, Debug methods

// Usage
logrusLogger := logrus.New()
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithLogger(&LogrusAdapter{logger: logrusLogger}),
)
```

The Logger interface is simple and easy to implement:

```go
type Logger interface {
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    Debug(msg string, args ...any)
}
```

## Framework Support

Currently supported:
- ✅ **CloudWeGo Hertz** (auto-detection via framework interface{})

Framework detection is automatic - just pass your framework instance:
```go
// Works with CloudWeGo Hertz out of the box
err := openapi.EnableDocs(hertzFramework, httpServer)

// For custom frameworks, provide your own discoverer
err := openapi.EnableDocs(customFramework, httpServer,
    openapi.WithRouteDiscoverer(myCustomDiscoverer),
)
```

## Troubleshooting

### Common Issues

1. **No routes found**: Ensure `EnableDocs` is called after route registration
2. **Pattern not matching**: Use debug logging to check regex patterns
3. **Missing schemas**: DTO types need to be exported structs
4. **Generic schemas in production**: When source files aren't available, the generator uses fallback schemas

**Solutions for production schemas**:
1. **Use go:generate annotations** (recommended):
   ```go
   // Add annotations to your handlers
   //go:generate openapi-gen -request dto.LoginRequest -response dto.AuthResponse
   func (c *authController) Login(ctx context.Context, c *app.RequestContext) {
       // Handler implementation
   }
   
   // Generate schema files
   go generate ./...
   
   // Use static schemas in production
   openapi.WithSchemaDir("./schemas")
   ```

2. **Include schema files in Docker builds**:
   ```dockerfile
   # Build stage
   FROM golang:1.21-alpine AS builder
   WORKDIR /app
   COPY . .
   RUN go mod download
   RUN go generate ./...
   RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .
   
   # Production stage
   FROM alpine:latest
   COPY --from=builder /app/myapp .
   COPY --from=builder /app/schemas ./schemas
   CMD ["./myapp"]
   ```

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

# Test with different configurations
go run main.go  # with defaults
go run examples/custom-config/main.go  # with custom config
go run examples/custom-framework/main.go  # with custom discoverer
```
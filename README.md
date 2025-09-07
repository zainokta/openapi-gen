# OpenAPI Generator for Go Web Frameworks

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.25-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A powerful, framework-agnostic OpenAPI documentation generator for Go web applications. Automatically generates comprehensive OpenAPI 3.0.3 specifications from your route definitions with intelligent AST analysis, Docker support, and flexible customization options. Works seamlessly as a library in any Go application with production-ready fallback mechanisms.

## ✨ Features

- **🚀 Zero Configuration**: Works out of the box with sensible defaults
- **🧠 Intelligent Generation**: AST analysis of route paths and handlers for accurate schemas
- **🎨 Flexible Customization**: Override any aspect of the generated documentation
- **📝 Multiple Frameworks**: Extensible architecture supports any Go web framework
- **⚡ High Performance**: Zero runtime cost, documentation generated at startup
- **🔧 Options Pattern**: Clean, extensible API with functional options
- **📊 Generic Logging**: Integrate with any logging framework
- **🐳 Docker Ready**: Static schema files for production deployment
- **📦 Library Support**: Works seamlessly as external library in any Go application
- **🔍 AST Analysis**: Automatic request/response schema generation from handler code
- **🏗️ go:generate Support**: Compile-time schema generation for production environments

## 🚀 Quick Start

### Installation

```bash
go get github.com/zainokta/openapi-gen
```

### Basic Usage

#### CloudWeGo Hertz

```go
package main

import (
    "github.com/cloudwego/hertz/pkg/app/server"
    "github.com/zainokta/openapi-gen"
    "github.com/zainokta/openapi-gen/integration"
)

func main() {
    h := server.Default()
    
    // Add your routes here
    h.GET("/api/v1/users", getUsersHandler)
    h.POST("/api/v1/users", createUserHandler)
    
    // Enable OpenAPI documentation with proper library usage
    err := openapi.EnableDocs(h, integration.NewHertzServerAdapter(h))
    if err != nil {
        panic(err)
    }
    
    h.Spin()
}
```

#### Gin

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/zainokta/openapi-gen"
    "github.com/zainokta/openapi-gen/integration"
)

func main() {
    r := gin.Default()
    
    // Add your routes here
    r.GET("/api/v1/users", getUsersHandler)
    r.POST("/api/v1/users", createUserHandler)
    
    // Enable OpenAPI documentation with proper library usage
    err := openapi.EnableDocs(r, integration.NewGinServerAdapter(r))
    if err != nil {
        panic(err)
    }
    
    r.Run()
}
```

**That's it!** Your API documentation is now available at:
- **Swagger UI**: `http://localhost:8080/docs`
- **OpenAPI Spec**: `http://localhost:8080/openapi.json`

## 🐳 Docker & Production Usage

### Development vs Production

```go
// Development Mode (with AST analysis)
func setupDevelopment() error {
    h := server.Default()
    return openapi.EnableDocs(h, integration.NewHertzServerAdapter(h))
}

// Production Mode (with static schemas)
func setupProduction() error {
    h := server.Default()
    return openapi.EnableDocs(h, integration.NewHertzServerAdapter(h),
        openapi.WithSchemaDir("./schemas"),
    )
}
```

### Docker Build with Schema Files

Include schema files in your Docker build:

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download

# Generate schema files
RUN go generate ./...

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .

# Production stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary AND schema files
COPY --from=builder /app/myapp .
COPY --from=builder /app/schemas ./schemas

CMD ["./myapp"]
```

## 🏗️ go:generate Schema Generation

For production environments where source code is not available, use `go:generate` annotations to create static schema files at build time.

### Adding go:generate Annotations

Add annotations to your handlers to specify request and response types:

```go
//go:generate openapi-gen -request dto.LoginRequest -response dto.AuthResponse -handler Login .
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

//go:generate openapi-gen -response dto.UserResponse -handler GetUser .
func (c *userController) GetUser(ctx context.Context, c *app.RequestContext) {
    userID := c.Param("id")
    
    // Handler implementation
    user := userService.GetUser(userID)
    c.JSON(200, user)
}

//go:generate openapi-gen -request dto.CreateUserRequest -handler CreateUser .
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

# Or use the cmd/openapi-gen tool directly
go run ./cmd/openapi-gen -output ./schemas handlers/*.go

# Install the tool globally
go install ./cmd/openapi-gen
openapi-gen -output ./schemas handlers/*.go
```

This creates JSON schema files in the specified directory (default: `./schemas`).

### CLI Tool Features

The `cmd/openapi-gen/` tool provides advanced schema generation capabilities:

- **Automatic struct analysis** - Parses Go structs and generates OpenAPI/JSON schemas
- **JSON tag support** - Uses JSON tag names instead of Go variable names  
- **Package root detection** - Automatically finds the package root and generates schemas there
- **Type-aware generation** - Handles basic types, arrays, maps, pointers, and custom types
- **Recursive directory search** - Finds struct definitions in subdirectories

### CLI Options

```bash
openapi-gen [options] <files>

Options:
  -output string     Output directory for schema files (default "./schemas")
  -verbose           Enable verbose output
  -request string    Request type in format package.TypeName
  -response string   Response type in format package.TypeName
  -handler string    Handler name (auto-detected if not provided)
```

### Example Usage

```bash
# Generate schemas with verbose output
openapi-gen -verbose -output ./api-schemas handlers/*.go

# Generate specific handler schema
openapi-gen -request dto.LoginRequest -response dto.AuthResponse -handler Login . handlers/auth.go

# Generate all schemas in project
go generate ./...
```

### Generated Schema Format

Each handler generates a JSON schema file:

```json
{
  "handlerName": "LoginHandler",
  "requestSchema": {
    "type": "object",
    "properties": {
      "email": {"type": "string"},
      "password": {"type": "string"}
    },
    "required": ["email", "password"]
  },
  "responseSchema": {
    "type": "object", 
    "properties": {
      "access_token": {"type": "string"},
      "refresh_token": {"type": "string"},
      "expires_in": {"type": "integer", "format": "int64"},
      "token_type": {"type": "string"}
    },
    "required": ["access_token", "refresh_token", "expires_in", "token_type"]
  }
}
```

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

## 🎯 Advanced Usage

### Custom Configuration

```go
// Create custom configuration
cfg := openapi.NewConfig()
cfg.Title = "My Awesome API"
cfg.Description = "A comprehensive API for my application"
cfg.Version = "2.0.0"
cfg.Contact.Name = "API Team"
cfg.Contact.Email = "api@example.com"

// Enable with custom config
err := openapi.EnableDocs(framework, integration.NewHertzServerAdapter(framework),
    openapi.WithConfig(cfg),
)
```

### Environment-Based Configuration

```go
import "os"

func getOpenAPIConfig() *openapi.Config {
    cfg := openapi.NewConfig()
    
    if os.Getenv("ENV") == "production" {
        cfg.SchemaDir = "./schemas"
    }
    
    return cfg
}

// Use environment-based config
err := openapi.EnableDocs(h, integration.NewHertzServerAdapter(h),
    openapi.WithConfig(getOpenAPIConfig()),
)
```

### Custom Logging

```go
// With slog (recommended)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithSlogLogger(logger),
)

// With any custom logger
type MyLogger struct{}
func (l *MyLogger) Info(msg string, args ...any) { /* implementation */ }
func (l *MyLogger) Warn(msg string, args ...any) { /* implementation */ }
func (l *MyLogger) Error(msg string, args ...any) { /* implementation */ }
func (l *MyLogger) Debug(msg string, args ...any) { /* implementation */ }

err := openapi.EnableDocs(framework, httpServer,
    openapi.WithLogger(&MyLogger{}),
)
```

### Route Customization

```go
import "github.com/zainokta/openapi-gen/example"

err := openapi.EnableDocs(framework, httpServer,
    openapi.WithCustomizer(example.CustomizeAuthentication),
    openapi.WithCustomizer(example.CustomizeMFA),
    openapi.WithCustomizer(func(generator *openapi.Generator) error {
        om := generator.GetOverrideManager()
        om.Override("GET", "/api/v1/users", openapi.RouteMetadata{
            Tags:        "users",
            Summary:     "List Users",
            Description: "Retrieve a paginated list of all users",
        })
        return nil
    }),
)
```

### Custom Framework Integration

```go
// Implement the RouteDiscoverer interface for your framework
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
)
```

## 🏗️ Architecture

### Three Levels of Customization

1. **🤖 Algorithmic**: Intelligent generation from route paths
2. **📋 Preset**: Common patterns applied automatically  
3. **✏️ Custom**: User-defined overrides for specific needs

### Example Generated Documentation

| Route | Generated | Enhanced | Custom |
|-------|-----------|----------|---------|
| `POST /api/v1/auth/login` | `Create Auth Login` | `User Authentication` | `Authenticate user with email and password. Returns JWT tokens.` |
| `GET /api/v1/users/:id` | `Get Users Id` | `Get User Details` | `Retrieve user information by unique identifier` |

## 🔧 Options API

All configuration is done through functional options:

```go
openapi.EnableDocs(framework, integration.NewHertzServerAdapter(framework),
    // Configuration Options
    openapi.WithConfig(cfg),                    // Custom configuration
    openapi.WithSchemaDir("./schemas"),        // Static schema files directory
    
    // Logging & Discovery
    openapi.WithSlogLogger(logger),            // slog logger (convenience)
    openapi.WithLogger(customLogger),          // Any logger interface
    openapi.WithRouteDiscoverer(discoverer),   // Custom framework integration
    openapi.WithCustomizer(customizeFunc),     // Route customizations
)
```

## 🌐 Framework Support

### Currently Supported
- ✅ **CloudWeGo Hertz** - Full auto-detection support
- ✅ **Gin** - Full auto-detection support

### Coming Soon
- 🔄 **Echo** - Interface ready, implementation planned  
- 🔄 **Fiber** - Interface ready, implementation planned
- 🔄 **Chi** - Interface ready, implementation planned

*Framework integration is designed to be pluggable. Contributions welcome!*

## 📚 Documentation

- **[Examples](./example/README.md)**: Comprehensive usage examples
- **[Customization Guide](./example/README.md#customization-types)**: Learn how to customize documentation
- **[Framework Integration](./example/README.md#custom-framework-integration)**: Add support for your framework

## 🚨 Troubleshooting

### Common Issues

#### Generic schemas in production
When source files aren't available, the generator uses fallback schemas.

**Solutions**:
1. **Use go:generate annotations** (recommended):
   ```go
   // Add annotations to your handlers
   //go:generate openapi-gen -request dto.CreateUserRequest -response dto.UserResponse -handler CreateUser .
   func (c *userController) CreateUser(ctx context.Context, c *app.RequestContext) {
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
   FROM golang:1.25-alpine AS builder
   WORKDIR /app
   COPY . .
   RUN go mod download
   
   # Install and run the schema generator
   RUN go install ./cmd/openapi-gen
   RUN go generate ./...
   
   # Build the application
   RUN CGO_ENABLED=0 GOOS=linux go build -o myapp .
   
   # Production stage
   FROM alpine:latest
   COPY --from=builder /app/myapp .
   COPY --from=builder /app/schemas ./schemas
   CMD ["./myapp"]
   ```

#### Import path issues when using as library
Make sure to use the correct import paths:

```go
import (
    "github.com/zainokta/openapi-gen"
    "github.com/zainokta/openapi-gen/integration"
)

// Correct usage
openapi.EnableDocs(h, integration.NewHertzServerAdapter(h))
```

### Environment Variables

Control behavior with environment variables:

```bash
OPENAPI_SCHEMA_DIR=./schemas    # Set schema files directory
```

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines for details.

### TODO Roadmap
- [x] **Docker and Production Support** - Complete with static schema files and go:generate
- [x] **Library Usage Improvements** - Cross-package AST analysis and configuration options  
- [x] **AST-based Schema Generation** - Automatic request/response schema extraction
- [x] **go:generate Schema Generation** - Compile-time schema generation for production
- [x] **Gin Framework Support** - Complete integration with Gin framework
- [ ] Additional framework integrations (Echo, Fiber, Chi)
- [ ] Plugin system for custom analyzers
- [ ] OpenAPI 3.1 support
- [ ] Performance optimizations for large APIs
- [ ] Enhanced validation tag support

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Built with ❤️ using:
- [CloudWeGo Hertz](https://github.com/cloudwego/hertz) - High-performance Go HTTP framework
- [Swagger UI](https://swagger.io/tools/swagger-ui/) - Interactive API documentation

---

**Made with ❤️ for the Go community**
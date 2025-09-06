# OpenAPI Generator for Go Web Frameworks

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.21-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A powerful, framework-agnostic OpenAPI documentation generator for Go web applications. Automatically generates comprehensive OpenAPI 3.0.3 specifications from your route definitions with intelligent algorithmic analysis and flexible customization options.

## ✨ Features

- **🚀 Zero Configuration**: Works out of the box with sensible defaults
- **🧠 Intelligent Generation**: Algorithmic analysis of route paths and handlers
- **🎨 Flexible Customization**: Override any aspect of the generated documentation
- **📝 Multiple Frameworks**: Extensible architecture supports any Go web framework
- **⚡ High Performance**: Zero runtime cost, documentation generated at startup
- **🔧 Options Pattern**: Clean, extensible API with functional options
- **📊 Generic Logging**: Integrate with any logging framework

## 🚀 Quick Start

### Installation

```bash
go get github.com/zainokta/openapi-gen
```

### Basic Usage

```go
package main

import (
    "github.com/zainokta/openapi-gen"
    "github.com/cloudwego/hertz/pkg/app/server"
)

func main() {
    h := server.Default()
    
    // Add your routes here
    h.GET("/api/v1/users", getUsersHandler)
    h.POST("/api/v1/users", createUserHandler)
    
    // Enable OpenAPI documentation with one line
    err := openapi.EnableDocs(h, h)
    if err != nil {
        panic(err)
    }
    
    h.Spin()
}
```

**That's it!** Your API documentation is now available at:
- **Swagger UI**: `http://localhost:8080/docs`
- **OpenAPI Spec**: `http://localhost:8080/openapi.json`

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
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),
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
openapi.EnableDocs(framework, httpServer,
    openapi.WithConfig(cfg),              // Custom configuration
    openapi.WithSlogLogger(logger),       // slog logger (convenience)
    openapi.WithLogger(customLogger),     // Any logger interface
    openapi.WithRouteDiscoverer(discoverer), // Custom framework integration
    openapi.WithCustomizer(customizeFunc),   // Route customizations
)
```

## 🌐 Framework Support

### Currently Supported
- ✅ **CloudWeGo Hertz** - Full auto-detection support

### Coming Soon
- 🔄 **Gin** - Interface ready, implementation planned
- 🔄 **Echo** - Interface ready, implementation planned  
- 🔄 **Fiber** - Interface ready, implementation planned
- 🔄 **Chi** - Interface ready, implementation planned

*Framework integration is designed to be pluggable. Contributions welcome!*

## 📈 Performance

- **Zero Runtime Cost**: Documentation generated once at startup
- **Memory Efficient**: ~100KB for typical API with 20+ endpoints

## 📚 Documentation

- **[Examples](./example/README.md)**: Comprehensive usage examples
- **[Customization Guide](./example/README.md#customization-types)**: Learn how to customize documentation
- **[Framework Integration](./example/README.md#custom-framework-integration)**: Add support for your framework

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines for details.

### TODO Roadmap
- [ ] Request/response body schema generator improvements
- [ ] Additional framework integrations (Gin, Echo, Fiber, Chi)
- [ ] Plugin system for custom analyzers
- [ ] OpenAPI 3.1 support

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Built with ❤️ using:
- [CloudWeGo Hertz](https://github.com/cloudwego/hertz) - High-performance Go HTTP framework
- [Swagger UI](https://swagger.io/tools/swagger-ui/) - Interactive API documentation

---

**Made with ❤️ for the Go community**
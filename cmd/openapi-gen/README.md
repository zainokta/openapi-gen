# OpenAPI Schema Generator CLI

This directory contains the CLI tool for generating static OpenAPI schema files from `go:generate` annotations.

## Overview

The `cmd/openapi-gen/` tool is a standalone CLI that generates static OpenAPI schema files by analyzing Go struct definitions and parsing `go:generate` annotations in your handler code. It creates JSON schema files that can be consumed by the main OpenAPI generator.

## Key Features

- **Automatic struct analysis** - Parses Go structs and generates OpenAPI/JSON schemas
- **JSON tag support** - Uses JSON tag names instead of Go variable names
- **Package root detection** - Automatically finds the package root and generates schemas there
- **go:generate integration** - Works seamlessly with Go's generate tool
- **Type-aware generation** - Handles basic types, arrays, maps, pointers, and custom types

## Usage

### Basic Usage

```bash
# Generate schemas from annotated handlers
go run . -output ./schemas example/handlers.go

# Or use with go:generate
go generate ./...
```

### Advanced Usage with go:generate

Add annotations to your handlers:

```go
//go:generate openapi-gen -request dto.LoginRequest -response dto.AuthResponse -handler LoginHandler .
func LoginHandler(ctx context.Context, c *app.RequestContext) {
    // Handler implementation
}
```

Then run:
```bash
# Generate schemas for all annotated handlers
go generate ./...

# Generate from a specific directory
cd example && go generate ./...
```

### CLI Options

- `-output`: Output directory for schema files (default: `./schemas`)
- `-verbose`: Enable verbose output
- `-request`: Request type in format `package.TypeName`
- `-response`: Response type in format `package.TypeName`  
- `-handler`: Handler name (auto-detected if not provided)

## How It Works

### 1. Package Root Detection
The tool automatically finds the package root by searching up the directory tree for `go.mod`. All schemas are generated in the package root's `schemas/` directory, ensuring consistent output location regardless of where the command is run.

### 2. Struct Analysis
The tool parses Go struct definitions and generates OpenAPI schemas:
- Converts Go types to JSON Schema types
- Uses JSON tag names for property names
- Handles required fields based on `omitempty` tags
- Supports nested structs, arrays, maps, and pointers

### 3. go:generate Integration
The tool integrates with Go's generate system:
- Parses `go:generate` comments to extract configuration
- Supports multiple handlers in the same file
- Each handler gets its own schema file

## Generated Schema Files

Each handler generates a JSON file named after the handler function:

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

## Integration with Main OpenAPI Generator

The generated schema files are automatically loaded by the main OpenAPI generator:

```go
// Configure the generator to use static schemas
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithSchemaDir("./schemas"),
)
```

The main generator will:
1. Load all JSON schema files from the specified directory
2. Register them as handler schemas
3. Use them when generating the complete OpenAPI specification

## Example Structure

```
project-root/
├── go.mod
├── schemas/                    # Generated here automatically
│   ├── LoginHandler.json
│   ├── CreateUserHandler.json
│   └── ...
├── cmd/
│   └── openapi-gen/
│       ├── main.go
│       └── example/
│           ├── handlers.go     # Your annotated handlers
│           └── dto/
│               └── types.go     # Your DTO types
└── main.go                     # Your application
```

### Example

The `example/` directory contains a complete working example with:

1. **Handler implementations** (`example/handlers.go`) - Complete handlers with dummy service implementations
2. **DTO types** (`example/dto/types.go`) - Request and response type definitions

#### Example Handler Structure:

```go
//go:generate openapi-gen -request dto.LoginRequest -response dto.AuthResponse
func LoginHandler(ctx context.Context, c *app.RequestContext) {
    var req dto.LoginRequest
    if err := c.BindAndValidate(&req); err != nil {
        c.JSON(400, map[string]interface{}{"error": err.Error()})
        return
    }
    
    // Handler implementation
    resp := authService.Login(&req)
    c.JSON(200, resp)
}
```

#### DTO Types:

```go
package dto

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type AuthResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
    TokenType    string `json:"token_type"`
}
```

### Generate Schema Files

```bash
# Generate schemas from the example
go run . -output ./schemas example/handlers.go

# Or use go:generate in the example directory
cd example && go generate ./...
```

This will generate JSON schema files in the specified directory.

## CLI Options

- `-output`: Output directory for schema files (default: `./schemas`)
- `-verbose`: Enable verbose output

## Generated Schema Files

Each handler gets a JSON file named after the handler function:

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

## Integration with OpenAPI Generator

Use the generated schema files with the OpenAPI generator:

```go
err := openapi.EnableDocs(framework, httpServer,
    openapi.WithSchemaDir("./schemas"),
)
```
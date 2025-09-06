package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/zainokta/openapi-gen/analyzer"
	"github.com/zainokta/openapi-gen/integration"
	"github.com/zainokta/openapi-gen/logger"
	"github.com/zainokta/openapi-gen/parser"
	"github.com/zainokta/openapi-gen/spec"
)

// Generator is the main OpenAPI specification generator
type Generator struct {
	config          *Config
	logger          logger.Logger
	discoverer      integration.RouteDiscoverer
	pathParser      *parser.PathParser
	overrideManager *OverrideManager
	structParser    *parser.StructParser
	schemaRegistry  *analyzer.SchemaRegistry
	handlerAnalyzer analyzer.HandlerAnalyzer
	spec            *spec.OpenAPISpec
}

// NewGenerator creates a new OpenAPI generator with options
func NewGenerator(framework any, httpServer integration.HTTPServer, options *Options) (*Generator, error) {
	var discoverer integration.RouteDiscoverer
	var err error

	// Use custom discoverer if provided, otherwise auto-discover
	if options.customDiscoverer != nil {
		discoverer = options.customDiscoverer
	} else {
		// Create framework-agnostic discoverer
		discoverer, err = integration.NewAutoDiscoverer(framework)
		if err != nil {
			return nil, fmt.Errorf("failed to create route discoverer: %w", err)
		}
	}

	// Create components
	pathParser := parser.NewPathParser()
	overrideManager := NewOverrideManager()
	structParser := parser.NewStructParser()
	schemaRegistry := analyzer.NewSchemaRegistry()
	handlerAnalyzer := integration.NewHertzHandlerAnalyzer()

	generator := &Generator{
		config:          options.config,
		logger:          options.logger,
		discoverer:      discoverer,
		pathParser:      pathParser,
		overrideManager: overrideManager,
		structParser:    structParser,
		schemaRegistry:  schemaRegistry,
		handlerAnalyzer: handlerAnalyzer,
	}

	// Initialize common DTO schemas
	generator.structParser.RegisterDTOSchemas()
	generator.schemaRegistry.RegisterCommonDTOs()

	return generator, nil
}

// GetOverrideManager returns the override manager for customization
func (g *Generator) GetOverrideManager() *OverrideManager {
	return g.overrideManager
}

// GetSchemaRegistry returns the schema registry for manual schema registration
func (g *Generator) GetSchemaRegistry() *analyzer.SchemaRegistry {
	return g.schemaRegistry
}

// GetLogger returns the configured logger instance
func (g *Generator) GetLogger() logger.Logger {
	return g.logger
}

// GenerateSpec generates the complete OpenAPI specification
func (g *Generator) GenerateSpec() (*spec.OpenAPISpec, error) {
	// Discover routes from the framework
	routes, err := g.discoverer.DiscoverRoutes()
	if err != nil {
		return nil, fmt.Errorf("failed to discover routes: %w", err)
	}

	g.logger.Info("Discovered routes", "count", len(routes), "framework", g.discoverer.GetFrameworkName())

	// Initialize OpenAPI spec
	g.spec = &spec.OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: spec.Info{
			Title:       g.config.Title,
			Description: g.config.Description,
			Version:     g.config.Version,
			Contact: spec.Contact{
				Name:  g.config.Contact.Name,
				Email: g.config.Contact.Email,
				URL:   g.config.Contact.URL,
			},
		},
		Servers: []spec.Server{
			{
				URL:         g.config.GetServerURL(),
				Description: g.config.GetServerDescription(),
			},
		},
		Paths: make(map[string]spec.PathItem),
		Components: spec.Components{
			Schemas:         make(map[string]spec.Schema),
			SecuritySchemes: g.generateSecuritySchemes(),
		},
		Security: []spec.SecurityRequirement{
			{
				"bearerAuth": []string{},
			},
		},
		Tags: make([]spec.Tag, 0),
	}

	// Process routes and generate OpenAPI paths
	tags := make(map[string]bool)
	for _, route := range routes {
		if err := g.processRoute(route, tags); err != nil {
			g.logger.Warn("Failed to process route", "method", route.Method, "path", route.Path, "error", err)
			continue
		}
	}

	// Generate tags from collected unique tags
	g.spec.Tags = g.generateTagsFromSet(tags)

	// Add schemas from both struct parser and schema registry
	allSchemas := make(map[string]spec.Schema)

	// Add schemas from struct parser (basic types)
	for name, schema := range g.structParser.GetSchemas() {
		allSchemas[name] = schema
	}

	// Add schemas from schema registry (handler DTOs)
	for name, schema := range g.schemaRegistry.GetAllSchemas() {
		allSchemas[name] = schema
	}

	g.spec.Components.Schemas = allSchemas

	g.logger.Info("Generated OpenAPI spec",
		"paths", len(g.spec.Paths),
		"tags", len(g.spec.Tags),
		"schemas", len(g.spec.Components.Schemas))

	return g.spec, nil
}

// processRoute processes a single route and adds it to the OpenAPI spec
func (g *Generator) processRoute(route spec.RouteInfo, tags map[string]bool) error {
	// Analyze handler to extract request/response types
	if route.Handler != nil {
		handlerSchema := g.handlerAnalyzer.AnalyzeHandler(route.Handler)

		// Register the discovered schemas with the schema registry
		if handlerSchema.RequestSchema.Type != "" {
			g.schemaRegistry.RegisterRequestSchema(route.Method, route.Path, handlerSchema.RequestSchema)
		}
		if handlerSchema.ResponseSchema.Type != "" {
			g.schemaRegistry.RegisterResponseSchema(route.Method, route.Path, handlerSchema.ResponseSchema)
		}
	}

	// Parse route using algorithm
	parsed := g.pathParser.ParseRoute(route.Method, route.Path)

	// Apply overrides
	metadata := g.overrideManager.GetMetadata(route.Method, route.Path, parsed)

	// Collect tags
	tags[metadata.Tags] = true

	// Create OpenAPI operation
	operation := g.createOperation(route, metadata)

	// Add to spec
	g.addOperationToSpec(route.Method, route.Path, operation)

	return nil
}

// createOperation creates an OpenAPI operation from route information
func (g *Generator) createOperation(route spec.RouteInfo, metadata RouteMetadata) spec.Operation {
	operation := spec.Operation{
		Tags:        []string{metadata.Tags},
		Summary:     metadata.Summary,
		Description: metadata.Description,
		OperationID: g.generateOperationID(route.Method, route.Path),
		Parameters:  g.extractParameters(route.Path),
		Responses:   g.generateResponses(route),
	}

	// Add request body for methods that typically have one
	if g.hasRequestBody(route.Method) {
		requestBody := g.generateRequestBodyFromRoute(route)
		operation.RequestBody = &requestBody
	}

	// Add security if not a public endpoint
	if !g.isPublicEndpoint(route.Path) {
		operation.Security = []spec.SecurityRequirement{
			{"bearerAuth": []string{}},
		}
	} else {
		operation.Security = []spec.SecurityRequirement{} // No auth required
	}

	return operation
}

// extractParameters extracts parameters from route path
func (g *Generator) extractParameters(path string) []spec.Parameter {
	var params []spec.Parameter

	// Extract path parameters (e.g., :id, :token)
	paramRegex := regexp.MustCompile(`:(\w+)`)
	matches := paramRegex.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		if len(match) > 1 {
			paramName := match[1]
			param := spec.Parameter{
				Name:        paramName,
				In:          "path",
				Required:    true,
				Description: fmt.Sprintf("Path parameter: %s", paramName),
				Schema:      spec.Schema{Type: "string"},
			}
			params = append(params, param)
		}
	}

	// Add common query parameters for certain endpoints
	if strings.Contains(path, "mfa") && strings.Contains(path, "verify") {
		params = append(params, spec.Parameter{
			Name:        "challenge",
			In:          "query",
			Required:    true,
			Description: "MFA challenge ID",
			Schema:      spec.Schema{Type: "string"},
		})
	}

	return params
}

// generateResponses generates responses using dynamic schema resolution
func (g *Generator) generateResponses(route spec.RouteInfo) map[string]spec.Response {
	responses := make(map[string]spec.Response)

	// Try to get schema from handler analysis
	handlerSchema := g.handlerAnalyzer.AnalyzeHandler(route.Handler)

	var successSchema spec.Schema
	if handlerSchema.ResponseSchema.Type != "" {
		successSchema = handlerSchema.ResponseSchema
	} else {
		// Fallback to generic success schema
		successSchema = spec.Schema{
			Type: "object",
			Properties: map[string]spec.Schema{
				"data":    {Type: "object", Description: "Response data"},
				"message": {Type: "string", Description: "Success message"},
			},
		}
	}

	// Success response
	responses["200"] = spec.Response{
		Description: "Success",
		Content: map[string]spec.MediaType{
			"application/json": {
				Schema: successSchema,
			},
		},
	}

	// Error responses (reuse existing logic)
	errorResponses := g.generateDefaultResponses()
	for code, response := range errorResponses {
		if code != "200" { // Don't override success response
			responses[code] = response
		}
	}

	return responses
}

// generateDefaultResponses generates default responses for an operation
func (g *Generator) generateDefaultResponses() map[string]spec.Response {
	responses := make(map[string]spec.Response)

	// Success response
	responses["200"] = spec.Response{
		Description: "Success",
		Content: map[string]spec.MediaType{
			"application/json": {
				Schema: spec.Schema{
					Type: "object",
					Properties: map[string]spec.Schema{
						"data":    {Type: "object", Description: "Response data"},
						"message": {Type: "string", Description: "Success message"},
					},
				},
			},
		},
	}

	// Error responses
	responses["400"] = spec.Response{
		Description: "Bad Request",
		Content: map[string]spec.MediaType{
			"application/json": {
				Schema: g.getErrorSchema(),
			},
		},
	}

	responses["401"] = spec.Response{
		Description: "Unauthorized",
		Content: map[string]spec.MediaType{
			"application/json": {
				Schema: g.getErrorSchema(),
			},
		},
	}

	responses["500"] = spec.Response{
		Description: "Internal Server Error",
		Content: map[string]spec.MediaType{
			"application/json": {
				Schema: g.getErrorSchema(),
			},
		},
	}

	return responses
}

// getErrorSchema returns the standard error schema
func (g *Generator) getErrorSchema() spec.Schema {
	return spec.Schema{
		Type: "object",
		Properties: map[string]spec.Schema{
			"error":   {Type: "string", Description: "Error message"},
			"code":    {Type: "integer", Description: "Error code"},
			"details": {Type: "object", Description: "Additional error details"},
		},
		Required: []string{"error", "code"},
	}
}

// generateRequestBodyFromRoute generates request body using dynamic schema resolution
func (g *Generator) generateRequestBodyFromRoute(route spec.RouteInfo) spec.RequestBody {
	// Try to get schema from handler analysis
	handlerSchema := g.handlerAnalyzer.AnalyzeHandler(route.Handler)

	var schema spec.Schema
	if handlerSchema.RequestSchema.Type != "" {
		schema = handlerSchema.RequestSchema
	} else {
		// Fallback to generic schema
		schema = spec.Schema{
			Type: "object",
			Properties: map[string]spec.Schema{
				"data": {Type: "object", Description: "Request data"},
			},
		}
	}

	return spec.RequestBody{
		Required: true,
		Content: map[string]spec.MediaType{
			"application/json": {
				Schema: schema,
			},
		},
	}
}

// hasRequestBody determines if an operation should have a request body
func (g *Generator) hasRequestBody(method string) bool {
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// isPublicEndpoint determines if an endpoint requires authentication
func (g *Generator) isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/",
		"/health",
		"/docs",
		"/openapi.json",
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/oauth/login",
		"/api/v1/oauth/callback",
		"/api/v1/oauth/providers",
		"/api/v1/auth/password-reset/request",
		"/api/v1/auth/password-reset/confirm",
	}

	for _, publicPath := range publicPaths {
		if path == publicPath || strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	// Check for password reset validate endpoint (has path parameter)
	if strings.Contains(path, "/password-reset/validate/") {
		return true
	}

	return false
}

// generateOperationID generates a unique operation ID
func (g *Generator) generateOperationID(method, path string) string {
	// Use path parser to generate consistent ID
	return g.pathParser.GenerateHandlerName(method, path)
}

// addOperationToSpec adds an operation to the OpenAPI spec
func (g *Generator) addOperationToSpec(method, path string, operation spec.Operation) {
	// Get or create path item
	pathItem := g.spec.Paths[path]

	// Add operation based on method
	switch strings.ToUpper(method) {
	case "GET":
		pathItem.Get = &operation
	case "POST":
		pathItem.Post = &operation
	case "PUT":
		pathItem.Put = &operation
	case "PATCH":
		pathItem.Patch = &operation
	case "DELETE":
		pathItem.Delete = &operation
	case "HEAD":
		pathItem.Head = &operation
	case "OPTIONS":
		pathItem.Options = &operation
	case "TRACE":
		pathItem.Trace = &operation
	}

	g.spec.Paths[path] = pathItem
}

// generateTagsFromSet generates tag definitions from collected tags
func (g *Generator) generateTagsFromSet(tags map[string]bool) []spec.Tag {
	var result []spec.Tag

	for tagName := range tags {
		tag := spec.Tag{
			Name:        tagName,
			Description: g.generateTagDescription(tagName),
		}
		result = append(result, tag)
	}

	return result
}

// generateTagDescription generates description for a tag
func (g *Generator) generateTagDescription(tagName string) string {
	descriptions := map[string]string{
		"auth":              "User authentication and session management",
		"authentication":    "User authentication and session management",
		"oauth":             "OAuth 2.0 authentication with external providers",
		"external-auth":     "External authentication providers",
		"user":              "User account management and profile operations",
		"mfa":               "Multi-factor authentication management",
		"multi-factor-auth": "Multi-factor authentication management",
		"password-reset":    "Password reset functionality",
		"system":            "System health and information endpoints",
		"monitoring":        "System monitoring and health checks",
		"info":              "Service information endpoints",
		"security":          "Security-related operations",
	}

	if desc, exists := descriptions[tagName]; exists {
		return desc
	}

	// Generate description from tag name
	caser := cases.Title(language.English)
	return fmt.Sprintf("%s related operations", caser.String(tagName))
}

// generateSecuritySchemes generates security scheme definitions
func (g *Generator) generateSecuritySchemes() map[string]spec.SecurityScheme {
	return map[string]spec.SecurityScheme{
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "JWT Bearer token authentication",
		},
	}
}

// ServeSwaggerUI serves the Swagger UI and OpenAPI spec
func (g *Generator) ServeSwaggerUI(h integration.HTTPServer) error {
	// Generate the spec first
	spec, err := g.GenerateSpec()
	if err != nil {
		return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
	}

	// Serve OpenAPI spec JSON
	h.GET("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(spec)
	})

	// Serve Swagger UI
	h.GET("/docs", func(w http.ResponseWriter, r *http.Request) {
		html := g.generateSwaggerHTML()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	})

	g.logger.Info("Swagger UI endpoints registered", "spec_url", "/openapi.json", "docs_url", "/docs")

	return nil
}

// generateSwaggerHTML generates the Swagger UI HTML
func (g *Generator) generateSwaggerHTML() string {
	return `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Auth Service API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.28.1/swagger-ui.css" />
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.28.1/favicon-32x32.png" sizes="32x32" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            background: #fafafa;
        }
        .swagger-ui .info .title {
            color: #3b82f6;
        }
        .swagger-ui .scheme-container {
            background: #f8fafc;
            border: 1px solid #e2e8f0;
        }
        #swagger-ui {
            max-width: 1460px;
            margin: 0 auto;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.28.1/swagger-ui-bundle.js" charset="UTF-8"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.28.1/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
    <script>
        window.onload = function() {
            console.log('Initializing Swagger UI...');
            
            const ui = SwaggerUIBundle({
                url: '/openapi.json',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                showExtensions: true,
                showCommonExtensions: true,
                tryItOutEnabled: true,
                onComplete: function() {
                    console.log('Swagger UI loaded successfully');
                },
                onFailure: function(error) {
                    console.error('Failed to load Swagger UI:', error);
                }
            });

            // Test if openapi.json is accessible
            fetch('/openapi.json')
                .then(response => {
                    if (!response.ok) {
                        throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                    }
                    return response.json();
                })
                .then(data => {
                    console.log('OpenAPI spec loaded successfully:', data);
                })
                .catch(error => {
                    console.error('Failed to load OpenAPI spec:', error);
                });
        };
    </script>
</body>
</html>`
}

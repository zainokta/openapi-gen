package analyzer

import (
	"reflect"
	"strings"

	"github.com/zainokta/openapi-gen/spec"
)

// SchemaRegistry manages manual schema registration and overrides
type SchemaRegistry struct {
	requestSchemas  map[string]spec.Schema // key: "METHOD /path"
	responseSchemas map[string]spec.Schema
	typeSchemas     map[reflect.Type]spec.Schema // Direct type mapping
	schemaGen       *SchemaGenerator
}

// HandlerSchema represents request and response schemas for a handler
type HandlerSchema struct {
	RequestSchema  spec.Schema
	ResponseSchema spec.Schema
}

// NewSchemaRegistry creates a new schema registry
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		requestSchemas:  make(map[string]spec.Schema),
		responseSchemas: make(map[string]spec.Schema),
		typeSchemas:     make(map[reflect.Type]spec.Schema),
		schemaGen:       NewSchemaGenerator(),
	}
}

// RegisterRequestSchema registers a request schema for a specific endpoint
func (sr *SchemaRegistry) RegisterRequestSchema(method, path string, schema spec.Schema) {
	key := sr.createRouteKey(method, path)
	sr.requestSchemas[key] = schema
}

// RegisterResponseSchema registers a response schema for a specific endpoint
func (sr *SchemaRegistry) RegisterResponseSchema(method, path string, schema spec.Schema) {
	key := sr.createRouteKey(method, path)
	sr.responseSchemas[key] = schema
}

// RegisterHandlerSchemas registers both request and response schemas for an endpoint
func (sr *SchemaRegistry) RegisterHandlerSchemas(method, path string, reqSchema, respSchema spec.Schema) {
	sr.RegisterRequestSchema(method, path, reqSchema)
	sr.RegisterResponseSchema(method, path, respSchema)
}

// RegisterHandlerTypes registers schemas from Go types for an endpoint
func (sr *SchemaRegistry) RegisterHandlerTypes(method, path string, reqType, respType reflect.Type) {
	if reqType != nil {
		reqSchema := sr.schemaGen.GenerateSchemaFromType(reqType)
		sr.RegisterRequestSchema(method, path, reqSchema)
	}

	if respType != nil {
		respSchema := sr.schemaGen.GenerateSchemaFromType(respType)
		sr.RegisterResponseSchema(method, path, respSchema)
	}
}

// RegisterTypeSchema registers a schema for a specific Go type
func (sr *SchemaRegistry) RegisterTypeSchema(t reflect.Type, schema spec.Schema) {
	sr.typeSchemas[t] = schema
}

// GetRequestSchema retrieves request schema for an endpoint
func (sr *SchemaRegistry) GetRequestSchema(method, path string) (spec.Schema, bool) {
	key := sr.createRouteKey(method, path)
	schema, exists := sr.requestSchemas[key]
	return schema, exists
}

// GetResponseSchema retrieves response schema for an endpoint
func (sr *SchemaRegistry) GetResponseSchema(method, path string) (spec.Schema, bool) {
	key := sr.createRouteKey(method, path)
	schema, exists := sr.responseSchemas[key]
	return schema, exists
}

// GetHandlerSchemas retrieves both request and response schemas for an endpoint
func (sr *SchemaRegistry) GetHandlerSchemas(method, path string) HandlerSchema {
	reqSchema, _ := sr.GetRequestSchema(method, path)
	respSchema, _ := sr.GetResponseSchema(method, path)

	return HandlerSchema{
		RequestSchema:  reqSchema,
		ResponseSchema: respSchema,
	}
}

// GetTypeSchema retrieves schema for a specific Go type
func (sr *SchemaRegistry) GetTypeSchema(t reflect.Type) (spec.Schema, bool) {
	schema, exists := sr.typeSchemas[t]
	return schema, exists
}

// HasRequestSchema checks if a request schema exists for an endpoint
func (sr *SchemaRegistry) HasRequestSchema(method, path string) bool {
	key := sr.createRouteKey(method, path)
	_, exists := sr.requestSchemas[key]
	return exists
}

// HasResponseSchema checks if a response schema exists for an endpoint
func (sr *SchemaRegistry) HasResponseSchema(method, path string) bool {
	key := sr.createRouteKey(method, path)
	_, exists := sr.responseSchemas[key]
	return exists
}

// GenerateSchemaFromType generates schema using the internal schema generator
func (sr *SchemaRegistry) GenerateSchemaFromType(t reflect.Type) spec.Schema {
	// Check if we have a manual override first
	if schema, exists := sr.GetTypeSchema(t); exists {
		return schema
	}

	// Generate using schema generator
	return sr.schemaGen.GenerateSchemaFromType(t)
}

// ListRegisteredSchemas returns all registered schemas for debugging
func (sr *SchemaRegistry) ListRegisteredSchemas() map[string]interface{} {
	return map[string]interface{}{
		"request_schemas":    sr.requestSchemas,
		"response_schemas":   sr.responseSchemas,
		"type_schemas_count": len(sr.typeSchemas),
	}
}

// ClearAll clears all registered schemas
func (sr *SchemaRegistry) ClearAll() {
	sr.requestSchemas = make(map[string]spec.Schema)
	sr.responseSchemas = make(map[string]spec.Schema)
	sr.typeSchemas = make(map[reflect.Type]spec.Schema)
	sr.schemaGen.ClearCache()
}

// createRouteKey creates a consistent key for method+path combinations
func (sr *SchemaRegistry) createRouteKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// Bulk registration methods

// RegisterCommonDTOs registers schemas for common DTO patterns
func (sr *SchemaRegistry) RegisterCommonDTOs() {
	// Register common response wrappers
	sr.registerCommonResponseSchemas()
	sr.registerCommonErrorSchemas()
}

// registerCommonResponseSchemas registers common success response patterns
func (sr *SchemaRegistry) registerCommonResponseSchemas() {
	successSchema := spec.Schema{
		Type: "object",
		Properties: map[string]spec.Schema{
			"data":    {Type: "object", Description: "Response data"},
			"message": {Type: "string", Description: "Success message"},
		},
		Required: []string{"data"},
	}

	// Register for common endpoints that don't need specific schemas
	commonEndpoints := []struct{ method, path string }{
		{"GET", "/health"},
		{"GET", "/"},
	}

	for _, endpoint := range commonEndpoints {
		sr.RegisterResponseSchema(endpoint.method, endpoint.path, successSchema)
	}
}

// registerCommonErrorSchemas registers common error response patterns
func (sr *SchemaRegistry) registerCommonErrorSchemas() {
	errorSchema := spec.Schema{
		Type: "object",
		Properties: map[string]spec.Schema{
			"error":   {Type: "string", Description: "Error message"},
			"code":    {Type: "integer", Description: "Error code"},
			"details": {Type: "object", Description: "Additional error details"},
		},
		Required: []string{"error", "code"},
	}

	// This can be used for error responses across all endpoints
	// The generator will use this as a template for error schemas
	sr.typeSchemas[reflect.TypeOf(struct {
		Error   string      `json:"error"`
		Code    int         `json:"code"`
		Details interface{} `json:"details,omitempty"`
	}{})] = errorSchema
}

// GetSchemaGenerator returns the internal schema generator for advanced usage
func (sr *SchemaRegistry) GetSchemaGenerator() *SchemaGenerator {
	return sr.schemaGen
}

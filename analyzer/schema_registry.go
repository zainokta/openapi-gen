package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/zainokta/openapi-gen/spec"
)

// SchemaRegistry manages manual schema registration and overrides
type SchemaRegistry struct {
	requestSchemas  map[string]spec.Schema // key: "METHOD /path"
	responseSchemas map[string]spec.Schema
	typeSchemas     map[reflect.Type]spec.Schema // Direct type mapping
	routeMetadata   map[string]spec.RouteInfo    // key: "METHOD /path"
	handlerSchemas  map[string]HandlerSchema     // key: handler name
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
		routeMetadata:   make(map[string]spec.RouteInfo),
		handlerSchemas:  make(map[string]HandlerSchema),
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

// RegisterHandlerTypesWithMetadata registers schemas from Go types with additional metadata
func (sr *SchemaRegistry) RegisterHandlerTypesWithMetadata(method, path string, reqType, respType reflect.Type, metadata spec.RouteInfo) {
	// Register the types as schemas
	sr.RegisterHandlerTypes(method, path, reqType, respType)

	// Store metadata for later use by the generator
	key := sr.createRouteKey(method, path)
	sr.routeMetadata[key] = metadata
}

// RegisterHandlerTypesFromValues registers schemas from actual Go values (used by generated code)
func (sr *SchemaRegistry) RegisterHandlerTypesFromValues(method, path string, reqValue, respValue interface{}) {
	var reqType, respType reflect.Type

	if reqValue != nil {
		reqType = reflect.TypeOf(reqValue)
		// Handle pointer types
		if reqType.Kind() == reflect.Pointer {
			reqType = reqType.Elem()
		}
	}

	if respValue != nil {
		respType = reflect.TypeOf(respValue)
		// Handle pointer types
		if respType.Kind() == reflect.Pointer {
			respType = respType.Elem()
		}
	}

	sr.RegisterHandlerTypes(method, path, reqType, respType)
}

// RegisterHandlerTypesFromValuesWithMetadata registers schemas from values with metadata
func (sr *SchemaRegistry) RegisterHandlerTypesFromValuesWithMetadata(method, path string, reqValue, respValue interface{}, metadata spec.RouteInfo) {
	sr.RegisterHandlerTypesFromValues(method, path, reqValue, respValue)

	key := sr.createRouteKey(method, path)
	sr.routeMetadata[key] = metadata
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

// GetAllSchemas returns all registered schemas as a single map
func (sr *SchemaRegistry) GetAllSchemas() map[string]spec.Schema {
	allSchemas := make(map[string]spec.Schema)

	// Add request schemas
	for key, schema := range sr.requestSchemas {
		// Create a unique name for the schema
		name := sr.generateSchemaName(key, "request")
		allSchemas[name] = schema
	}

	// Add response schemas
	for key, schema := range sr.responseSchemas {
		// Create a unique name for the schema
		name := sr.generateSchemaName(key, "response")
		allSchemas[name] = schema
	}

	// Add type schemas
	for t, schema := range sr.typeSchemas {
		name := t.Name()
		if name != "" {
			allSchemas[name] = schema
		}
	}

	return allSchemas
}

// generateSchemaName generates a unique schema name from route key
func (sr *SchemaRegistry) generateSchemaName(routeKey, schemaType string) string {
	// Convert "POST /auth/login" to "PostAuthLoginRequest"
	cleanKey := strings.ReplaceAll(routeKey, " ", "")
	cleanKey = strings.ReplaceAll(cleanKey, "/", "_")
	cleanKey = strings.ReplaceAll(cleanKey, ":", "")

	// Capitalize first letter
	if len(cleanKey) > 0 {
		cleanKey = strings.ToUpper(cleanKey[:1]) + cleanKey[1:]
	}

	return cleanKey + schemaType
}

// ClearAll clears all registered schemas
func (sr *SchemaRegistry) ClearAll() {
	sr.requestSchemas = make(map[string]spec.Schema)
	sr.responseSchemas = make(map[string]spec.Schema)
	sr.typeSchemas = make(map[reflect.Type]spec.Schema)
	sr.routeMetadata = make(map[string]spec.RouteInfo)
	sr.handlerSchemas = make(map[string]HandlerSchema)
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

// GetRouteMetadata retrieves metadata for a specific endpoint
func (sr *SchemaRegistry) GetRouteMetadata(method, path string) (spec.RouteInfo, bool) {
	key := sr.createRouteKey(method, path)
	metadata, exists := sr.routeMetadata[key]
	return metadata, exists
}

// GetSchemaGenerator returns the internal schema generator for advanced usage
func (sr *SchemaRegistry) GetSchemaGenerator() *SchemaGenerator {
	return sr.schemaGen
}

// RegisterHandlerSchema registers a schema for a specific handler by name
func (sr *SchemaRegistry) RegisterHandlerSchema(handlerName string, schema HandlerSchema) {
	sr.handlerSchemas[handlerName] = schema
}

// GetHandlerSchema retrieves a schema for a specific handler by name
func (sr *SchemaRegistry) GetHandlerSchema(handlerName string) (HandlerSchema, bool) {
	schema, exists := sr.handlerSchemas[handlerName]
	return schema, exists
}

// HasHandlerSchema checks if a schema exists for a specific handler
func (sr *SchemaRegistry) HasHandlerSchema(handlerName string) bool {
	_, exists := sr.handlerSchemas[handlerName]
	return exists
}

// LoadStaticSchemas loads schema files from a directory
func (sr *SchemaRegistry) LoadStaticSchemas(schemaDir string) error {
	if _, err := os.Stat(schemaDir); os.IsNotExist(err) {
		// Schema directory doesn't exist, that's okay
		return nil
	}

	// Read all JSON files in the schema directory
	files, err := filepath.Glob(filepath.Join(schemaDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to read schema files: %w", err)
	}

	for _, file := range files {
		if err := sr.loadSchemaFile(file); err != nil {
			// Log error but continue loading other files
			fmt.Printf("Warning: failed to load schema file %s: %v\n", file, err)
			continue
		}
	}

	return nil
}

// loadSchemaFile loads a single schema file and registers it
func (sr *SchemaRegistry) loadSchemaFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the schema file
	var schemaFile struct {
		HandlerName    string                 `json:"handlerName"`
		RequestSchema  map[string]interface{} `json:"requestSchema,omitempty"`
		ResponseSchema map[string]interface{} `json:"responseSchema,omitempty"`
	}

	if err := json.Unmarshal(data, &schemaFile); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if schemaFile.HandlerName == "" {
		return fmt.Errorf("schema file missing handlerName")
	}

	// Convert map[string]interface{} to spec.Schema
	handlerSchema := HandlerSchema{}
	
	if schemaFile.RequestSchema != nil {
		handlerSchema.RequestSchema = sr.convertToSpecSchema(schemaFile.RequestSchema)
	}
	
	if schemaFile.ResponseSchema != nil {
		handlerSchema.ResponseSchema = sr.convertToSpecSchema(schemaFile.ResponseSchema)
	}

	// Register the handler schema
	sr.RegisterHandlerSchema(schemaFile.HandlerName, handlerSchema)

	return nil
}

// convertToSpecSchema converts a map[string]interface{} to spec.Schema
func (sr *SchemaRegistry) convertToSpecSchema(schemaMap map[string]interface{}) spec.Schema {
	schema := spec.Schema{}
	
	if typ, ok := schemaMap["type"].(string); ok {
		schema.Type = typ
	}
	
	if desc, ok := schemaMap["description"].(string); ok {
		schema.Description = desc
	}
	
	if props, ok := schemaMap["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]spec.Schema)
		for key, value := range props {
			if propMap, ok := value.(map[string]interface{}); ok {
				schema.Properties[key] = sr.convertToSpecSchema(propMap)
			}
		}
	}
	
	if required, ok := schemaMap["required"].([]interface{}); ok {
		schema.Required = make([]string, len(required))
		for i, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required[i] = reqStr
			}
		}
	}
	
	if format, ok := schemaMap["format"].(string); ok {
		schema.Format = format
	}
	
	if items, ok := schemaMap["items"].(map[string]interface{}); ok {
		itemSchema := sr.convertToSpecSchema(items)
		schema.Items = &itemSchema
	}
	
	if additionalProps, ok := schemaMap["additionalProperties"].(map[string]interface{}); ok {
		additionalSchema := sr.convertToSpecSchema(additionalProps)
		schema.AdditionalProperties = &additionalSchema
	}

	return schema
}

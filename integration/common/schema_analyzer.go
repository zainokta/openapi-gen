package common

import (
	"reflect"

	"github.com/zainokta/openapi-gen/analyzer"
	"github.com/zainokta/openapi-gen/spec"
)

// SchemaAnalyzer provides utilities for analyzing and generating OpenAPI schemas
type SchemaAnalyzer struct {
	schemaGen    *analyzer.SchemaGenerator
	typeResolver *TypeResolver
}

// NewSchemaAnalyzer creates a new SchemaAnalyzer
func NewSchemaAnalyzer() *SchemaAnalyzer {
	return &SchemaAnalyzer{
		schemaGen:    analyzer.NewSchemaGenerator(),
		typeResolver: NewTypeResolver(),
	}
}

// GetSchemaGenerator returns the internal schema generator
func (sa *SchemaAnalyzer) GetSchemaGenerator() *analyzer.SchemaGenerator {
	return sa.schemaGen
}

// GetTypeResolver returns the internal type resolver
func (sa *SchemaAnalyzer) GetTypeResolver() *TypeResolver {
	return sa.typeResolver
}

// GenerateFallbackSchemas generates generic schemas for Docker/production environments
func (sa *SchemaAnalyzer) GenerateFallbackSchemas() analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	// Generate generic request schema for POST/PUT/PATCH methods
	schema.RequestSchema = spec.Schema{
		Type: "object",
		Properties: map[string]spec.Schema{
			"data": {
				Type:        "object",
				Description: "Request payload (schema analysis unavailable in production mode)",
				AdditionalProperties: &spec.Schema{Type: "any"},
			},
		},
		Description: "Generic request schema - AST analysis not available",
	}

	// Generate generic response schema
	schema.ResponseSchema = spec.Schema{
		Type: "object",
		Properties: map[string]spec.Schema{
			"data": {
				Type:        "object",
				Description: "Response data",
				AdditionalProperties: &spec.Schema{Type: "any"},
			},
			"message": {
				Type:        "string",
				Description: "Response message",
				Example:     "Success",
			},
		},
		Description: "Generic response schema - AST analysis not available",
	}

	return schema
}

// GenerateSchemaFromType generates an OpenAPI schema from a Go type
func (sa *SchemaAnalyzer) GenerateSchemaFromType(typ reflect.Type) spec.Schema {
	if typ == nil {
		return spec.Schema{}
	}
	return sa.schemaGen.GenerateSchemaFromType(typ)
}

// AnalyzeHandlerSignature analyzes a handler function signature to extract request/response types
func (sa *SchemaAnalyzer) AnalyzeHandlerSignature(handlerType reflect.Type, framework string) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	if handlerType == nil || handlerType.Kind() != reflect.Func {
		return schema
	}

	// Extract request and response types based on framework
	switch framework {
	case "hertz":
		return sa.AnalyzeHertzHandlerSignature(handlerType)
	case "gin":
		return sa.AnalyzeGinHandlerSignature(handlerType)
	default:
		return schema
	}
}

// AnalyzeHertzHandlerSignature analyzes a Hertz handler signature
func (sa *SchemaAnalyzer) AnalyzeHertzHandlerSignature(handlerType reflect.Type) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	// Hertz handler signature: func(ctx context.Context, c *app.RequestContext)
	// We need to analyze the function body to extract actual request/response types
	// This is a simplified analysis - the real work is done in AST analysis

	return schema
}

// AnalyzeGinHandlerSignature analyzes a Gin handler signature
func (sa *SchemaAnalyzer) AnalyzeGinHandlerSignature(handlerType reflect.Type) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	// Gin handler signature: func(c *gin.Context)
	// We need to analyze the function body to extract actual request/response types
	// This is a simplified analysis - the real work is done in AST analysis

	return schema
}

// EnhanceSchemaWithMetadata enhances a schema with additional metadata
func (sa *SchemaAnalyzer) EnhanceSchemaWithMetadata(schema spec.Schema, title, description string) spec.Schema {
	if title != "" {
		schema.Title = title
	}
	if description != "" {
		schema.Description = description
	}
	return schema
}

// AddExampleToSchema adds an example to a schema
func (sa *SchemaAnalyzer) AddExampleToSchema(schema spec.Schema, example interface{}) spec.Schema {
	schema.Example = example
	return schema
}

// SetRequiredFields marks fields as required in a schema
func (sa *SchemaAnalyzer) SetRequiredFields(schema spec.Schema, requiredFields []string) spec.Schema {
	schema.Required = requiredFields
	return schema
}

// AddValidationToSchema adds validation constraints to a schema
func (sa *SchemaAnalyzer) AddValidationToSchema(schema spec.Schema, min, max *float64, pattern string) spec.Schema {
	if min != nil {
		schema.Minimum = min
	}
	if max != nil {
		schema.Maximum = max
	}
	if pattern != "" {
		schema.Pattern = pattern
	}
	return schema
}

// MergeSchemas merges two schemas into one
func (sa *SchemaAnalyzer) MergeSchemas(schema1, schema2 spec.Schema) spec.Schema {
	merged := schema1

	// Merge properties
	if schema2.Properties != nil {
		if merged.Properties == nil {
			merged.Properties = make(map[string]spec.Schema)
		}
		for k, v := range schema2.Properties {
			merged.Properties[k] = v
		}
	}

	// Merge required fields
	merged.Required = append(merged.Required, schema2.Required...)

	// Merge other fields
	if schema2.Description != "" {
		merged.Description = schema2.Description
	}
	if schema2.Title != "" {
		merged.Title = schema2.Title
	}

	return merged
}

// CreateObjectSchema creates an object schema with given properties
func (sa *SchemaAnalyzer) CreateObjectSchema(properties map[string]spec.Schema, required []string) spec.Schema {
	return spec.Schema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

// CreateArraySchema creates an array schema with given item type
func (sa *SchemaAnalyzer) CreateArraySchema(items spec.Schema) spec.Schema {
	return spec.Schema{
		Type:  "array",
		Items: &items,
	}
}

// CreateMapSchema creates a map schema with given key and value types
func (sa *SchemaAnalyzer) CreateMapSchema(keySchema, valueSchema spec.Schema) spec.Schema {
	return spec.Schema{
		Type: "object",
		AdditionalProperties: &valueSchema,
	}
}

// CreateStringSchema creates a string schema with validation
func (sa *SchemaAnalyzer) CreateStringSchema(minLength, maxLength *int64, pattern, format string) spec.Schema {
	schema := spec.Schema{Type: "string"}
	
	if minLength != nil {
		minInt := int(*minLength)
		schema.MinLength = &minInt
	}
	if maxLength != nil {
		maxInt := int(*maxLength)
		schema.MaxLength = &maxInt
	}
	if pattern != "" {
		schema.Pattern = pattern
	}
	if format != "" {
		schema.Format = format
	}
	
	return schema
}

// CreateNumberSchema creates a number schema with validation
func (sa *SchemaAnalyzer) CreateNumberSchema(minimum, maximum *float64, multipleOf *float64, format string) spec.Schema {
	schema := spec.Schema{Type: "number"}
	
	if minimum != nil {
		schema.Minimum = minimum
	}
	if maximum != nil {
		schema.Maximum = maximum
	}
	if multipleOf != nil {
		schema.MultipleOf = multipleOf
	}
	if format != "" {
		schema.Format = format
	}
	
	return schema
}

// CreateIntegerSchema creates an integer schema with validation
func (sa *SchemaAnalyzer) CreateIntegerSchema(minimum, maximum *int64, multipleOf *int64, format string) spec.Schema {
	schema := spec.Schema{Type: "integer"}
	
	if minimum != nil {
		minFloat := float64(*minimum)
		schema.Minimum = &minFloat
	}
	if maximum != nil {
		maxFloat := float64(*maximum)
		schema.Maximum = &maxFloat
	}
	if multipleOf != nil {
		multFloat := float64(*multipleOf)
		schema.MultipleOf = &multFloat
	}
	if format != "" {
		schema.Format = format
	}
	
	return schema
}

// CreateBooleanSchema creates a boolean schema
func (sa *SchemaAnalyzer) CreateBooleanSchema() spec.Schema {
	return spec.Schema{Type: "boolean"}
}

// CreateReferenceSchema creates a reference schema
func (sa *SchemaAnalyzer) CreateReferenceSchema(ref string) spec.Schema {
	return spec.Schema{
		Ref: ref,
	}
}

// IsComplexType checks if a type is complex (struct, slice, map, etc.)
func (sa *SchemaAnalyzer) IsComplexType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}

	switch typ.Kind() {
	case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map, reflect.Interface:
		return true
	case reflect.Ptr:
		return sa.IsComplexType(typ.Elem())
	default:
		return false
	}
}

// ShouldIncludeType checks if a type should be included in schema generation
func (sa *SchemaAnalyzer) ShouldIncludeType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}

	// Skip certain built-in types
	switch typ.String() {
	case "context.Context", "error", "io.Reader", "io.Writer":
		return false
	}

	// Skip unexported types
	if typ.Name() != "" && typ.PkgPath() != "" {
		return typ.Name()[0] >= 'A' && typ.Name()[0] <= 'Z'
	}

	return true
}

// ExtractTypeDocumentation extracts documentation from a type (simplified)
func (sa *SchemaAnalyzer) ExtractTypeDocumentation(typ reflect.Type) string {
	// This is a simplified implementation
	// In a full implementation, we would extract comments from AST
	return ""
}

// NormalizeSchema normalizes a schema by removing empty fields and ensuring consistency
func (sa *SchemaAnalyzer) NormalizeSchema(schema spec.Schema) spec.Schema {
	// Remove empty properties
	if len(schema.Properties) == 0 {
		schema.Properties = nil
	}
	if len(schema.Required) == 0 {
		schema.Required = nil
	}

	return schema
}

// ValidateSchema performs basic validation on a schema
func (sa *SchemaAnalyzer) ValidateSchema(schema spec.Schema) bool {
	// Basic validation checks
	if schema.Type == "" {
		return false
	}

	// Check that array has items
	if schema.Type == "array" && schema.Items == nil {
		return false
	}

	// Check that object properties are valid
	if schema.Type == "object" && schema.Properties != nil {
		for _, prop := range schema.Properties {
			if prop.Type == "" {
				return false
			}
		}
	}

	return true
}
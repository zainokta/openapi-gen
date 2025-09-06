package analyzer

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/zainokta/openapi-gen/spec"
)

// SchemaGenerator generates OpenAPI schemas from Go types using reflection
type SchemaGenerator struct {
	typeCache    map[reflect.Type]spec.Schema
	processing   map[reflect.Type]bool // Prevent infinite recursion
	maxDepth     int
	currentDepth int
}

// NewSchemaGenerator creates a new schema generator
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		typeCache:  make(map[reflect.Type]spec.Schema),
		processing: make(map[reflect.Type]bool),
		maxDepth:   10, // Prevent deep recursion
	}
}

// GenerateSchemaFromType generates OpenAPI schema from Go type
func (sg *SchemaGenerator) GenerateSchemaFromType(t reflect.Type) spec.Schema {
	// Check cache first
	if schema, exists := sg.typeCache[t]; exists {
		return schema
	}

	// Prevent infinite recursion
	if sg.processing[t] {
		return spec.Schema{Type: "object", Description: fmt.Sprintf("Circular reference to %s", t.String())}
	}

	if sg.currentDepth >= sg.maxDepth {
		return spec.Schema{Type: "object", Description: "Max depth reached"}
	}

	sg.processing[t] = true
	sg.currentDepth++
	defer func() {
		delete(sg.processing, t)
		sg.currentDepth--
	}()

	schema := sg.generateSchema(t)
	sg.typeCache[t] = schema
	return schema
}

// generateSchema is the core schema generation logic
func (sg *SchemaGenerator) generateSchema(t reflect.Type) spec.Schema {
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		return sg.generateSchema(t.Elem())
	}

	// Handle basic types
	if schema := sg.handleBasicType(t); schema.Type != "" {
		return schema
	}

	// Handle complex types
	switch t.Kind() {
	case reflect.Struct:
		return sg.handleStruct(t)
	case reflect.Slice, reflect.Array:
		return sg.handleArray(t)
	case reflect.Map:
		return sg.handleMap(t)
	case reflect.Interface:
		return sg.handleInterface(t)
	default:
		return spec.Schema{
			Type:        "object",
			Description: fmt.Sprintf("Unsupported type: %s", t.Kind()),
		}
	}
}

// handleBasicType handles Go basic types to OpenAPI types
func (sg *SchemaGenerator) handleBasicType(t reflect.Type) spec.Schema {
	switch t.Kind() {
	case reflect.String:
		return spec.Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return spec.Schema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return spec.Schema{Type: "integer", Minimum: float64Ptr(0)}
	case reflect.Float32, reflect.Float64:
		return spec.Schema{Type: "number"}
	case reflect.Bool:
		return spec.Schema{Type: "boolean"}
	}

	// Handle special known types
	if t == reflect.TypeOf(time.Time{}) {
		return spec.Schema{
			Type:   "string",
			Format: "date-time",
		}
	}

	return spec.Schema{} // Empty schema for unknown types
}

// handleStruct converts Go struct to OpenAPI object schema
func (sg *SchemaGenerator) handleStruct(t reflect.Type) spec.Schema {
	schema := spec.Schema{
		Type:       "object",
		Properties: make(map[string]spec.Schema),
		Required:   []string{},
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name from json tag or field name
		fieldName := sg.getFieldName(field)
		if fieldName == "-" {
			continue // Skip fields marked as ignored
		}

		// Generate schema for field type
		fieldSchema := sg.GenerateSchemaFromType(field.Type)

		// Extract field metadata from tags
		sg.applyFieldTags(field, &fieldSchema)

		// Add to properties
		schema.Properties[fieldName] = fieldSchema

		// Check if field is required
		if sg.isFieldRequired(field) {
			schema.Required = append(schema.Required, fieldName)
		}
	}

	return schema
}

// handleArray converts Go slice/array to OpenAPI array schema
func (sg *SchemaGenerator) handleArray(t reflect.Type) spec.Schema {
	itemType := t.Elem()
	itemSchema := sg.GenerateSchemaFromType(itemType)

	return spec.Schema{
		Type:  "array",
		Items: &itemSchema,
	}
}

// handleMap converts Go map to OpenAPI object schema
func (sg *SchemaGenerator) handleMap(t reflect.Type) spec.Schema {
	valueType := t.Elem()
	valueSchema := sg.GenerateSchemaFromType(valueType)

	return spec.Schema{
		Type:                 "object",
		AdditionalProperties: &valueSchema,
	}
}

// handleInterface handles interface types
func (sg *SchemaGenerator) handleInterface(t reflect.Type) spec.Schema {
	return spec.Schema{
		Type:        "object",
		Description: fmt.Sprintf("Interface type: %s", t.String()),
	}
}

// getFieldName extracts field name from json tag or uses struct field name
func (sg *SchemaGenerator) getFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return sg.toSnakeCase(field.Name)
	}

	// Parse json tag (e.g., "field_name,omitempty")
	parts := strings.Split(tag, ",")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return sg.toSnakeCase(field.Name)
}

// applyFieldTags applies struct tag information to schema
func (sg *SchemaGenerator) applyFieldTags(field reflect.StructField, schema *spec.Schema) {
	// Apply validation tags
	if validateTag := field.Tag.Get("validate"); validateTag != "" {
		sg.applyValidationTags(validateTag, schema)
	}

	// Apply example from tag
	if example := field.Tag.Get("example"); example != "" {
		schema.Example = example
	}

	// Apply description from tag
	if desc := field.Tag.Get("description"); desc != "" {
		schema.Description = desc
	}
}

// applyValidationTags applies validation rules to schema
func (sg *SchemaGenerator) applyValidationTags(validateTag string, schema *spec.Schema) {
	rules := strings.Split(validateTag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)

		if rule == "required" {
			// Required is handled at struct level
			continue
		}

		if strings.HasPrefix(rule, "min=") {
			// Handle min length/value
			if val := strings.TrimPrefix(rule, "min="); val != "" {
				switch schema.Type {
				case "string":
					if minLen := parseInt(val); minLen >= 0 {
						schema.MinLength = &minLen
					}
				case "integer", "number":
					if minVal := parseFloat(val); minVal != nil {
						schema.Minimum = minVal
					}
				}
			}
		}

		if strings.HasPrefix(rule, "max=") {
			// Handle max length/value
			if val := strings.TrimPrefix(rule, "max="); val != "" {
				switch schema.Type {
				case "string":
					if maxLen := parseInt(val); maxLen >= 0 {
						schema.MaxLength = &maxLen
					}
				case "integer", "number":
					if maxVal := parseFloat(val); maxVal != nil {
						schema.Maximum = maxVal
					}
				}
			}
		}

		if rule == "email" && schema.Type == "string" {
			schema.Format = "email"
		}
	}
}

// isFieldRequired checks if field is required based on validate tag
func (sg *SchemaGenerator) isFieldRequired(field reflect.StructField) bool {
	validateTag := field.Tag.Get("validate")
	return strings.Contains(validateTag, "required")
}

// toSnakeCase converts PascalCase to snake_case
func (sg *SchemaGenerator) toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && ('A' <= r && r <= 'Z') {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// Helper functions

func float64Ptr(v float64) *float64 {
	return &v
}

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

func parseFloat(s string) *float64 {
	var result float64
	if n, err := fmt.Sscanf(s, "%f", &result); n > 0 && err == nil {
		return &result
	}
	return nil
}

// ClearCache clears the type cache (useful for testing)
func (sg *SchemaGenerator) ClearCache() {
	sg.typeCache = make(map[reflect.Type]spec.Schema)
}

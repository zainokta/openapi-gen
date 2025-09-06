package parser

import (
	"github.com/zainokta/openapi-gen/spec"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// RouteParser parses route information from Go source files
type RouteParser struct {
	fileSet *token.FileSet
	routes  []spec.RouteInfo
}

// NewRouteParser creates a new route parser
func NewRouteParser() *RouteParser {
	return &RouteParser{
		fileSet: token.NewFileSet(),
		routes:  make([]spec.RouteInfo, 0),
	}
}

// ParseRoutesFromFile parses routes from a Go source file
func (p *RouteParser) ParseRoutesFromFile(filename string) error {
	src, err := parser.ParseFile(p.fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	ast.Inspect(src, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			p.parseRouteCall(n)
		}
		return true
	})

	return nil
}

// parseRouteCall extracts route information from method calls like h.GET, h.POST, etc.
func (p *RouteParser) parseRouteCall(call *ast.CallExpr) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		method := strings.ToUpper(sel.Sel.Name)
		if p.isHTTPMethod(method) && len(call.Args) >= 2 {
			route := spec.RouteInfo{
				Method: method,
			}

			// Extract path from first argument
			if path := p.extractStringLiteral(call.Args[0]); path != "" {
				route.Path = path
			}

			// Extract handler from second argument
			if handler := p.extractHandlerInfo(call.Args[1]); handler != "" {
				route.HandlerName = handler
			}

			if route.Path != "" && route.HandlerName != "" {
				p.routes = append(p.routes, route)
			}
		}
	}
}

// isHTTPMethod checks if the given string is an HTTP method
func (p *RouteParser) isHTTPMethod(method string) bool {
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	for _, m := range httpMethods {
		if method == m {
			return true
		}
	}
	return false
}

// extractStringLiteral extracts a string literal from an AST expression
func (p *RouteParser) extractStringLiteral(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		// Remove quotes
		value := lit.Value
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			return value[1 : len(value)-1]
		}
	}
	return ""
}

// extractHandlerInfo extracts handler information from an AST expression
func (p *RouteParser) extractHandlerInfo(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", ident.Name, e.Sel.Name)
		}
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// GetRoutes returns the parsed routes
func (p *RouteParser) GetRoutes() []spec.RouteInfo {
	return p.routes
}

// StructParser parses struct information for schema generation
type StructParser struct {
	schemas map[string]spec.Schema
}

// NewStructParser creates a new struct parser
func NewStructParser() *StructParser {
	return &StructParser{
		schemas: make(map[string]spec.Schema),
	}
}

// ParseStruct parses a Go struct using reflection
func (p *StructParser) ParseStruct(t reflect.Type) spec.Schema {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return p.parseBasicType(t)
	}

	// Check if we've already parsed this type
	typeName := t.Name()
	if _, exists := p.schemas[typeName]; exists {
		return spec.Schema{Ref: fmt.Sprintf("#/components/schemas/%s", typeName)}
	}

	schema := spec.Schema{
		Type:       "object",
		Properties: make(map[string]spec.Schema),
		Required:   make([]string, 0),
	}

	// Parse struct fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		validateTag := field.Tag.Get("validate")

		fieldName, omitEmpty := p.parseJSONTag(jsonTag)
		if fieldName == "-" {
			continue
		}

		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		fieldSchema := p.ParseStruct(field.Type)
		p.applyValidationTags(validateTag, &fieldSchema)

		schema.Properties[fieldName] = fieldSchema

		// Add to required fields if not omitempty and not optional
		if !omitEmpty && !p.isOptionalFromValidation(validateTag) {
			schema.Required = append(schema.Required, fieldName)
		}
	}

	// Cache the schema for reuse
	if typeName != "" {
		p.schemas[typeName] = schema
	}

	return schema
}

// parseJSONTag parses the json struct tag
func (p *StructParser) parseJSONTag(tag string) (name string, omitEmpty bool) {
	if tag == "" {
		return "", false
	}

	parts := strings.Split(tag, ",")
	name = parts[0]

	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
		}
	}

	return name, omitEmpty
}

// applyValidationTags applies validation tags to schema
func (p *StructParser) applyValidationTags(tag string, schema *spec.Schema) {
	if tag == "" {
		return
	}

	validations := strings.Split(tag, ",")
	for _, validation := range validations {
		p.applyValidationRule(validation, schema)
	}
}

// applyValidationRule applies a single validation rule to schema
func (p *StructParser) applyValidationRule(rule string, schema *spec.Schema) {
	if rule == "required" {
		// Required is handled at the struct level
		return
	}

	if strings.HasPrefix(rule, "min=") {
		if value, err := strconv.Atoi(rule[4:]); err == nil {
			switch schema.Type {
			case "string":
				minLen := value
				schema.MinLength = &minLen
			case "number", "integer":
				minVal := float64(value)
				schema.Minimum = &minVal
			case "array":
				minItems := value
				schema.MinItems = &minItems
			}
		}
	}

	if strings.HasPrefix(rule, "max=") {
		if value, err := strconv.Atoi(rule[4:]); err == nil {
			switch schema.Type {
			case "string":
				maxLen := value
				schema.MaxLength = &maxLen
			case "number", "integer":
				maxVal := float64(value)
				schema.Maximum = &maxVal
			case "array":
				maxItems := value
				schema.MaxItems = &maxItems
			}
		}
	}

	if rule == "email" {
		schema.Format = "email"
	}

	if strings.HasPrefix(rule, "oneof=") {
		enumStr := rule[6:]
		enumValues := strings.Split(enumStr, " ")
		schema.Enum = enumValues
	}

	if strings.HasPrefix(rule, "len=") {
		if value, err := strconv.Atoi(rule[4:]); err == nil {
			if schema.Type == "string" {
				schema.MinLength = &value
				schema.MaxLength = &value
			}
		}
	}
}

// isOptionalFromValidation checks if field is optional based on validation tags
func (p *StructParser) isOptionalFromValidation(tag string) bool {
	return !strings.Contains(tag, "required")
}

// parseBasicType converts Go basic types to OpenAPI types
func (p *StructParser) parseBasicType(t reflect.Type) spec.Schema {
	switch t.Kind() {
	case reflect.String:
		return spec.Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return spec.Schema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return spec.Schema{Type: "integer", Minimum: func() *float64 { v := float64(0); return &v }()}
	case reflect.Float32, reflect.Float64:
		return spec.Schema{Type: "number"}
	case reflect.Bool:
		return spec.Schema{Type: "boolean"}
	case reflect.Array, reflect.Slice:
		itemSchema := p.ParseStruct(t.Elem())
		return spec.Schema{Type: "array", Items: &itemSchema}
	case reflect.Map:
		valueSchema := p.ParseStruct(t.Elem())
		return spec.Schema{Type: "object", AdditionalProperties: &valueSchema}
	case reflect.Interface:
		return spec.Schema{} // Empty schema for interface{}
	default:
		return spec.Schema{Type: "object"}
	}
}

// GetSchemas returns all parsed schemas
func (p *StructParser) GetSchemas() map[string]spec.Schema {
	return p.schemas
}

// CommentParser extracts documentation from Go comments
type CommentParser struct{}

// NewCommentParser creates a new comment parser
func NewCommentParser() *CommentParser {
	return &CommentParser{}
}

// ParseHandlerComments extracts documentation from handler function comments
func (p *CommentParser) ParseHandlerComments(comments string) (summary, description string, tags []string) {
	lines := strings.Split(strings.TrimSpace(comments), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "//"))

		if i == 0 && line != "" {
			summary = line
		} else if line != "" && summary != "" {
			if description == "" {
				description = line
			} else {
				description += " " + line
			}
		}

		// Extract tags from comments like @tags auth,user
		if strings.HasPrefix(line, "@tags ") {
			tagStr := strings.TrimPrefix(line, "@tags ")
			tags = strings.Split(tagStr, ",")
			for i, tag := range tags {
				tags[i] = strings.TrimSpace(tag)
			}
		}
	}

	return summary, description, tags
}

// RegisterDTOSchemas registers common DTO schemas
func (sp *StructParser) RegisterDTOSchemas() {
	// Register common types used in DTOs
	sp.registerCommonTypes()
}

// registerCommonTypes registers common Go types used in DTOs
func (sp *StructParser) registerCommonTypes() {
	// Register time.Time
	timeSchema := spec.Schema{
		Type:    "string",
		Format:  "date-time",
		Example: "2023-01-01T00:00:00Z",
	}
	sp.schemas["Time"] = timeSchema

	// Register common validation schemas
	emailSchema := spec.Schema{
		Type:    "string",
		Format:  "email",
		Example: "user@example.com",
	}
	sp.schemas["Email"] = emailSchema

	uuidSchema := spec.Schema{
		Type:    "string",
		Format:  "uuid",
		Example: "123e4567-e89b-12d3-a456-426614174000",
	}
	sp.schemas["UUID"] = uuidSchema
}

// PathParameterParser extracts path parameters from route paths
type PathParameterParser struct{}

// NewPathParameterParser creates a new path parameter parser
func NewPathParameterParser() *PathParameterParser {
	return &PathParameterParser{}
}

// ExtractPathParameters extracts path parameters from a route path
func (p *PathParameterParser) ExtractPathParameters(path string) []spec.Parameter {
	var params []spec.Parameter

	// Match patterns like :id, :token, etc.
	re := regexp.MustCompile(`:(\w+)`)
	matches := re.FindAllStringSubmatch(path, -1)

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

	return params
}

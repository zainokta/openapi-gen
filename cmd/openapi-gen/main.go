package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SchemaAnnotation represents a go:generate annotation for schema generation
type SchemaAnnotation struct {
	HandlerName  string `json:"handlerName"`
	RequestType  string `json:"requestType,omitempty"`
	ResponseType string `json:"responseType,omitempty"`
	FilePath     string `json:"filePath"`
	LineNumber   int    `json:"lineNumber"`
}

// SchemaFile represents the generated schema file structure
type SchemaFile struct {
	HandlerName    string                 `json:"handlerName"`
	RequestSchema  map[string]interface{} `json:"requestSchema,omitempty"`
	ResponseSchema map[string]interface{} `json:"responseSchema,omitempty"`
}

func main() {
	var (
		outputDir  = flag.String("output", "./schemas", "Output directory for schema files")
		verbose    = flag.Bool("verbose", false, "Verbose output")
		requestType  = flag.String("request", "", "Request type in format package.TypeName")
		responseType = flag.String("response", "", "Response type in format package.TypeName")
		handlerName  = flag.String("handler", "", "Handler name (auto-detected if not provided)")
	)
	flag.Parse()

	if len(flag.Args()) == 0 {
		log.Fatal("Please specify at least one Go file to process")
	}

	// Expand . to the actual file path if needed
	args := make([]string, len(flag.Args()))
	for i, arg := range flag.Args() {
		if arg == "." {
			// Get current directory and find the first .go file
			currentDir, err := os.Getwd()
			if err != nil {
				log.Fatal("Failed to get current directory")
			}
			
			// Find Go files in current directory
			files, err := filepath.Glob(filepath.Join(currentDir, "*.go"))
			if err != nil || len(files) == 0 {
				log.Fatal("No Go files found in current directory")
			}
			args[i] = files[0] // Use the first Go file found
		} else {
			args[i] = arg
		}
	}

	// Find package root and create output directory there
	packageRoot, err := findPackageRoot()
	if err != nil {
		log.Fatalf("Failed to find package root: %v", err)
	}

	// Create output directory in package root
	outputPath := filepath.Join(packageRoot, *outputDir)
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Check if we're using the new flag-based approach
	if *requestType != "" || *responseType != "" {
		// Single annotation mode using flags
		if *handlerName == "" {
			// Try to extract handler name from the first file
			if len(args) > 0 {
				*handlerName = extractHandlerNameFromFile(args[0])
			}
			if *handlerName == "" {
				// If we can't extract the handler name, use a generic name based on the request/response types
				if *requestType != "" {
					parts := strings.Split(*requestType, ".")
					if len(parts) > 1 {
						*handlerName = strings.TrimSuffix(parts[1], "Request") + "Handler"
					}
				} else if *responseType != "" {
					parts := strings.Split(*responseType, ".")
					if len(parts) > 1 {
						*handlerName = strings.TrimSuffix(parts[1], "Response") + "Handler"
					}
				}
			}
			if *handlerName == "" {
				log.Fatal("Handler name is required when using -request or -response flags")
			}
		}

		annotation := SchemaAnnotation{
			HandlerName:  *handlerName,
			RequestType:  *requestType,
			ResponseType: *responseType,
			FilePath:     args[0], // Use first file as reference
			LineNumber:   1,
		}

		if *verbose {
			log.Printf("Generating schema for handler: %s", *handlerName)
		}

		if err := generateSchemaFile(annotation, outputPath, *verbose); err != nil {
			log.Fatalf("Error generating schema for %s: %v", *handlerName, err)
		}

		log.Printf("Generated 1 schema file in %s", outputPath)
		return
	}

	// Original comment-based parsing mode
	annotations := make([]SchemaAnnotation, 0)

	// Process each file
	for _, filePath := range args {
		fileAnnotations, err := processFile(filePath, *verbose)
		if err != nil {
			log.Printf("Error processing %s: %v", filePath, err)
			continue
		}
		annotations = append(annotations, fileAnnotations...)
	}

	if *verbose {
		log.Printf("Found %d schema annotations", len(annotations))
	}

	// Generate schema files
	for _, annotation := range annotations {
		if err := generateSchemaFile(annotation, outputPath, *verbose); err != nil {
			log.Printf("Error generating schema for %s: %v", annotation.HandlerName, err)
		}
	}

	log.Printf("Generated %d schema files in %s", len(annotations), outputPath)
}

// processFile parses a Go file and extracts schema annotations
func processFile(filePath string, verbose bool) ([]SchemaAnnotation, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	annotations := make([]SchemaAnnotation, 0)

	// Look for go:generate annotations with our specific pattern
	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			if strings.Contains(comment.Text, "go:generate") && strings.Contains(comment.Text, "openapi-gen") {
				annotation, err := parseAnnotation(comment.Text, filePath, fset.Position(comment.Pos()).Line)
				if err != nil {
					if verbose {
						log.Printf("Warning: Failed to parse annotation in %s: %v", filePath, err)
					}
					continue
				}

				// Extract handler name from the function context
				handlerName := extractHandlerName(node, comment.Pos())
				if handlerName == "" {
					if verbose {
						log.Printf("Warning: Could not extract handler name for annotation in %s", filePath)
					}
					continue
				}

				annotation.HandlerName = handlerName
				annotations = append(annotations, *annotation)
			}
		}
	}

	return annotations, nil
}

// parseAnnotation parses a go:generate comment line
func parseAnnotation(comment, filePath string, lineNumber int) (*SchemaAnnotation, error) {
	// Remove the //go:generate prefix and clean up
	cleanComment := strings.TrimSpace(strings.TrimPrefix(comment, "//go:generate"))

	// Check if this is our annotation
	if !strings.Contains(cleanComment, "openapi-gen") {
		return nil, fmt.Errorf("not an openapi-gen annotation")
	}

	// Remove "openapi-gen" to get the args
	args := strings.TrimSpace(strings.TrimPrefix(cleanComment, "openapi-gen"))

	// Parse arguments using simple regex patterns
	annotation := &SchemaAnnotation{
		FilePath:   filePath,
		LineNumber: lineNumber,
	}

	// Parse request type
	if reqMatch := regexp.MustCompile(`-request\s+(\S+)`).FindStringSubmatch(args); len(reqMatch) > 1 {
		annotation.RequestType = reqMatch[1]
	}

	// Parse response type
	if respMatch := regexp.MustCompile(`-response\s+(\S+)`).FindStringSubmatch(args); len(respMatch) > 1 {
		annotation.ResponseType = respMatch[1]
	}

	// Validate that we have at least one type
	if annotation.RequestType == "" && annotation.ResponseType == "" {
		return nil, fmt.Errorf("annotation must specify at least request or response type")
	}

	return annotation, nil
}

// extractHandlerNameFromFile extracts the handler name from a go:generate comment
func extractHandlerNameFromFile(filePath string) string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return ""
	}

	// Look for go:generate annotations with our specific pattern
	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			if strings.Contains(comment.Text, "go:generate") && strings.Contains(comment.Text, "openapi-gen") {
				handlerName := extractHandlerName(node, comment.Pos())
				if handlerName != "" {
					return handlerName
				}
			}
		}
	}

	return ""
}

// extractHandlerName extracts the function name that follows a go:generate comment
func extractHandlerName(node *ast.File, commentPos token.Pos) string {
	var handlerName string

	// Find the function declaration that follows this comment
	ast.Inspect(node, func(n ast.Node) bool {
		if handlerName != "" {
			return false // Stop once we found it
		}

		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			// Check if this function comes after the comment
			if funcDecl.Pos() > commentPos {
				handlerName = funcDecl.Name.Name
				return false
			}
		}
		return true
	})

	return handlerName
}

// generateSchemaFile generates a JSON schema file for a handler
func generateSchemaFile(annotation SchemaAnnotation, outputDir string, verbose bool) error {
	schemaFile := SchemaFile{
		HandlerName: annotation.HandlerName,
	}

	// Get the directory of the handler file to resolve imports
	handlerDir := filepath.Dir(annotation.FilePath)

	// Generate schemas by analyzing the actual struct definitions
	if annotation.RequestType != "" {
		schema, err := generateSchemaFromType(annotation.RequestType, handlerDir, verbose)
		if err != nil {
			log.Printf("Error generating request schema for %s: %v", annotation.RequestType, err)
		} else {
			schemaFile.RequestSchema = schema
		}
	}

	if annotation.ResponseType != "" {
		schema, err := generateSchemaFromType(annotation.ResponseType, handlerDir, verbose)
		if err != nil {
			log.Printf("Error generating response schema for %s: %v", annotation.ResponseType, err)
		} else {
			schemaFile.ResponseSchema = schema
		}
	}

	// Generate file name
	fileName := fmt.Sprintf("%s.json", sanitizeFileName(annotation.HandlerName))
	filePath := filepath.Join(outputDir, fileName)

	// Write JSON file
	jsonData, err := json.MarshalIndent(schemaFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	if verbose {
		log.Printf("Generated schema file: %s", filePath)
	}

	return nil
}

// generateSchemaFromType generates an OpenAPI schema by analyzing the actual Go struct
func generateSchemaFromType(typeName, searchDir string, verbose bool) (map[string]interface{}, error) {
	// Parse the type name (e.g., "dto.LoginRequest" -> package="dto", typeName="LoginRequest")
	parts := strings.Split(typeName, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid type name format: %s, expected package.TypeName", typeName)
	}

	packageName := parts[0]
	structName := parts[1]

	if verbose {
		log.Printf("Analyzing type: %s from package: %s", structName, packageName)
	}

	// Find the package and struct definition
	structDef, err := findStructDefinition(packageName, structName, searchDir, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to find struct definition: %w", err)
	}

	// Generate OpenAPI schema from the struct
	schema := generateOpenAPISchema(structDef)

	return schema, nil
}

// findStructDefinition finds a struct definition in the specified package
func findStructDefinition(packageName, structName, searchDir string, verbose bool) (*ast.StructType, error) {
	// Search for Go files in the search directory and subdirectories
	files, err := filepath.Glob(filepath.Join(searchDir, "**/*.go"))
	if err != nil {
		return nil, fmt.Errorf("failed to search for Go files: %w", err)
	}

	if verbose {
		log.Printf("Searching for struct %s.%s in %d files", packageName, structName, len(files))
	}

	for _, file := range files {
		structDef, err := findStructInFile(file, packageName, structName)
		if err == nil {
			return structDef, nil
		}
	}

	return nil, fmt.Errorf("struct %s.%s not found in package", packageName, structName)
}

// findStructInFile searches for a struct definition in a specific file
func findStructInFile(filePath, packageName, structName string) (*ast.StructType, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	// Check if this file has the right package name
	if node.Name.Name != packageName {
		return nil, fmt.Errorf("wrong package name: %s, expected %s", node.Name.Name, packageName)
	}

	var foundStruct *ast.StructType

	// Search for the struct definition
	ast.Inspect(node, func(n ast.Node) bool {
		if foundStruct != nil {
			return false
		}

		if typeSpec, ok := n.(*ast.TypeSpec); ok && typeSpec.Name.Name == structName {
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				foundStruct = structType
				return false
			}
		}
		return true
	})

	if foundStruct == nil {
		return nil, fmt.Errorf("struct %s not found in file", structName)
	}

	return foundStruct, nil
}

// generateOpenAPISchema generates an OpenAPI schema from an AST struct definition
func generateOpenAPISchema(structDef *ast.StructType) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
		"required":   make([]string, 0),
	}

	for _, field := range structDef.Fields.List {
		for _, name := range field.Names {
			fieldSchema := generateFieldSchema(field.Type)
			
			// Get JSON tag name if available, otherwise use field name
			jsonName := getJSONTagName(field, name.Name)
			schema["properties"].(map[string]interface{})[jsonName] = fieldSchema

			// Check if field has a JSON tag that indicates it's required
			if hasRequiredTag(field) {
				schema["required"] = append(schema["required"].([]string), jsonName)
			}
		}
	}

	return schema
}

// generateFieldSchema generates an OpenAPI schema for a field based on its type
func generateFieldSchema(expr ast.Expr) map[string]interface{} {
	switch t := expr.(type) {
	case *ast.Ident:
		return generateBasicTypeSchema(t.Name)
	case *ast.SelectorExpr:
		// Handle external types like time.Time
		if x, ok := t.X.(*ast.Ident); ok {
			if x.Name == "time" && t.Sel.Name == "Time" {
				return map[string]interface{}{
					"type":   "string",
					"format": "date-time",
				}
			}
			return map[string]interface{}{
				"type":        "object",
				"description": fmt.Sprintf("External type: %s.%s", x.Name, t.Sel.Name),
			}
		}
		return map[string]interface{}{
			"type":        "object",
			"description": "External type",
		}
	case *ast.ArrayType:
		// Handle arrays/slices
		elemSchema := generateFieldSchema(t.Elt)
		return map[string]interface{}{
			"type":        "array",
			"items":       elemSchema,
			"description": "Array of " + elemSchema["type"].(string),
		}
	case *ast.MapType:
		// Handle maps
		valueSchema := generateFieldSchema(t.Value)
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": valueSchema,
		}
	case *ast.StarExpr:
		// Handle pointers (dereference to underlying type)
		return generateFieldSchema(t.X)
	case *ast.InterfaceType:
		// Handle interface{} as any type
		return map[string]interface{}{
			"type":        "object",
			"description": "Interface type",
		}
	default:
		return map[string]interface{}{
			"type":        "object",
			"description": "Unknown type",
		}
	}
}

// generateBasicTypeSchema generates OpenAPI schema for basic Go types
func generateBasicTypeSchema(typeName string) map[string]interface{} {
	switch typeName {
	case "string":
		return map[string]interface{}{"type": "string"}
	case "int", "int8", "int16", "int32", "int64":
		return map[string]interface{}{"type": "integer", "format": "int64"}
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return map[string]interface{}{"type": "integer", "format": "int64"}
	case "float32", "float64":
		return map[string]interface{}{"type": "number", "format": "double"}
	case "bool":
		return map[string]interface{}{"type": "boolean"}
	case "time.Time":
		return map[string]interface{}{"type": "string", "format": "date-time"}
	default:
		return map[string]interface{}{
			"type":        "object",
			"description": fmt.Sprintf("Type: %s", typeName),
		}
	}
}

// getJSONTagName extracts the JSON tag name from a field
func getJSONTagName(field *ast.Field, defaultName string) string {
	if field.Tag != nil {
		tagValue := strings.Trim(field.Tag.Value, "`")
		if strings.Contains(tagValue, "json:") {
			// Extract JSON tag content
			jsonTag := regexp.MustCompile(`json:"([^"]*)"`).FindStringSubmatch(tagValue)
			if len(jsonTag) > 1 {
				// Split by comma to handle options like omitempty
				tagParts := strings.Split(jsonTag[1], ",")
				if tagParts[0] != "" {
					return tagParts[0]
				}
			}
		}
	}
	return defaultName
}

// hasRequiredTag checks if a field has a JSON tag indicating it's required
func hasRequiredTag(field *ast.Field) bool {
	if field.Tag != nil {
		tagValue := strings.Trim(field.Tag.Value, "`")
		if strings.Contains(tagValue, "json:") {
			// Extract JSON tag content
			jsonTag := regexp.MustCompile(`json:"([^"]*)"`).FindStringSubmatch(tagValue)
			if len(jsonTag) > 1 {
				// Check if the tag contains omitempty
				return !strings.Contains(jsonTag[1], "omitempty")
			}
		}
	}
	return false
}

// findPackageRoot finds the root directory of the Go package by looking for go.mod
func findPackageRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Search up the directory tree for go.mod
	dir := currentDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the root directory
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found in directory tree")
}

// sanitizeFileName creates a safe filename from handler name
func sanitizeFileName(handlerName string) string {
	// Replace common problematic characters
	safeName := strings.ReplaceAll(handlerName, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	safeName = strings.ReplaceAll(safeName, ":", "_")
	safeName = strings.ReplaceAll(safeName, "*", "_")

	// Remove any remaining non-alphanumeric characters except underscores
	reg := regexp.MustCompile(`[^\w-]`)
	safeName = reg.ReplaceAllString(safeName, "_")

	return strings.TrimSpace(safeName)
}

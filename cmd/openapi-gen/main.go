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
	"slices"
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

// PackageContext tracks the current package directory for resolving nested struct references
type PackageContext struct {
	// RootSearchDir is the original search directory (usually project root)
	RootSearchDir string
	// CurrentPackageDir is the directory of the package being analyzed (for same-package struct resolution)
	CurrentPackageDir string
	// CurrentPackageName is the name of the package being analyzed
	CurrentPackageName string
	// VisitedTypes tracks types to prevent infinite recursion
	VisitedTypes map[string]bool
}

func main() {
	var (
		outputDir    = flag.String("output", "./schemas", "Output directory for schema files")
		verbose      = flag.Bool("verbose", false, "Verbose output")
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
	if *requestType != "" || *responseType != "" || *handlerName != "" {
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
				log.Fatal("Handler name is required when using flags")
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
	reqMatch := regexp.MustCompile(`-request\s+(\S+)`).FindStringSubmatch(args)
	if len(reqMatch) > 1 {
		annotation.RequestType = reqMatch[1]
	}

	// Parse response type
	respMatch := regexp.MustCompile(`-response\s+(\S+)`).FindStringSubmatch(args)
	if len(respMatch) > 1 {
		annotation.ResponseType = respMatch[1]
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

	// Get the package root directory to search for schemas
	packageRoot, err := findPackageRoot()
	if err != nil {
		return fmt.Errorf("failed to find package root: %w", err)
	}

	// Generate schemas by analyzing the actual struct definitions
	if annotation.RequestType != "" {
		schema, err := generateSchemaFromType(annotation.RequestType, packageRoot, verbose)
		if err != nil {
			log.Printf("Warning: Could not generate request schema for %s: %v", annotation.RequestType, err)
		} else {
			schemaFile.RequestSchema = schema
			if verbose {
				log.Printf("Successfully generated request schema for %s", annotation.RequestType)
			}
		}
	}

	if annotation.ResponseType != "" {
		schema, err := generateSchemaFromType(annotation.ResponseType, packageRoot, verbose)
		if err != nil {
			log.Printf("Warning: Could not generate response schema for %s: %v", annotation.ResponseType, err)
		} else {
			schemaFile.ResponseSchema = schema
			if verbose {
				log.Printf("Successfully generated response schema for %s", annotation.ResponseType)
			}
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

// isBuiltinType checks if a type is a built-in Go type or standard library type
func isBuiltinType(typeName string) bool {
	// Check for simple built-in types
	builtinTypes := map[string]bool{
		"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "bool": true, "byte": true, "rune": true,
		"interface{}": true, "any": true,
	}

	if builtinTypes[typeName] {
		return true
	}

	// Check for standard library types
	stdTypes := map[string]bool{
		"time.Time":                true,
		"time.Duration":            true,
		"net/url.URL":              true,
		"encoding/json.RawMessage": true,
		"encoding/json.Number":     true,
		"io.Reader":                true, "io.Writer": true, "io.ReadWriter": true,
		"net/http.Cookie":  true,
		"net/mail.Address": true,
		"math/big.Int":     true, "math/big.Float": true,
	}

	return stdTypes[typeName]
}

// parseComplexTypeExpression parses complex type expressions like arrays, maps, and pointers
func parseComplexTypeExpression(typeName string) (map[string]interface{}, error) {
	// Handle pointer types
	if strings.HasPrefix(typeName, "*") {
		innerType := strings.TrimPrefix(typeName, "*")
		innerSchema, err := parseComplexTypeExpression(innerType)
		if err != nil {
			return nil, err
		}
		// Pointers reference the underlying type
		return innerSchema, nil
	}

	// Handle array/slice types
	if strings.HasPrefix(typeName, "[]") {
		elementType := strings.TrimPrefix(typeName, "[]")
		elementSchema, err := parseComplexTypeExpression(elementType)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"type":        "array",
			"items":       elementSchema,
			"description": fmt.Sprintf("Array of %s", elementType),
		}, nil
	}

	// Handle map types
	if strings.HasPrefix(typeName, "map[") {
		// Parse map[K]V format
		mapRegex := regexp.MustCompile(`map\[([^\]]+)\](.+)`)
		matches := mapRegex.FindStringSubmatch(typeName)
		if len(matches) != 3 {
			return nil, fmt.Errorf("invalid map type format: %s", typeName)
		}

		keyType := matches[1]
		valueType := matches[2]

		// For OpenAPI, map keys should be strings
		if keyType != "string" {
			return map[string]interface{}{
				"type":        "object",
				"description": fmt.Sprintf("Map with %s keys (non-string keys not supported in OpenAPI)", keyType),
			}, nil
		}

		valueSchema, err := parseComplexTypeExpression(valueType)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": valueSchema,
		}, nil
	}

	// Handle simple built-in types
	if isBuiltinType(typeName) {
		return generateBasicTypeSchema(typeName), nil
	}

	// Handle package-qualified types (e.g., time.Time, mypackage.MyType)
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		if len(parts) == 2 {
			packageName := parts[0]
			typeNameOnly := parts[1]

			// Handle known standard library types
			if packageName == "time" && typeNameOnly == "Time" {
				return generateBasicTypeSchema("time.Time"), nil
			}
			if packageName == "time" && typeNameOnly == "Duration" {
				return map[string]interface{}{
					"type":        "string",
					"format":      "duration",
					"description": "Time duration",
				}, nil
			}
			if packageName == "net/url" && typeNameOnly == "URL" {
				return map[string]interface{}{
					"type":        "string",
					"format":      "uri",
					"description": "URL",
				}, nil
			}
			if packageName == "encoding/json" && typeNameOnly == "RawMessage" {
				return map[string]interface{}{
					"type":        "object",
					"description": "Raw JSON message",
				}, nil
			}
			if packageName == "encoding/json" && typeNameOnly == "Number" {
				return map[string]interface{}{
					"type":        "number",
					"description": "JSON number",
				}, nil
			}
			if packageName == "io" && (typeNameOnly == "Reader" || typeNameOnly == "Writer" || typeNameOnly == "ReadWriter") {
				return map[string]interface{}{
					"type":        "string",
					"format":      "binary",
					"description": fmt.Sprintf("IO %s", typeNameOnly),
				}, nil
			}
			if packageName == "net/http" && typeNameOnly == "Cookie" {
				return map[string]interface{}{
					"type":        "object",
					"description": "HTTP cookie",
				}, nil
			}
			if packageName == "net/mail" && typeNameOnly == "Address" {
				return map[string]interface{}{
					"type":        "string",
					"format":      "email",
					"description": "Email address",
				}, nil
			}
			if packageName == "math/big" && (typeNameOnly == "Int" || typeNameOnly == "Float") {
				return map[string]interface{}{
					"type":        "string",
					"description": fmt.Sprintf("Big %s number", typeNameOnly),
				}, nil
			}
		}
	}

	// Unknown type - return a generic object schema
	return map[string]interface{}{
		"type":        "object",
		"description": fmt.Sprintf("Unknown type: %s", typeName),
	}, nil
}

// generateSchemaFromType generates an OpenAPI schema by analyzing the actual Go struct
func generateSchemaFromType(typeName, searchDir string, verbose bool) (map[string]interface{}, error) {
	if verbose {
		log.Printf("Analyzing type: %s", typeName)
	}

	// First, try to parse as a complex type expression (built-in types, arrays, maps, etc.)
	if !strings.Contains(typeName, ".") || isBuiltinType(typeName) {
		schema, err := parseComplexTypeExpression(typeName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse type expression %s: %w", typeName, err)
		}
		return schema, nil
	}

	// Parse the type name (e.g., "dto.LoginRequest" -> package="dto", typeName="LoginRequest")
	parts := strings.Split(typeName, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid type name format: %s, expected package.TypeName", typeName)
	}

	packageName := parts[0]
	structName := parts[1]

	// Check if this is a standard library type we can handle directly
	fullTypeName := fmt.Sprintf("%s.%s", packageName, structName)
	if isBuiltinType(fullTypeName) {
		schema, err := parseComplexTypeExpression(fullTypeName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse standard library type %s: %w", fullTypeName, err)
		}
		return schema, nil
	}

	if verbose {
		log.Printf("Analyzing custom struct type: %s from package: %s", structName, packageName)
	}

	// Find the package and struct definition
	structDef, err := findStructDefinition(packageName, structName, searchDir, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to find struct definition: %w", err)
	}

	// Generate OpenAPI schema from the struct with proper package context
	packageRoot, err := findPackageRoot()
	if err != nil {
		packageRoot = "." // fallback to current directory
	}

	// Find the actual package directory for the target package
	// We need to find the directory that contains the specific struct we found
	packageDirs, err := findPackageDirectories(packageName, searchDir, verbose)
	var targetPackageDir string
	if err == nil && len(packageDirs) > 0 {
		// If we have multiple package directories, try to find the one that contains our struct
		targetPackageDir = packageDirs[0] // Default to first match
		for _, dir := range packageDirs {
			// Check if this directory contains the struct we're looking for
			if structExistsInDirectory(structName, dir, packageName) {
				targetPackageDir = dir
				break
			}
		}
		if verbose {
			log.Printf("Found package directory for %s: %s (from %d candidates)", packageName, targetPackageDir, len(packageDirs))
		}
	} else {
		targetPackageDir = searchDir // Fallback to search directory
		if verbose {
			log.Printf("No specific directory found for package %s, using: %s", packageName, targetPackageDir)
		}
	}

	// Create proper package context
	context := &PackageContext{
		RootSearchDir:      packageRoot,
		CurrentPackageDir:  targetPackageDir,
		CurrentPackageName: packageName,
		VisitedTypes:       make(map[string]bool),
	}

	if verbose {
		log.Printf("Created context for %s.%s - packageDir: %s, packageName: %s", packageName, structName, context.CurrentPackageDir, context.CurrentPackageName)
	}

	// Generate schema with proper context
	schema := generateStructSchemaWithContext(structDef, context)

	return schema, nil
}

// findPackageDirectories recursively searches for directories containing Go files with the target package name
func findPackageDirectories(packageName, searchDir string, verbose bool) ([]string, error) {
	var packageDirs []string

	// Walk through all directories in searchDir
	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories that are likely not Go packages
		if info.IsDir() {
			// Skip hidden directories and common non-package directories
			dirName := filepath.Base(path)
			if strings.HasPrefix(dirName, ".") || dirName == "vendor" || dirName == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Parse the file to check its package name
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			return nil // Skip files that can't be parsed
		}

		// If this file has the target package name, add its directory to our list
		if node.Name.Name == packageName {
			dir := filepath.Dir(path)
			if !slices.Contains(packageDirs, dir) {
				packageDirs = append(packageDirs, dir)
				if verbose {
					log.Printf("Found package directory: %s", dir)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory tree: %w", err)
	}

	return packageDirs, nil
}

// findStructDefinition finds a struct definition in the specified package
func findStructDefinition(packageName, structName, searchDir string, verbose bool) (*ast.StructType, error) {
	if verbose {
		log.Printf("Searching for struct %s.%s in directory: %s", packageName, structName, searchDir)
	}

	// First, try to find all directories that contain the target package
	packageDirs, err := findPackageDirectories(packageName, searchDir, verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to find package directories: %w", err)
	}

	if len(packageDirs) == 0 {
		return nil, fmt.Errorf("no directories found for package %s", packageName)
	}

	if verbose {
		log.Printf("Found %d directories for package %s", len(packageDirs), packageName)
	}

	// Search for the struct in each package directory
	for _, packageDir := range packageDirs {
		if verbose {
			log.Printf("Searching in package directory: %s", packageDir)
		}

		// Get all Go files in this package directory
		packageFiles, err := filepath.Glob(filepath.Join(packageDir, "*.go"))
		if err != nil {
			continue // Skip this directory if we can't read it
		}

		// Search for the struct in each file
		for _, file := range packageFiles {
			structDef, err := findStructInFile(file, packageName, structName)
			if err == nil {
				if verbose {
					log.Printf("Found struct %s.%s in file: %s", packageName, structName, file)
				}
				return structDef, nil
			}
		}
	}

	// If we get here, the struct was not found in any package directory
	// As a fallback, try the original approach of searching all files
	if verbose {
		log.Printf("Package directory search failed, trying fallback search across all files")
	}

	files, err := filepath.Glob(filepath.Join(searchDir, "**/*.go"))
	if err != nil {
		return nil, fmt.Errorf("struct %s.%s not found in package (searched %d directories) and fallback search failed: %w",
			packageName, structName, len(packageDirs), err)
	}

	for _, file := range files {
		structDef, err := findStructInFile(file, packageName, structName)
		if err == nil {
			if verbose {
				log.Printf("Found struct %s.%s in file (fallback search): %s", packageName, structName, file)
			}
			return structDef, nil
		}
	}

	return nil, fmt.Errorf("struct %s.%s not found in package (searched %d directories and %d total files)",
		packageName, structName, len(packageDirs), len(files))
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

// generateStructSchemaWithContext generates an OpenAPI schema with package context and cycle detection
func generateStructSchemaWithContext(structDef *ast.StructType, context *PackageContext) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
		"required":   make([]string, 0),
	}

	for _, field := range structDef.Fields.List {
		for _, name := range field.Names {
			fieldSchema := resolveFieldTypeSchema(field.Type, context)

			// Get field name from JSON tag first, then form tag as fallback
			fieldName := getJSONTagName(field, name.Name)
			if fieldName == name.Name {
				// No JSON tag found, try form tag
				fieldName = getFormTagName(field, name.Name)
			}
			schema["properties"].(map[string]interface{})[fieldName] = fieldSchema

			// Check if field has a JSON or form tag that indicates it's required
			if hasRequiredTag(field) {
				schema["required"] = append(schema["required"].([]string), fieldName)
			}
		}
	}

	return schema
}

// resolveFieldTypeSchema analyzes a field type and generates the appropriate OpenAPI schema
func resolveFieldTypeSchema(expr ast.Expr, context *PackageContext) map[string]interface{} {
	switch t := expr.(type) {
	case *ast.Ident:
		// Handle both basic types and custom structs in the current package
		if isBuiltinType(t.Name) {
			return generateBasicTypeSchema(t.Name)
		}

		// This is a potential custom struct in the current package context
		return resolveNestedStructInCurrentPackage(t.Name, context)

	case *ast.StructType:
		// Handle inline struct definitions
		return generateStructSchemaWithContext(t, context)

	case *ast.SelectorExpr:
		// Handle cross-package struct references like dto.UserDTO or time.Time
		if x, ok := t.X.(*ast.Ident); ok {
			packageName := x.Name
			typeName := t.Sel.Name

			return resolveCrossPackageStruct(packageName, typeName, context)
		}
		return map[string]interface{}{
			"type":        "object",
			"description": "External type",
		}

	case *ast.ArrayType:
		// Handle arrays/slices with recursive element analysis
		elemSchema := resolveFieldTypeSchema(t.Elt, context)
		return map[string]interface{}{
			"type":        "array",
			"items":       elemSchema,
			"description": fmt.Sprintf("Array of %s", getTypeDescription(elemSchema)),
		}

	case *ast.MapType:
		// Handle maps with recursive value analysis
		valueSchema := resolveFieldTypeSchema(t.Value, context)
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": valueSchema,
		}

	case *ast.StarExpr:
		// Handle pointers (dereference to underlying type)
		return resolveFieldTypeSchema(t.X, context)

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

// getFormTagName extracts the form tag name from a field
func getFormTagName(field *ast.Field, defaultName string) string {
	if field.Tag != nil {
		tagValue := strings.Trim(field.Tag.Value, "`")
		if strings.Contains(tagValue, "form:") {
			// Extract form tag content
			formTag := regexp.MustCompile(`form:"([^"]*)"`).FindStringSubmatch(tagValue)
			if len(formTag) > 1 {
				// Split by comma to handle options like omitempty
				tagParts := strings.Split(formTag[1], ",")
				if tagParts[0] != "" {
					return tagParts[0]
				}
			}
		}
	}
	return defaultName
}

// hasRequiredTag checks if a field has a JSON or form tag indicating it's required
func hasRequiredTag(field *ast.Field) bool {
	if field.Tag != nil {
		tagValue := strings.Trim(field.Tag.Value, "`")

		// Check JSON tag first
		if strings.Contains(tagValue, "json:") {
			jsonTag := regexp.MustCompile(`json:"([^"]*)"`).FindStringSubmatch(tagValue)
			if len(jsonTag) > 1 {
				// Check if the tag contains omitempty
				return !strings.Contains(jsonTag[1], "omitempty")
			}
		}

		// If no JSON tag, check form tag
		if strings.Contains(tagValue, "form:") {
			formTag := regexp.MustCompile(`form:"([^"]*)"`).FindStringSubmatch(tagValue)
			if len(formTag) > 1 {
				// Check if the tag contains omitempty
				return !strings.Contains(formTag[1], "omitempty")
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

// getTypeDescription extracts a readable description from a schema
func getTypeDescription(schema map[string]interface{}) string {
	if desc, ok := schema["description"]; ok {
		if descStr, ok := desc.(string); ok {
			return descStr
		}
	}
	if typeVal, ok := schema["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			return typeStr
		}
	}
	return "unknown"
}

// resolveNestedStructInCurrentPackage resolves a struct reference within the current package context
func resolveNestedStructInCurrentPackage(structName string, context *PackageContext) map[string]interface{} {
	fullTypeName := fmt.Sprintf("%s.%s", context.CurrentPackageName, structName)

	// Check for circular references
	if context.VisitedTypes[fullTypeName] {
		return map[string]interface{}{
			"type":        "object",
			"description": fmt.Sprintf("Circular reference to %s", fullTypeName),
		}
	}

	// Ensure we have a package name - this is crucial for cross-package nested resolution
	currentPackageName := context.CurrentPackageName
	if currentPackageName == "" && context.CurrentPackageDir != context.RootSearchDir {
		// Discover the package name from the directory
		packageFiles, err := filepath.Glob(filepath.Join(context.CurrentPackageDir, "*.go"))
		if err == nil && len(packageFiles) > 0 {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, packageFiles[0], nil, parser.PackageClauseOnly)
			if err == nil {
				currentPackageName = node.Name.Name
				context.CurrentPackageName = currentPackageName // Update the context
			}
		}
	}

	// Try to find the struct in the current package directory
	structDef, err := findStructInPackageDirectory(structName, context.CurrentPackageDir, currentPackageName)
	if err == nil && structDef != nil {
		// Update fullTypeName with discovered package name if needed
		if context.CurrentPackageName != currentPackageName {
			fullTypeName = fmt.Sprintf("%s.%s", currentPackageName, structName)
		}

		// Mark as visited to prevent cycles
		context.VisitedTypes[fullTypeName] = true

		// Generate schema with current context
		schema := generateStructSchemaWithContext(structDef, context)

		// Remove from visited after processing (allow reuse in different branches)
		delete(context.VisitedTypes, fullTypeName)
		return schema
	}

	// Fall back to basic type schema if struct not found
	return map[string]interface{}{
		"type":        "object",
		"description": fmt.Sprintf("Type: %s (not found in package %s at %s)", structName, currentPackageName, context.CurrentPackageDir),
	}
}

// resolveCrossPackageStruct resolves a struct reference from another package (e.g., dto.UserDTO)
func resolveCrossPackageStruct(packageName, typeName string, context *PackageContext) map[string]interface{} {
	fullTypeName := packageName + "." + typeName

	// Handle known standard library types first
	if packageName == "time" && typeName == "Time" {
		return map[string]interface{}{
			"type":   "string",
			"format": "date-time",
		}
	}

	// Check for circular references
	if context.VisitedTypes[fullTypeName] {
		return map[string]interface{}{
			"type":        "object",
			"description": fmt.Sprintf("Circular reference to %s", fullTypeName),
		}
	}

	// Try to find and analyze the cross-package struct
	structDef, err := findStructDefinition(packageName, typeName, context.RootSearchDir, false)
	if err == nil && structDef != nil {
		// Find the package directory for the target package
		packageDirs, err := findPackageDirectories(packageName, context.RootSearchDir, false) // Disable verbose
		var targetPackageDir string
		if err == nil && len(packageDirs) > 0 {
			targetPackageDir = packageDirs[0] // Use the first match
		} else {
			targetPackageDir = context.RootSearchDir // Fallback
		}

		// Verify the package name from the actual directory
		actualPackageName := packageName
		if targetPackageDir != context.RootSearchDir {
			packageFiles, err := filepath.Glob(filepath.Join(targetPackageDir, "*.go"))
			if err == nil && len(packageFiles) > 0 {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, packageFiles[0], nil, parser.PackageClauseOnly)
				if err == nil {
					actualPackageName = node.Name.Name
				}
			}
		}

		// Create new context for the target package with verified package name
		newContext := &PackageContext{
			RootSearchDir:      context.RootSearchDir,
			CurrentPackageDir:  targetPackageDir,
			CurrentPackageName: actualPackageName,    // Use verified package name
			VisitedTypes:       context.VisitedTypes, // Share visited types to prevent cross-package cycles
		}

		// Mark as visited to prevent cycles
		context.VisitedTypes[fullTypeName] = true

		// Generate schema with the new package context
		schema := generateStructSchemaWithContext(structDef, newContext)

		// Remove from visited after processing
		delete(context.VisitedTypes, fullTypeName)
		return schema
	}

	return map[string]interface{}{
		"type":        "object",
		"description": fmt.Sprintf("External type: %s.%s", packageName, typeName),
	}
}

// findStructInPackageDirectory finds a struct definition in a specific package directory
func findStructInPackageDirectory(structName, packageDir, expectedPackageName string) (*ast.StructType, error) {
	// Get all Go files in the package directory
	packageFiles, err := filepath.Glob(filepath.Join(packageDir, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("failed to find Go files in %s: %w", packageDir, err)
	}

	if len(packageFiles) == 0 {
		return nil, fmt.Errorf("no Go files found in directory %s", packageDir)
	}

	// Verify package name if provided
	if expectedPackageName != "" {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, packageFiles[0], nil, parser.PackageClauseOnly)
		if err != nil || node.Name.Name != expectedPackageName {
			return nil, fmt.Errorf("package name mismatch in directory %s", packageDir)
		}
	}

	// Search for the struct in all files of this package
	for _, file := range packageFiles {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			continue // Skip files that can't be parsed
		}

		// Look for the struct definition
		var foundStruct *ast.StructType
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

		if foundStruct != nil {
			return foundStruct, nil
		}
	}

	return nil, fmt.Errorf("struct %s not found in package directory %s", structName, packageDir)
}

// structExistsInDirectory checks if a struct exists in a specific package directory
func structExistsInDirectory(structName, packageDir, expectedPackageName string) bool {
	_, err := findStructInPackageDirectory(structName, packageDir, expectedPackageName)
	return err == nil
}

package integration

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/zainokta/openapi-gen/analyzer"
	openapiParser "github.com/zainokta/openapi-gen/parser"
	"github.com/zainokta/openapi-gen/spec"
)

// HertzRouteDiscoverer implements RouteDiscoverer for CloudWeGo Hertz
type HertzRouteDiscoverer struct {
	engine *server.Hertz
}

// NewHertzRouteDiscoverer creates a new Hertz route discoverer
func NewHertzRouteDiscoverer(engine *server.Hertz) *HertzRouteDiscoverer {
	return &HertzRouteDiscoverer{
		engine: engine,
	}
}

// DiscoverRoutes discovers all routes from Hertz engine using Routes() method
func (h *HertzRouteDiscoverer) DiscoverRoutes() ([]spec.RouteInfo, error) {
	var routes []spec.RouteInfo

	// Use Hertz's built-in Routes() method to get all registered routes
	hertzRoutes := h.engine.Routes()

	for _, route := range hertzRoutes {
		routeInfo := spec.RouteInfo{
			Method:      route.Method,
			Path:        route.Path,
			HandlerName: h.extractHandlerName(route),
			Handler:     route.HandlerFunc,
		}

		routes = append(routes, routeInfo)
	}

	return routes, nil
}

// extractHandlerName extracts handler name from Hertz route info
func (h *HertzRouteDiscoverer) extractHandlerName(route route.RouteInfo) string {
	// Try to extract meaningful handler name from the route
	if route.HandlerFunc != nil {
		// Use reflection to get function name if possible
		handlerValue := reflect.ValueOf(route.HandlerFunc)
		if handlerValue.IsValid() {
			handlerType := handlerValue.Type()
			if handlerType.Kind() == reflect.Func {
				// Try to get the function name from runtime
				funcName := handlerType.String()
				
				// Try to get function name using runtime.FuncForPC
				if pc := handlerValue.Pointer(); pc != 0 {
					if fn := runtime.FuncForPC(pc); fn != nil {
						runtimeFuncName := fn.Name()
						if runtimeFuncName != "" && !isGenericFuncSignature(runtimeFuncName) {
							cleanName := h.parseHandlerNameFromFunction(runtimeFuncName)
							if cleanName != "" {
								return cleanName
							}
						}
					}
				}
				
				// Fallback to type string method
				if !isGenericFuncSignature(funcName) {
					// Parse the function name to extract just the method name
					cleanName := h.parseHandlerNameFromFunction(funcName)
					if cleanName != "" {
						return cleanName
					}
				}
			}
		}
	}

	// Fallback: generate handler name based on path and method using pure algorithm
	parser := openapiParser.NewPathParser()
	return parser.GenerateHandlerName(route.Method, route.Path)
}

// parseHandlerNameFromFunction parses handler name from various function name patterns
func (h *HertzRouteDiscoverer) parseHandlerNameFromFunction(fullName string) string {
	// Handle different patterns from external modules:
	// 1. external-app/internal/handlers.(*UserHandler).CreateUser-fm
	// 2. github.com/user/app/pkg/api.(*Controller).Method-fm
	// 3. some.domain/path/handlers.(*Handler).Method.func1
	// 4. app/handlers.Function

	// Pattern 1 & 2: Method receivers (*Type).Method
	if strings.Contains(fullName, "(*") && strings.Contains(fullName, ").") {
		return h.extractMethodFromReceiver(fullName)
	}

	// Pattern 3: Function calls (may include .func1, .func2 suffixes)
	if strings.Contains(fullName, ".") {
		return h.extractFunctionName(fullName)
	}

	// Pattern 4: Simple function names
	return fullName
}

// extractMethodFromReceiver extracts method name from receiver pattern
func (h *HertzRouteDiscoverer) extractMethodFromReceiver(fullName string) string {
	// Find the last occurrence of ).MethodName
	parenIdx := strings.LastIndex(fullName, ").")
	if parenIdx == -1 {
		return ""
	}

	// Extract everything after ).
	methodPart := fullName[parenIdx+2:]

	// Remove common suffixes
	methodPart = strings.TrimSuffix(methodPart, "-fm")
	methodPart = strings.TrimSuffix(methodPart, ".func1")
	methodPart = strings.TrimSuffix(methodPart, ".func2")

	// Extract just the method name (in case there are more dots)
	if dotIdx := strings.Index(methodPart, "."); dotIdx != -1 {
		methodPart = methodPart[:dotIdx]
	}

	return methodPart
}

// extractFunctionName extracts function name from various dot-separated patterns
func (h *HertzRouteDiscoverer) extractFunctionName(fullName string) string {
	// Split by dots and take the last meaningful part
	parts := strings.Split(fullName, ".")
	if len(parts) == 0 {
		return ""
	}

	// Work backwards to find the actual function name
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		
		// Skip common suffixes
		if part == "func1" || part == "func2" || part == "func3" || 
		   strings.HasSuffix(part, "-fm") {
			continue
		}
		
		// Skip receiver types (surrounded by parentheses patterns)
		if strings.HasPrefix(part, "(*") || strings.HasSuffix(part, ")") {
			continue
		}
		
		// This should be our function name
		if part != "" && !strings.Contains(part, "/") {
			return strings.TrimSuffix(part, "-fm")
		}
	}

	return ""
}

// GetFrameworkName returns the framework name
func (h *HertzRouteDiscoverer) GetFrameworkName() string {
	return "CloudWeGo Hertz"
}

// isGenericFuncSignature checks if the function signature is generic
func isGenericFuncSignature(signature string) bool {
	// Check if it's a generic function signature like "func(context.Context, *app.RequestContext)"
	return signature == "func(context.Context, *app.RequestContext)" ||
		signature == "func(*app.RequestContext)" ||
		len(signature) < 10 // Too short to be meaningful
}

// HertzServerAdapter adapts a Hertz server to implement the HTTPServer interface
type HertzServerAdapter struct {
	hertz *server.Hertz
}

// NewHertzServerAdapter creates a new adapter for Hertz server
func NewHertzServerAdapter(hertz *server.Hertz) HTTPServer {
	return &HertzServerAdapter{hertz: hertz}
}

// GET implements the HTTPServer interface by adapting to Hertz
func (h *HertzServerAdapter) GET(path string, handler HTTPHandler) {
	// Convert the generic HTTPHandler to a Hertz HandlerFunc
	hertzHandler := func(ctx context.Context, c *app.RequestContext) {
		// Create a response writer that adapts Hertz RequestContext to http.ResponseWriter
		rw := &hertzResponseWriter{
			ctx:     c,
			headers: make(http.Header),
		}

		// Create a request from Hertz RequestContext
		req := &http.Request{
			Method: string(c.Method()),
			Header: make(http.Header),
		}

		// Copy headers from Hertz to standard HTTP
		c.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.Add(string(key), string(value))
		})

		// Call the generic handler
		handler(rw, req)
	}

	h.hertz.GET(path, hertzHandler)
}

// hertzResponseWriter adapts Hertz RequestContext to http.ResponseWriter
type hertzResponseWriter struct {
	ctx     *app.RequestContext
	headers http.Header
}

func (w *hertzResponseWriter) Header() http.Header {
	return w.headers
}

func (w *hertzResponseWriter) Write(data []byte) (int, error) {
	w.ctx.Write(data)
	return len(data), nil
}

func (w *hertzResponseWriter) WriteHeader(statusCode int) {
	// Apply all stored headers to the Hertz response
	for key, values := range w.headers {
		for _, value := range values {
			w.ctx.Response.Header.Set(key, value)
		}
	}
	w.ctx.SetStatusCode(statusCode)
}

// HertzHandlerAnalyzer analyzes CloudWeGo Hertz handlers
type HertzHandlerAnalyzer struct {
	schemaGen      *analyzer.SchemaGenerator
	typeRegistry   *analyzer.DynamicTypeRegistry
	sourceFilePath string // Path to the source file being analyzed
	config         interface{} // Configuration passed from library consumer
}

// NewHertzHandlerAnalyzer creates a new Hertz handler analyzer
func NewHertzHandlerAnalyzer() *HertzHandlerAnalyzer {
	return &HertzHandlerAnalyzer{
		schemaGen:    analyzer.NewSchemaGenerator(),
		typeRegistry: analyzer.NewDynamicTypeRegistry(),
	}
}

// GetFrameworkName returns the framework name
func (h *HertzHandlerAnalyzer) GetFrameworkName() string {
	return "CloudWeGo Hertz"
}

// GetSchemaGenerator returns the internal schema generator for testing
func (h *HertzHandlerAnalyzer) GetSchemaGenerator() *analyzer.SchemaGenerator {
	return h.schemaGen
}

// SetConfig sets the configuration for the analyzer (implements HandlerAnalyzer interface)
func (h *HertzHandlerAnalyzer) SetConfig(config interface{}) {
	h.config = config
}

// isProductionMode checks if running in production mode based on config
func (h *HertzHandlerAnalyzer) isProductionMode() bool {
	if h.config != nil {
		// Try to assert as our Config type
		if cfg, ok := h.config.(interface{ IsProductionMode() bool }); ok {
			return cfg.IsProductionMode()
		}
	}
	return false
}

// isASTAnalysisEnabled checks if AST analysis should be performed
func (h *HertzHandlerAnalyzer) isASTAnalysisEnabled() bool {
	if h.config != nil {
		// Try to assert as our Config type
		if cfg, ok := h.config.(interface{ IsASTAnalysisEnabled() bool }); ok {
			return cfg.IsASTAnalysisEnabled()
		}
	}
	return true // Default to enabled if no config
}

// ExtractTypes extracts request and response types from Hertz handler function
func (h *HertzHandlerAnalyzer) ExtractTypes(handler interface{}) (requestType, responseType reflect.Type, err error) {
	if handler == nil {
		return nil, nil, fmt.Errorf("handler is nil")
	}

	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("handler is not a function")
	}

	handlerType := handlerValue.Type()

	// Validate Hertz handler signature: func(ctx context.Context, c *app.RequestContext)
	if err := h.validateHertzSignature(handlerType); err != nil {
		return nil, nil, fmt.Errorf("invalid Hertz handler signature: %w", err)
	}

	// Use AST analysis to examine the handler's body for BindAndValidate calls
	reqType, respType := h.inferTypesFromContext(handlerValue)

	return reqType, respType, nil
}

// AnalyzeHandler analyzes handler and returns schemas with Docker-compatible fallbacks
func (h *HertzHandlerAnalyzer) AnalyzeHandler(handler interface{}) analyzer.HandlerSchema {
	// First, try to analyze using reflection
	reqType, respType, err := h.ExtractTypes(handler)

	schema := analyzer.HandlerSchema{}

	if err == nil && (reqType != nil || respType != nil) {
		// Reflection analysis worked
		if reqType != nil {
			schema.RequestSchema = h.schemaGen.GenerateSchemaFromType(reqType)
		}
		if respType != nil {
			schema.ResponseSchema = h.schemaGen.GenerateSchemaFromType(respType)
		}
		return schema
	}

	// Second, try AST analysis (only if enabled and source files are available)
	if h.isASTAnalysisEnabled() && !h.isProductionMode() && h.areSourceFilesAvailable() {
		if astSchema := h.tryASTAnalysis(handler); astSchema.RequestSchema.Type != "" || astSchema.ResponseSchema.Type != "" {
			return astSchema
		}
	}

	// Final fallback: Generate generic schemas for Docker/production environments
	return h.generateFallbackSchemas()
}

// areSourceFilesAvailable checks if Go source files are available (not in Docker/production)
func (h *HertzHandlerAnalyzer) areSourceFilesAvailable() bool {
	// Quick check: try to find any .go file in common locations
	wd, err := os.Getwd()
	if err != nil {
		return false
	}

	// Check for .go files in current directory and common subdirectories
	checkDirs := []string{
		wd,
		filepath.Join(wd, "internal"),
		filepath.Join(wd, "pkg"),
		filepath.Join(wd, "cmd"),
	}

	for _, dir := range checkDirs {
		if files, err := os.ReadDir(dir); err == nil {
			for _, file := range files {
				if strings.HasSuffix(file.Name(), ".go") {
					return true
				}
			}
		}
	}

	return false
}

// tryASTAnalysis attempts AST-based analysis when source files are available
func (h *HertzHandlerAnalyzer) tryASTAnalysis(handler interface{}) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	handlerValue := reflect.ValueOf(handler)
	if !handlerValue.IsValid() {
		return schema
	}

	handlerType := handlerValue.Type()
	handlerName := handlerType.String()

	// Check if this is a wrapped Hertz handler
	if handlerName == "app.HandlerFunc" {
		// Try to get the original handler name from runtime info
		if originalHandlerName := h.getOriginalHandlerName(handlerValue); originalHandlerName != "" {
			// Get the full name for source file resolution
			pc := handlerValue.Pointer()
			var fullName string
			if pc != 0 {
				if fn := runtime.FuncForPC(pc); fn != nil {
					fullName = fn.Name()
				}
			}
			return h.analyzeHandlerByName(originalHandlerName, fullName)
		}
	}

	return schema
}

// generateFallbackSchemas generates generic schemas for Docker/production environments
func (h *HertzHandlerAnalyzer) generateFallbackSchemas() analyzer.HandlerSchema {
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
			"status": {
				Type:        "integer",
				Description: "HTTP status code",
				Example:     200,
			},
		},
		Required: []string{"status"},
		Description: "Generic response schema - AST analysis not available",
	}

	return schema
}

// getOriginalHandlerName attempts to extract the original handler name from runtime info for external modules
func (h *HertzHandlerAnalyzer) getOriginalHandlerName(handlerValue reflect.Value) string {
	// Get the function pointer
	pc := handlerValue.Pointer()
	if pc == 0 {
		return ""
	}

	// Get function info
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}

	// Get the full function name
	fullName := fn.Name()

	// Enhanced parsing for external modules
	return h.parseHandlerNameFromFunction(fullName)
}

// parseHandlerNameFromFunction parses handler name from various function name patterns
func (h *HertzHandlerAnalyzer) parseHandlerNameFromFunction(fullName string) string {
	// Handle different patterns from external modules:
	// 1. external-app/internal/handlers.(*UserHandler).CreateUser-fm
	// 2. github.com/user/app/pkg/api.(*Controller).Method-fm
	// 3. some.domain/path/handlers.(*Handler).Method.func1
	// 4. app/handlers.Function

	// Pattern 1 & 2: Method receivers (*Type).Method
	if strings.Contains(fullName, "(*") && strings.Contains(fullName, ").") {
		return h.extractMethodFromReceiver(fullName)
	}

	// Pattern 3: Function calls (may include .func1, .func2 suffixes)
	if strings.Contains(fullName, ".") {
		return h.extractFunctionName(fullName)
	}

	// Pattern 4: Simple function names
	return fullName
}

// extractMethodFromReceiver extracts method name from receiver pattern
func (h *HertzHandlerAnalyzer) extractMethodFromReceiver(fullName string) string {
	// Find the last occurrence of ).MethodName
	parenIdx := strings.LastIndex(fullName, ").")
	if parenIdx == -1 {
		return ""
	}

	// Extract everything after ).
	methodPart := fullName[parenIdx+2:]

	// Remove common suffixes
	methodPart = strings.TrimSuffix(methodPart, "-fm")
	methodPart = strings.TrimSuffix(methodPart, ".func1")
	methodPart = strings.TrimSuffix(methodPart, ".func2")

	// Extract just the method name (in case there are more dots)
	if dotIdx := strings.Index(methodPart, "."); dotIdx != -1 {
		methodPart = methodPart[:dotIdx]
	}

	return methodPart
}

// extractFunctionName extracts function name from various dot-separated patterns
func (h *HertzHandlerAnalyzer) extractFunctionName(fullName string) string {
	// Split by dots and take the last meaningful part
	parts := strings.Split(fullName, ".")
	if len(parts) == 0 {
		return ""
	}

	// Work backwards to find the actual function name
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		
		// Skip common suffixes
		if part == "func1" || part == "func2" || part == "func3" || 
		   strings.HasSuffix(part, "-fm") {
			continue
		}
		
		// Skip receiver types (surrounded by parentheses patterns)
		if strings.HasPrefix(part, "(*") || strings.HasSuffix(part, ")") {
			continue
		}
		
		// This should be our function name
		if part != "" && !strings.Contains(part, "/") {
			return strings.TrimSuffix(part, "-fm")
		}
	}

	// Fallback: return the last part
	lastPart := parts[len(parts)-1]
	return strings.TrimSuffix(lastPart, "-fm")
}

// analyzeHandlerByName analyzes a handler based on its method name using AST
func (h *HertzHandlerAnalyzer) analyzeHandlerByName(methodName string, handlerFuncName string) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	// Try to find the handler file and analyze it using AST
	if sourceFile := h.findHandlerSourceFile(handlerFuncName); sourceFile != "" {
		return h.analyzeHandlerWithAST(sourceFile, methodName)
	}

	return schema
}

// findHandlerSourceFile attempts to find the source file containing the handler for library usage
func (h *HertzHandlerAnalyzer) findHandlerSourceFile(handlerFuncName string) string {
	// Extract package path from handler function name
	// Example: some-service/internal/interfaces/http/handlers.(*SomeHandler).Method-fm
	// -> some-service/internal/interfaces/http/handlers

	if !strings.Contains(handlerFuncName, ".") {
		return ""
	}

	// Extract the package path before the receiver or function
	pkgPath := h.extractPackagePathFromFunction(handlerFuncName)
	if pkgPath == "" {
		return ""
	}

	// Try to find the source file using multiple strategies for library usage
	return h.findSourceFileInConsumerModule(pkgPath)
}

// extractPackagePathFromFunction extracts clean package path from function name
func (h *HertzHandlerAnalyzer) extractPackagePathFromFunction(handlerFuncName string) string {
	// Handle different function name patterns:
	// 1. some-service/pkg/handlers.(*Handler).Method-fm
	// 2. some-service/pkg/handlers.Function
	// 3. some-service/pkg/handlers.Function.func1

	// Find the last occurrence of .) or just .
	var pkgPath string

	// Pattern 1: (*Type).Method-fm
	if strings.Contains(handlerFuncName, "(*") && strings.Contains(handlerFuncName, ").") {
		start := strings.LastIndex(handlerFuncName, "(*")
		if start > 0 {
			pkgPath = handlerFuncName[:start-1] // -1 to remove the dot before (*
		}
	} else {
		// Pattern 2 & 3: Simple function calls
		lastDot := strings.LastIndex(handlerFuncName, ".")
		if lastDot > 0 {
			pkgPath = handlerFuncName[:lastDot]
		}
	}

	return strings.TrimSpace(pkgPath)
}

// findSourceFileInConsumerModule finds source files in the consuming application's module
func (h *HertzHandlerAnalyzer) findSourceFileInConsumerModule(pkgPath string) string {
	// Get the consuming application's working directory
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Get the consuming application's module name
	consumerModule := h.getCurrentModuleName()
	if consumerModule == "" {
		return ""
	}

	// Remove the consumer module prefix to get relative path
	relativePkgPath := strings.TrimPrefix(pkgPath, consumerModule+"/")
	if relativePkgPath == pkgPath {
		// If no prefix was removed, the package might be using a different pattern
		// Try to extract the relative part differently
		parts := strings.Split(pkgPath, "/")
		if len(parts) > 2 {
			// Skip the first part (likely module domain) and reconstruct
			relativePkgPath = strings.Join(parts[1:], "/")
		}
	}

	// Convert package path to file system path
	pkgDir := filepath.Join(wd, filepath.FromSlash(relativePkgPath))

	// Strategy 1: Look for .go files in the exact package directory
	if sourceFile := h.findGoFilesInDirectory(pkgDir); sourceFile != "" {
		return sourceFile
	}

	// Strategy 2: Try common handler directory patterns
	commonPatterns := []string{
		filepath.Join(wd, "handlers"),
		filepath.Join(wd, "internal", "handlers"),
		filepath.Join(wd, "pkg", "handlers"),
		filepath.Join(wd, "api", "handlers"),
		filepath.Join(wd, "internal", "api", "handlers"),
	}

	for _, pattern := range commonPatterns {
		if sourceFile := h.findGoFilesInDirectory(pattern); sourceFile != "" {
			return sourceFile
		}
	}

	// Strategy 3: Search recursively in common directories (limited depth)
	commonRoots := []string{
		filepath.Join(wd, "internal"),
		filepath.Join(wd, "pkg"),
		filepath.Join(wd, "cmd"),
	}

	for _, root := range commonRoots {
		if sourceFile := h.findGoFilesRecursive(root, 3); sourceFile != "" { // max depth 3
			return sourceFile
		}
	}

	return ""
}

// findGoFilesInDirectory finds the first .go file in a directory
func (h *HertzHandlerAnalyzer) findGoFilesInDirectory(dir string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".go") && !strings.HasSuffix(file.Name(), "_test.go") {
			return filepath.Join(dir, file.Name())
		}
	}
	return ""
}

// findGoFilesRecursive searches for .go files recursively with depth limit
func (h *HertzHandlerAnalyzer) findGoFilesRecursive(root string, maxDepth int) string {
	if maxDepth <= 0 {
		return ""
	}

	// Check current directory first
	if sourceFile := h.findGoFilesInDirectory(root); sourceFile != "" {
		return sourceFile
	}

	// Check subdirectories
	files, err := os.ReadDir(root)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if file.IsDir() {
			subDir := filepath.Join(root, file.Name())
			if sourceFile := h.findGoFilesRecursive(subDir, maxDepth-1); sourceFile != "" {
				return sourceFile
			}
		}
	}

	return ""
}

// analyzeHandlerWithAST analyzes a handler using AST parsing with error handling
func (h *HertzHandlerAnalyzer) analyzeHandlerWithAST(sourceFile string, methodName string) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	// Check if source file exists (Docker-compatible check)
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		// Source file not available, return empty schema
		// This allows fallback mechanisms to take over
		return schema
	}

	// Parse the source file with error handling
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, sourceFile, nil, parser.ParseComments)
	if err != nil {
		// Parse error, likely due to missing file or syntax issues
		return schema
	}

	// Parse imports to populate the dynamic type registry
	h.typeRegistry.ParseImports(src)

	// Find the handler method
	var methodDecl *ast.FuncDecl
	for _, decl := range src.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == methodName {
			methodDecl = fn
			break
		}
	}

	if methodDecl == nil {
		// Method not found in this source file
		return schema
	}

	// Store the source file path for type resolution
	h.sourceFilePath = sourceFile

	// Build import map from the source file
	imports := make(map[string]string)
	for _, imp := range src.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		alias := ""

		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			alias = parts[len(parts)-1]
		}

		if alias != "_" && alias != "." {
			imports[alias] = path
		}
	}

	// Extract request schema from BindAndValidate calls using AST
	reqSchema := h.extractRequestSchemaFromAST(methodDecl, imports)
	if reqSchema.Type != "" {
		schema.RequestSchema = reqSchema
	}

	// Extract response schema from JSON calls
	respType := h.extractResponseTypeFromAST(methodDecl)
	if respType != nil {
		schema.ResponseSchema = h.schemaGen.GenerateSchemaFromType(respType)
	}

	return schema
}

// extractRequestTypeFromAST extracts request type from AST function declaration and returns schema
func (h *HertzHandlerAnalyzer) extractRequestSchemaFromAST(funcDecl *ast.FuncDecl, imports map[string]string) spec.Schema {
	var requestSchema spec.Schema

	// Walk through the function body looking for BindAndValidate calls
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if requestSchema.Type != "" {
			return false // Stop once we found it
		}

		if callExpr, ok := n.(*ast.CallExpr); ok {
			if h.isBindAndValidateCall(callExpr) {
				// Extract the type from the address-of expression
				if len(callExpr.Args) > 0 {
					if unaryExpr, ok := callExpr.Args[0].(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
						if ident, ok := unaryExpr.X.(*ast.Ident); ok {
							// Find the variable declaration and resolve its type using AST
							if schema := h.resolveVariableTypeFromAST(ident, funcDecl, imports); schema.Type != "" {
								requestSchema = schema
								return false
							}
						}
					}
				}
			}
		}
		return true
	})

	return requestSchema
}

// resolveVariableTypeFromAST resolves a variable's type from AST declarations
func (h *HertzHandlerAnalyzer) resolveVariableTypeFromAST(ident *ast.Ident, funcDecl *ast.FuncDecl, imports map[string]string) spec.Schema {
	// Look for variable declarations in the function body
	var foundSchema spec.Schema

	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if foundSchema.Type != "" {
			return false // Stop once we found a type
		}

		// Check for variable declarations: var req dto.SomeType
		if declStmt, ok := n.(*ast.DeclStmt); ok {
			if genDecl, ok := declStmt.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.Name == ident.Name && valueSpec.Type != nil {
								foundSchema = h.resolveASTTypeToSchema(valueSpec.Type, imports)
								return false
							}
						}
					}
				}
			}
		}

		// Check for short variable declarations: req := dto.SomeType{}
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			if assignStmt.Tok == token.DEFINE || assignStmt.Tok == token.ASSIGN {
				for i, lhs := range assignStmt.Lhs {
					if lhsIdent, ok := lhs.(*ast.Ident); ok && lhsIdent.Name == ident.Name {
						if i < len(assignStmt.Rhs) {
							foundSchema = h.resolveExpressionToSchema(assignStmt.Rhs[i], imports)
							if foundSchema.Type != "" {
								return false
							}
						}
					}
				}
			}
		}
		return true
	})

	return foundSchema
}

// resolveASTTypeToSchema converts an AST type expression to OpenAPI schema
func (h *HertzHandlerAnalyzer) resolveASTTypeToSchema(typeExpr ast.Expr, imports map[string]string) spec.Schema {
	switch t := typeExpr.(type) {
	case *ast.SelectorExpr:
		// Handle package.Type expressions like dto.RegisterUserRequest
		return h.resolveTypeFromSelectorAST(t, imports)
	case *ast.Ident:
		// Handle local types
		return spec.Schema{Type: "object", Description: "Local type: " + t.Name}
	}
	return spec.Schema{Type: "object", Description: "Unknown AST type"}
}

// resolveExpressionToSchema resolves expressions like &dto.SomeType{} to schema
func (h *HertzHandlerAnalyzer) resolveExpressionToSchema(expr ast.Expr, imports map[string]string) spec.Schema {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		// Handle struct literals like dto.RegisterUserRequest{}
		if selExpr, ok := e.Type.(*ast.SelectorExpr); ok {
			return h.resolveTypeFromSelectorAST(selExpr, imports)
		}
	case *ast.UnaryExpr:
		// Handle address-of expressions like &dto.RegisterUserRequest{}
		if e.Op == token.AND {
			return h.resolveExpressionToSchema(e.X, imports)
		}
	}
	return spec.Schema{Type: "object", Description: "Unknown expression type"}
}

// extractResponseTypeFromAST extracts response type from AST function declaration
func (h *HertzHandlerAnalyzer) extractResponseTypeFromAST(funcDecl *ast.FuncDecl) reflect.Type {
	var responseType reflect.Type

	// Walk through the function body looking for JSON calls
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if responseType != nil {
			return false // Stop once we found it
		}

		if callExpr, ok := n.(*ast.CallExpr); ok {
			if h.isJSONCall(callExpr) {
				// Extract the type from the second argument (response data)
				if len(callExpr.Args) >= 2 {
					resolvedType := h.resolveTypeFromExpr(callExpr.Args[1])
					if resolvedType != nil {
						responseType = resolvedType
						return false
					}
				}
			}
		}
		return true
	})

	return responseType
}

// validateHertzSignature validates that the function has a Hertz handler signature
func (h *HertzHandlerAnalyzer) validateHertzSignature(handlerType reflect.Type) error {
	// Expected: func(ctx context.Context, c *app.RequestContext)
	if handlerType.NumIn() != 2 {
		return fmt.Errorf("expected 2 parameters, got %d", handlerType.NumIn())
	}

	if handlerType.NumOut() != 0 {
		return fmt.Errorf("expected no return values, got %d", handlerType.NumOut())
	}

	// Check first parameter: context.Context
	firstParam := handlerType.In(0)
	if !h.isContextType(firstParam) {
		return fmt.Errorf("first parameter should be context.Context, got %s", firstParam)
	}

	// Check second parameter: *app.RequestContext
	secondParam := handlerType.In(1)
	if !h.isRequestContextType(secondParam) {
		return fmt.Errorf("second parameter should be *app.RequestContext, got %s", secondParam)
	}

	return nil
}

// isContextType checks if type is context.Context
func (h *HertzHandlerAnalyzer) isContextType(t reflect.Type) bool {
	return t.String() == "context.Context"
}

// isRequestContextType checks if type is *app.RequestContext
func (h *HertzHandlerAnalyzer) isRequestContextType(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr &&
		strings.Contains(t.String(), "RequestContext")
}

// inferTypesFromContext attempts to infer types from handler context by parsing AST
func (h *HertzHandlerAnalyzer) inferTypesFromContext(handlerValue reflect.Value) (requestType, responseType reflect.Type) {
	// Get the function's source location
	pc := handlerValue.Pointer()
	funcForPC := runtime.FuncForPC(pc)
	if funcForPC == nil {
		return nil, nil
	}

	fileName, _ := funcForPC.FileLine(pc)
	if fileName == "" {
		return nil, nil
	}

	h.sourceFilePath = fileName // Store for later use in type resolution

	// Parse the source file
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		return nil, nil
	}

	// Parse imports to populate the dynamic type registry
	h.typeRegistry.ParseImports(src)

	// Find the function declaration
	funcName := funcForPC.Name()
	funcDecl := h.findFunctionDecl(src, funcName)
	if funcDecl == nil {
		return nil, nil
	}

	// Extract types from the function body using dynamic registry
	reqType := h.extractRequestType(funcDecl)
	respType := h.extractResponseType(funcDecl)

	return reqType, respType
}

// findFunctionDecl finds the function declaration by name
func (h *HertzHandlerAnalyzer) findFunctionDecl(file *ast.File, funcName string) *ast.FuncDecl {
	// Extract the simple function name (remove package prefix)
	parts := strings.Split(funcName, ".")
	simpleName := parts[len(parts)-1]

	// Remove any receiver information from method names
	if idx := strings.LastIndex(simpleName, "-"); idx != -1 {
		simpleName = simpleName[idx+1:]
	}

	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == simpleName {
				return funcDecl
			}
		}
	}
	return nil
}

// extractRequestType analyzes BindAndValidate calls to determine request type
func (h *HertzHandlerAnalyzer) extractRequestType(funcDecl *ast.FuncDecl) reflect.Type {
	var requestType reflect.Type

	// Walk through the function body looking for BindAndValidate calls
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if h.isBindAndValidateCall(callExpr) {
				// Extract the type from the address-of expression
				if len(callExpr.Args) > 0 {
					if unaryExpr, ok := callExpr.Args[0].(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
						if ident, ok := unaryExpr.X.(*ast.Ident); ok {
							// Try to resolve the type from variable declarations
							resolvedType := h.resolveTypeFromIdent(ident, funcDecl)
							if resolvedType != nil {
								requestType = resolvedType
								return false // Stop walking once we find it
							}
						}
					}
				}
			}
		}
		return true
	})

	return requestType
}

// extractResponseType analyzes JSON response calls to determine response type
func (h *HertzHandlerAnalyzer) extractResponseType(funcDecl *ast.FuncDecl) reflect.Type {
	var responseType reflect.Type

	// Walk through the function body looking for JSON calls
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if h.isJSONCall(callExpr) {
				// Extract the type from the second argument (response data)
				if len(callExpr.Args) >= 2 {
					resolvedType := h.resolveTypeFromExpr(callExpr.Args[1])
					if resolvedType != nil {
						responseType = resolvedType
						return false // Stop walking once we find a concrete type
					}
				}
			}
		}
		return true
	})

	return responseType
}

// isBindAndValidateCall checks if the call expression is a binding call (framework-agnostic)
func (h *HertzHandlerAnalyzer) isBindAndValidateCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		// Support multiple binding patterns for different frameworks
		bindMethods := []string{
			"BindAndValidate", // Hertz
			"ShouldBind",      // Gin
			"ShouldBindJSON",  // Gin
			"Bind",            // Echo, Fiber
			"BindJSON",        // Echo, Fiber
			"ParseBody",       // Fiber
			"BodyParser",      // Fiber
		}

		methodName := selExpr.Sel.Name
		for _, bindMethod := range bindMethods {
			if methodName == bindMethod {
				return true
			}
		}
	}
	return false
}

// isJSONCall checks if the call expression is a JSON response call (framework-agnostic)
func (h *HertzHandlerAnalyzer) isJSONCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		// Support multiple JSON response patterns for different frameworks
		jsonMethods := []string{
			"JSON",         // Hertz, Gin, Echo, Fiber
			"IndentedJSON", // Gin
			"SecureJSON",   // Gin
			"JSONP",        // Gin
			"Status",       // Sometimes followed by JSON
		}

		methodName := selExpr.Sel.Name
		for _, jsonMethod := range jsonMethods {
			if methodName == jsonMethod {
				return true
			}
		}
	}

	// Also check for standard library json.NewEncoder calls
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			// Check for json.NewEncoder(w).Encode(data) patterns
			if ident.Name == "json" && selExpr.Sel.Name == "Encode" {
				return true
			}
		}
	}

	return false
}

// resolveTypeFromIdent attempts to resolve the type of an identifier from variable declarations
func (h *HertzHandlerAnalyzer) resolveTypeFromIdent(ident *ast.Ident, funcDecl *ast.FuncDecl) reflect.Type {
	var foundType reflect.Type

	// Look for variable declarations in the function body
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if foundType != nil {
			return false // Stop once we found a type
		}

		if declStmt, ok := n.(*ast.DeclStmt); ok {
			if genDecl, ok := declStmt.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.Name == ident.Name && valueSpec.Type != nil {
								foundType = h.resolveTypeFromAST(valueSpec.Type)
								return false
							}
						}
					}
				}
			}
		}
		// Also check for short variable declarations and regular assignments
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			// Handle both := (DEFINE) and = (ASSIGN) tokens
			if assignStmt.Tok == token.DEFINE || assignStmt.Tok == token.ASSIGN {
				for i, lhs := range assignStmt.Lhs {
					if lhsIdent, ok := lhs.(*ast.Ident); ok && lhsIdent.Name == ident.Name {
						if i < len(assignStmt.Rhs) {
							foundType = h.resolveTypeFromExpr(assignStmt.Rhs[i])
							if foundType != nil {
								return false
							}
						}
					}
				}
			}
		}
		return true
	})
	return foundType
}

// resolveTypeFromExpr attempts to resolve the type from an expression
func (h *HertzHandlerAnalyzer) resolveTypeFromExpr(expr ast.Expr) reflect.Type {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		// Handle struct literals like dto.RegisterUserRequest{} or &dto.RegisterUserRequest{}
		if _, ok := e.Type.(*ast.SelectorExpr); ok {
			return h.resolveTypeFromSelector()
		}
		if ident, ok := e.Type.(*ast.Ident); ok {
			return h.resolveLocalType(ident.Name)
		}

	case *ast.UnaryExpr:
		// Handle address-of expressions like &dto.RegisterUserRequest{}
		if e.Op == token.AND {
			if innerType := h.resolveTypeFromExpr(e.X); innerType != nil {
				// Return the non-pointer version since we're detecting the base type
				if innerType.Kind() == reflect.Pointer {
					return innerType.Elem()
				}
				return innerType
			}
		}

	case *ast.Ident:
		// Handle identifiers - could be variables or local types
		return h.resolveLocalType(e.Name)

	case *ast.SelectorExpr:
		// Handle package.Type expressions
		return h.resolveTypeFromSelector()

	case *ast.CallExpr:
		// Handle function calls that return typed values
		// This could be constructor functions or type conversions
		if selExpr, ok := e.Fun.(*ast.SelectorExpr); ok {
			// Check for package.NewType() patterns
			funcName := selExpr.Sel.Name
			if strings.HasPrefix(funcName, "New") || strings.HasPrefix(funcName, "Create") {
				// Try to infer return type from function name
				typeName := strings.TrimPrefix(funcName, "New")
				typeName = strings.TrimPrefix(typeName, "Create")
				if typeName != "" {
					if pkgIdent, ok := selExpr.X.(*ast.Ident); ok {
						return h.typeRegistry.GetType(pkgIdent.Name, typeName)
					}
				}
			}
		}
	}
	return nil
}

// resolveTypeFromAST resolves type from AST type expression
func (h *HertzHandlerAnalyzer) resolveTypeFromAST(typeExpr ast.Expr) reflect.Type {
	switch t := typeExpr.(type) {
	case *ast.SelectorExpr:
		return h.resolveTypeFromSelector()
	case *ast.Ident:
		return h.resolveLocalType(t.Name)
	}
	return nil
}

// resolveTypeFromSelector attempts to resolve type from package.Type selector using dynamic AST parsing
func (h *HertzHandlerAnalyzer) resolveTypeFromSelector() reflect.Type {
	// For now, return nil to let the new AST-based approach handle it
	// This method will be used by the AST-based schema generation instead
	return nil
}

// resolveTypeFromSelectorAST resolves type from package.Type selector and returns schema directly
func (h *HertzHandlerAnalyzer) resolveTypeFromSelectorAST(selExpr *ast.SelectorExpr, imports map[string]string) spec.Schema {
	if pkgIdent, ok := selExpr.X.(*ast.Ident); ok {
		packageAlias := pkgIdent.Name
		typeName := selExpr.Sel.Name

		// Get the actual package path from imports
		if packagePath, exists := imports[packageAlias]; exists {
			// Use the new AST-based struct parsing
			return h.parseStructFromPackage(packagePath, typeName)
		}
	}

	return spec.Schema{
		Type:        "object",
		Description: "Could not resolve type from selector",
	}
}

// resolveLocalType attempts to resolve local types from the current package
func (h *HertzHandlerAnalyzer) resolveLocalType(typeName string) reflect.Type {
	// Try to resolve types from the current package scope
	// This is useful for types defined in the same package as the handler

	// First, try using reflection to get the type directly
	// This works for types that are accessible in the current runtime
	if typ := h.tryResolveTypeByReflection(typeName); typ != nil {
		return typ
	}

	// Fallback to the original approach using runtime caller
	if typ := h.resolveLocalTypeByCaller(typeName); typ != nil {
		return typ
	}

	return nil
}

// tryResolveTypeByReflection attempts to resolve a type using reflection
// This is more reliable for types defined in the same package
func (h *HertzHandlerAnalyzer) tryResolveTypeByReflection(typeName string) reflect.Type {
	// Try to find the type by checking common package aliases
	// Since we can't access the private typeCache directly, we'll try common patterns

	// Common package aliases to check
	commonAliases := []string{"", "main", "."}

	for _, alias := range commonAliases {
		if typ := h.typeRegistry.GetType(alias, typeName); typ != nil {
			return typ
		}
	}

	// Try to resolve using the current package context
	// This is a simple approach that works for common cases
	// We can try to find types that are accessible through the current module

	// For now, return nil to let the fallback method handle it
	return nil
}

// resolveLocalTypeByCaller is the original approach using runtime caller
func (h *HertzHandlerAnalyzer) resolveLocalTypeByCaller(typeName string) reflect.Type {
	// Since we're analyzing a handler, we should use the source file's package context
	// The source file path is available from the AST analysis
	if h.sourceFilePath != "" {
		// Extract package path from file path
		// Convert /home/zainokta/projects/openapi-gen/test/demo_analyzer.go
		// to auth-service/test
		packagePath := h.extractPackageFromFilePath(h.sourceFilePath)
		if packagePath != "" {
			return h.tryResolveTypeFromPackage(packagePath, typeName)
		}
	}

	// Fallback: Get the current calling context to determine the package
	pc, _, _, ok := runtime.Caller(3) // Go up 3 levels to get the original caller
	if !ok {
		return nil
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return nil
	}

	// Extract package path from function name
	fullName := fn.Name()

	// Try to extract a cleaner package path
	packagePath := h.extractPackageFromFunctionName(fullName)
	if packagePath == "" {
		return nil
	}

	// Remove receiver type information if it's a method
	if idx := strings.LastIndex(packagePath, "("); idx != -1 {
		packagePath = packagePath[:idx]
	}

	// Try to load the package types if not already loaded
	if err := h.typeRegistry.LoadPackageTypes(packagePath); err != nil {
		// If we can't load by full path, try just the package name
		parts := strings.Split(packagePath, "/")
		if len(parts) > 0 {
			simplePackage := parts[len(parts)-1]
			if simpleType := h.typeRegistry.GetType(simplePackage, typeName); simpleType != nil {
				return simpleType
			}
		}
		return nil
	}

	// Look up the type in the loaded package
	// Try both full path and simple package name
	if fullType := h.typeRegistry.GetType(packagePath, typeName); fullType != nil {
		return fullType
	}

	// Also try with just the package name as alias
	parts := strings.Split(packagePath, "/")
	if len(parts) > 0 {
		simplePackage := parts[len(parts)-1]
		if simpleType := h.typeRegistry.GetType(simplePackage, typeName); simpleType != nil {
			return simpleType
		}
	}

	return nil
}

// extractPackageFromFilePath converts a file path to a Go package path
func (h *HertzHandlerAnalyzer) extractPackageFromFilePath(filePath string) string {
	// Get the working directory to find the project root
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Check if the file path is within the current directory
	if !strings.HasPrefix(filePath, wd) {
		return ""
	}

	// Remove the working directory prefix
	relativePath := strings.TrimPrefix(filePath, wd)
	relativePath = strings.TrimPrefix(relativePath, "/")

	// Remove the file name
	relativePath = filepath.Dir(relativePath)

	// Convert to package path by replacing path separators with slashes
	packagePath := filepath.ToSlash(relativePath)

	// Assuming the project is in the GOPATH or is a Go module,
	// we need to construct the full import path
	// For now, let's assume the project root is the module name
	// This is a simplification - in a real scenario we'd read go.mod
	if goModPath := h.findGoModPath(wd); goModPath != "" {
		moduleName := h.getModuleNameFromGoMod(goModPath)
		if moduleName != "" {
			// Get the path relative to the go.mod directory
			goModDir := filepath.Dir(goModPath)
			if relToMod, err := filepath.Rel(goModDir, wd); err == nil {
				if relToMod == "." {
					relToMod = ""
				} else {
					relToMod = filepath.ToSlash(relToMod)
				}

				// Construct full package path
				if relToMod != "" {
					return moduleName + "/" + relToMod + "/" + packagePath
				}
				return moduleName + "/" + packagePath
			}
		}
	}

	// Fallback: use current module name
	if moduleName := h.getCurrentModuleName(); moduleName != "" {
		return moduleName + "/" + packagePath
	}
	return packagePath
}

// getModuleFromRuntimeCaller attempts to extract module name from runtime caller info
// This helps identify the consuming application's module when used as a library
func (h *HertzHandlerAnalyzer) getModuleFromRuntimeCaller() string {
	// Walk up the call stack to find the first caller outside our library
	for i := 1; i < 20; i++ {
		pc, _, _, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		funcName := fn.Name()
		// Skip our own library functions
		if strings.Contains(funcName, "github.com/zainokta/openapi-gen/") {
			continue
		}

		// Extract module name from function name
		if moduleName := h.extractModuleFromFunctionName(funcName); moduleName != "" {
			return moduleName
		}
	}
	return ""
}

// extractModuleFromFunctionName extracts module name from a full function name
func (h *HertzHandlerAnalyzer) extractModuleFromFunctionName(funcName string) string {
	// Function name format: module.com/path/package.function or module.com/path/package.(*Type).method
	parts := strings.Split(funcName, "/")
	if len(parts) < 2 {
		return ""
	}

	// Find the module root (typically domain/org pattern)
	for i, part := range parts {
		// Look for domain-like patterns (contains dots)
		if strings.Contains(part, ".") && i < len(parts)-1 {
			// Reconstruct module path up to the package level
			moduleParts := parts[:i+2] // domain.com/org typically
			potentialModule := strings.Join(moduleParts, "/")
			
			// Remove function/method part
			if dotIdx := strings.Index(potentialModule, "."); dotIdx > 0 {
				// Check if this looks like a module path
				if strings.Count(potentialModule[:dotIdx], "/") >= 1 {
					return potentialModule[:dotIdx]
				}
			}
			return potentialModule
		}
	}
	return ""
}

// findGoModPath looks for go.mod file in parent directories
func (h *HertzHandlerAnalyzer) findGoModPath(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}
	return ""
}

// getModuleNameFromGoMod reads the module name from go.mod file
func (h *HertzHandlerAnalyzer) getModuleNameFromGoMod(goModPath string) string {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}
	return ""
}

// tryResolveTypeFromPackage attempts to resolve a type from a specific package
func (h *HertzHandlerAnalyzer) tryResolveTypeFromPackage(packagePath, typeName string) reflect.Type {

	// Try to load the package types
	err := h.typeRegistry.LoadPackageTypes(packagePath)
	if err != nil {
		return nil
	}

	// Try to find the type in the package
	if typ := h.typeRegistry.GetType("", typeName); typ != nil {
		return typ
	}

	// Try with the last part of the package path as alias
	parts := strings.Split(packagePath, "/")
	if len(parts) > 0 {
		simpleAlias := parts[len(parts)-1]
		if typ := h.typeRegistry.GetType(simpleAlias, typeName); typ != nil {
			return typ
		}
	}

	return nil
}

// extractPackageFromFunctionName extracts a clean package path from a function name
func (h *HertzHandlerAnalyzer) extractPackageFromFunctionName(functionName string) string {
	// Function name format: package/path/functionName or package/path.(*ReceiverType).methodName
	// We need to extract just the package/path part

	// Remove any receiver type information
	if idx := strings.Index(functionName, "(*"); idx != -1 {
		functionName = functionName[:idx]
	}
	if idx := strings.Index(functionName, "."); idx != -1 {
		functionName = functionName[:idx]
	}

	// Remove function name part
	lastSlash := strings.LastIndex(functionName, "/")
	if lastSlash == -1 {
		return "" // No path, just package name
	}

	// Check if there's a dot after the last slash (indicating package.function)
	lastDot := strings.LastIndex(functionName, ".")
	if lastDot > lastSlash {
		return functionName[:lastDot]
	}

	return functionName
}

// getCurrentModuleName gets the consuming application's Go module name dynamically
// This is critical for library usage - we need the consumer's module, not our own
func (h *HertzHandlerAnalyzer) getCurrentModuleName() string {
	// First, try to get the module name from runtime caller context
	// This helps identify the actual application using the library
	if moduleName := h.getModuleFromRuntimeCaller(); moduleName != "" {
		return moduleName
	}

	// Fallback: Get current working directory (consumer's directory)
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Find the go.mod file in the consuming application
	if goModPath := h.findGoModPath(wd); goModPath != "" {
		return h.getModuleNameFromGoMod(goModPath)
	}

	return ""
}

// parseStructFromPackage finds and parses a struct definition from any package
func (h *HertzHandlerAnalyzer) parseStructFromPackage(packagePath, typeName string) spec.Schema {
	// Find source files in the package
	sourceFiles := h.findPackageSourceFiles(packagePath)

	for _, sourceFile := range sourceFiles {
		if schema := h.parseStructFromFile(sourceFile, typeName); schema.Type != "" {
			return schema
		}
	}

	// If not found, return a basic object schema
	return spec.Schema{
		Type:        "object",
		Description: "Could not parse struct: " + packagePath + "." + typeName,
	}
}

// findPackageSourceFiles finds all Go source files in a package directory for library usage
func (h *HertzHandlerAnalyzer) findPackageSourceFiles(packagePath string) []string {
	var sourceFiles []string

	// Get consumer's working directory
	wd, err := os.Getwd()
	if err != nil {
		return sourceFiles
	}

	// Get consumer's module name for proper path resolution
	consumerModule := h.getCurrentModuleName()

	// Strategy 1: Try direct package path resolution
	relativePath := strings.TrimPrefix(packagePath, consumerModule+"/")
	possibleDirs := []string{
		filepath.Join(wd, filepath.FromSlash(relativePath)),
		filepath.Join(wd, filepath.FromSlash(packagePath)),
	}

	// Strategy 2: Try common patterns if direct resolution fails
	if relativePath != packagePath {
		// Add more possible locations based on common Go project structures
		parts := strings.Split(relativePath, "/")
		if len(parts) > 1 {
			// Try skipping the first part (internal/pkg/cmd)
			skippedFirst := strings.Join(parts[1:], "/")
			possibleDirs = append(possibleDirs, filepath.Join(wd, filepath.FromSlash(skippedFirst)))
		}
	}

	// Collect source files from all possible directories
	for _, dir := range possibleDirs {
		if files := h.collectGoFilesFromDir(dir); len(files) > 0 {
			sourceFiles = append(sourceFiles, files...)
		}
	}

	// If no files found, try a broader search in common directories
	if len(sourceFiles) == 0 {
		sourceFiles = h.findPackageFilesRecursively(wd, packagePath)
	}

	return sourceFiles
}

// collectGoFilesFromDir collects all .go files from a directory
func (h *HertzHandlerAnalyzer) collectGoFilesFromDir(dir string) []string {
	var files []string
	dirFiles, err := os.ReadDir(dir)
	if err != nil {
		return files
	}

	for _, file := range dirFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".go") && !strings.HasSuffix(file.Name(), "_test.go") {
			files = append(files, filepath.Join(dir, file.Name()))
		}
	}
	return files
}

// findPackageFilesRecursively searches for package files in common directories
func (h *HertzHandlerAnalyzer) findPackageFilesRecursively(root string, targetPackage string) []string {
	var sourceFiles []string

	// Extract the last part of the package path as directory name
	parts := strings.Split(targetPackage, "/")
	targetDirName := parts[len(parts)-1]

	// Search in common Go project directories
	commonDirs := []string{
		filepath.Join(root, "internal"),
		filepath.Join(root, "pkg"),
		filepath.Join(root, "cmd"),
		root, // Also search in root
	}

	for _, searchRoot := range commonDirs {
		filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}

			// Skip if too deep (avoid infinite searches)
			if strings.Count(strings.TrimPrefix(path, searchRoot), string(filepath.Separator)) > 5 {
				return filepath.SkipDir
			}

			if d.IsDir() && d.Name() == targetDirName {
				// Found a directory with the target name, collect .go files
				files := h.collectGoFilesFromDir(path)
				sourceFiles = append(sourceFiles, files...)
			}

			return nil
		})

		// Stop searching if we found files
		if len(sourceFiles) > 0 {
			break
		}
	}

	return sourceFiles
}

// parseStructFromFile parses a struct definition from a specific source file with Docker compatibility
func (h *HertzHandlerAnalyzer) parseStructFromFile(sourceFile, typeName string) spec.Schema {
	// Check if source file exists (Docker-compatible)
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		return spec.Schema{} // File not available, return empty schema
	}

	// Parse the source file with error handling
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, sourceFile, nil, parser.ParseComments)
	if err != nil {
		// Parse failed, return empty schema to allow fallbacks
		return spec.Schema{}
	}

	// Build import map for this file
	imports := make(map[string]string)
	for _, imp := range src.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		alias := ""

		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			alias = parts[len(parts)-1]
		}

		if alias != "_" && alias != "." {
			imports[alias] = path
		}
	}

	// Find the struct declaration
	for _, decl := range src.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == typeName {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						// Generate schema from AST struct
						return h.schemaGen.GenerateSchemaFromStructAST(structType, imports)
					}
				}
			}
		}
	}

	return spec.Schema{} // Not found in this file
}

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
	"github.com/zainokta/openapi-gen/integration/common"
	openapiParser "github.com/zainokta/openapi-gen/parser"
	"github.com/zainokta/openapi-gen/spec"
)

// HertzRouteDiscoverer implements RouteDiscoverer for CloudWeGo Hertz
type HertzRouteDiscoverer struct {
	engine               *server.Hertz
	handlerNameExtractor *common.HandlerNameExtractor
}

// NewHertzRouteDiscoverer creates a new Hertz route discoverer
func NewHertzRouteDiscoverer(engine *server.Hertz) *HertzRouteDiscoverer {
	return &HertzRouteDiscoverer{
		engine:               engine,
		handlerNameExtractor: common.NewHandlerNameExtractor(),
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
							cleanName := h.handlerNameExtractor.ParseHandlerNameFromFunction(runtimeFuncName)
							if cleanName != "" {
								return cleanName
							}
						}
					}
				}

				// Fallback to type string method
				if !isGenericFuncSignature(funcName) {
					// Parse the function name to extract just the method name
					cleanName := h.handlerNameExtractor.ParseHandlerNameFromFunction(funcName)
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

// isGenericFuncSignature checks if the function signature is generic
func isGenericFuncSignature(signature string) bool {
	// Check if it's a generic function signature like "func(context.Context, *app.RequestContext)"
	return signature == "func(context.Context, *app.RequestContext)" ||
		signature == "func(*app.RequestContext)" ||
		len(signature) < 10 // Too short to be meaningful
}

// GetFrameworkName returns the framework name
func (h *HertzRouteDiscoverer) GetFrameworkName() string {
	return "CloudWeGo Hertz"
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
	handlerNameExtractor *common.HandlerNameExtractor
	astAnalyzer          *common.ASTAnalyzer
	typeResolver         *common.TypeResolver
	schemaAnalyzer       *common.SchemaAnalyzer
	sourceFilePath       string      // Path to the source file being analyzed
	config               interface{} // Configuration passed from library consumer
}

// NewHertzHandlerAnalyzer creates a new Hertz handler analyzer
func NewHertzHandlerAnalyzer() *HertzHandlerAnalyzer {
	return &HertzHandlerAnalyzer{
		handlerNameExtractor: common.NewHandlerNameExtractor(),
		astAnalyzer:          common.NewASTAnalyzer(),
		typeResolver:         common.NewTypeResolver(),
		schemaAnalyzer:       common.NewSchemaAnalyzer(),
	}
}

// GetFrameworkName returns the framework name
func (h *HertzHandlerAnalyzer) GetFrameworkName() string {
	return "CloudWeGo Hertz"
}

// GetSchemaGenerator returns the internal schema generator for testing
func (h *HertzHandlerAnalyzer) GetSchemaGenerator() *analyzer.SchemaGenerator {
	return h.schemaAnalyzer.GetSchemaGenerator()
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
			schema.RequestSchema = h.schemaAnalyzer.GetSchemaGenerator().GenerateSchemaFromType(reqType)
		}
		if respType != nil {
			schema.ResponseSchema = h.schemaAnalyzer.GetSchemaGenerator().GenerateSchemaFromType(respType)
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
	return h.schemaAnalyzer.GenerateFallbackSchemas()
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
		if originalHandlerName := h.handlerNameExtractor.GetOriginalHandlerName(handlerValue); originalHandlerName != "" {
			// Get the full name for source file resolution
			pc := handlerValue.Pointer()
			var fullName string
			if pc != 0 {
				if fn := runtime.FuncForPC(pc); fn != nil {
					fullName = fn.Name()
				}
			}
			// Try to find the handler file and analyze it using AST
			if sourceFile := h.astAnalyzer.FindHandlerSourceFile(fullName); sourceFile != "" {
				return h.astAnalyzer.AnalyzeHandlerWithAST(sourceFile, originalHandlerName, "hertz")
			}
		}
	}

	return schema
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
	h.astAnalyzer.GetTypeRegistry().ParseImports(src)

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
							resolvedType := h.astAnalyzer.ExtractTypeFromCompositeLit(&ast.CompositeLit{Type: ident})
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
					resolvedType := h.astAnalyzer.ExtractTypeFromCallExpr(callExpr)
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

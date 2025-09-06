package integration

import (
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

	"github.com/gin-gonic/gin"

	"github.com/zainokta/openapi-gen/analyzer"
	"github.com/zainokta/openapi-gen/integration/common"
	openapiParser "github.com/zainokta/openapi-gen/parser"
	"github.com/zainokta/openapi-gen/spec"
)

// GinRouteDiscoverer implements RouteDiscoverer for Gin
type GinRouteDiscoverer struct {
	engine               *gin.Engine
	handlerNameExtractor *common.HandlerNameExtractor
}

// NewGinRouteDiscoverer creates a new Gin route discoverer
func NewGinRouteDiscoverer(engine *gin.Engine) *GinRouteDiscoverer {
	return &GinRouteDiscoverer{
		engine:               engine,
		handlerNameExtractor: common.NewHandlerNameExtractor(),
	}
}

// DiscoverRoutes discovers all routes from Gin engine using Routes() method
func (g *GinRouteDiscoverer) DiscoverRoutes() ([]spec.RouteInfo, error) {
	var routes []spec.RouteInfo

	// Use Gin's built-in Routes() method to get all registered routes
	ginRoutes := g.engine.Routes()

	for _, route := range ginRoutes {
		routeInfo := spec.RouteInfo{
			Method:      route.Method,
			Path:        route.Path,
			HandlerName: g.extractHandlerName(route),
			Handler:     route.HandlerFunc,
		}

		routes = append(routes, routeInfo)
	}

	return routes, nil
}

// extractHandlerName extracts handler name from Gin route info
func (g *GinRouteDiscoverer) extractHandlerName(route gin.RouteInfo) string {
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
						if runtimeFuncName != "" && !isGinGenericFuncSignature(runtimeFuncName) {
							cleanName := g.handlerNameExtractor.ParseHandlerNameFromFunction(runtimeFuncName)
							if cleanName != "" {
								return cleanName
							}
						}
					}
				}

				// Fallback to type string method
				if !isGinGenericFuncSignature(funcName) {
					// Parse the function name to extract just the method name
					cleanName := g.handlerNameExtractor.ParseHandlerNameFromFunction(funcName)
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

// GetFrameworkName returns the framework name
func (g *GinRouteDiscoverer) GetFrameworkName() string {
	return "Gin"
}

// isGinGenericFuncSignature checks if the function signature is generic
func isGinGenericFuncSignature(signature string) bool {
	// Check if it's a generic function signature like "func(*gin.Context)"
	return signature == "func(*gin.Context)" ||
		len(signature) < 10 // Too short to be meaningful
}

// GinServerAdapter adapts a Gin server to implement the HTTPServer interface
type GinServerAdapter struct {
	gin *gin.Engine
}

// NewGinServerAdapter creates a new adapter for Gin server
func NewGinServerAdapter(gin *gin.Engine) HTTPServer {
	return &GinServerAdapter{gin: gin}
}

// GET implements the HTTPServer interface by adapting to Gin
func (g *GinServerAdapter) GET(path string, handler HTTPHandler) {
	// Convert the generic HTTPHandler to a Gin HandlerFunc
	ginHandler := func(c *gin.Context) {
		// Create a response writer that adapts Gin Context to http.ResponseWriter
		rw := &ginResponseWriter{
			ctx:     c,
			headers: make(http.Header),
		}

		// Create a request from Gin Context
		req := c.Request

		// Call the generic handler
		handler(rw, req)
	}

	g.gin.GET(path, ginHandler)
}

// ginResponseWriter adapts Gin Context to http.ResponseWriter
type ginResponseWriter struct {
	ctx     *gin.Context
	headers http.Header
}

func (w *ginResponseWriter) Header() http.Header {
	return w.headers
}

func (w *ginResponseWriter) Write(data []byte) (int, error) {
	w.ctx.Writer.Write(data)
	return len(data), nil
}

func (w *ginResponseWriter) WriteHeader(statusCode int) {
	// Apply all stored headers to the Gin response
	for key, values := range w.headers {
		for _, value := range values {
			w.ctx.Writer.Header().Set(key, value)
		}
	}
	w.ctx.Writer.WriteHeader(statusCode)
}

// GinHandlerAnalyzer analyzes Gin handlers
type GinHandlerAnalyzer struct {
	handlerNameExtractor *common.HandlerNameExtractor
	astAnalyzer          *common.ASTAnalyzer
	typeResolver         *common.TypeResolver
	schemaAnalyzer       *common.SchemaAnalyzer
	sourceFilePath       string      // Path to the source file being analyzed
	config               interface{} // Configuration passed from library consumer
}

// NewGinHandlerAnalyzer creates a new Gin handler analyzer
func NewGinHandlerAnalyzer() *GinHandlerAnalyzer {
	return &GinHandlerAnalyzer{
		handlerNameExtractor: common.NewHandlerNameExtractor(),
		astAnalyzer:          common.NewASTAnalyzer(),
		typeResolver:         common.NewTypeResolver(),
		schemaAnalyzer:       common.NewSchemaAnalyzer(),
	}
}

// GetFrameworkName returns the framework name
func (g *GinHandlerAnalyzer) GetFrameworkName() string {
	return "Gin"
}

// GetSchemaGenerator returns the internal schema generator for testing
func (g *GinHandlerAnalyzer) GetSchemaGenerator() *analyzer.SchemaGenerator {
	return g.schemaAnalyzer.GetSchemaGenerator()
}

// SetConfig sets the configuration for the analyzer (implements HandlerAnalyzer interface)
func (g *GinHandlerAnalyzer) SetConfig(config interface{}) {
	g.config = config
}

// isProductionMode checks if running in production mode based on config
func (g *GinHandlerAnalyzer) isProductionMode() bool {
	if g.config != nil {
		// Try to assert as our Config type
		if cfg, ok := g.config.(interface{ IsProductionMode() bool }); ok {
			return cfg.IsProductionMode()
		}
	}
	return false
}

// isASTAnalysisEnabled checks if AST analysis should be performed
func (g *GinHandlerAnalyzer) isASTAnalysisEnabled() bool {
	if g.config != nil {
		// Try to assert as our Config type
		if cfg, ok := g.config.(interface{ IsASTAnalysisEnabled() bool }); ok {
			return cfg.IsASTAnalysisEnabled()
		}
	}
	return true // Default to enabled if no config
}

// ExtractTypes extracts request and response types from Gin handler function
func (g *GinHandlerAnalyzer) ExtractTypes(handler interface{}) (requestType, responseType reflect.Type, err error) {
	if handler == nil {
		return nil, nil, fmt.Errorf("handler is nil")
	}

	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("handler is not a function")
	}

	handlerType := handlerValue.Type()

	// Validate Gin handler signature: func(c *gin.Context)
	if err := g.validateGinSignature(handlerType); err != nil {
		return nil, nil, fmt.Errorf("invalid Gin handler signature: %w", err)
	}

	// Use AST analysis to examine the handler's body for ShouldBind calls
	reqType, respType := g.inferTypesFromContext(handlerValue)

	return reqType, respType, nil
}

// AnalyzeHandler analyzes handler and returns schemas with Docker-compatible fallbacks
func (g *GinHandlerAnalyzer) AnalyzeHandler(handler interface{}) analyzer.HandlerSchema {
	// First, try to analyze using reflection
	reqType, respType, err := g.ExtractTypes(handler)

	schema := analyzer.HandlerSchema{}

	if err == nil && (reqType != nil || respType != nil) {
		// Reflection analysis worked
		if reqType != nil {
			schema.RequestSchema = g.schemaAnalyzer.GetSchemaGenerator().GenerateSchemaFromType(reqType)
		}
		if respType != nil {
			schema.ResponseSchema = g.schemaAnalyzer.GetSchemaGenerator().GenerateSchemaFromType(respType)
		}
		return schema
	}

	// Second, try AST analysis (only if enabled and source files are available)
	if g.isASTAnalysisEnabled() && !g.isProductionMode() && g.areSourceFilesAvailable() {
		if astSchema := g.tryASTAnalysis(handler); astSchema.RequestSchema.Type != "" || astSchema.ResponseSchema.Type != "" {
			return astSchema
		}
	}

	// Final fallback: Generate generic schemas for Docker/production environments
	return g.schemaAnalyzer.GenerateFallbackSchemas()
}

// areSourceFilesAvailable checks if Go source files are available (not in Docker/production)
func (g *GinHandlerAnalyzer) areSourceFilesAvailable() bool {
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
func (g *GinHandlerAnalyzer) tryASTAnalysis(handler interface{}) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	handlerValue := reflect.ValueOf(handler)
	if !handlerValue.IsValid() {
		return schema
	}

	handlerType := handlerValue.Type()
	handlerName := handlerType.String()

	// Check if this is a wrapped Gin handler
	if handlerName == "gin.HandlerFunc" {
		// Try to get the original handler name from runtime info
		if originalHandlerName := g.handlerNameExtractor.GetOriginalHandlerName(handlerValue); originalHandlerName != "" {
			// Get the full name for source file resolution
			pc := handlerValue.Pointer()
			var fullName string
			if pc != 0 {
				if fn := runtime.FuncForPC(pc); fn != nil {
					fullName = fn.Name()
				}
			}
			// Try to find the handler file and analyze it using AST
			if sourceFile := g.astAnalyzer.FindHandlerSourceFile(fullName); sourceFile != "" {
				return g.astAnalyzer.AnalyzeHandlerWithAST(sourceFile, originalHandlerName, "gin")
			}
		}
	}

	return schema
}

// isShouldBindCall checks if the call expression is a Gin ShouldBind call
func (g *GinHandlerAnalyzer) isShouldBindCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		// Support Gin binding patterns
		bindMethods := []string{
			"ShouldBind",
			"ShouldBindJSON",
			"ShouldBindXML",
			"ShouldBindQuery",
			"ShouldBindUri",
			"ShouldBindHeader",
			"ShouldBindYAML",
			"ShouldBindTOML",
			"Bind",
			"BindJSON",
			"BindXML",
			"BindQuery",
			"BindUri",
			"BindHeader",
			"BindYAML",
			"BindTOML",
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

// isJSONCall checks if the call expression is a JSON response call
func (g *GinHandlerAnalyzer) isJSONCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		// Support Gin JSON response patterns
		jsonMethods := []string{
			"JSON",
			"IndentedJSON",
			"SecureJSON",
			"JSONP",
			"PureJSON",
		}

		methodName := selExpr.Sel.Name
		for _, jsonMethod := range jsonMethods {
			if methodName == jsonMethod {
				return true
			}
		}
	}

	return false
}

// resolveTypeFromExpr attempts to resolve the type from an expression
func (g *GinHandlerAnalyzer) resolveTypeFromExpr(expr ast.Expr, packageName string) reflect.Type {
	// This is a simplified implementation - in practice you'd want more complete type resolution
	return nil
}

// validateGinSignature validates that the function has a Gin handler signature
func (g *GinHandlerAnalyzer) validateGinSignature(handlerType reflect.Type) error {
	// Expected: func(c *gin.Context)
	if handlerType.NumIn() != 1 {
		return fmt.Errorf("expected 1 parameter, got %d", handlerType.NumIn())
	}

	if handlerType.NumOut() != 0 {
		return fmt.Errorf("expected no return values, got %d", handlerType.NumOut())
	}

	// Check parameter: *gin.Context
	firstParam := handlerType.In(0)
	if !g.isGinContextType(firstParam) {
		return fmt.Errorf("parameter should be *gin.Context, got %s", firstParam)
	}

	return nil
}

// isGinContextType checks if type is *gin.Context
func (g *GinHandlerAnalyzer) isGinContextType(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr &&
		strings.Contains(t.String(), "gin.Context")
}

// inferTypesFromContext attempts to infer types from handler context by parsing AST
func (g *GinHandlerAnalyzer) inferTypesFromContext(handlerValue reflect.Value) (requestType, responseType reflect.Type) {
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

	g.sourceFilePath = fileName // Store for later use in type resolution

	// Parse the source file
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		return nil, nil
	}

	// Parse imports to populate the dynamic type registry
	g.astAnalyzer.GetTypeRegistry().ParseImports(src)

	// Find the function declaration
	funcName := funcForPC.Name()
	funcDecl := g.findFunctionDecl(src, funcName)
	if funcDecl == nil {
		return nil, nil
	}

	// Extract types from the function body using dynamic registry
	reqType := g.extractRequestType(funcDecl, src.Name.Name)
	respType := g.extractResponseType(funcDecl, src.Name.Name)

	return reqType, respType
}

// findFunctionDecl finds the function declaration by name
func (g *GinHandlerAnalyzer) findFunctionDecl(file *ast.File, funcName string) *ast.FuncDecl {
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

// extractRequestType analyzes ShouldBind calls to determine request type
func (g *GinHandlerAnalyzer) extractRequestType(funcDecl *ast.FuncDecl, packageName string) reflect.Type {
	var requestType reflect.Type

	// Walk through the function body looking for ShouldBind calls
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if g.isShouldBindCall(callExpr) {
				// Extract the type from the address-of expression
				if len(callExpr.Args) > 0 {
					if unaryExpr, ok := callExpr.Args[0].(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
						if ident, ok := unaryExpr.X.(*ast.Ident); ok {
							// Try to resolve the type from variable declarations
							resolvedType := g.resolveTypeFromIdent(ident, funcDecl, packageName)
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
func (g *GinHandlerAnalyzer) extractResponseType(funcDecl *ast.FuncDecl, packageName string) reflect.Type {
	var responseType reflect.Type

	// Walk through the function body looking for JSON calls
	ast.Inspect(funcDecl, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if g.isJSONCall(callExpr) {
				// Extract the type from the second argument (response data)
				if len(callExpr.Args) >= 2 {
					resolvedType := g.resolveTypeFromExpr(callExpr.Args[1], packageName)
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

// resolveTypeFromIdent attempts to resolve the type of an identifier from variable declarations
func (g *GinHandlerAnalyzer) resolveTypeFromIdent(ident *ast.Ident, funcDecl *ast.FuncDecl, packageName string) reflect.Type {
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
								foundType = g.typeResolver.ResolveTypeFromAST(valueSpec.Type, packageName)
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
							foundType = g.resolveTypeFromExpr(assignStmt.Rhs[i], packageName)
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

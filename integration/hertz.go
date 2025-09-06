package integration

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
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
				if !isGenericFuncSignature(funcName) {
					return funcName
				}
			}
		}
	}

	// Fallback: generate handler name based on path and method using pure algorithm
	parser := openapiParser.NewPathParser()
	return parser.GenerateHandlerName(route.Method, route.Path)
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
	schemaGen    *analyzer.SchemaGenerator
	typeRegistry *analyzer.DynamicTypeRegistry
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

// AnalyzeHandler analyzes handler and returns schemas
func (h *HertzHandlerAnalyzer) AnalyzeHandler(handler interface{}) analyzer.HandlerSchema {
	reqType, respType, err := h.ExtractTypes(handler)

	schema := analyzer.HandlerSchema{}

	if err != nil {
		// Return empty schemas if analysis fails
		return schema
	}

	// Generate schemas from types
	if reqType != nil {
		schema.RequestSchema = h.schemaGen.GenerateSchemaFromType(reqType)
	}

	if respType != nil {
		schema.ResponseSchema = h.schemaGen.GenerateSchemaFromType(respType)
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

	// Parse the source file
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		return nil, nil
	}

	// Parse imports to populate the dynamic type registry
	h.typeRegistry.ParseImports(src)

	// Find the function declaration
	funcDecl := h.findFunctionDecl(src, funcForPC.Name())
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
							requestType = h.resolveTypeFromIdent(ident, funcDecl)
							return false // Stop walking once we find it
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
					responseType = h.resolveTypeFromExpr(callExpr.Args[1])
					if responseType != nil {
						return false // Stop walking once we find a concrete type
					}
				}
			}
		}
		return true
	})

	return responseType
}

// isBindAndValidateCall checks if the call expression is a BindAndValidate call
func (h *HertzHandlerAnalyzer) isBindAndValidateCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		return selExpr.Sel.Name == "BindAndValidate"
	}
	return false
}

// isJSONCall checks if the call expression is a JSON call
func (h *HertzHandlerAnalyzer) isJSONCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		return selExpr.Sel.Name == "JSON"
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
		// Also check for short variable declarations
		if assignStmt, ok := n.(*ast.AssignStmt); ok && assignStmt.Tok == token.DEFINE {
			for i, lhs := range assignStmt.Lhs {
				if lhsIdent, ok := lhs.(*ast.Ident); ok && lhsIdent.Name == ident.Name {
					if i < len(assignStmt.Rhs) {
						foundType = h.resolveTypeFromExpr(assignStmt.Rhs[i])
						return false
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
		// Handle struct literals like dto.RegisterUserRequest{}
		if selExpr, ok := e.Type.(*ast.SelectorExpr); ok {
			return h.resolveTypeFromSelector(selExpr)
		}
		if ident, ok := e.Type.(*ast.Ident); ok {
			return h.resolveLocalType(ident.Name)
		}
	case *ast.Ident:
		// Handle identifiers
		return h.resolveLocalType(e.Name)
	case *ast.SelectorExpr:
		// Handle package.Type expressions
		return h.resolveTypeFromSelector(e)
	}
	return nil
}

// resolveTypeFromAST resolves type from AST type expression
func (h *HertzHandlerAnalyzer) resolveTypeFromAST(typeExpr ast.Expr) reflect.Type {
	switch t := typeExpr.(type) {
	case *ast.SelectorExpr:
		return h.resolveTypeFromSelector(t)
	case *ast.Ident:
		return h.resolveLocalType(t.Name)
	}
	return nil
}

// resolveTypeFromSelector attempts to resolve type from package.Type selector using dynamic registry
func (h *HertzHandlerAnalyzer) resolveTypeFromSelector(selExpr *ast.SelectorExpr) reflect.Type {
	if pkgIdent, ok := selExpr.X.(*ast.Ident); ok {
		packageAlias := pkgIdent.Name
		typeName := selExpr.Sel.Name

		// Use the dynamic type registry to resolve the type
		if reflectType := h.typeRegistry.GetType(packageAlias, typeName); reflectType != nil {
			return reflectType
		}

		// Try to load the package if not already loaded
		if packagePath := h.typeRegistry.GetPackagePath(packageAlias); packagePath != "" {
			if err := h.typeRegistry.LoadPackageTypes(packagePath); err == nil {
				// Try again after loading
				return h.typeRegistry.GetType(packageAlias, typeName)
			}
		}
	}
	return nil
}

// resolveLocalType attempts to resolve local types from the current package
func (h *HertzHandlerAnalyzer) resolveLocalType(typeName string) reflect.Type {
	// Try to resolve types from the current package scope
	// This is useful for types defined in the same package as the handler

	// Get the current calling context to determine the package
	pc, _, _, ok := runtime.Caller(2) // Go up 2 levels to get the original caller
	if !ok {
		return nil
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return nil
	}

	// Extract package path from function name
	fullName := fn.Name()
	lastSlash := strings.LastIndex(fullName, "/")
	lastDot := strings.LastIndex(fullName, ".")

	if lastSlash == -1 || lastDot == -1 || lastDot <= lastSlash {
		return nil
	}

	// Extract package path (everything before the last dot after the last slash)
	packagePath := fullName[:lastDot]

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
		return h.typeRegistry.GetType(simplePackage, typeName)
	}

	return nil
}

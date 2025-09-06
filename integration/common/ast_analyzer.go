package common

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/zainokta/openapi-gen/analyzer"
)

// ASTAnalyzer provides utilities for AST-based handler analysis
type ASTAnalyzer struct {
	typeRegistry *analyzer.DynamicTypeRegistry
	schemaGen    *analyzer.SchemaGenerator
}

// NewASTAnalyzer creates a new AST analyzer
func NewASTAnalyzer() *ASTAnalyzer {
	return &ASTAnalyzer{
		typeRegistry: analyzer.NewDynamicTypeRegistry(),
		schemaGen:    analyzer.NewSchemaGenerator(),
	}
}

// GetTypeRegistry returns the internal type registry
func (a *ASTAnalyzer) GetTypeRegistry() *analyzer.DynamicTypeRegistry {
	return a.typeRegistry
}

// FindHandlerSourceFile attempts to find the source file containing the handler for library usage
func (a *ASTAnalyzer) FindHandlerSourceFile(handlerFuncName string) string {
	// Extract package path from handler function name
	// Example: some-service/internal/interfaces/http/handlers.(*SomeHandler).Method-fm
	// -> some-service/internal/interfaces/http/handlers
	if !strings.Contains(handlerFuncName, ".") {
		return ""
	}

	// Extract the package path before the receiver or function
	pkgPath := a.ExtractPackagePathFromFunction(handlerFuncName)
	if pkgPath == "" {
		return ""
	}

	// Try to find the source file using multiple strategies for library usage
	return a.FindSourceFileInConsumerModule(pkgPath)
}

// ExtractPackagePathFromFunction extracts clean package path from function name
func (a *ASTAnalyzer) ExtractPackagePathFromFunction(handlerFuncName string) string {
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

// FindSourceFileInConsumerModule finds source files in the consuming application's module
func (a *ASTAnalyzer) FindSourceFileInConsumerModule(pkgPath string) string {
	// Get the consuming application's working directory
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Get the consuming application's module name
	consumerModule := a.GetCurrentModuleName()
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
	if sourceFile := a.FindGoFilesInDirectory(pkgDir); sourceFile != "" {
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
		if sourceFile := a.FindGoFilesInDirectory(pattern); sourceFile != "" {
			return sourceFile
		}
	}

	return ""
}

// GetCurrentModuleName gets the consuming application's Go module name dynamically
func (a *ASTAnalyzer) GetCurrentModuleName() string {
	// First, try to get the module name from runtime caller context
	// This helps identify the actual application using the library
	if moduleName := a.GetModuleFromRuntimeCaller(); moduleName != "" {
		return moduleName
	}

	// Fallback: Get current working directory (consumer's directory)
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Find the go.mod file in the consuming application
	if goModPath := a.FindGoModPath(wd); goModPath != "" {
		return a.GetModuleNameFromGoMod(goModPath)
	}

	return ""
}

// GetModuleFromRuntimeCaller attempts to extract module name from runtime caller info
func (a *ASTAnalyzer) GetModuleFromRuntimeCaller() string {
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

		fullName := fn.Name()
		// Skip functions from our own package
		if strings.Contains(fullName, "github.com/openapi-gen/openapi-gen") {
			continue
		}

		// Extract module path from function name
		parts := strings.Split(fullName, "/")
		if len(parts) > 2 {
			// Reconstruct module path (e.g., github.com/user/module)
			return strings.Join(parts[:3], "/")
		}
	}

	return ""
}

// FindGoModPath finds the go.mod file path
func (a *ASTAnalyzer) FindGoModPath(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// GetModuleNameFromGoMod extracts module name from go.mod file
func (a *ASTAnalyzer) GetModuleNameFromGoMod(goModPath string) string {
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

// FindGoFilesInDirectory looks for Go source files in a directory
func (a *ASTAnalyzer) FindGoFilesInDirectory(dir string) string {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return ""
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".go") {
			return filepath.Join(dir, file.Name())
		}
	}

	return ""
}

// AnalyzeHandlerWithAST analyzes a handler using AST parsing with error handling
func (a *ASTAnalyzer) AnalyzeHandlerWithAST(sourceFile string, methodName string, frameworkType string) analyzer.HandlerSchema {
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
	a.typeRegistry.ParseImports(src)

	// Find the handler method
	var methodDecl *ast.FuncDecl
	for _, decl := range src.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == methodName {
			methodDecl = fn
			break
		}
	}

	if methodDecl == nil {
		return schema
	}

	// Extract request and response types based on framework
	switch frameworkType {
	case string(FrameworkHertz):
		return a.ExtractHertzHandlerTypes(methodDecl, sourceFile)
	case string(FrameworkGin):
		return a.ExtractGinHandlerTypes(methodDecl, sourceFile)
	}

	return schema
}

// ExtractHertzHandlerTypes extracts request/response types from Hertz handler
func (a *ASTAnalyzer) ExtractHertzHandlerTypes(methodDecl *ast.FuncDecl, sourceFile string) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	// Look for BindAndValidate calls to extract request type
	if reqType := a.ExtractHertzRequestType(methodDecl); reqType != nil {
		schema.RequestSchema = a.schemaGen.GenerateSchemaFromType(reqType)
	}

	// Look for JSON calls to extract response type
	if respType := a.ExtractHertzResponseType(methodDecl); respType != nil {
		schema.ResponseSchema = a.schemaGen.GenerateSchemaFromType(respType)
	}

	return schema
}

// ExtractGinHandlerTypes extracts request/response types from Gin handler
func (a *ASTAnalyzer) ExtractGinHandlerTypes(methodDecl *ast.FuncDecl, sourceFile string) analyzer.HandlerSchema {
	schema := analyzer.HandlerSchema{}

	// Look for ShouldBind calls to extract request type
	if reqType := a.ExtractGinRequestType(methodDecl); reqType != nil {
		schema.RequestSchema = a.schemaGen.GenerateSchemaFromType(reqType)
	}

	// Look for JSON calls to extract response type
	if respType := a.ExtractGinResponseType(methodDecl); respType != nil {
		schema.ResponseSchema = a.schemaGen.GenerateSchemaFromType(respType)
	}

	return schema
}

// ExtractHertzRequestType extracts request type from Hertz handler AST
func (a *ASTAnalyzer) ExtractHertzRequestType(methodDecl *ast.FuncDecl) reflect.Type {
	// Look for BindAndValidate calls in the function body
	ast.Inspect(methodDecl.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if a.IsHertzBindCall(callExpr) {
				if reqType := a.ExtractTypeFromCallExpr(callExpr); reqType != nil {
					return false
				}
			}
		}
		return true
	})
	return nil
}

// ExtractHertzResponseType extracts response type from Hertz handler AST
func (a *ASTAnalyzer) ExtractHertzResponseType(methodDecl *ast.FuncDecl) reflect.Type {
	// Look for JSON calls in the function body
	ast.Inspect(methodDecl.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if a.IsHertzJSONCall(callExpr) {
				if respType := a.ExtractTypeFromCallExpr(callExpr); respType != nil {
					return false
				}
			}
		}
		return true
	})
	return nil
}

// ExtractGinRequestType extracts request type from Gin handler AST
func (a *ASTAnalyzer) ExtractGinRequestType(methodDecl *ast.FuncDecl) reflect.Type {
	// Look for ShouldBind calls in the function body
	ast.Inspect(methodDecl.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if a.IsGinBindCall(callExpr) {
				if reqType := a.ExtractTypeFromCallExpr(callExpr); reqType != nil {
					return false
				}
			}
		}
		return true
	})
	return nil
}

// ExtractGinResponseType extracts response type from Gin handler AST
func (a *ASTAnalyzer) ExtractGinResponseType(methodDecl *ast.FuncDecl) reflect.Type {
	// Look for JSON calls in the function body
	ast.Inspect(methodDecl.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if a.IsGinJSONCall(callExpr) {
				if respType := a.ExtractTypeFromCallExpr(callExpr); respType != nil {
					return false
				}
			}
		}
		return true
	})
	return nil
}

// IsHertzBindCall checks if a call expression is a Hertz BindAndValidate call
func (a *ASTAnalyzer) IsHertzBindCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		return selExpr.Sel.Name == "BindAndValidate"
	}
	return false
}

// IsHertzJSONCall checks if a call expression is a Hertz JSON call
func (a *ASTAnalyzer) IsHertzJSONCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		return selExpr.Sel.Name == "JSON"
	}
	return false
}

// IsGinBindCall checks if a call expression is a Gin ShouldBind call
func (a *ASTAnalyzer) IsGinBindCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		return selExpr.Sel.Name == "ShouldBind" || selExpr.Sel.Name == "ShouldBindJSON"
	}
	return false
}

// IsGinJSONCall checks if a call expression is a Gin JSON call
func (a *ASTAnalyzer) IsGinJSONCall(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		return selExpr.Sel.Name == "JSON"
	}
	return false
}

// ExtractTypeFromCallExpr extracts type information from a call expression
func (a *ASTAnalyzer) ExtractTypeFromCallExpr(callExpr *ast.CallExpr) reflect.Type {
	if len(callExpr.Args) == 0 {
		return nil
	}

	// Look for address-of operator (&) for struct types
	if unaryExpr, ok := callExpr.Args[0].(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
		if compositeLit, ok := unaryExpr.X.(*ast.CompositeLit); ok {
			return a.ExtractTypeFromCompositeLit(compositeLit)
		}
	}

	// Direct composite literal
	if compositeLit, ok := callExpr.Args[0].(*ast.CompositeLit); ok {
		return a.ExtractTypeFromCompositeLit(compositeLit)
	}

	return nil
}

// ExtractTypeFromCompositeLit extracts type from composite literal
func (a *ASTAnalyzer) ExtractTypeFromCompositeLit(compositeLit *ast.CompositeLit) reflect.Type {
	switch typeExpr := compositeLit.Type.(type) {
	case *ast.Ident:
		// Simple type name
		return a.typeRegistry.GetType("", typeExpr.Name)
	case *ast.SelectorExpr:
		// Qualified type like pkg.Type
		if pkgIdent, ok := typeExpr.X.(*ast.Ident); ok {
			return a.typeRegistry.GetType(pkgIdent.Name, typeExpr.Sel.Name)
		}
	}
	return nil
}

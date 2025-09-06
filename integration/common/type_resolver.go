package common

import (
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/zainokta/openapi-gen/analyzer"
)

// TypeResolver provides utilities for resolving Go types from various sources
type TypeResolver struct {
	typeRegistry *analyzer.DynamicTypeRegistry
	fileUtils    *FileSystemUtilities
}

// NewTypeResolver creates a new TypeResolver
func NewTypeResolver() *TypeResolver {
	return &TypeResolver{
		typeRegistry: analyzer.NewDynamicTypeRegistry(),
		fileUtils:    NewFileSystemUtilities(),
	}
}

// GetTypeRegistry returns the internal type registry
func (tr *TypeResolver) GetTypeRegistry() *analyzer.DynamicTypeRegistry {
	return tr.typeRegistry
}

// ResolveTypeFromPackage resolves a type from a package path
func (tr *TypeResolver) ResolveTypeFromPackage(packagePath, typeName string) reflect.Type {
	// Try to load the package by full path
	if err := tr.typeRegistry.LoadPackageTypes(packagePath); err != nil {
		// If we can't load by full path, try just the package name
		parts := strings.Split(packagePath, "/")
		if len(parts) > 0 {
			simplePackage := parts[len(parts)-1]
			if simpleType := tr.typeRegistry.GetType(simplePackage, typeName); simpleType != nil {
				return simpleType
			}
		}
		return nil
	}

	// Look up the type in the loaded package
	// Try both full path and simple package name
	if fullType := tr.typeRegistry.GetType(packagePath, typeName); fullType != nil {
		return fullType
	}

	// Also try with just the package name as alias
	parts := strings.Split(packagePath, "/")
	if len(parts) > 0 {
		simplePackage := parts[len(parts)-1]
		if simpleType := tr.typeRegistry.GetType(simplePackage, typeName); simpleType != nil {
			return simpleType
		}
	}

	return nil
}

// ResolveTypeFromAST resolves type from AST type expression
func (tr *TypeResolver) ResolveTypeFromAST(typeExpr ast.Expr, currentPackage string) reflect.Type {
	switch expr := typeExpr.(type) {
	case *ast.Ident:
		// Simple type name
		return tr.typeRegistry.GetType(currentPackage, expr.Name)

	case *ast.SelectorExpr:
		// Qualified type like pkg.Type
		if pkgIdent, ok := expr.X.(*ast.Ident); ok {
			// Try to resolve the package alias
			if pkgPath := tr.ResolvePackageAlias(pkgIdent.Name, currentPackage); pkgPath != "" {
				return tr.typeRegistry.GetType(pkgPath, expr.Sel.Name)
			}
			// Fallback: try with the alias as package name
			return tr.typeRegistry.GetType(pkgIdent.Name, expr.Sel.Name)
		}

	case *ast.StarExpr:
		// Pointer type (*Type)
		if baseType := tr.ResolveTypeFromAST(expr.X, currentPackage); baseType != nil {
			return reflect.PointerTo(baseType)
		}

	case *ast.ArrayType:
		// Array or slice type
		if elemType := tr.ResolveTypeFromAST(expr.Elt, currentPackage); elemType != nil {
			if expr.Len != nil {
				// Array type
				return reflect.ArrayOf(10, elemType) // Default length, can be enhanced
			}
			// Slice type
			return reflect.SliceOf(elemType)
		}

	case *ast.MapType:
		// Map type
		if keyType := tr.ResolveTypeFromAST(expr.Key, currentPackage); keyType != nil {
			if valueType := tr.ResolveTypeFromAST(expr.Value, currentPackage); valueType != nil {
				return reflect.MapOf(keyType, valueType)
			}
		}

	case *ast.InterfaceType:
		// Interface type - return interface{}
		return reflect.TypeOf((*interface{})(nil)).Elem()

	case *ast.StructType:
		// Anonymous struct type
		return reflect.TypeOf(struct{}{})
	}

	return nil
}

// ResolvePackageAlias resolves a package alias to its full path
func (tr *TypeResolver) ResolvePackageAlias(alias, currentPackage string) string {
	// This is a simplified implementation
	// In a full implementation, we would track import aliases from AST

	// Try some common patterns
	if alias == "ctx" || alias == "context" {
		return "context"
	}

	// Try to find the package in the current module
	wd, _ := os.Getwd()
	if wd != "" {
		// Try to find a package with this name
		pkgPath := tr.FindPackagePathByName(alias, wd)
		if pkgPath != "" {
			return pkgPath
		}
	}

	return ""
}

// FindPackagePathByName finds a package path by its name
func (tr *TypeResolver) FindPackagePathByName(packageName, baseDir string) string {
	// Try common package locations
	patterns := []string{
		filepath.Join(baseDir, packageName),
		filepath.Join(baseDir, "internal", packageName),
		filepath.Join(baseDir, "pkg", packageName),
		filepath.Join(baseDir, "handlers"),
		filepath.Join(baseDir, "api"),
		filepath.Join(baseDir, "internal", "api"),
		filepath.Join(baseDir, "internal", "handlers"),
	}

	for _, pattern := range patterns {
		if tr.fileUtils.IsDirectory(pattern) && tr.fileUtils.HasGoFiles(pattern) {
			// Convert file path to package path
			return tr.ConvertFilePathToPackagePath(pattern, baseDir)
		}
	}

	return ""
}

// ConvertFilePathToPackagePath converts a file path to a Go package path
func (tr *TypeResolver) ConvertFilePathToPackagePath(filePath, baseDir string) string {
	// Get the module name
	goModPath := tr.fileUtils.FindGoModPath(baseDir)
	if goModPath == "" {
		return ""
	}

	moduleName := tr.fileUtils.GetModuleNameFromGoMod(goModPath)
	if moduleName == "" {
		return ""
	}

	// Convert relative path to package path
	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return ""
	}

	// Convert to forward slashes and combine with module name
	pkgPath := filepath.ToSlash(relPath)
	return moduleName + "/" + pkgPath
}

// ExtractTypeFromFunction extracts return types from a function declaration
func (tr *TypeResolver) ExtractTypeFromFunction(funcDecl *ast.FuncDecl, currentPackage string) []reflect.Type {
	if funcDecl.Type.Results == nil {
		return nil
	}

	var types []reflect.Type
	for _, field := range funcDecl.Type.Results.List {
		if len(field.Names) == 0 {
			// Anonymous return value
			if typ := tr.ResolveTypeFromAST(field.Type, currentPackage); typ != nil {
				types = append(types, typ)
			}
		} else {
			// Named return values - create a type for each name
			for range field.Names {
				if typ := tr.ResolveTypeFromAST(field.Type, currentPackage); typ != nil {
					types = append(types, typ)
				}
			}
		}
	}

	return types
}

// ExtractParameterTypes extracts parameter types from a function declaration
func (tr *TypeResolver) ExtractParameterTypes(funcDecl *ast.FuncDecl, currentPackage string) []reflect.Type {
	if funcDecl.Type.Params == nil {
		return nil
	}

	var types []reflect.Type
	for _, field := range funcDecl.Type.Params.List {
		if len(field.Names) == 0 {
			// Anonymous parameter
			if typ := tr.ResolveTypeFromAST(field.Type, currentPackage); typ != nil {
				types = append(types, typ)
			}
		} else {
			// Named parameters - create a type for each name
			for range field.Names {
				if typ := tr.ResolveTypeFromAST(field.Type, currentPackage); typ != nil {
					types = append(types, typ)
				}
			}
		}
	}

	return types
}

// FindFunctionDecl finds a function declaration by name in an AST file
func (tr *TypeResolver) FindFunctionDecl(file *ast.File, funcName string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
			return fn
		}
	}
	return nil
}

// GetFunctionType gets the reflect.Type of a function by its name
func (tr *TypeResolver) GetFunctionType(funcName string) reflect.Type {
	// Try to get the function from runtime
	fn := runtime.FuncForPC(tr.FindFunctionPC(funcName))
	if fn == nil {
		return nil
	}

	// This is a simplified approach - in practice, we would need more
	// sophisticated type resolution for runtime functions
	return nil
}

// FindFunctionPC finds the program counter for a function by name
func (tr *TypeResolver) FindFunctionPC(funcName string) uintptr {
	// This is a placeholder implementation
	// In practice, we would need to maintain a mapping of function names to PCs
	return 0
}

// IsContextType checks if a type is a context type
func (tr *TypeResolver) IsContextType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}

	// Check if it's context.Context
	if typ.String() == "context.Context" {
		return true
	}

	// Check if it implements context.Context
	contextInterface := reflect.TypeOf((*interface{})(nil)).Elem()
	return typ.Implements(contextInterface)
}

// IsErrorType checks if a type is an error type
func (tr *TypeResolver) IsErrorType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}

	// Check if it's error interface
	errorInterface := reflect.TypeOf((*error)(nil)).Elem()
	return typ.Implements(errorInterface)
}

// GetElementType gets the element type of a slice, array, or pointer
func (tr *TypeResolver) GetElementType(typ reflect.Type) reflect.Type {
	if typ == nil {
		return nil
	}

	switch typ.Kind() {
	case reflect.Slice, reflect.Array:
		return typ.Elem()
	case reflect.Pointer:
		return typ.Elem()
	default:
		return nil
	}
}

// IsPointerType checks if a type is a pointer type
func (tr *TypeResolver) IsPointerType(typ reflect.Type) bool {
	return typ != nil && typ.Kind() == reflect.Ptr
}

// IsSliceType checks if a type is a slice type
func (tr *TypeResolver) IsSliceType(typ reflect.Type) bool {
	return typ != nil && typ.Kind() == reflect.Slice
}

// IsMapType checks if a type is a map type
func (tr *TypeResolver) IsMapType(typ reflect.Type) bool {
	return typ != nil && typ.Kind() == reflect.Map
}

// GetUnderlyingType gets the underlying type (dereferences pointers)
func (tr *TypeResolver) GetUnderlyingType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

// GetTypeString returns a string representation of a type
func (tr *TypeResolver) GetTypeString(typ reflect.Type) string {
	if typ == nil {
		return "nil"
	}
	return typ.String()
}

// AreTypesEqual checks if two types are equal
func (tr *TypeResolver) AreTypesEqual(t1, t2 reflect.Type) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	return t1 == t2
}

// IsAssignable checks if a type can be assigned to another
func (tr *TypeResolver) IsAssignable(from, to reflect.Type) bool {
	if from == nil || to == nil {
		return false
	}
	return from.AssignableTo(to)
}

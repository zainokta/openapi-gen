package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strings"
	"sync"

	"github.com/zainokta/openapi-gen/spec"

	"golang.org/x/tools/go/packages"
)

// HandlerAnalyzer analyzes handler functions to extract request/response types
type HandlerAnalyzer interface {
	ExtractTypes(handler interface{}) (requestType, responseType reflect.Type, err error)
	AnalyzeHandler(handler interface{}) HandlerSchema
	GetFrameworkName() string
}

// DynamicTypeRegistry manages automatic type discovery from any imported package
type DynamicTypeRegistry struct {
	mu          sync.RWMutex
	typeCache   map[string]map[string]reflect.Type // packagePath -> typeName -> reflect.Type
	importCache map[string]string                  // alias -> full package path
	packageObjs map[string]*types.Package          // cache loaded packages
}

// NewDynamicTypeRegistry creates a new dynamic type registry
func NewDynamicTypeRegistry() *DynamicTypeRegistry {
	return &DynamicTypeRegistry{
		typeCache:   make(map[string]map[string]reflect.Type),
		importCache: make(map[string]string),
		packageObjs: make(map[string]*types.Package),
	}
}

// ParseImports analyzes import statements from an AST file
func (dtr *DynamicTypeRegistry) ParseImports(file *ast.File) {
	dtr.mu.Lock()
	defer dtr.mu.Unlock()

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		alias := ""

		if imp.Name != nil {
			// Explicit alias: import alias "package"
			alias = imp.Name.Name
		} else {
			// Default alias: extract from path
			parts := strings.Split(path, "/")
			alias = parts[len(parts)-1]
		}

		// Skip blank imports and dot imports
		if alias != "_" && alias != "." {
			dtr.importCache[alias] = path
		}
	}
}

// LoadPackageTypes loads and caches all types from a package
func (dtr *DynamicTypeRegistry) LoadPackageTypes(packagePath string) error {
	dtr.mu.Lock()
	defer dtr.mu.Unlock()

	// Check if already loaded
	if _, exists := dtr.typeCache[packagePath]; exists {
		return nil
	}

	// Load package using go/packages
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, packagePath)
	if err != nil || len(pkgs) == 0 {
		return fmt.Errorf("failed to load package %s: %w", packagePath, err)
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return fmt.Errorf("package %s has errors: %v", packagePath, pkg.Errors)
	}

	dtr.packageObjs[packagePath] = pkg.Types
	dtr.typeCache[packagePath] = make(map[string]reflect.Type)

	// Walk through all defined types in the package
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)

		// Only process exported type names (structs, interfaces, etc.)
		if obj.Exported() {
			if typeName, ok := obj.(*types.TypeName); ok {
				// Convert go/types.Type to reflect.Type
				if reflectType := dtr.convertToReflectType(typeName.Type()); reflectType != nil {
					dtr.typeCache[packagePath][name] = reflectType
				}
			}
		}
	}

	return nil
}

// convertToReflectType converts a go/types.Type to reflect.Type
func (dtr *DynamicTypeRegistry) convertToReflectType(t types.Type) reflect.Type {
	// This is complex because go/types.Type and reflect.Type are different systems
	// We'll handle the most common cases that appear in handler analysis

	switch underlying := t.Underlying().(type) {
	case *types.Struct:
		// For struct types, try to match by name
		typeName := t.String()
		return dtr.tryResolveByName(typeName)

	case *types.Interface:
		// Handle interface types
		if underlying.Empty() {
			// This is interface{} or any
			return nil // We can't resolve empty interfaces meaningfully
		}
		// For named interfaces, try to resolve by name
		typeName := t.String()
		return dtr.tryResolveByName(typeName)

	case *types.Basic:
		// Handle basic types (string, int, bool, etc.)
		return dtr.convertBasicType(underlying)

	case *types.Slice:
		// Handle slice types
		elemType := dtr.convertToReflectType(underlying.Elem())
		if elemType != nil {
			return reflect.SliceOf(elemType)
		}
		return nil

	case *types.Array:
		// Handle array types
		elemType := dtr.convertToReflectType(underlying.Elem())
		if elemType != nil {
			return reflect.ArrayOf(int(underlying.Len()), elemType)
		}
		return nil

	case *types.Pointer:
		// Handle pointer types
		elemType := dtr.convertToReflectType(underlying.Elem())
		if elemType != nil {
			return reflect.PointerTo(elemType)
		}
		return nil

	case *types.Map:
		// Handle map types
		keyType := dtr.convertToReflectType(underlying.Key())
		valueType := dtr.convertToReflectType(underlying.Elem())
		if keyType != nil && valueType != nil {
			return reflect.MapOf(keyType, valueType)
		}
		return nil

	default:
		// For other types, try name-based resolution as fallback
		typeName := t.String()
		return dtr.tryResolveByName(typeName)
	}
}

// convertBasicType converts basic Go types to reflect.Type
func (dtr *DynamicTypeRegistry) convertBasicType(basic *types.Basic) reflect.Type {
	switch basic.Kind() {
	case types.Bool:
		return reflect.TypeOf(false)
	case types.Int:
		return reflect.TypeOf(int(0))
	case types.Int8:
		return reflect.TypeOf(int8(0))
	case types.Int16:
		return reflect.TypeOf(int16(0))
	case types.Int32:
		return reflect.TypeOf(int32(0))
	case types.Int64:
		return reflect.TypeOf(int64(0))
	case types.Uint:
		return reflect.TypeOf(uint(0))
	case types.Uint8:
		return reflect.TypeOf(uint8(0))
	case types.Uint16:
		return reflect.TypeOf(uint16(0))
	case types.Uint32:
		return reflect.TypeOf(uint32(0))
	case types.Uint64:
		return reflect.TypeOf(uint64(0))
	case types.Float32:
		return reflect.TypeOf(float32(0))
	case types.Float64:
		return reflect.TypeOf(float64(0))
	case types.Complex64:
		return reflect.TypeOf(complex64(0))
	case types.Complex128:
		return reflect.TypeOf(complex128(0))
	case types.String:
		return reflect.TypeOf("")
	case types.UnsafePointer:
		return reflect.TypeOf((*int)(nil)).Elem() // Simplified representation
	default:
		return nil
	}
}

// tryResolveByName attempts to resolve a type by its string representation
func (dtr *DynamicTypeRegistry) tryResolveByName(typeName string) reflect.Type {
	// Extract the simple type name from the full package path
	parts := strings.Split(typeName, ".")
	if len(parts) < 2 {
		return nil
	}

	packageName := parts[len(parts)-2]
	simpleTypeName := parts[len(parts)-1]

	// Try to find this type in our loaded packages
	for pkgPath, typeMap := range dtr.typeCache {
		// Check if this package matches the type's package
		if strings.HasSuffix(pkgPath, "/"+packageName) || strings.HasSuffix(pkgPath, packageName) {
			if reflectType, exists := typeMap[simpleTypeName]; exists {
				return reflectType
			}
		}
	}

	return nil
}

// GetType retrieves a type by package alias and type name
func (dtr *DynamicTypeRegistry) GetType(packageAlias, typeName string) reflect.Type {
	dtr.mu.RLock()
	defer dtr.mu.RUnlock()

	// Resolve package path from alias
	packagePath, exists := dtr.importCache[packageAlias]
	if !exists {
		return nil
	}

	// Ensure package is loaded
	if _, loaded := dtr.typeCache[packagePath]; !loaded {
		// Unlock to avoid deadlock, then load
		dtr.mu.RUnlock()
		err := dtr.LoadPackageTypes(packagePath)
		dtr.mu.RLock()
		if err != nil {
			return nil
		}
	}

	// Get the type
	if pkgTypes, exists := dtr.typeCache[packagePath]; exists {
		return pkgTypes[typeName]
	}

	return nil
}

// GetPackagePath returns the full package path for an alias
func (dtr *DynamicTypeRegistry) GetPackagePath(alias string) string {
	dtr.mu.RLock()
	defer dtr.mu.RUnlock()
	return dtr.importCache[alias]
}

// HandlerAnalysisResult contains the result of handler analysis
type HandlerAnalysisResult struct {
	RequestType    reflect.Type
	ResponseType   reflect.Type
	RequestSchema  spec.Schema
	ResponseSchema spec.Schema
	Error          error
}

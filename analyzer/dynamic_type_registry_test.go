package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDynamicTypeRegistry_ParseImports(t *testing.T) {
	// Sample Go code with imports
	src := `package handlers

import (
	"context"
	"auth-service/internal/interfaces/http/dto"
	"github.com/cloudwego/hertz/pkg/app"
	customDto "some/other/dto"
)

func TestHandler(ctx context.Context, c *app.RequestContext) {
	// Handler implementation
}`

	// Parse the source code
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	assert.NoError(t, err, "Failed to parse source")

	// Test the dynamic type registry
	registry := NewDynamicTypeRegistry()
	registry.ParseImports(file)

	// Verify imports were parsed correctly
	dtoPath := registry.GetPackagePath("dto")
	assert.Equal(t, "auth-service/internal/interfaces/http/dto", dtoPath, "DTO package path should be correctly parsed")

	customPath := registry.GetPackagePath("customDto")
	assert.Equal(t, "some/other/dto", customPath, "Custom DTO package path should be correctly parsed")

	// Verify standard library imports are handled
	contextPath := registry.GetPackagePath("context")
	assert.Equal(t, "context", contextPath, "Standard library package path should be correctly parsed")
}

func TestDynamicTypeRegistry_NewRegistry(t *testing.T) {
	registry := NewDynamicTypeRegistry()
	assert.NotNil(t, registry, "Registry should not be nil")

	// Test that it starts with empty caches
	assert.Empty(t, registry.GetPackagePath("nonexistent"), "Should return empty for non-existent package")
	assert.Nil(t, registry.GetType("pkg", "Type"), "Should return nil for non-existent type")
}

func TestNewSchemaRegistry(t *testing.T) {
	registry := NewSchemaRegistry()
	assert.NotNil(t, registry, "Schema registry should not be nil")
}

func TestNewSchemaGenerator(t *testing.T) {
	generator := NewSchemaGenerator()
	assert.NotNil(t, generator, "Schema generator should not be nil")
}

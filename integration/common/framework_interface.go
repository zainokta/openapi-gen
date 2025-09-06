package common

import (
	"reflect"
	"strings"
)

// FrameworkType represents the type of web framework being analyzed
type FrameworkType string

const (
	FrameworkHertz FrameworkType = "hertz"
	FrameworkGin   FrameworkType = "gin"
)

// FrameworkDetector detects the framework type from various sources
type FrameworkDetector struct{}

// NewFrameworkDetector creates a new FrameworkDetector
func NewFrameworkDetector() *FrameworkDetector {
	return &FrameworkDetector{}
}

// DetectFromType detects framework from a Go type
func (fd *FrameworkDetector) DetectFromType(typ reflect.Type) FrameworkType {
	if typ == nil {
		return ""
	}
	
	// Check for Hertz types
	if fd.IsHertzType(typ) {
		return FrameworkHertz
	}
	
	// Check for Gin types
	if fd.IsGinType(typ) {
		return FrameworkGin
	}
	
	return ""
}

// DetectFromVariable detects framework from a variable value
func (fd *FrameworkDetector) DetectFromVariable(value interface{}) FrameworkType {
	if value == nil {
		return ""
	}
	
	return fd.DetectFromType(reflect.TypeOf(value))
}

// DetectFromFunction detects framework from a function signature
func (fd *FrameworkDetector) DetectFromFunction(funcType reflect.Type) FrameworkType {
	if funcType == nil || funcType.Kind() != reflect.Func {
		return ""
	}
	
	// Check function signature patterns
	if fd.IsHertzFunctionSignature(funcType) {
		return FrameworkHertz
	}
	
	if fd.IsGinFunctionSignature(funcType) {
		return FrameworkGin
	}
	
	return ""
}

// IsHertzType checks if a type is from Hertz framework
func (fd *FrameworkDetector) IsHertzType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	
	typeStr := typ.String()
	
	// Check for Hertz specific types
	hertzTypes := []string{
		"github.com/cloudwego/hertz/pkg/app.RequestContext",
		"github.com/cloudwego/hertz/pkg/app.Context",
		"app.RequestContext",
		"app.Context",
	}
	
	for _, hertzType := range hertzTypes {
		if typeStr == hertzType || 
		   strings.Contains(typeStr, "hertz") ||
		   strings.Contains(typeStr, "app.RequestContext") ||
		   strings.Contains(typeStr, "app.Context") {
			return true
		}
	}
	
	return false
}

// IsGinType checks if a type is from Gin framework
func (fd *FrameworkDetector) IsGinType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	
	typeStr := typ.String()
	
	// Check for Gin specific types
	ginTypes := []string{
		"github.com/gin-gonic/gin.Context",
		"gin.Context",
	}
	
	for _, ginType := range ginTypes {
		if typeStr == ginType || 
		   strings.Contains(typeStr, "gin-gonic") ||
		   strings.Contains(typeStr, "gin.Context") {
			return true
		}
	}
	
	return false
}

// IsHertzFunctionSignature checks if a function signature matches Hertz pattern
func (fd *FrameworkDetector) IsHertzFunctionSignature(funcType reflect.Type) bool {
	if funcType == nil || funcType.Kind() != reflect.Func {
		return false
	}
	
	// Hertz handler signatures:
	// func(ctx context.Context, c *app.RequestContext)
	// func(c *app.RequestContext)
	
	if funcType.NumIn() < 1 || funcType.NumIn() > 2 {
		return false
	}
	
	// Check for app.RequestContext parameter
	for i := 0; i < funcType.NumIn(); i++ {
		param := funcType.In(i)
		if fd.IsHertzType(param) {
			return true
		}
	}
	
	return false
}

// IsGinFunctionSignature checks if a function signature matches Gin pattern
func (fd *FrameworkDetector) IsGinFunctionSignature(funcType reflect.Type) bool {
	if funcType == nil || funcType.Kind() != reflect.Func {
		return false
	}
	
	// Gin handler signature:
	// func(c *gin.Context)
	
	if funcType.NumIn() != 1 {
		return false
	}
	
	param := funcType.In(0)
	return fd.IsGinType(param)
}
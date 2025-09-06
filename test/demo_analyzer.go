package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/zainokta/openapi-gen/analyzer"
	"github.com/zainokta/openapi-gen/spec"
	"github.com/zainokta/openapi-gen/test/handlers"
	"github.com/zainokta/openapi-gen/test/models"
)

func main() {
	// Run the enhanced integration test
	if err := TestHandlerIntegration(); err != nil {
		fmt.Printf("❌ Test failed: %v\n", err)
	} else {
		fmt.Println("✅ All handler integration tests passed!")
	}
}

// HandlerInfo contains information about a handler method
type HandlerInfo struct {
	Name        string
	Method      string
	Path        string
	Description string
	ReqType     reflect.Type
	RespType    reflect.Type
}

// TestHandlerIntegration tests schema generation using actual handler implementations
func TestHandlerIntegration() error {
	registry := analyzer.NewSchemaRegistry()
	
	// Discover handler methods using reflection
	handlerInfos, err := discoverHandlerMethods()
	if err != nil {
		return fmt.Errorf("failed to discover handler methods: %w", err)
	}
	
	fmt.Printf("🔍 Discovered %d handler methods\n", len(handlerInfos))
	
	// Test each handler method
	for _, handlerInfo := range handlerInfos {
		fmt.Printf("🧪 Testing %s (%s %s)...\n", handlerInfo.Name, handlerInfo.Method, handlerInfo.Path)
		
		// Register handler types with metadata
		metadata := spec.RouteInfo{
			Method:      handlerInfo.Method,
			Path:        handlerInfo.Path,
			HandlerName: handlerInfo.Name,
			Description: handlerInfo.Description,
		}
		
		registry.RegisterHandlerTypesWithMetadata(
			handlerInfo.Method,
			handlerInfo.Path,
			handlerInfo.ReqType,
			handlerInfo.RespType,
			metadata,
		)
		
		// Test request schema generation
		if handlerInfo.ReqType != nil {
			reqSchema, exists := registry.GetRequestSchema(handlerInfo.Method, handlerInfo.Path)
			if !exists {
				return fmt.Errorf("request schema not found for %s %s", handlerInfo.Method, handlerInfo.Path)
			}
			
			if err := validateRequestSchema(handlerInfo, reqSchema); err != nil {
				return fmt.Errorf("%s request schema validation failed: %w", handlerInfo.Name, err)
			}
			
			fmt.Printf("  ✅ Request schema validated for %s\n", handlerInfo.Name)
		}
		
		// Test response schema generation
		respSchema, exists := registry.GetResponseSchema(handlerInfo.Method, handlerInfo.Path)
		if !exists {
			return fmt.Errorf("response schema not found for %s %s", handlerInfo.Method, handlerInfo.Path)
		}
		
		if err := validateResponseSchema(respSchema); err != nil {
			return fmt.Errorf("%s response schema validation failed: %w", handlerInfo.Name, err)
		}
		
		fmt.Printf("  ✅ Response schema validated for %s\n", handlerInfo.Name)
	}
	
	// Test complex nested structures
	fmt.Println("🧪 Testing complex nested structures...")
	if err := testComplexStructures(registry); err != nil {
		return fmt.Errorf("complex structures test failed: %w", err)
	}
	
	// Test error response schemas
	fmt.Println("🧪 Testing error response schemas...")
	if err := testErrorSchemas(registry); err != nil {
		return fmt.Errorf("error schemas test failed: %w", err)
	}
	
	// Test metadata extraction
	fmt.Println("🧪 Testing metadata extraction...")
	if err := testMetadataExtraction(registry); err != nil {
		return fmt.Errorf("metadata extraction test failed: %w", err)
	}
	
	// Test JSON serialization for all schemas
	fmt.Println("🧪 Testing JSON serialization...")
	if err := testJSONSerialization(registry); err != nil {
		return fmt.Errorf("JSON serialization test failed: %w", err)
	}
	
	return nil
}

// discoverHandlerMethods uses reflection to discover handler methods and their metadata
func discoverHandlerMethods() ([]HandlerInfo, error) {
	handlerType := reflect.TypeOf(&handlers.UserHandler{})
	
	var handlerInfos []HandlerInfo
	
	// Expected handler methods and their configurations
	expectedHandlers := map[string]struct {
		method      string
		path        string
		reqModel    any
		description string
	}{
		"CreateUser": {
			method:      "POST",
			path:        "/users",
			reqModel:    models.CreateUserRequest{},
			description: "handles user creation requests",
		},
		"GetUser": {
			method:      "GET",
			path:        "/users/{id}",
			reqModel:    nil, // No request body
			description: "handles retrieving a single user by ID",
		},
		"UpdateUser": {
			method:      "PUT",
			path:        "/users/{id}",
			reqModel:    models.UpdateUserRequest{},
			description: "handles updating an existing user",
		},
		"ListUsers": {
			method:      "GET",
			path:        "/users",
			reqModel:    nil, // No request body
			description: "handles retrieving a list of users with pagination",
		},
		"DeleteUser": {
			method:      "DELETE",
			path:        "/users/{id}",
			reqModel:    nil, // No request body
			description: "handles deleting a user",
		},
	}
	
	for methodName, config := range expectedHandlers {
		_, exists := handlerType.MethodByName(methodName)
		if !exists {
			return nil, fmt.Errorf("method %s not found in UserHandler", methodName)
		}
		
		var reqType, respType reflect.Type
		
		// Set request type
		if config.reqModel != nil {
			reqType = reflect.TypeOf(config.reqModel)
		}
		
		// All handlers return SuccessResponse
		respType = reflect.TypeOf(models.SuccessResponse{})
		
		handlerInfos = append(handlerInfos, HandlerInfo{
			Name:        methodName,
			Method:      config.method,
			Path:        config.path,
			Description: config.description,
			ReqType:     reqType,
			RespType:    respType,
		})
	}
	
	return handlerInfos, nil
}

// validateRequestSchema validates request schema based on handler type
func validateRequestSchema(handlerInfo HandlerInfo, schema spec.Schema) error {
	switch handlerInfo.Name {
	case "CreateUser":
		expectedFields := map[string]string{
			"name":     "string",
			"email":    "string",
			"password": "string",
			"age":      "integer",
			"phone":    "string",
		}
		return validateSchemaFields(schema, expectedFields, "request")
		
	case "UpdateUser":
		expectedFields := map[string]string{
			"name":  "string",
			"email": "string",
			"age":   "integer",
			"phone": "string",
		}
		return validateSchemaFields(schema, expectedFields, "request")
		
	case "GetUser", "ListUsers", "DeleteUser":
		// These methods should not have request schemas
		if len(schema.Properties) > 0 {
			return fmt.Errorf("unexpected request schema for %s method", handlerInfo.Name)
		}
		return nil
		
	default:
		return fmt.Errorf("unknown handler method: %s", handlerInfo.Name)
	}
}

// validateResponseSchema validates response schema based on handler type
func validateResponseSchema(schema spec.Schema) error {
	// All handlers return SuccessResponse with message and data fields
	expectedFields := map[string]string{
		"message": "string",
		"data":    "object",
	}
	
	return validateSchemaFields(schema, expectedFields, "response")
}

// validateSchemaFields validates that schema contains expected fields
func validateSchemaFields(schema spec.Schema, expectedFields map[string]string, schemaType string) error {
	if len(schema.Properties) == 0 && len(expectedFields) > 0 {
		return fmt.Errorf("schema has no properties but expected %d fields", len(expectedFields))
	}
	
	for fieldName, expectedType := range expectedFields {
		fieldSchema, exists := schema.Properties[fieldName]
		if !exists {
			return fmt.Errorf("missing expected field '%s' in %s schema", fieldName, schemaType)
		}
		
		if fieldSchema.Type != expectedType {
			return fmt.Errorf("field '%s' has type '%s' but expected '%s'", fieldName, fieldSchema.Type, expectedType)
		}
	}
	
	return nil
}

// testComplexStructures tests nested and complex model structures
func testComplexStructures(registry *analyzer.SchemaRegistry) error {
	// Test UserResponse (nested in SuccessResponse.Data)
	userRespType := reflect.TypeOf(models.UserResponse{})
	userSchema := registry.GenerateSchemaFromType(userRespType)
	
	expectedUserFields := map[string]string{
		"id":         "integer",
		"name":       "string",
		"email":      "string",
		"age":        "integer",
		"phone":      "string",
		"created_at": "string",
		"updated_at": "string",
	}
	
	if err := validateSchemaFields(userSchema, expectedUserFields, "UserResponse"); err != nil {
		return fmt.Errorf("UserResponse validation failed: %w", err)
	}
	
	// Test UserListResponse (array of UserResponse)
	userListType := reflect.TypeOf(models.UserListResponse{})
	userListSchema := registry.GenerateSchemaFromType(userListType)
	
	expectedListFields := map[string]string{
		"users": "array",
		"total": "integer",
		"page":  "integer",
		"size":  "integer",
	}
	
	if err := validateSchemaFields(userListSchema, expectedListFields, "UserListResponse"); err != nil {
		return fmt.Errorf("UserListResponse validation failed: %w", err)
	}
	
	// Test that users array contains UserResponse objects
	if usersSchema, exists := userListSchema.Properties["users"]; exists {
		if usersSchema.Type != "array" || usersSchema.Items == nil {
			return fmt.Errorf("users field should be an array with items")
		}
		
		// Check that array items have UserResponse fields
		itemSchema := *usersSchema.Items
		for fieldName, expectedType := range expectedUserFields {
			if fieldSchema, exists := itemSchema.Properties[fieldName]; !exists || fieldSchema.Type != expectedType {
				return fmt.Errorf("users array items should contain UserResponse field '%s' of type '%s'", fieldName, expectedType)
			}
		}
	}
	
	fmt.Println("  ✅ Complex nested structures validated")
	return nil
}

// testErrorSchemas tests error response model validation
func testErrorSchemas(registry *analyzer.SchemaRegistry) error {
	errorType := reflect.TypeOf(models.ErrorResponse{})
	errorSchema := registry.GenerateSchemaFromType(errorType)
	
	expectedErrorFields := map[string]string{
		"error":   "string",
		"details": "object",
		"code":    "integer",
	}
	
	if err := validateSchemaFields(errorSchema, expectedErrorFields, "ErrorResponse"); err != nil {
		return fmt.Errorf("ErrorResponse validation failed: %w", err)
	}
	
	fmt.Println("  ✅ Error response schema validated")
	return nil
}

// testMetadataExtraction tests route metadata functionality
func testMetadataExtraction(registry *analyzer.SchemaRegistry) error {
	// Test metadata retrieval for CreateUser endpoint
	metadata, exists := registry.GetRouteMetadata("POST", "/users")
	if !exists {
		return fmt.Errorf("metadata not found for POST /users")
	}
	
	if metadata.Method != "POST" {
		return fmt.Errorf("expected method 'POST', got '%s'", metadata.Method)
	}
	
	if metadata.Path != "/users" {
		return fmt.Errorf("expected path '/users', got '%s'", metadata.Path)
	}
	
	if metadata.HandlerName != "CreateUser" {
		return fmt.Errorf("expected handler name 'CreateUser', got '%s'", metadata.HandlerName)
	}
	
	// Test path parameter extraction
	getUserMetadata, exists := registry.GetRouteMetadata("GET", "/users/{id}")
	if !exists {
		return fmt.Errorf("metadata not found for GET /users/{id}")
	}
	
	if !strings.Contains(getUserMetadata.Path, "{id}") {
		return fmt.Errorf("path parameter not detected in GET /users/{id}")
	}
	
	fmt.Println("  ✅ Route metadata extraction validated")
	return nil
}

// testJSONSerialization tests JSON serialization of generated schemas
func testJSONSerialization(registry *analyzer.SchemaRegistry) error {
	// Test serialization of various schemas
	testTypes := []struct {
		name string
		typ  reflect.Type
	}{
		{"UserResponse", reflect.TypeOf(models.UserResponse{})},
		{"CreateUserRequest", reflect.TypeOf(models.CreateUserRequest{})},
		{"UpdateUserRequest", reflect.TypeOf(models.UpdateUserRequest{})},
		{"UserListResponse", reflect.TypeOf(models.UserListResponse{})},
		{"ErrorResponse", reflect.TypeOf(models.ErrorResponse{})},
		{"SuccessResponse", reflect.TypeOf(models.SuccessResponse{})},
	}
	
	totalSize := 0
	for _, testType := range testTypes {
		schema := registry.GenerateSchemaFromType(testType.typ)
		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal %s schema to JSON: %w", testType.name, err)
		}
		
		totalSize += len(schemaJSON)
		fmt.Printf("  ✅ %s schema serialized (%d bytes)\n", testType.name, len(schemaJSON))
	}
	
	fmt.Printf("  ✅ All schemas serialized successfully (total: %d bytes)\n", totalSize)
	return nil
}

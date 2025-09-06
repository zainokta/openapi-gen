package example

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/zainokta/openapi-gen/cmd/openapi-gen/example/dto"
)

// Dummy service implementations
type AuthService struct{}
type UserService struct{}

var authService = &AuthService{}
var userService = &UserService{}

// AuthService implementation
func (s *AuthService) Login(req *dto.LoginRequest) *dto.AuthResponse {
	// Dummy implementation - in real app, this would validate credentials
	return &dto.AuthResponse{
		AccessToken:  "dummy-access-token-" + req.Email,
		RefreshToken: "dummy-refresh-token-" + req.Email,
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	}
}

// UserService implementation
func (s *UserService) CreateUser(req *dto.CreateUserRequest) *dto.UserResponse {
	// Dummy implementation - in real app, this would create a user in database
	now := time.Now().Format(time.RFC3339)
	return &dto.UserResponse{
		ID:        "user-" + req.Email,
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (s *UserService) GetUser(userID string) *dto.UserResponse {
	// Dummy implementation - in real app, this would fetch user from database
	now := time.Now().Format(time.RFC3339)
	return &dto.UserResponse{
		ID:        userID,
		Name:      "John Doe",
		Email:     "john@example.com",
		Age:       30,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Example handlers with go:generate annotations

//go:generate openapi-gen -request dto.LoginRequest -response dto.AuthResponse -handler LoginHandler .
func LoginHandler(ctx context.Context, c *app.RequestContext) {
	var req dto.LoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(400, map[string]interface{}{"error": err.Error()})
		return
	}

	// Handler implementation
	resp := authService.Login(&req)
	c.JSON(200, resp)
}

//go:generate openapi-gen -request dto.CreateUserRequest -response dto.UserResponse -handler CreateUserHandler .
func CreateUserHandler(ctx context.Context, c *app.RequestContext) {
	var req dto.CreateUserRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(400, map[string]interface{}{"error": err.Error()})
		return
	}

	// Handler implementation
	user := userService.CreateUser(&req)
	c.JSON(201, user)
}

//go:generate openapi-gen -response dto.UserResponse -handler GetUserHandler .
func GetUserHandler(ctx context.Context, c *app.RequestContext) {
	userID := c.Param("id")

	// Handler implementation
	user := userService.GetUser(userID)
	c.JSON(200, user)
}

//go:generate openapi-gen -request dto.UpdateUserRequest -response dto.UserResponse -handler UpdateUserHandler .
func UpdateUserHandler(ctx context.Context, c *app.RequestContext) {
	userID := c.Param("id")
	var req dto.UpdateUserRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(400, map[string]interface{}{"error": err.Error()})
		return
	}

	// Handler implementation
	user := userService.GetUser(userID) // In real app, this would update the user
	user.Name = req.Name
	user.Email = req.Email
	user.Age = req.Age
	c.JSON(200, user)
}

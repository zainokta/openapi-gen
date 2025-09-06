package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/zainokta/openapi-gen/test/models"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	// In a real application, you would inject dependencies here:
	// userService services.UserService
	// logger      logger.Logger
}

// NewUserHandler creates a new UserHandler instance
func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

// CreateUser handles user creation requests
// POST /users
func (h *UserHandler) CreateUser(ctx context.Context, c *app.RequestContext) {
	var req models.CreateUserRequest
	
	// Bind and validate the request
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request data",
			Details: err.Error(),
			Code:    400,
		})
		return
	}

	// Additional business validation
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{
			Error:   "Validation failed",
			Details: err.Error(),
			Code:    422,
		})
		return
	}

	// Business logic - in a real app, this would call a service
	// user, err := h.userService.CreateUser(ctx, &req)
	
	// Simulate user creation for demo
	user := models.UserResponse{
		ID:        12345,
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		Phone:     req.Phone,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, models.SuccessResponse{
		Message: "User created successfully",
		Data:    user,
	})
}

// GetUser handles retrieving a single user by ID
// GET /users/:id
func (h *UserHandler) GetUser(ctx context.Context, c *app.RequestContext) {
	// Extract path parameter
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Details: "User ID must be a valid integer",
			Code:    400,
		})
		return
	}

	// Business logic - in a real app, this would call a service
	// user, err := h.userService.GetUser(ctx, userID)
	
	// Simulate user retrieval for demo
	if userID != 12345 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "User not found",
			Details: fmt.Sprintf("No user found with ID %d", userID),
			Code:    404,
		})
		return
	}

	user := models.UserResponse{
		ID:        userID,
		Name:      "John Doe",
		Email:     "john.doe@example.com",
		Age:       30,
		Phone:     "+1234567890",
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "User retrieved successfully",
		Data:    user,
	})
}

// UpdateUser handles updating an existing user
// PUT /users/:id
func (h *UserHandler) UpdateUser(ctx context.Context, c *app.RequestContext) {
	// Extract path parameter
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Details: "User ID must be a valid integer",
			Code:    400,
		})
		return
	}

	var req models.UpdateUserRequest
	
	// Bind and validate the request
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request data",
			Details: err.Error(),
			Code:    400,
		})
		return
	}

	// Additional business validation
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{
			Error:   "Validation failed",
			Details: err.Error(),
			Code:    422,
		})
		return
	}

	// Business logic - in a real app, this would call a service
	// user, err := h.userService.UpdateUser(ctx, userID, &req)
	
	// Simulate user update for demo
	if userID != 12345 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "User not found",
			Details: fmt.Sprintf("No user found with ID %d", userID),
			Code:    404,
		})
		return
	}

	user := models.UserResponse{
		ID:        userID,
		Name:      "Updated Name",
		Email:     "updated.email@example.com",
		Age:       35,
		Phone:     "+1987654321",
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "User updated successfully",
		Data:    user,
	})
}

// ListUsers handles retrieving a list of users with pagination
// GET /users
func (h *UserHandler) ListUsers(ctx context.Context, c *app.RequestContext) {
	// Parse query parameters for pagination
	page, _ := strconv.Atoi(c.Query("page"))
	if page == 0 {
		page = 1
	}
	size, _ := strconv.Atoi(c.Query("size"))
	if size == 0 {
		size = 10
	}
	
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	// Business logic - in a real app, this would call a service
	// users, total, err := h.userService.ListUsers(ctx, page, size)
	
	// Simulate user listing for demo
	users := []models.UserResponse{
		{
			ID:        12345,
			Name:      "John Doe",
			Email:     "john.doe@example.com",
			Age:       30,
			Phone:     "+1234567890",
			CreatedAt: "2024-01-01T00:00:00Z",
			UpdatedAt: "2024-01-01T00:00:00Z",
		},
		{
			ID:        12346,
			Name:      "Jane Smith",
			Email:     "jane.smith@example.com",
			Age:       28,
			Phone:     "+1987654321",
			CreatedAt: "2024-01-02T00:00:00Z",
			UpdatedAt: "2024-01-02T00:00:00Z",
		},
	}

	response := models.UserListResponse{
		Users: users,
		Total: 2,
		Page:  page,
		Size:  size,
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "Users retrieved successfully",
		Data:    response,
	})
}

// DeleteUser handles deleting a user
// DELETE /users/:id
func (h *UserHandler) DeleteUser(ctx context.Context, c *app.RequestContext) {
	// Extract path parameter
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid user ID",
			Details: "User ID must be a valid integer",
			Code:    400,
		})
		return
	}

	// Business logic - in a real app, this would call a service
	// err := h.userService.DeleteUser(ctx, userID)
	
	// Simulate user deletion for demo
	if userID != 12345 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "User not found",
			Details: fmt.Sprintf("No user found with ID %d", userID),
			Code:    404,
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Message: "User deleted successfully",
		Data:    nil,
	})
}
package models

import (
	"fmt"
)

// CreateUserRequest represents the request body for creating a new user
type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Age      int    `json:"age" validate:"min=18,max=120"`
	Phone    string `json:"phone,omitempty" validate:"e164"`
}

// UpdateUserRequest represents the request body for updating an existing user
type UpdateUserRequest struct {
	Name  string `json:"name,omitempty" validate:"min=2,max=100"`
	Email string `json:"email,omitempty" validate:"omitempty,email"`
	Age   *int   `json:"age,omitempty" validate:"min=18,max=120"`
	Phone string `json:"phone,omitempty" validate:"omitempty,e164"`
}

// UserResponse represents the response structure for user data
type UserResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Age       int    `json:"age,omitempty"`
	Phone     string `json:"phone,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UserListResponse represents the response structure for listing users
type UserListResponse struct {
	Users []UserResponse `json:"users"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string      `json:"error"`
	Details interface{} `json:"details,omitempty"`
	Code    int         `json:"code"`
}

// SuccessResponse represents a standard success response
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Validate validates the CreateUserRequest (simplified for demo)
func (r *CreateUserRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(r.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

// Validate validates the UpdateUserRequest (simplified for demo)
func (r *UpdateUserRequest) Validate() error {
	// For update, all fields are optional, but if provided, they should be valid
	return nil
}
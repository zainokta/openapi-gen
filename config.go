package openapi

import (
	"fmt"
)

// Config represents the configuration for the OpenAPI generator
type Config struct {
	Environment string  `json:"environment,omitempty"`
	ServerPort  int     `json:"server_port,omitempty"`
	ServerURL   string  `json:"server_url,omitempty"` // Optional override for server URL
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Version     string  `json:"version,omitempty"`
	Contact     Contact `json:"contact,omitempty"`

	// Schema directory configuration
	SchemaDir   string  `json:"schema_dir,omitempty"`         // Path to generated schema files
}


// Contact represents contact information for the API
type Contact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// NewConfig creates a new OpenAPI configuration with defaults
func NewConfig() *Config {
	return &Config{
		Environment: "development",
		ServerPort:  8080,
		Title:       "API Documentation",
		Description: "Automatically generated API documentation",
		Version:     "1.0.0",
		Contact: Contact{
			Name: "API Team",
		},
		// Default schema directory
		SchemaDir: "./schemas",
	}
}

// NewProductionConfig creates a configuration suitable for Docker/production environments
func NewProductionConfig() *Config {
	config := NewConfig()
	config.Environment = "production"
	return config
}

// NewDevelopmentConfig creates a configuration suitable for development
func NewDevelopmentConfig() *Config {
	config := NewConfig()
	config.Environment = "development"
	return config
}

// GetServerURL returns the server URL for the OpenAPI spec
func (c *Config) GetServerURL() string {
	if c.ServerURL != "" {
		return c.ServerURL
	}
	return fmt.Sprintf("http://localhost:%d", c.ServerPort)
}

// GetServerDescription returns the server description
func (c *Config) GetServerDescription() string {
	return fmt.Sprintf("%s environment", c.Environment)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ServerPort <= 0 {
		return fmt.Errorf("server port must be positive, got %d", c.ServerPort)
	}
	if c.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	if c.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	return nil
}

// SetSchemaDir sets the schema directory path
func (c *Config) SetSchemaDir(path string) *Config {
	c.SchemaDir = path
	return c
}

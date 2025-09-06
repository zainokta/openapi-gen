package openapi

import (
	"fmt"
	"log/slog"

	"github.com/zainokta/openapi-gen/integration"
)

// EnableDocs enables OpenAPI documentation with customization options
func EnableDocs(framework interface{}, h integration.HTTPServer, cfg *Config, logger *slog.Logger, customizers ...func(*Generator) error) error {
	// Create the OpenAPI generator
	generator, err := NewGenerator(cfg, logger, framework, h)
	if err != nil {
		return fmt.Errorf("failed to create OpenAPI generator: %w", err)
	}

	// Apply custom configuration
	for _, customizer := range customizers {
		if err := customizer(generator); err != nil {
			return fmt.Errorf("customization failed: %w", err)
		}
	}

	// Serve Swagger UI and OpenAPI spec
	if err := generator.ServeSwaggerUI(h); err != nil {
		return fmt.Errorf("failed to setup Swagger UI: %w", err)
	}

	logger.Info("OpenAPI documentation enabled with customization",
		"swagger_ui", "/docs",
		"openapi_spec", "/openapi.json")

	return nil
}

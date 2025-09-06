package openapi

import (
	"fmt"
	"log/slog"

	"github.com/zainokta/openapi-gen/integration"
	"github.com/zainokta/openapi-gen/logger"
)

// Option is a functional option for configuring OpenAPI generation
type Option func(*Options)

// Options holds configuration for OpenAPI generation
type Options struct {
	config           *Config
	logger           logger.Logger
	customDiscoverer integration.RouteDiscoverer
	customizers      []func(*Generator) error
}

// WithConfig sets a custom configuration for OpenAPI generation
//
// Example:
//
//	cfg := openapi.NewConfig()
//	cfg.Title = "My API"
//	cfg.Version = "2.0.0"
//	cfg.Description = "My awesome API"
//
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithConfig(cfg),
//	)
func WithConfig(cfg *Config) Option {
	return func(opts *Options) {
		opts.config = cfg
	}
}

// WithLogger sets a custom logger for OpenAPI generation
//
// Accepts any logger that implements the Logger interface, providing
// flexibility to integrate with any logging framework.
//
// Example with custom logger:
//
//	type MyLogger struct{}
//	func (l *MyLogger) Info(msg string, args ...any) { /* implementation */ }
//	// ... implement other methods
//	
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithLogger(&MyLogger{}),
//	)
//
// Example with slog adapter:
//
//	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	adapter := openapi.NewSlogAdapter(slogLogger)
//	
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithLogger(adapter),
//	)
func WithLogger(l logger.Logger) Option {
	return func(opts *Options) {
		opts.logger = l
	}
}

// WithSlogLogger is a convenience function for slog users
//
// This function automatically wraps the slog.Logger with SlogAdapter,
// making it easier for users already using slog.
//
// Example:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithSlogLogger(logger),
//	)
func WithSlogLogger(l *slog.Logger) Option {
	return func(opts *Options) {
		opts.logger = logger.NewSlogAdapter(l)
	}
}

// WithRouteDiscoverer sets a custom route discoverer for framework integration
//
// Example:
//
//	type MyFrameworkDiscoverer struct {
//		framework *MyFramework
//	}
//
//	func (d *MyFrameworkDiscoverer) DiscoverRoutes() ([]spec.RouteInfo, error) {
//		// Custom route discovery logic
//		return routes, nil
//	}
//
//	func (d *MyFrameworkDiscoverer) GetFrameworkName() string {
//		return "MyFramework"
//	}
//
//	discoverer := &MyFrameworkDiscoverer{framework: myFramework}
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithRouteDiscoverer(discoverer),
//	)
func WithRouteDiscoverer(discoverer integration.RouteDiscoverer) Option {
	return func(opts *Options) {
		opts.customDiscoverer = discoverer
	}
}

// WithCustomizer adds a customization function to modify the generated OpenAPI spec
//
// Example:
//
//	// Using predefined customizers
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithCustomizer(example.CustomizeAuthentication),
//		openapi.WithCustomizer(example.CustomizeMFA),
//	)
//
//	// Using inline customization
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithCustomizer(func(generator *openapi.Generator) error {
//			om := generator.GetOverrideManager()
//			om.Override("GET", "/custom/endpoint", openapi.RouteMetadata{
//				Tags:        "custom",
//				Summary:     "My Custom Endpoint",
//				Description: "Does something custom",
//			})
//			return nil
//		}),
//	)
func WithCustomizer(customizer func(*Generator) error) Option {
	return func(opts *Options) {
		opts.customizers = append(opts.customizers, customizer)
	}
}

// processOptions applies all provided options and sets defaults for missing values
func processOptions(opts ...Option) *Options {
	options := &Options{
		customizers: make([]func(*Generator) error, 0),
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(options)
	}

	// Set defaults for missing values
	if options.config == nil {
		options.config = NewConfig()
	}
	if options.logger == nil {
		options.logger = logger.NewSlogAdapter(slog.Default())
	}

	return options
}

// EnableDocs enables OpenAPI documentation with flexible configuration options
//
// Example usage:
//
//	// Minimal usage with defaults
//	err := openapi.EnableDocs(framework, httpServer)
//
//	// Custom configuration
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithConfig(myConfig),
//		openapi.WithLogger(myLogger),
//	)
//
//	// With customizations
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithCustomizer(example.CustomizeAuthentication),
//		openapi.WithCustomizer(example.CustomizeMFA),
//	)
//
//	// Combined usage
//	err := openapi.EnableDocs(framework, httpServer,
//		openapi.WithConfig(myConfig),
//		openapi.WithLogger(myLogger),
//		openapi.WithRouteDiscoverer(myDiscoverer),
//		openapi.WithCustomizer(example.CustomizeAuthentication),
//	)
func EnableDocs(framework any, h integration.HTTPServer, opts ...Option) error {
	// Process options to get customizers
	options := processOptions(opts...)

	// Create the OpenAPI generator
	generator, err := NewGenerator(framework, h, options)
	if err != nil {
		return fmt.Errorf("failed to create OpenAPI generator: %w", err)
	}

	// Apply custom configuration
	for _, customizer := range options.customizers {
		if err := customizer(generator); err != nil {
			return fmt.Errorf("customization failed: %w", err)
		}
	}

	// Serve Swagger UI and OpenAPI spec
	if err := generator.ServeSwaggerUI(h); err != nil {
		return fmt.Errorf("failed to setup Swagger UI: %w", err)
	}

	// Use logger from generator (already processed in NewGenerator)
	generator.GetLogger().Info("OpenAPI documentation enabled with customization",
		"swagger_ui", "/docs",
		"openapi_spec", "/openapi.json")

	return nil
}

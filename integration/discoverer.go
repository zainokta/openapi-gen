package integration

import (
	"github.com/zainokta/openapi-gen/spec"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// RouteDiscoverer interface for framework-agnostic route discovery
type RouteDiscoverer interface {
	DiscoverRoutes() ([]spec.RouteInfo, error)
	GetFrameworkName() string
}

// AutoDiscoverer automatically detects the framework and creates appropriate discoverer
type AutoDiscoverer struct {
	discoverer RouteDiscoverer
}

// NewAutoDiscoverer creates a discoverer based on the provided framework instance
func NewAutoDiscoverer(framework interface{}) (*AutoDiscoverer, error) {
	var discoverer RouteDiscoverer

	switch f := framework.(type) {
	case *server.Hertz:
		discoverer = NewHertzRouteDiscoverer(f)
	default:
		return nil, fmt.Errorf("unsupported framework type: %T", framework)
	}

	return &AutoDiscoverer{discoverer: discoverer}, nil
}

// DiscoverRoutes discovers routes using the appropriate discoverer
func (a *AutoDiscoverer) DiscoverRoutes() ([]spec.RouteInfo, error) {
	return a.discoverer.DiscoverRoutes()
}

// GetFrameworkName returns the detected framework name
func (a *AutoDiscoverer) GetFrameworkName() string {
	return a.discoverer.GetFrameworkName()
}

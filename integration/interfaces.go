package integration

import "net/http"

// HTTPHandler defines a generic HTTP handler function
type HTTPHandler func(w http.ResponseWriter, r *http.Request)

// HTTPServer defines the interface for HTTP server operations needed by OpenAPI docs
type HTTPServer interface {
	GET(path string, handler HTTPHandler)
}

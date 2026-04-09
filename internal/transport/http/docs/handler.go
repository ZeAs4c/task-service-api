// Package docs provides HTTP handlers for serving OpenAPI/Swagger documentation.
// It embeds the OpenAPI specification file and serves the Swagger UI interface
// for interactive API exploration and testing.
//
// The OpenAPI specification is embedded into the binary using Go's embed package,
// eliminating the need for external file dependencies at runtime.
package docs

import (
	"embed"
	"net/http"

	"example.com/taskservice/internal/transport/http/handlers"
)

// openAPISpec embeds the OpenAPI 3.0 specification file into the compiled binary.
// This ensures the documentation is always available and matches the exact
// API version that was built.
//
//go:embed openapi.json
var openAPISpec embed.FS

// Handler manages the serving of OpenAPI specification and Swagger UI.
// It caches the specification content in memory for fast responses.
type Handler struct {
	// spec contains the raw JSON content of the OpenAPI specification.
	// It is loaded once at initialization and served from memory.
	spec []byte
}

// NewHandler creates a new documentation handler instance.
// It reads and caches the embedded OpenAPI specification file.
// This function panics if the embedded file cannot be read,
// as the API cannot function properly without its documentation specification.
func NewHandler() *Handler {
	spec, err := openAPISpec.ReadFile("openapi.json")
	if err != nil {
		// Panic is acceptable here because this is a fatal initialization error.
		// The service cannot serve API documentation without the spec file.
		panic(err)
	}

	return &Handler{spec: spec}
}

// ServeSpec serves the raw OpenAPI JSON specification.
// This endpoint is consumed by Swagger UI and can also be used by external
// tools for client generation, validation, or documentation scraping.
//
// Endpoint: GET /swagger/openapi.json
//
// Headers:
//   - Content-Type: application/json; charset=utf-8
//   - Cache-Control: no-store (prevents caching to ensure latest version)
func (h *Handler) ServeSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Prevent caching to ensure clients always get the latest spec.
	// This is especially important during development and after deployments.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	// Write errors are ignored as there's no recovery mechanism for a
	// failed response body write. The connection would already be broken.
	_, _ = w.Write(h.spec)
}

// ServeUI serves the Swagger UI HTML interface.
// This provides an interactive documentation explorer where users can
// view all endpoints, models, and execute test requests directly from the browser.
//
// Endpoint: GET /swagger/
//
// Headers:
//   - Content-Type: text/html; charset=utf-8
//
// Note: Swagger UI loads the specification from /swagger/openapi.json
// after the page renders.
func (h *Handler) ServeUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(swaggerUIHTML))
}

// RedirectToUI redirects requests from /swagger to /swagger/ (with trailing slash).
// This provides a more user-friendly experience by correcting common URL typos
// and ensuring relative paths in Swagger UI resolve correctly.
//
// Endpoint: GET /swagger
//
// Redirect: 301 Moved Permanently to /swagger/
func (h *Handler) RedirectToUI(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
}

// _ is a dummy function used to ensure the handler package maintains
// references to the DTO types defined in the handlers package.
// This prevents the Go compiler from optimizing away the type information
// that Swagger needs for generating accurate OpenAPI schemas.
//
// Without this reference, the types might be omitted from the compiled binary,
// causing Swagger to generate incomplete or incorrect documentation.
func _() {
	_ = handlers.TaskMutationDTO{}
	_ = handlers.TaskDTO{}
	_ = handlers.NewTaskDTO(nil)
}

// swaggerUIHTML contains the complete Swagger UI HTML page.
// It is embedded as a constant to avoid external file dependencies
// and simplify deployment.
//
// The HTML loads Swagger UI from the unpkg CDN and configures it to
// fetch the OpenAPI specification from /swagger/openapi.json.
//
// Custom styling is applied for better visual integration:
//   - Light gray background (#f6f7fb) for comfortable viewing
//   - Centered content with max-width 1440px on large screens
var swaggerUIHTML = []byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Task Service Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #f6f7fb; }
    #swagger-ui { max-width: 1440px; margin: 0 auto; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: '/swagger/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
        layout: 'BaseLayout'
      });
    };
  </script>
</body>
</html>`)

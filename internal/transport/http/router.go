// Package transporthttp provides HTTP routing configuration for the Task Service API.
// It defines all API endpoints, maps them to their respective handlers, and configures
// middleware and route-specific settings.
package transporthttp

import (
	"net/http"

	"github.com/gorilla/mux"

	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
)

// NewRouter creates and configures the main application router.
// It sets up all HTTP routes for the API and documentation endpoints.
//
// The router is configured with StrictSlash(true), which automatically redirects
// paths with trailing slashes to their non-trailing-slash counterparts (or vice versa).
// For example, /tasks/ → /tasks and /tasks → /tasks/ are normalized consistently.
//
// Route Structure:
//
//	/swagger/openapi.json    - OpenAPI 3.0 specification (JSON)
//	/swagger/                - Swagger UI interactive documentation
//	/swagger                 - Redirects to /swagger/
//
//	/api/v1/tasks            - Task collection endpoints (CRUD)
//	/api/v1/tasks/{id}       - Individual task endpoints
//	/api/v1/templates        - Template collection endpoints
//	/api/v1/templates/{id}/generate - Task generation from templates
//
// Parameters:
//   - taskHandler: Handler for task-related endpoints (CRUD operations)
//   - docsHandler: Handler for serving OpenAPI specification and Swagger UI
//
// Returns:
//   - *mux.Router: Configured Gorilla Mux router ready to serve HTTP requests
func NewRouter(taskHandler *httphandlers.TaskHandler, docsHandler *swaggerdocs.Handler) *mux.Router {
	// Create a new router with strict slash normalization.
	// This ensures consistent URL handling regardless of trailing slash presence.
	router := mux.NewRouter().StrictSlash(true)

	// ========================================================================
	// Documentation Routes
	// ========================================================================
	// These routes serve the OpenAPI specification and Swagger UI.
	// They are placed at the root level (not under /api) for easy access.

	// Serve the raw OpenAPI JSON specification.
	// Used by Swagger UI and external API tools.
	router.HandleFunc("/swagger/openapi.json", docsHandler.ServeSpec).Methods(http.MethodGet)

	// Serve the Swagger UI HTML interface.
	// Note: The trailing slash is important for correct relative path resolution.
	router.HandleFunc("/swagger/", docsHandler.ServeUI).Methods(http.MethodGet)

	// Redirect /swagger to /swagger/ for user convenience.
	// This handles the common case where users forget the trailing slash.
	router.HandleFunc("/swagger", docsHandler.RedirectToUI).Methods(http.MethodGet)

	// ========================================================================
	// API Routes (v1)
	// ========================================================================
	// All API endpoints are grouped under the /api/v1 path prefix.
	// This allows for future API versioning by adding /api/v2, etc.
	api := router.PathPrefix("/api/v1").Subrouter()

	// Task Collection Routes
	// These operate on the collection of tasks as a whole.

	// POST /api/v1/tasks - Create a new task (regular or template)
	api.HandleFunc("/tasks", taskHandler.Create).Methods(http.MethodPost)

	// GET /api/v1/tasks - List tasks with pagination
	// Query parameters: page, page_size
	api.HandleFunc("/tasks", taskHandler.List).Methods(http.MethodGet)

	// Individual Task Routes
	// These operate on a specific task identified by its ID.
	// The ID pattern [0-9]+ ensures only positive integers are matched.

	// GET /api/v1/tasks/{id} - Retrieve a single task by ID
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.GetByID).Methods(http.MethodGet)

	// PUT /api/v1/tasks/{id} - Update an existing task (partial updates supported)
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.Update).Methods(http.MethodPut)

	// DELETE /api/v1/tasks/{id} - Delete a task permanently
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.Delete).Methods(http.MethodDelete)

	// Template Routes
	// These endpoints manage recurring task templates.

	// GET /api/v1/templates - List all task templates
	api.HandleFunc("/templates", taskHandler.GetTemplates).Methods(http.MethodGet)

	// POST /api/v1/templates/{id}/generate - Generate concrete tasks from a template
	// Query parameters: until (YYYY-MM-DD) - generate tasks up to this date
	api.HandleFunc("/templates/{id:[0-9]+}/generate", taskHandler.GenerateTasks).Methods(http.MethodPost)

	return router
}

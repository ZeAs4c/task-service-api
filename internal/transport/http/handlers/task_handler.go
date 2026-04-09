// Package handlers provides HTTP request/response handling for the Task Service API.
// It contains the TaskHandler which orchestrates request validation, usecase invocation,
// and response formatting for all task-related endpoints.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	taskdomain "example.com/taskservice/internal/domain/task"
	"example.com/taskservice/internal/usecase/task"
	taskusecase "example.com/taskservice/internal/usecase/task"
)

// TaskHandler handles HTTP requests for task management operations.
// It acts as the presentation layer in Clean Architecture, translating between
// HTTP concerns (requests, responses, status codes) and domain usecases.
//
// The handler delegates all business logic to the usecase layer and focuses solely on:
//   - Request parsing and validation
//   - Response formatting
//   - HTTP status code selection
//   - Error handling
type TaskHandler struct {
	usecase taskusecase.Usecase
	service *task.Service
}

// NewTaskHandler creates a new TaskHandler with the provided dependencies.
// Both usecase and service are injected following the Dependency Inversion Principle.
//
// The service parameter provides additional methods that are not part of the
// Usecase interface, such as template generation operations.
func NewTaskHandler(usecase taskusecase.Usecase, service *task.Service) *TaskHandler {
	return &TaskHandler{usecase: usecase, service: service}
}

// ============================================================================
// Core CRUD Handlers
// ============================================================================

// Create handles POST /api/v1/tasks
// It creates a new task (regular or template) based on the JSON request body.
//
// Request Body: TaskMutationDTO (JSON)
// Response: 201 Created with TaskDTO on success
// Errors:
//   - 400 Bad Request: Invalid JSON, unknown fields, or validation failure
//   - 500 Internal Server Error: Database or unexpected errors
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req TaskMutationDTO
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// Default scheduled date to current time if not provided.
	// This ensures tasks always have a meaningful scheduled_at value.
	scheduledAt := req.ScheduledAt
	if scheduledAt.IsZero() {
		scheduledAt = time.Now().UTC()
	}

	created, err := h.usecase.Create(r.Context(), taskusecase.CreateInput{
		Title:            req.Title,
		Description:      req.Description,
		Status:           req.Status,
		RecurrenceType:   req.RecurrenceType,
		RecurrenceRule:   req.RecurrenceRule,
		IsTemplate:       req.IsTemplate,
		ParentTemplateID: req.ParentTemplateID,
		ScheduledAt:      scheduledAt,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, NewTaskDTO(created))
}

// GetByID handles GET /api/v1/tasks/{id}
// It retrieves a single task by its unique identifier.
//
// Path Parameters:
//   - id: Task ID (positive integer)
//
// Response: 200 OK with TaskDTO on success
// Errors:
//   - 400 Bad Request: Invalid or missing ID parameter
//   - 404 Not Found: Task with the given ID does not exist
func (h *TaskHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	task, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, NewTaskDTO(task))
}

// Update handles PUT /api/v1/tasks/{id}
// It modifies an existing task with the fields provided in the request body.
// Omitted fields retain their current values (PATCH-like behavior via PUT).
//
// Path Parameters:
//   - id: Task ID (positive integer)
//
// Request Body: TaskMutationDTO (JSON, partial updates supported)
// Response: 200 OK with updated TaskDTO on success
// Errors:
//   - 400 Bad Request: Invalid ID, JSON, or validation failure
//   - 404 Not Found: Task with the given ID does not exist
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req TaskMutationDTO
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	updated, err := h.usecase.Update(r.Context(), id, taskusecase.UpdateInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		ScheduledAt: &req.ScheduledAt,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, NewTaskDTO(updated))
}

// Delete handles DELETE /api/v1/tasks/{id}
// It permanently removes a task from the system.
//
// Path Parameters:
//   - id: Task ID (positive integer)
//
// Response: 204 No Content on success (empty body)
// Errors:
//   - 400 Bad Request: Invalid or missing ID parameter
//   - 404 Not Found: Task with the given ID does not exist
func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.usecase.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// List handles GET /api/v1/tasks
// It retrieves a paginated list of regular tasks (non-templates).
//
// Query Parameters:
//   - page: Page number, starting from 1 (default: 1)
//   - page_size: Number of items per page, max 100 (default: 20)
//
// Response: 200 OK with paginated response:
//
//	{
//	  "tasks": [...],
//	  "meta": {
//	    "total": 42,
//	    "page": 1,
//	    "page_size": 20
//	  }
//	}
//
// Errors:
//   - 500 Internal Server Error: Database errors
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters from query string.
	// Invalid values are silently converted to zero (handled by defaults below).
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	tasks, total, err := h.usecase.ListPaginated(r.Context(), page, pageSize)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	// Build paginated response structure.
	response := struct {
		Tasks []TaskDTO `json:"tasks"`
		Meta  struct {
			Total    int `json:"total"`
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
		} `json:"meta"`
	}{
		Tasks: make([]TaskDTO, len(tasks)),
	}

	for i := range tasks {
		response.Tasks[i] = NewTaskDTO(&tasks[i])
	}

	// Apply default values for pagination parameters.
	// This ensures the response metadata reflects the actual values used.
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	response.Meta.Total = total
	response.Meta.Page = page
	response.Meta.PageSize = pageSize

	writeJSON(w, http.StatusOK, response)
}

// ============================================================================
// Template Handlers
// ============================================================================

// GetTemplates handles GET /api/v1/templates
// It retrieves all recurring task templates from the system.
//
// Response: 200 OK with array of TaskDTO (only templates)
// Errors:
//   - 500 Internal Server Error: Database errors
func (h *TaskHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.usecase.GetTemplates(r.Context())
	if err != nil {
		// Note: This uses http.Error instead of writeUsecaseError for simplicity.
		// Consider refactoring to use writeUsecaseError for consistency.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]TaskDTO, len(templates))
	for i, tmpl := range templates {
		response[i] = NewTaskDTO(&tmpl)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GenerateTasks handles POST /api/v1/templates/{id}/generate
// It creates concrete task instances from a template for dates up to the specified limit.
//
// Path Parameters:
//   - id: Template ID (positive integer)
//
// Query Parameters:
//   - until: End date in YYYY-MM-DD format (required, inclusive)
//
// Response: 200 OK with array of generated TaskDTO instances
// Errors:
//   - 400 Bad Request: Invalid template ID, missing until parameter, or invalid date format
//   - 404 Not Found: Template with the given ID does not exist
//   - 500 Internal Server Error: Generation or database errors
func (h *TaskHandler) GenerateTasks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid template id", http.StatusBadRequest)
		return
	}

	untilStr := r.URL.Query().Get("until")
	if untilStr == "" {
		http.Error(w, "until parameter is required (format: YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	// Parse date in YYYY-MM-DD format (UTC).
	// The reference date "2006-01-02" is Go's canonical date format representation.
	until, err := time.Parse("2006-01-02", untilStr)
	if err != nil {
		http.Error(w, "invalid until date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	generated, err := h.usecase.GenerateTasksFromTemplate(r.Context(), id, until)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]TaskDTO, len(generated))
	for i, task := range generated {
		response[i] = NewTaskDTO(&task)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ============================================================================
// Helper Functions
// ============================================================================

// getIDFromRequest extracts and validates the task ID from URL path parameters.
// It ensures the ID is a positive integer.
//
// Returns:
//   - int64: Validated task ID
//   - error: Descriptive error if ID is missing, invalid, or non-positive
func getIDFromRequest(r *http.Request) (int64, error) {
	rawID := mux.Vars(r)["id"]
	if rawID == "" {
		return 0, errors.New("missing task id")
	}

	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, errors.New("invalid task id")
	}

	if id <= 0 {
		return 0, errors.New("invalid task id")
	}

	return id, nil
}

// decodeJSON parses the request body as JSON into the destination struct.
// It disallows unknown fields to prevent silent ignoring of typos in client requests.
//
// This strict parsing helps catch client errors early and maintains API contract clarity.
func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	return nil
}

// writeUsecaseError maps domain/usecase errors to appropriate HTTP status codes.
// This function acts as an error translator between the business layer and HTTP layer.
//
// Mapping:
//   - ErrNotFound        → 404 Not Found
//   - ErrInvalidInput    → 400 Bad Request
//   - Any other error    → 500 Internal Server Error
func writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, taskdomain.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, taskusecase.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

// writeError writes a standardized error response as JSON.
// The response format is: {"error": "<error message>"}
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

// writeJSON serializes the payload as JSON and writes it to the response.
// It sets the Content-Type header and status code before writing the body.
//
// Note: JSON encoding errors are silently ignored as there's no recovery mechanism
// once the response headers have been written.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

// max returns the larger of two integers.
// Used for default value calculations in pagination.
//
// Note: This function is currently unused but kept for utility purposes.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

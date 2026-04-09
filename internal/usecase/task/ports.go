// Package task provides the business logic and usecase implementations for task management.
// This file defines the interfaces (ports) that enable dependency inversion between layers,
// following the Dependency Inversion Principle from SOLID.
//
// Interfaces defined here:
//   - Repository: Data access abstraction (implemented by infrastructure layer)
//   - Usecase: Business logic abstraction (implemented by service layer)
package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

// Repository defines the interface for task data persistence operations.
// It acts as a port in the Hexagonal Architecture pattern, allowing the business
// logic (usecase layer) to remain completely decoupled from database implementation
// details such as PostgreSQL, in-memory storage, or mock implementations for testing.
//
// Implementations of this interface must be provided by the infrastructure layer
// (e.g., internal/repository/postgres).
//
// All methods accept a context.Context for request-scoped values, cancellation,
// and timeouts. This is essential for proper resource management in production.
type Repository interface {
	// ========================================================================
	// Regular Task Operations
	// ========================================================================

	// Create inserts a new regular task (non-template) into the data store.
	// The task must have IsTemplate set to false.
	// Returns the created task with its auto-generated ID populated.
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)

	// GetByID retrieves a task by its unique identifier.
	// Returns taskdomain.ErrNotFound if no task exists with the given ID.
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)

	// Update modifies an existing task with the provided fields.
	// Only non-zero fields should be updated.
	// Returns taskdomain.ErrNotFound if the task doesn't exist.
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)

	// Delete removes a task permanently from the data store.
	// Returns taskdomain.ErrNotFound if no task exists with the given ID.
	Delete(ctx context.Context, id int64) error

	// List retrieves all regular tasks (non-templates) from the data store.
	// Results are typically ordered by ID descending (newest first).
	//
	// Deprecated: Use ListWithPagination for better performance with large datasets.
	List(ctx context.Context) ([]taskdomain.Task, error)

	// ListWithPagination retrieves a paginated subset of regular tasks.
	// Tasks are ordered by scheduled_at descending, then ID descending.
	//
	// Parameters:
	//   - limit: Maximum number of tasks to return
	//   - offset: Number of tasks to skip before returning results
	ListWithPagination(ctx context.Context, limit, offset int) ([]taskdomain.Task, error)

	// CountTasks returns the total number of regular tasks (non-templates).
	// Used for pagination metadata calculation.
	CountTasks(ctx context.Context) (int, error)

	// GetTasksByDateRange retrieves all regular tasks scheduled within a date range.
	// The range is inclusive: scheduled_at BETWEEN from AND to.
	// Results are ordered by scheduled_at ascending.
	GetTasksByDateRange(ctx context.Context, from, to time.Time) ([]taskdomain.Task, error)

	// ========================================================================
	// Template Operations
	// ========================================================================

	// CreateTemplate inserts a new recurring task template into the data store.
	// The template must have IsTemplate set to true and include valid recurrence
	// configuration (RecurrenceType and RecurrenceRule).
	// Returns the created template with its auto-generated ID populated.
	CreateTemplate(ctx context.Context, template *taskdomain.Task) (*taskdomain.Task, error)

	// GetTemplateByID retrieves a recurring task template by its ID.
	// Only returns tasks where is_template = true.
	// Returns taskdomain.ErrNotFound if no template exists with the given ID.
	GetTemplateByID(ctx context.Context, id int64) (*taskdomain.Task, error)

	// GetAllTemplates retrieves all recurring task templates from the data store.
	// Results are typically ordered by ID descending (newest first).
	GetAllTemplates(ctx context.Context) ([]taskdomain.Task, error)

	// ========================================================================
	// Recurring Instance Operations
	// ========================================================================

	// CreateRecurringInstance creates a concrete task instance from a template.
	// This method should handle duplicate prevention (e.g., via ON CONFLICT DO NOTHING).
	//
	// Returns nil, nil if the task already exists (this is not considered an error).
	// Returns the created task on successful insertion.
	CreateRecurringInstance(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
}

// Usecase defines the interface for task-related business logic operations.
// It acts as the primary entry point for the transport layer (HTTP handlers)
// to interact with the domain logic without coupling to implementation details.
//
// This interface follows the Command-Query Separation (CQS) principle where
// methods that modify state (Create, Update, Delete) return the affected entity,
// while query methods (GetByID, List, ListPaginated) only return data.
//
// Implementations of this interface should handle:
//   - Input validation and normalization
//   - Business rule enforcement
//   - Orchestration of repository calls
//   - Transaction management (if applicable)
//   - Domain event publishing (future enhancement)
type Usecase interface {
	// ========================================================================
	// Command Methods (State-Changing)
	// ========================================================================

	// Create creates a new task (regular or template) after validating the input.
	// For regular tasks, status defaults to "new" if not provided.
	// For templates, recurrence configuration is validated.
	//
	// Returns ErrInvalidInput if validation fails.
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)

	// Update modifies an existing task with the provided fields.
	// Only non-zero fields in UpdateInput are applied; others retain current values.
	//
	// Returns ErrInvalidInput if validation fails.
	// Returns taskdomain.ErrNotFound if the task doesn't exist.
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)

	// Delete removes a task permanently from the system.
	//
	// Returns taskdomain.ErrNotFound if the task doesn't exist.
	Delete(ctx context.Context, id int64) error

	// GenerateTasksFromTemplate creates concrete task instances from a template
	// for all dates matching the recurrence rules up to the specified limit.
	//
	// The generation stops when the next calculated date exceeds the until parameter.
	// Duplicate prevention is handled at the repository level.
	//
	// Returns the list of successfully generated tasks (excluding duplicates).
	// Returns taskdomain.ErrNotFound if the template doesn't exist.
	GenerateTasksFromTemplate(ctx context.Context, templateID int64, until time.Time) ([]taskdomain.Task, error)

	// ========================================================================
	// Query Methods (Read-Only)
	// ========================================================================

	// GetByID retrieves a single task by its unique identifier.
	//
	// Returns taskdomain.ErrNotFound if the task doesn't exist.
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)

	// List retrieves all regular tasks (non-templates) without pagination.
	//
	// Deprecated: Use ListPaginated for better performance and client experience.
	List(ctx context.Context) ([]taskdomain.Task, error)

	// ListPaginated retrieves a paginated subset of regular tasks.
	// Applies sensible defaults: page=1, pageSize=20, maxPageSize=100.
	//
	// Returns:
	//   - []taskdomain.Task: The tasks for the requested page
	//   - int: Total count of all tasks (for pagination metadata)
	//   - error: Any error that occurred during retrieval
	ListPaginated(ctx context.Context, page, pageSize int) ([]taskdomain.Task, int, error)

	// GetTemplates retrieves all recurring task templates.
	GetTemplates(ctx context.Context) ([]taskdomain.Task, error)
}

// CreateInput contains all the data required to create a new task.
// It serves as a Data Transfer Object (DTO) between the transport and usecase layers,
// decoupling the HTTP request structure from the business logic.
//
// Fields with zero values are treated as "not provided" and will receive
// sensible defaults during validation.
type CreateInput struct {
	// Title is the short summary of the task.
	// Required - cannot be empty or whitespace-only.
	Title string

	// Description provides additional task details.
	// Optional - defaults to empty string.
	Description string

	// Status indicates the initial lifecycle state.
	// Optional - defaults to StatusNew if not provided.
	// Valid values: StatusNew, StatusInProgress, StatusDone.
	Status taskdomain.Status

	// RecurrenceType defines the recurrence strategy for template tasks.
	// Required when IsTemplate is true. Ignored for regular tasks.
	RecurrenceType taskdomain.RecurrenceType

	// RecurrenceRule contains the configuration parameters for recurrence.
	// Required when RecurrenceType is set.
	RecurrenceRule *taskdomain.RecurrenceRule

	// IsTemplate indicates whether this is a recurring template.
	// When true, RecurrenceType and RecurrenceRule must be provided.
	IsTemplate bool

	// ParentTemplateID references the template that generated this task.
	// Should only be set internally for generated recurring instances.
	ParentTemplateID *int64

	// ScheduledAt is the planned execution date and time in UTC.
	// Optional - if zero value, defaults to current time for regular tasks.
	ScheduledAt time.Time
}

// UpdateInput contains the fields that can be modified on an existing task.
// It uses pointers for optional fields to distinguish between "not provided"
// and "set to zero value". This enables PATCH-like behavior via PUT requests.
//
// Only non-nil fields are applied to the task; nil fields retain their current values.
type UpdateInput struct {
	// Title is the short summary of the task.
	// If nil, the current title is preserved.
	Title string

	// Description provides additional task details.
	// If nil, the current description is preserved.
	Description string

	// Status indicates the lifecycle state.
	// If nil, the current status is preserved.
	Status taskdomain.Status

	// ScheduledAt is the planned execution date and time in UTC.
	// If nil, the current scheduled date is preserved.
	// To clear the scheduled date, pass a pointer to zero time.Time.
	ScheduledAt *time.Time
}

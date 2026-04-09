// Package postgres implements the task.Repository interface using PostgreSQL as the data store.
// It provides CRUD operations for both regular tasks and recurring task templates,
// using pgx/v5 as the database driver.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

// Repository implements the task.Repository interface for PostgreSQL.
// It encapsulates all database operations and connection pool management.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL repository instance with the provided connection pool.
// The pool must be already established and ready for use.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ============================================================================
// Regular Task Operations
// ============================================================================

// Create inserts a new regular task (non-template) into the database.
// The task must have IsTemplate set to false.
// Returns the created task with its auto-generated ID populated.
//
// Note: The scheduled_at field is optional. If zero value is provided,
// NULL will be stored in the database.
func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
        INSERT INTO tasks (title, description, status, scheduled_at, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, title, description, status, scheduled_at, created_at, updated_at
    `

	// Convert zero time.Time to NULL for database storage.
	// This allows tasks to be created without a specific scheduled date.
	var scheduledAt *time.Time
	if !task.ScheduledAt.IsZero() {
		scheduledAt = &task.ScheduledAt
	}

	row := r.pool.QueryRow(ctx, query,
		task.Title,
		task.Description,
		task.Status,
		scheduledAt,
		task.CreatedAt,
		task.UpdatedAt,
	)

	created, err := scanTaskWithScheduledAt(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

// GetByID retrieves a task by its unique identifier.
// Returns taskdomain.ErrNotFound if no task exists with the given ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
        SELECT id, title, description, status, scheduled_at, created_at, updated_at
        FROM tasks
        WHERE id = $1
    `

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTaskWithScheduledAt(row)
	if err != nil {
		// Translate pgx-specific "no rows" error to domain error.
		// This maintains layer separation - the usecase layer doesn't need to
		// know about PostgreSQL-specific error types.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}
		return nil, err
	}

	return found, nil
}

// Update modifies an existing task with the provided fields.
// Only non-zero fields are updated (except for ID and timestamps).
// Returns taskdomain.ErrNotFound if the task doesn't exist.
func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
        UPDATE tasks
        SET title = $1,
            description = $2,
            status = $3,
            scheduled_at = $4,
            updated_at = $5
        WHERE id = $6
        RETURNING id, title, description, status, scheduled_at, created_at, updated_at
    `

	var scheduledAt *time.Time
	if !task.ScheduledAt.IsZero() {
		scheduledAt = &task.ScheduledAt
	}

	row := r.pool.QueryRow(ctx, query,
		task.Title,
		task.Description,
		task.Status,
		scheduledAt,
		task.UpdatedAt,
		task.ID,
	)

	updated, err := scanTaskWithScheduledAt(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}
		return nil, err
	}

	return updated, nil
}

// Delete removes a task from the database by its ID.
// Returns taskdomain.ErrNotFound if no task exists with the given ID.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	// Check if any row was actually deleted.
	// pgx doesn't return ErrNoRows for DELETE operations,
	// so we check RowsAffected() instead.
	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

// List retrieves all regular tasks (non-templates) from the database.
// Results are ordered by ID in descending order (newest first).
//
// Deprecated: Use ListWithPagination for better performance with large datasets.
func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
        SELECT id, title, description, status, scheduled_at, created_at, updated_at
        FROM tasks
        WHERE is_template = false
        ORDER BY id DESC
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTaskWithScheduledAt(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}

	// Check for errors that occurred during iteration.
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// ListWithPagination retrieves a paginated subset of regular tasks.
// Tasks are ordered by scheduled_at (descending), then by ID (descending).
// This ordering ensures that upcoming tasks appear first.
//
// Parameters:
//   - limit: maximum number of tasks to return
//   - offset: number of tasks to skip before starting to return rows
func (r *Repository) ListWithPagination(ctx context.Context, limit, offset int) ([]taskdomain.Task, error) {
	const query = `
        SELECT id, title, description, status, scheduled_at, created_at, updated_at
        FROM tasks
        WHERE is_template = false
        ORDER BY scheduled_at DESC, id DESC
        LIMIT $1 OFFSET $2
    `

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []taskdomain.Task
	for rows.Next() {
		task, err := scanTaskWithScheduledAt(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

// CountTasks returns the total number of regular tasks (non-templates) in the database.
// This is typically used for pagination metadata.
func (r *Repository) CountTasks(ctx context.Context) (int, error) {
	const query = `SELECT COUNT(*) FROM tasks WHERE is_template = false`

	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	return count, err
}

// ============================================================================
// Template Operations
// ============================================================================

// CreateTemplate inserts a new recurring task template into the database.
// The template must have IsTemplate set to true and include recurrence configuration.
// Returns the created template with its auto-generated ID populated.
func (r *Repository) CreateTemplate(ctx context.Context, template *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
        INSERT INTO tasks (
            title, description, status, scheduled_at,
            recurrence_type, recurrence_rule, is_template,
            created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        RETURNING id, title, description, status, scheduled_at,
            recurrence_type, recurrence_rule, is_template, parent_template_id,
            created_at, updated_at
    `

	var scheduledAt *time.Time
	if !template.ScheduledAt.IsZero() {
		scheduledAt = &template.ScheduledAt
	}

	var task taskdomain.Task
	var recurrenceTypeStr *string
	var recurrenceRuleJSON []byte
	var parentTemplateID *int64
	var scheduledAtResult *time.Time

	err := r.pool.QueryRow(ctx, query,
		template.Title,
		template.Description,
		template.Status,
		scheduledAt,
		template.RecurrenceType,
		template.RecurrenceRule,
		true, // is_template is always true for this method
		template.CreatedAt,
		template.UpdatedAt,
	).Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Status,
		&scheduledAtResult,
		&recurrenceTypeStr,
		&recurrenceRuleJSON,
		&task.IsTemplate,
		&parentTemplateID,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Reconstruct nullable fields from database values.
	if scheduledAtResult != nil {
		task.ScheduledAt = *scheduledAtResult
	}

	if recurrenceTypeStr != nil {
		task.RecurrenceType = taskdomain.RecurrenceType(*recurrenceTypeStr)
	}
	task.ParentTemplateID = parentTemplateID

	// Unmarshal JSONB rule if present.
	if recurrenceRuleJSON != nil {
		var rule taskdomain.RecurrenceRule
		if err := rule.Scan(recurrenceRuleJSON); err != nil {
			return nil, err
		}
		task.RecurrenceRule = &rule
	}

	return &task, nil
}

// GetTemplateByID retrieves a recurring task template by its ID.
// Only returns tasks where is_template = true.
// Returns taskdomain.ErrNotFound if no template exists with the given ID.
func (r *Repository) GetTemplateByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
        SELECT id, title, description, status, scheduled_at,
            recurrence_type, recurrence_rule, is_template, parent_template_id,
            created_at, updated_at
        FROM tasks
        WHERE id = $1 AND is_template = true
    `

	row := r.pool.QueryRow(ctx, query, id)

	var task taskdomain.Task
	var recurrenceTypeStr *string
	var recurrenceRuleJSON []byte
	var parentTemplateID *int64
	var scheduledAt *time.Time

	err := row.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Status,
		&scheduledAt,
		&recurrenceTypeStr,
		&recurrenceRuleJSON,
		&task.IsTemplate,
		&parentTemplateID,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}
		return nil, err
	}

	if scheduledAt != nil {
		task.ScheduledAt = *scheduledAt
	}

	if recurrenceTypeStr != nil {
		task.RecurrenceType = taskdomain.RecurrenceType(*recurrenceTypeStr)
	}
	task.ParentTemplateID = parentTemplateID

	if recurrenceRuleJSON != nil {
		var rule taskdomain.RecurrenceRule
		if err := rule.Scan(recurrenceRuleJSON); err != nil {
			return nil, err
		}
		task.RecurrenceRule = &rule
	}

	return &task, nil
}

// GetAllTemplates retrieves all recurring task templates from the database.
// Results are ordered by ID in descending order (newest first).
func (r *Repository) GetAllTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
        SELECT id, title, description, status, scheduled_at,
            recurrence_type, recurrence_rule, is_template, parent_template_id,
            created_at, updated_at
        FROM tasks
        WHERE is_template = true
        ORDER BY id DESC
    `

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []taskdomain.Task
	for rows.Next() {
		var task taskdomain.Task
		var recurrenceTypeStr *string
		var recurrenceRuleJSON []byte
		var parentTemplateID *int64
		var scheduledAt *time.Time

		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&task.Status,
			&scheduledAt,
			&recurrenceTypeStr,
			&recurrenceRuleJSON,
			&task.IsTemplate,
			&parentTemplateID,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if scheduledAt != nil {
			task.ScheduledAt = *scheduledAt
		}

		if recurrenceTypeStr != nil {
			task.RecurrenceType = taskdomain.RecurrenceType(*recurrenceTypeStr)
		}
		task.ParentTemplateID = parentTemplateID

		if recurrenceRuleJSON != nil {
			var rule taskdomain.RecurrenceRule
			if err := rule.Scan(recurrenceRuleJSON); err != nil {
				return nil, err
			}
			task.RecurrenceRule = &rule
		}

		templates = append(templates, task)
	}

	return templates, nil
}

// ============================================================================
// Recurring Instance Operations
// ============================================================================

// CreateRecurringInstance creates a concrete task instance from a template.
// It uses INSERT ... ON CONFLICT DO NOTHING to prevent duplicate tasks
// for the same template on the same date.
//
// The unique constraint is defined on (parent_template_id, scheduled_at)
// and only applies when both fields are NOT NULL.
//
// Returns nil, nil if the task already exists (conflict detected) - this is not an error.
func (r *Repository) CreateRecurringInstance(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
        INSERT INTO tasks (
            title, description, status, scheduled_at,
            parent_template_id, is_template,
            created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (parent_template_id, scheduled_at) 
        WHERE parent_template_id IS NOT NULL AND scheduled_at IS NOT NULL
        DO NOTHING
        RETURNING id, title, description, status, scheduled_at, created_at, updated_at
    `

	var scheduledAt *time.Time
	if !task.ScheduledAt.IsZero() {
		scheduledAt = &task.ScheduledAt
	}

	row := r.pool.QueryRow(ctx, query,
		task.Title,
		task.Description,
		task.Status,
		scheduledAt,
		task.ParentTemplateID,
		false, // is_template is always false for generated instances
		task.CreatedAt,
		task.UpdatedAt,
	)

	created, err := scanTaskWithScheduledAt(row)
	if err != nil {
		// pgx returns ErrNoRows when ON CONFLICT DO NOTHING suppresses insertion.
		// This is expected behavior and not an error condition.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return created, nil
}

// GetTasksByDateRange retrieves all regular tasks scheduled within a date range.
// The range is inclusive: scheduled_at BETWEEN $1 AND $2.
// Results are ordered by scheduled_at in ascending order.
//
// This method is useful for generating calendar views or weekly reports.
func (r *Repository) GetTasksByDateRange(ctx context.Context, from, to time.Time) ([]taskdomain.Task, error) {
	const query = `
        SELECT id, title, description, status, scheduled_at, created_at, updated_at
        FROM tasks
        WHERE is_template = false AND scheduled_at BETWEEN $1 AND $2
        ORDER BY scheduled_at
    `

	rows, err := r.pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []taskdomain.Task
	for rows.Next() {
		task, err := scanTaskWithScheduledAt(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}

	return tasks, nil
}

// ============================================================================
// Internal Helpers
// ============================================================================

// taskScanner abstracts the Scan method from both pgx.Row and pgx.Rows.
// This allows scanTaskWithScheduledAt to work with single rows and result sets.
type taskScanner interface {
	Scan(dest ...any) error
}

// scanTaskWithScheduledAt scans a database row into a taskdomain.Task struct.
// It correctly handles the NULLABLE scheduled_at column by using a *time.Time pointer.
// This is the primary scanning function used throughout the repository.
func scanTaskWithScheduledAt(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task        taskdomain.Task
		status      string
		scheduledAt *time.Time
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&scheduledAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	if scheduledAt != nil {
		task.ScheduledAt = *scheduledAt
	}

	return &task, nil
}

// scanTask is a legacy alias for scanTaskWithScheduledAt.
// It's kept for backward compatibility with existing code.
//
// Deprecated: Use scanTaskWithScheduledAt directly.
func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	return scanTaskWithScheduledAt(scanner)
}

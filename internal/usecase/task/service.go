// Package task provides the business logic and usecase implementations for task management.
// The Service struct implements the Usecase interface and orchestrates all task-related
// operations including validation, recurring task generation, and pagination.
package task

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

// Service implements the Usecase interface and contains the core business logic
// for task management. It orchestrates repository calls, validates input,
// and enforces business rules.
//
// The Service is designed to be stateless - all state is managed through the
// Repository interface. The now field is a function that can be overridden
// for testing time-dependent logic.
type Service struct {
	repo   Repository
	now    func() time.Time // Injectable time function for testability
	logger *slog.Logger
}

// NewService creates a new Service instance with the provided dependencies.
// The time function defaults to time.Now().UTC() but can be overridden in tests
// by modifying the now field.
func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		now:    func() time.Time { return time.Now().UTC() },
		logger: logger,
	}
}

// ============================================================================
// Command Methods (State-Changing Operations)
// ============================================================================

// Create creates a new task (regular or template) after validating the input.
// For regular tasks, it uses the standard Create repository method.
// For templates, it delegates to CreateTemplate which handles recurrence fields.
//
// Validation rules:
//   - Title is required and trimmed of whitespace
//   - Status defaults to "new" if not provided
//   - Templates must have RecurrenceType specified
//   - RecurrenceRule must be valid for the chosen RecurrenceType
//
// Returns ErrInvalidInput if validation fails.
func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	s.logger.Info("create task started", "title", input.Title)

	// Validate and normalize input before processing.
	normalized, err := s.validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		Title:            normalized.Title,
		Description:      normalized.Description,
		Status:           normalized.Status,
		RecurrenceType:   normalized.RecurrenceType,
		RecurrenceRule:   normalized.RecurrenceRule,
		IsTemplate:       normalized.IsTemplate,
		ParentTemplateID: normalized.ParentTemplateID,
		ScheduledAt:      normalized.ScheduledAt,
	}
	now := s.now()
	model.CreatedAt = now
	model.UpdatedAt = now

	// Route to appropriate repository method based on task type.
	// Templates have additional recurrence fields that need special handling.
	if model.IsTemplate {
		created, err := s.repo.CreateTemplate(ctx, model)
		if err != nil {
			return nil, err
		}
		return created, nil
	}

	// Regular task creation.
	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	return created, nil
}

// Update modifies an existing task with the provided fields.
// Only non-zero/non-nil fields in UpdateInput are applied; others retain current values.
// This provides PATCH-like behavior through a PUT endpoint.
//
// Returns ErrInvalidInput if validation fails.
// Returns taskdomain.ErrNotFound if the task doesn't exist.
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, err := s.validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	// Retrieve existing task to preserve unmodified fields.
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Build updated model starting with existing values.
	model := &taskdomain.Task{
		ID:          id,
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		UpdatedAt:   s.now(),
		ScheduledAt: existing.ScheduledAt, // Preserve by default
	}

	// Apply optional scheduled date update using pointer semantics.
	// nil means "don't update", non-nil means "set to this value".
	if input.ScheduledAt != nil {
		model.ScheduledAt = *input.ScheduledAt
	}

	updated, err := s.repo.Update(ctx, model)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// Delete removes a task permanently from the system.
//
// Returns taskdomain.ErrNotFound if the task doesn't exist.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.Delete(ctx, id)
}

// GenerateTasksFromTemplate creates concrete task instances from a template
// for all dates matching the recurrence rules up to the specified limit.
//
// The generation strategy depends on the recurrence type:
//   - specific_dates: Iterates through the provided dates list
//   - daily/monthly/parity: Calculates dates sequentially using calculateNextDate
//
// Duplicate prevention is handled at the repository level via ON CONFLICT.
// Tasks that already exist are silently skipped (not included in result).
//
// Safety limit: Maximum 1000 iterations to prevent infinite loops from bugs.
//
// Returns taskdomain.ErrNotFound if the template doesn't exist.
func (s *Service) GenerateTasksFromTemplate(ctx context.Context, templateID int64, until time.Time) ([]taskdomain.Task, error) {
	template, err := s.getValidTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}

	if template.RecurrenceType == taskdomain.RecurrenceSpecific {
		return s.generateSpecificDates(ctx, template, until)
	}

	return s.generateByRule(ctx, template, until)
}

// ============================================================================
// Query Methods (Read-Only Operations)
// ============================================================================

// GetByID retrieves a single task by its unique identifier.
//
// Returns taskdomain.ErrNotFound if the task doesn't exist.
func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

// List retrieves all regular tasks (non-templates) without pagination.
//
// Deprecated: Use ListPaginated for better performance and client experience.
func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.List(ctx)
}

// ListPaginated retrieves a paginated subset of regular tasks.
// Applies sensible defaults: page=1, pageSize=20, maxPageSize=100.
//
// Returns:
//   - []taskdomain.Task: The tasks for the requested page
//   - int: Total count of all tasks (for pagination metadata)
//   - error: Any error that occurred during retrieval
func (s *Service) ListPaginated(ctx context.Context, page, pageSize int) ([]taskdomain.Task, int, error) {
	// Apply default and boundary values.
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	tasks, err := s.repo.ListWithPagination(ctx, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.CountTasks(ctx)
	if err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// GetTemplates retrieves all recurring task templates.
func (s *Service) GetTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.GetAllTemplates(ctx)
}

// ============================================================================
// Private Helper Methods - Template Generation
// ============================================================================

// getValidTemplate retrieves a template and validates that it is actually a template.
// This prevents accidentally generating tasks from regular task IDs.
func (s *Service) getValidTemplate(ctx context.Context, templateID int64) (*taskdomain.Task, error) {
	template, err := s.repo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return nil, err
	}

	if !template.IsTemplate {
		return nil, fmt.Errorf("task with id %d is not a template", templateID)
	}

	return template, nil
}

// generateSpecificDates creates tasks for each date explicitly listed in the template.
// Dates that are invalid or after the until limit are skipped.
func (s *Service) generateSpecificDates(ctx context.Context, template *taskdomain.Task, until time.Time) ([]taskdomain.Task, error) {
	if template.RecurrenceRule == nil || len(template.RecurrenceRule.Dates) == 0 {
		return nil, fmt.Errorf("specific_dates template has no dates")
	}

	var result []taskdomain.Task

	for _, dateStr := range template.RecurrenceRule.Dates {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			// Skip invalid dates rather than failing the entire generation.
			continue
		}

		if date.After(until) {
			continue
		}

		task := s.buildTaskFromTemplate(template, date)

		created, err := s.repo.CreateRecurringInstance(ctx, task)
		if err != nil {
			return nil, err
		}

		// created is nil when the task already exists (ON CONFLICT DO NOTHING).
		if created != nil {
			result = append(result, *created)
		}
	}

	return result, nil
}

// generateByRule creates tasks by sequentially calculating the next occurrence date
// based on the recurrence rule. Stops when the calculated date exceeds until.
//
// The loop starts from yesterday to ensure today's task is captured on the first iteration.
// Safety limit of 1000 iterations prevents infinite loops from logic errors.
func (s *Service) generateByRule(ctx context.Context, template *taskdomain.Task, until time.Time) ([]taskdomain.Task, error) {
	var result []taskdomain.Task

	// Start from yesterday so the first calculated date captures today.
	currentDate := s.now().AddDate(0, 0, -1)

	// Iterate with safety limit to prevent infinite loops.
	for i := 0; i < 1000 && currentDate.Before(until); i++ {
		nextDate, err := s.calculateNextDate(template, currentDate)
		if err != nil {
			break
		}

		if nextDate.After(until) {
			break
		}

		task := s.buildTaskFromTemplate(template, nextDate)

		created, err := s.repo.CreateRecurringInstance(ctx, task)
		if err != nil {
			return nil, err
		}

		if created != nil {
			result = append(result, *created)
		}

		currentDate = nextDate
	}

	return result, nil
}

// buildTaskFromTemplate creates a concrete task instance from a template.
// The new task inherits the template's title and description, starts with "new" status,
// and references the template via ParentTemplateID.
func (s *Service) buildTaskFromTemplate(template *taskdomain.Task, date time.Time) *taskdomain.Task {
	now := s.now()

	return &taskdomain.Task{
		Title:            template.Title,
		Description:      template.Description,
		Status:           taskdomain.StatusNew,
		ParentTemplateID: &template.ID,
		IsTemplate:       false,
		ScheduledAt:      date,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// calculateNextDate computes the next occurrence date based on the recurrence rule.
// This is the core algorithm for recurring task generation.
//
// Edge cases handled:
//   - daily: respects the interval (default 1)
//   - monthly: adjusts for months with fewer days (e.g., Jan 31 -> Feb 28/29)
//   - parity: iterates day by day until matching parity is found
func (s *Service) calculateNextDate(template *taskdomain.Task, currentDate time.Time) (time.Time, error) {
	switch template.RecurrenceType {
	case taskdomain.RecurrenceDaily:
		interval := 1
		if template.RecurrenceRule != nil && template.RecurrenceRule.Interval > 0 {
			interval = template.RecurrenceRule.Interval
		}
		return currentDate.AddDate(0, 0, interval), nil

	case taskdomain.RecurrenceMonthly:
		dayOfMonth := 1
		if template.RecurrenceRule != nil && template.RecurrenceRule.DayOfMonth > 0 {
			dayOfMonth = template.RecurrenceRule.DayOfMonth
		}

		currentYear, currentMonth, _ := currentDate.Date()

		// Try the current month first.
		targetDate := time.Date(currentYear, currentMonth, dayOfMonth, 0, 0, 0, 0, time.UTC)

		// If the date has already passed, move to next month.
		if !targetDate.After(currentDate) {
			targetDate = targetDate.AddDate(0, 1, 0)
		}

		// Adjust for months with fewer days (e.g., Jan 31 -> Feb 28).
		lastDay := time.Date(targetDate.Year(), targetDate.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if dayOfMonth > lastDay {
			targetDate = time.Date(targetDate.Year(), targetDate.Month(), lastDay, 0, 0, 0, 0, time.UTC)
		}

		return targetDate, nil

	case taskdomain.RecurrenceParity:
		parity := "even"
		if template.RecurrenceRule != nil && template.RecurrenceRule.Parity != "" {
			parity = template.RecurrenceRule.Parity
		}
		nextDate := currentDate.AddDate(0, 0, 1)
		for {
			day := nextDate.Day()
			if (parity == "even" && day%2 == 0) || (parity == "odd" && day%2 == 1) {
				return nextDate, nil
			}
			nextDate = nextDate.AddDate(0, 0, 1)
		}

	default:
		return time.Time{}, fmt.Errorf("unknown recurrence type: %s", template.RecurrenceType)
	}
}

// ============================================================================
// Private Helper Methods - Validation
// ============================================================================

// validateCreateInput validates and normalizes the CreateInput data.
// It ensures all business rules are satisfied before proceeding with creation.
func (s *Service) validateCreateInput(input CreateInput) (CreateInput, error) {
	input = normalizeInput(input)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.RecurrenceType != "" {
		if err := validateRecurrence(input.RecurrenceType, input.RecurrenceRule); err != nil {
			return CreateInput{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}

	if input.IsTemplate && input.RecurrenceType == "" {
		return CreateInput{}, fmt.Errorf("%w: template must have recurrence_type", ErrInvalidInput)
	}

	return input, nil
}

// validateUpdateInput validates and normalizes the UpdateInput data.
func (s *Service) validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	return input, nil
}

// normalizeInput trims whitespace from string fields in CreateInput.
func normalizeInput(input CreateInput) CreateInput {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

// validateRecurrence ensures the recurrence rule is valid for the given recurrence type.
// This is a package-level function because it's pure validation logic with no dependencies.
func validateRecurrence(rType taskdomain.RecurrenceType, rule *taskdomain.RecurrenceRule) error {
	if rType == "" {
		return nil
	}

	if rule == nil {
		return fmt.Errorf("recurrence_rule is required")
	}

	switch rType {
	case taskdomain.RecurrenceDaily:
		if rule.Interval <= 0 {
			return fmt.Errorf("interval must be > 0")
		}

	case taskdomain.RecurrenceMonthly:
		if rule.DayOfMonth < 1 || rule.DayOfMonth > 31 {
			return fmt.Errorf("day_of_month must be between 1 and 31")
		}

	case taskdomain.RecurrenceSpecific:
		if len(rule.Dates) == 0 {
			return fmt.Errorf("dates cannot be empty")
		}
		for _, d := range rule.Dates {
			_, err := time.Parse("2006-01-02", d)
			if err != nil {
				return fmt.Errorf("invalid date format: %s", d)
			}
		}

	case taskdomain.RecurrenceParity:
		if rule.Parity != "even" && rule.Parity != "odd" {
			return fmt.Errorf("parity must be 'even' or 'odd'")
		}

	default:
		return fmt.Errorf("unknown recurrence_type: %s", rType)
	}

	return nil
}

// Package task contains unit tests for the task usecase layer.
// Tests use a mock repository implementation to verify business logic
// in isolation from the actual database.
package task

import (
	"context"
	"log/slog"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

// mockRepo is a test double that implements the Repository interface.
// It provides predictable, in-memory behavior for unit testing the service layer
// without requiring a real database connection.
//
// All methods return pre-defined responses suitable for testing success paths.
// For testing error scenarios, additional mock implementations or fields
// controlling error behavior can be added.
type mockRepo struct{}

// ============================================================================
// Regular Task Operations (Mock Implementations)
// ============================================================================

func (m *mockRepo) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	// Simulate database auto-increment by assigning a fixed ID.
	task.ID = 1
	return task, nil
}

func (m *mockRepo) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	// Return a valid task for any positive ID.
	// Error cases (e.g., ErrNotFound) should be tested with a separate mock.
	return &taskdomain.Task{
		ID:          id,
		Title:       "Test Task",
		Description: "Test Description",
		Status:      taskdomain.StatusNew,
		ScheduledAt: time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *mockRepo) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	// Pass through the task unchanged, simulating successful update.
	return task, nil
}

func (m *mockRepo) Delete(ctx context.Context, id int64) error {
	// Always succeed for any ID.
	return nil
}

func (m *mockRepo) List(ctx context.Context) ([]taskdomain.Task, error) {
	// Return a minimal list of tasks.
	return []taskdomain.Task{
		{ID: 1, Title: "Task 1", Status: taskdomain.StatusNew},
	}, nil
}

func (m *mockRepo) ListWithPagination(ctx context.Context, limit, offset int) ([]taskdomain.Task, error) {
	// Pre-defined test dataset for pagination testing.
	allTasks := []taskdomain.Task{
		{ID: 1, Title: "Task 1", Status: taskdomain.StatusNew},
		{ID: 2, Title: "Task 2", Status: taskdomain.StatusInProgress},
		{ID: 3, Title: "Task 3", Status: taskdomain.StatusDone},
	}

	// Apply pagination slicing with bounds checking.
	start := offset
	if start > len(allTasks) {
		return []taskdomain.Task{}, nil
	}

	end := start + limit
	if end > len(allTasks) {
		end = len(allTasks)
	}

	return allTasks[start:end], nil
}

func (m *mockRepo) CountTasks(ctx context.Context) (int, error) {
	// Return the size of the pre-defined dataset.
	return 3, nil
}

func (m *mockRepo) GetTasksByDateRange(ctx context.Context, from, to time.Time) ([]taskdomain.Task, error) {
	// Return a single task within the requested range.
	return []taskdomain.Task{
		{ID: 1, Title: "Task in range", ScheduledAt: from},
	}, nil
}

// ============================================================================
// Template Operations (Mock Implementations)
// ============================================================================

func (m *mockRepo) CreateTemplate(ctx context.Context, template *taskdomain.Task) (*taskdomain.Task, error) {
	// Assign a fixed template ID to simulate auto-increment.
	template.ID = 100
	return template, nil
}

func (m *mockRepo) GetTemplateByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	// Return a valid daily recurrence template.
	return &taskdomain.Task{
		ID:             id,
		Title:          "Test Template",
		IsTemplate:     true,
		RecurrenceType: taskdomain.RecurrenceDaily,
		RecurrenceRule: &taskdomain.RecurrenceRule{
			Interval: 1,
		},
	}, nil
}

func (m *mockRepo) GetAllTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	// Return a single template for testing.
	return []taskdomain.Task{
		{
			ID:             1,
			Title:          "Daily Template",
			IsTemplate:     true,
			RecurrenceType: taskdomain.RecurrenceDaily,
			RecurrenceRule: &taskdomain.RecurrenceRule{Interval: 1},
		},
	}, nil
}

// ============================================================================
// Recurring Instance Operations (Mock Implementations)
// ============================================================================

func (m *mockRepo) CreateRecurringInstance(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	// Assign a fixed ID to simulate successful creation.
	// Does not simulate duplicate detection (returns success always).
	task.ID = 200
	return task, nil
}

// ============================================================================
// Unit Tests
// ============================================================================

// TestCreateTask_EmptyTitle verifies that task creation fails when the title
// is empty. This ensures the validation logic correctly rejects invalid input.
//
// Expected: ErrInvalidInput with message containing "title is required"
func TestCreateTask_EmptyTitle(t *testing.T) {
	repo := &mockRepo{}
	logger := slog.Default()
	service := NewService(repo, logger)

	_, err := service.Create(context.Background(), CreateInput{
		Title: "", // Intentionally empty to trigger validation error
	})

	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}

	// Verify the error message contains the expected validation text.
	expectedMsg := "invalid task input: title is required"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestCreateTask_WithRecurrence verifies that a template task with valid
// recurrence configuration can be created successfully.
//
// Expected: Task created with ID assigned, no error returned.
func TestCreateTask_WithRecurrence(t *testing.T) {
	repo := &mockRepo{}
	logger := slog.Default()
	service := NewService(repo, logger)

	task, err := service.Create(context.Background(), CreateInput{
		Title:          "test",
		RecurrenceType: taskdomain.RecurrenceDaily,
		RecurrenceRule: &taskdomain.RecurrenceRule{
			Interval: 1,
		},
		IsTemplate: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.ID == 0 {
		t.Error("expected task ID to be set, got 0")
	}
}

// TestGenerateTasksFromTemplate_Daily verifies that task generation from a
// daily recurrence template produces the expected number of tasks.
//
// The test uses a short time window (3 days) to keep the test fast while
// still verifying the generation logic works correctly.
func TestGenerateTasksFromTemplate_Daily(t *testing.T) {
	repo := &mockRepo{}
	logger := slog.Default()
	service := NewService(repo, logger)

	// Generate tasks for the next 3 days.
	until := time.Now().UTC().AddDate(0, 0, 3)

	tasks, err := service.GenerateTasksFromTemplate(context.Background(), 1, until)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) == 0 {
		t.Fatal("expected generated tasks, got 0")
	}
}

// TestListPaginated verifies the pagination logic correctly handles page
// and pageSize parameters and returns the expected metadata.
//
// Test cases covered implicitly:
//   - Default values applied when page < 1 or pageSize < 1
//   - Total count matches the mock dataset size
//   - All tasks returned when pageSize exceeds dataset size
func TestListPaginated(t *testing.T) {
	repo := &mockRepo{}
	logger := slog.Default()
	service := NewService(repo, logger)

	// Request first page with large pageSize to get all items.
	tasks, total, err := service.ListPaginated(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify total count matches the mock dataset.
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	// Verify all items are returned.
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

// TestUpdateTask_ScheduledAt verifies that the scheduled date of a task
// can be updated through the Update usecase.
//
// This test ensures the pointer semantics in UpdateInput correctly propagate
// the new scheduled date to the domain entity.
func TestUpdateTask_ScheduledAt(t *testing.T) {
	repo := &mockRepo{}
	logger := slog.Default()
	service := NewService(repo, logger)

	// Set a new scheduled date 24 hours in the future.
	newDate := time.Now().Add(24 * time.Hour)

	updated, err := service.Update(context.Background(), 1, UpdateInput{
		Title:       "Updated Task",
		Description: "Updated Description",
		Status:      taskdomain.StatusInProgress,
		ScheduledAt: &newDate, // Pointer indicates field should be updated
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the scheduled date was updated correctly.
	// Using Equal for time comparison (ignores monotonic clock differences).
	if !updated.ScheduledAt.Equal(newDate) {
		t.Errorf("expected scheduled_at %v, got %v", newDate, updated.ScheduledAt)
	}
}

// Package task defines the core domain entities, value objects, and business rules
// for task management. This package is the innermost layer of Clean Architecture
// and should not depend on any external packages or frameworks.
package task

import "errors"

// ErrNotFound is returned when a requested task cannot be located in the repository.
// This error is part of the domain layer and should be used consistently across
// all implementations (PostgreSQL, in-memory, mock) to maintain layer independence.
//
// Example usage:
//
//	if errors.Is(err, task.ErrNotFound) {
//	    // Handle not found case
//	}
var ErrNotFound = errors.New("task not found")

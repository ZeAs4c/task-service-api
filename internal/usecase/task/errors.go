// Package task provides the business logic and usecase implementations for task management.
// This package orchestrates the flow of data between the domain entities and the repository layer,
// enforcing business rules, validation, and transaction boundaries.
package task

import "errors"

// ErrInvalidInput is returned when the input data provided to a usecase fails validation.
// This error indicates that the request cannot be processed due to missing required fields,
// invalid values, or logical inconsistencies in the provided data.
//
// Common scenarios that trigger this error:
//   - Empty or whitespace-only title
//   - Invalid status value (not one of: new, in_progress, done)
//   - Template without recurrence_type specified
//   - Recurrence rule missing required fields for the chosen recurrence type
//   - Invalid date format in specific_dates
//   - Parity value not "even" or "odd"
//
// This error should be mapped to HTTP 400 Bad Request at the transport layer.
var ErrInvalidInput = errors.New("invalid task input")

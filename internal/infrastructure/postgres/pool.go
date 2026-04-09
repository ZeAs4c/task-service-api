// Package postgres provides infrastructure-level utilities for working with PostgreSQL.
// It handles connection pool management, configuration parsing, and health checks.
// This package belongs to the infrastructure layer of Clean Architecture.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open establishes a connection pool to PostgreSQL using the provided DSN (Data Source Name).
// It performs the following steps:
//  1. Validates that the DSN is not empty
//  2. Parses the DSN into a pgxpool configuration
//  3. Creates a connection pool with the parsed configuration
//  4. Pings the database to verify connectivity
//
// The returned pool is safe for concurrent use across multiple goroutines.
// The caller is responsible for closing the pool when it's no longer needed
// by calling pool.Close().
//
// Example DSN formats:
//   - "postgres://username:password@localhost:5432/database?sslmode=disable"
//   - "postgres://username:password@host:port/database?sslmode=require"
//
// Common errors:
//   - Empty DSN: returns an error indicating the DSN is required
//   - Invalid DSN format: returns a parsing error from pgxpool.ParseConfig
//   - Connection failure: returns an error after failing to ping the database
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	// Guard clause: ensure DSN is provided before attempting to connect.
	// This prevents cryptic "empty connection string" errors from the driver.
	if dsn == "" {
		return nil, fmt.Errorf("database dsn is empty")
	}

	// ParseConfig validates the DSN format and sets sensible defaults:
	// - MaxConns: 4 (can be overridden via DSN parameters)
	// - MinConns: 0
	// - MaxConnLifetime: 1 hour
	// - MaxConnIdleTime: 30 minutes
	// - HealthCheckPeriod: 1 minute
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// NewWithConfig creates the connection pool and immediately establishes
	// the minimum number of connections (MinConns) in the background.
	// It does not block waiting for connections to be established.
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Ping verifies that the database is reachable and responsive.
	// This ensures we don't return a pool that can't actually connect,
	// failing fast instead of discovering the issue on the first query.
	if err := pool.Ping(ctx); err != nil {
		// Clean up the pool resources before returning the error.
		// Without this, the caller would have no way to close the pool.
		pool.Close()
		return nil, err
	}

	return pool, nil
}

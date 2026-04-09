// Package main is the entry point for the Task Service API.
// It initializes dependencies, sets up routing, and starts the HTTP server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	infrastructurepostgres "example.com/taskservice/internal/infrastructure/postgres"
	postgresrepo "example.com/taskservice/internal/repository/postgres"
	transporthttp "example.com/taskservice/internal/transport/http"
	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
	"example.com/taskservice/internal/usecase/task"
)

// main is the application entry point.
// It performs the following steps:
//  1. Initializes structured logging (slog)
//  2. Loads configuration from environment variables
//  3. Sets up graceful shutdown handling
//  4. Establishes database connection pool
//  5. Wires up dependencies following Clean Architecture
//  6. Configures HTTP routes and middleware
//  7. Starts the HTTP server
func main() {
	// Initialize structured logger with text format output to stdout.
	// LevelInfo excludes debug logs in production.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load application configuration from environment variables.
	cfg := loadConfig()

	// Create a context that is cancelled on SIGINT (Ctrl+C) or SIGTERM (Docker stop).
	// This enables graceful shutdown of the HTTP server and database connections.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Establish a connection pool to PostgreSQL.
	// The pool is configured with the DSN from environment variables.
	pool, err := infrastructurepostgres.Open(ctx, cfg.DatabaseDSN)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	// Ensure the connection pool is closed when the application exits.
	defer pool.Close()

	// Dependency injection following Clean Architecture principles:
	// Repository (data access) -> Service (business logic) -> Handler (HTTP layer)
	taskRepo := postgresrepo.New(pool)
	taskService := task.NewService(taskRepo, logger)
	taskHandler := httphandlers.NewTaskHandler(taskService, taskService)
	docsHandler := swaggerdocs.NewHandler()
	router := transporthttp.NewRouter(taskHandler, docsHandler)

	// Configure the HTTP server with security best practices.
	// ReadHeaderTimeout mitigates Slowloris attacks.
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run the server in a goroutine so that it doesn't block the main thread.
	// This allows us to listen for shutdown signals concurrently.
	go func() {
		// Wait for the shutdown signal (SIGINT/SIGTERM).
		<-ctx.Done()

		// Create a context with timeout for graceful shutdown.
		// This gives in-flight requests up to 10 seconds to complete.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Attempt graceful shutdown of the HTTP server.
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown http server", "error", err)
		}
	}()

	logger.Info("http server started", "addr", cfg.HTTPAddr)

	// Start the HTTP server.
	// ListenAndServe blocks until the server is shut down or an error occurs.
	// ErrServerClosed is expected during graceful shutdown and should not be logged as an error.
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("listen and serve", "error", err)
		os.Exit(1)
	}
}

// config holds the application configuration loaded from environment variables.
type config struct {
	HTTPAddr    string
	DatabaseDSN string
}

// loadConfig reads configuration from environment variables with sensible defaults.
// It panics if required configuration values are missing.
func loadConfig() config {
	cfg := config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseDSN: envOrDefault("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/taskservice?sslmode=disable"),
	}

	// Database connection string is critical for the application to function.
	if cfg.DatabaseDSN == "" {
		panic(fmt.Errorf("DATABASE_DSN is required"))
	}

	return cfg
}

// envOrDefault retrieves the value of an environment variable.
// If the variable is not set or is empty, it returns the provided fallback value.
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

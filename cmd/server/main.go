// Package main provides the entry point for the CAIA Library server
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/caiatech/caia-library/internal/api"
	"github.com/caiatech/caia-library/internal/temporal/activities"
	"github.com/caiatech/caia-library/internal/temporal/workflows"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Initialize Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort: getEnv("TEMPORAL_HOST", "localhost:7233"),
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer temporalClient.Close()

	// Create worker for Temporal workflows
	w := worker.New(temporalClient, "caia-library", worker.Options{
		MaxConcurrentActivityExecutionSize: 10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})
	
	// Register workflow
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	
	// Register all activities
	w.RegisterActivity(activities.FetchDocumentActivity)
	w.RegisterActivity(activities.ExtractTextActivity)
	w.RegisterActivity(activities.GenerateEmbeddingsActivity)
	w.RegisterActivity(activities.StoreDocumentActivity)
	w.RegisterActivity(activities.IndexDocumentActivity)
	w.RegisterActivity(activities.MergeBranchActivity)

	// Start worker in background
	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Failed to start worker: %v", err)
		}
	}()

	// Initialize Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName:               "CAIA Library API",
		DisableStartupMessage: false,
		EnablePrintRoutes:     false,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))
	
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
		TimeFormat: "15:04:05",
		TimeZone: "UTC",
	}))
	
	app.Use(cors.New(cors.Config{
		AllowOrigins: getEnv("CORS_ORIGINS", "*"),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Initialize handlers
	h := api.NewHandlers(temporalClient)

	// API Routes
	setupRoutes(app, h)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Start server
	port := getEnv("PORT", "8080")
	log.Printf("Starting CAIA Library server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupRoutes configures all API routes
func setupRoutes(app *fiber.App, h *api.Handlers) {
	// Health check
	app.Get("/health", h.Health)
	
	// API v1 routes
	v1 := app.Group("/api/v1")
	
	// Document routes
	docs := v1.Group("/documents")
	docs.Post("/", h.IngestDocument)
	docs.Get("/:id", h.GetDocument)
	docs.Get("/", h.ListDocuments)
	
	// Workflow routes
	workflows := v1.Group("/workflows")
	workflows.Get("/:id", h.GetWorkflow)
	
	// Root redirect
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": "CAIA Library",
			"version": "0.1.0",
			"docs":    "https://github.com/caiatech/caia-library",
		})
	})
}

// getEnv retrieves an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
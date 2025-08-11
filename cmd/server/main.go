// Package main provides the entry point for the Caia Library server
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Caia-Tech/caia-library/internal/api"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/internal/temporal/activities"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
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

	// Initialize hybrid storage system
	repoPath := getEnv("CAIA_REPO_PATH", "./data/repo")
	govcRepoName := getEnv("GOVC_REPO_NAME", "caia-library")
	
	// Configure hybrid storage (govc primary with git fallback)
	storageConfig := storage.DefaultHybridConfig()
	storageConfig.PrimaryBackend = getEnv("PRIMARY_BACKEND", "govc")
	
	// Create metrics collector
	metricsCollector := storage.NewSimpleMetricsCollector()
	
	// Initialize hybrid storage
	hybridStorage, err := storage.NewHybridStorage(repoPath, govcRepoName, storageConfig, metricsCollector)
	if err != nil {
		log.Fatalf("Failed to initialize hybrid storage: %v", err)
	}
	defer hybridStorage.Close()
	
	// Set global storage for activities
	activities.SetGlobalStorage(hybridStorage, metricsCollector)

	// Create worker for Temporal workflows
	w := worker.New(temporalClient, "caia-library", worker.Options{
		MaxConcurrentActivityExecutionSize: 10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})
	
	// Register workflows
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	w.RegisterWorkflow(workflows.ScheduledIngestionWorkflow)
	w.RegisterWorkflow(workflows.BatchIngestionWorkflow)
	
	// Register all activities
	w.RegisterActivity(activities.FetchDocumentActivity)
	w.RegisterActivity(activities.ExtractTextActivity)
	w.RegisterActivity(activities.GenerateEmbeddingsActivity)
	w.RegisterActivity(activities.StoreDocumentActivity)
	w.RegisterActivity(activities.IndexDocumentActivity)
	w.RegisterActivity(activities.MergeBranchActivity)
	
	// Register collector activities
	collector := activities.NewCollectorActivities()
	w.RegisterActivity(collector.CollectFromSourceActivity)
	w.RegisterActivity(collector.CheckDuplicateActivity)
	
	// Register academic collector activities
	academicCollector := activities.NewAcademicCollectorActivities()
	w.RegisterActivity(academicCollector.CollectAcademicSourcesActivity)

	// Start worker in background
	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Failed to start worker: %v", err)
		}
	}()

	// Initialize Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName:               "Caia Library API",
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
	h := api.NewHandlers(temporalClient, repoPath)
	
	// Initialize storage handler for monitoring
	storageHandler := api.NewStorageHandler(hybridStorage, metricsCollector)

	// API Routes
	setupRoutes(app, h, storageHandler)

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
	log.Printf("Starting Caia Library server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupRoutes configures all API routes
func setupRoutes(app *fiber.App, h *api.Handlers, storageHandler *api.StorageHandler) {
	// Health check
	app.Get("/health", h.Health)
	
	// API v1 routes
	v1 := app.Group("/api/v1")
	
	// Document routes
	docs := v1.Group("/documents")
	docs.Post("/", h.IngestDocument)
	docs.Get("/:id", h.GetDocument)
	docs.Get("/", h.ListDocuments)
	
	// Ingestion routes
	ingestion := v1.Group("/ingestion")
	ingestion.Post("/scheduled", h.CreateScheduledIngestion)
	ingestion.Post("/batch", h.CreateBatchIngestion)
	
	// Workflow routes
	workflows := v1.Group("/workflows")
	workflows.Get("/:id", h.GetWorkflow)
	
	// Query routes (Git Query Language)
	query := v1.Group("/query")
	query.Post("/", h.ExecuteQuery)
	query.Get("/examples", h.GetQueryExamples)
	
	// Stats routes
	stats := v1.Group("/stats")
	stats.Get("/attribution", h.GetAttributionStats)
	
	// Storage monitoring routes
	storage := v1.Group("/storage")
	storage.Get("/stats", storageHandler.GetStorageStats)
	storage.Get("/metrics", storageHandler.GetStorageMetrics)
	storage.Get("/health", storageHandler.GetStorageHealth)
	storage.Delete("/metrics", storageHandler.ClearMetrics)
	
	// Root redirect
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": "Caia Library",
			"version": "0.1.0",
			"docs":    "https://github.com/Caia-Tech/caia-library",
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
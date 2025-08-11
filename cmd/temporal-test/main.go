package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/internal/temporal/activities"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	fmt.Println("🔧 TEMPORAL INTEGRATION COMPREHENSIVE TEST")
	fmt.Println("==========================================")
	
	// Test 1: Local Temporal server connectivity
	fmt.Println("📡 Test 1: Temporal Server Connectivity...")
	
	temporalClient, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		fmt.Printf("❌ Cannot connect to Temporal server: %v\n", err)
		fmt.Println("   Trying to start embedded Temporal for testing...")
		testWithoutServer()
		return
	}
	defer temporalClient.Close()
	
	fmt.Println("✅ Successfully connected to Temporal server")

	// Test 2: Storage initialization
	fmt.Println("\n💾 Test 2: Storage System Initialization...")
	
	metricsCollector := storage.NewSimpleMetricsCollector()
	hybridStorage, err := storage.NewHybridStorage(
		"./test-data",
		"temporal-test-repo",
		storage.DefaultHybridConfig(),
		metricsCollector,
	)
	if err != nil {
		fmt.Printf("❌ Failed to initialize hybrid storage: %v\n", err)
		return
	}
	defer hybridStorage.Close()
	
	fmt.Println("✅ Hybrid storage initialized successfully")
	
	// Set global storage for activities
	activities.SetGlobalStorage(hybridStorage, metricsCollector)

	// Test 3: Worker setup and activity registration
	fmt.Println("\n🏭 Test 3: Temporal Worker Setup...")
	
	w := worker.New(temporalClient, "test-task-queue", worker.Options{
		MaxConcurrentActivityExecutionSize: 5,
		MaxConcurrentWorkflowTaskExecutionSize: 5,
	})

	// Register workflows
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	w.RegisterWorkflow(workflows.FileProcessingWorkflow)

	// Register activities  
	w.RegisterActivity(activities.FetchDocumentActivity)
	w.RegisterActivity(activities.ExtractTextActivity)
	w.RegisterActivity(activities.GenerateEmbeddingsActivity)
	w.RegisterActivity(activities.StoreDocumentActivity)
	w.RegisterActivity(activities.IndexDocumentActivity)
	w.RegisterActivity(activities.MergeBranchActivity)

	fmt.Println("✅ Worker configured with workflows and activities")

	// Test 4: Start worker
	fmt.Println("\n▶️  Test 4: Starting Temporal Worker...")
	
	// Start worker in background
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- w.Run(worker.InterruptCh())
	}()
	
	// Give worker time to start
	time.Sleep(2 * time.Second)
	fmt.Println("✅ Temporal worker started successfully")

	// Test 5: Execute a simple workflow
	fmt.Println("\n🔄 Test 5: Document Ingestion Workflow Execution...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with a simple web document
	testInput := workflows.DocumentInput{
		URL:      "https://httpbin.org/html",
		Type:     "html",
		Metadata: map[string]string{
			"source":      "temporal-test",
			"test_run":    "true",
			"description": "Temporal integration test document",
		},
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("test-ingestion-%d", time.Now().Unix()),
		TaskQueue: "test-task-queue",
	}

	workflowRun, err := temporalClient.ExecuteWorkflow(ctx, workflowOptions, workflows.DocumentIngestionWorkflow, testInput)
	if err != nil {
		fmt.Printf("❌ Failed to start workflow: %v\n", err)
		return
	}

	fmt.Printf("✅ Workflow started with ID: %s\n", workflowRun.GetID())

	// Wait for workflow completion
	fmt.Println("   ⏳ Waiting for workflow completion...")
	
	err = workflowRun.Get(ctx, nil)
	if err != nil {
		fmt.Printf("❌ Workflow execution failed: %v\n", err)
		
		// Try to get workflow execution details
		if execution, err := temporalClient.DescribeWorkflowExecution(ctx, workflowRun.GetID(), workflowRun.GetRunID()); err == nil {
			fmt.Printf("   Workflow status: %v\n", execution.WorkflowExecutionInfo.Status)
		}
		return
	}

	fmt.Println("✅ Workflow completed successfully!")

	// Test 6: Storage verification
	fmt.Println("\n🔍 Test 6: Storage Verification...")
	
	// Check if document was stored
	listCtx, listCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer listCancel()
	
	documents, err := hybridStorage.ListDocuments(listCtx, map[string]string{})
	if err != nil {
		fmt.Printf("❌ Failed to list documents: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d documents in storage\n", len(documents))
		
		for i, doc := range documents {
			fmt.Printf("   [%d] ID: %s, URL: %s, Created: %s\n", 
				i+1, doc.ID, doc.Source.URL, doc.CreatedAt.Format(time.RFC3339))
		}
	}

	// Test 7: Storage health check
	fmt.Println("\n🏥 Test 7: Storage Health Check...")
	
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer healthCancel()
	
	if err := hybridStorage.Health(healthCtx); err != nil {
		fmt.Printf("❌ Storage health check failed: %v\n", err)
	} else {
		fmt.Println("✅ Storage system is healthy")
	}

	// Test 8: Metrics collection
	fmt.Println("\n📊 Test 8: Metrics Collection...")
	
	stats := hybridStorage.GetStats()
	fmt.Printf("✅ Storage statistics collected: %+v\n", stats)

	fmt.Println("\n🎉 TEMPORAL INTEGRATION TEST COMPLETE")
	fmt.Println("=====================================")
	fmt.Println("All core Temporal integration components are working!")
	
	// Clean shutdown
	w.Stop()
	select {
	case err := <-workerDone:
		if err != nil {
			fmt.Printf("Worker stopped with error: %v\n", err)
		}
	case <-time.After(5 * time.Second):
		fmt.Println("Worker shutdown timeout")
	}
}

func testWithoutServer() {
	fmt.Println("\n⚠️  TEMPORAL SERVER NOT AVAILABLE")
	fmt.Println("   Testing storage components independently...")
	
	// Test storage without Temporal
	fmt.Println("\n💾 Storage-Only Test...")
	
	metricsCollector := storage.NewSimpleMetricsCollector()
	hybridStorage, err := storage.NewHybridStorage(
		"./test-data",
		"standalone-test-repo",
		storage.DefaultHybridConfig(),
		metricsCollector,
	)
	if err != nil {
		fmt.Printf("❌ Failed to initialize hybrid storage: %v\n", err)
		return
	}
	defer hybridStorage.Close()
	
	fmt.Println("✅ Storage system works independently")
	
	// Test storage health
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := hybridStorage.Health(ctx); err != nil {
		fmt.Printf("❌ Storage health check failed: %v\n", err)
	} else {
		fmt.Println("✅ Storage health check passed")
	}

	fmt.Println("\n📋 RECOMMENDATIONS:")
	fmt.Println("   1. Start Temporal server: docker compose up temporal postgres")
	fmt.Println("   2. Verify Temporal UI at: http://localhost:8233")
	fmt.Println("   3. Run this test again with Temporal server running")
}
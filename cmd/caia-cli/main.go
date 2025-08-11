package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"go.temporal.io/sdk/client"
)

func main() {
	if len(os.Args) < 2 {
		showHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "ingest":
		if len(os.Args) < 3 {
			fmt.Println("❌ Usage: caia-cli ingest <url> [type]")
			os.Exit(1)
		}
		url := os.Args[2]
		docType := "html"
		if len(os.Args) > 3 {
			docType = os.Args[3]
		}
		ingestDocument(url, docType)

	case "list":
		listWorkflows()

	case "show":
		if len(os.Args) < 3 {
			fmt.Println("❌ Usage: caia-cli show <workflow-id>")
			os.Exit(1)
		}
		showWorkflow(os.Args[2])

	case "batch":
		if len(os.Args) < 3 {
			fmt.Println("❌ Usage: caia-cli batch <url1,url2,url3>")
			os.Exit(1)
		}
		urls := parseURLList(os.Args[2])
		batchIngest(urls)

	default:
		showHelp()
	}
}

func ingestDocument(url, docType string) {
	fmt.Printf("🔄 Ingesting document: %s (type: %s)\n", url, docType)
	
	// Connect to Temporal
	temporalClient, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to Temporal: %v", err)
	}
	defer temporalClient.Close()

	// Create workflow input
	input := workflows.DocumentInput{
		URL:  url,
		Type: docType,
		Metadata: map[string]string{
			"source":      "caia-cli",
			"triggered_at": time.Now().Format(time.RFC3339),
		},
	}

	// Start workflow
	workflowID := fmt.Sprintf("cli-ingest-%d", time.Now().Unix())
	workflowRun, err := temporalClient.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "caia-library",
		},
		workflows.DocumentIngestionWorkflow,
		input,
	)
	if err != nil {
		log.Fatalf("❌ Failed to start workflow: %v", err)
	}

	fmt.Printf("✅ Workflow started: %s\n", workflowRun.GetID())
	fmt.Printf("   Waiting for completion...\n")

	// Wait for result
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err = workflowRun.Get(ctx, nil)
	if err != nil {
		fmt.Printf("❌ Workflow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🎉 Document successfully ingested!\n")
	fmt.Printf("   Workflow ID: %s\n", workflowRun.GetID())
}

func batchIngest(urls []string) {
	fmt.Printf("🔄 Starting batch ingestion of %d URLs\n", len(urls))
	
	// Connect to Temporal
	temporalClient, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to Temporal: %v", err)
	}
	defer temporalClient.Close()

	// Create batch input
	var documents []workflows.DocumentInput
	for _, url := range urls {
		documents = append(documents, workflows.DocumentInput{
			URL:  url,
			Type: "html",
			Metadata: map[string]string{
				"source":      "caia-cli-batch",
				"triggered_at": time.Now().Format(time.RFC3339),
			},
		})
	}

	// Start batch workflow
	workflowID := fmt.Sprintf("cli-batch-%d", time.Now().Unix())
	workflowRun, err := temporalClient.ExecuteWorkflow(
		context.Background(),
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "caia-library",
		},
		workflows.BatchIngestionWorkflow,
		documents,
	)
	if err != nil {
		log.Fatalf("❌ Failed to start batch workflow: %v", err)
	}

	fmt.Printf("✅ Batch workflow started: %s\n", workflowRun.GetID())
	fmt.Printf("   Processing %d documents...\n", len(urls))

	// Wait for result
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = workflowRun.Get(ctx, nil)
	if err != nil {
		fmt.Printf("❌ Batch workflow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🎉 Batch ingestion completed!\n")
	fmt.Printf("   Workflow ID: %s\n", workflowRun.GetID())
}

func listWorkflows() {
	fmt.Println("📋 Recent workflows:")
	fmt.Println("   (Use 'temporal workflow list' for full details)")
	// This would require implementing workflow listing
	// For now, suggest using temporal CLI directly
}

func showWorkflow(workflowID string) {
	fmt.Printf("🔍 Workflow details: %s\n", workflowID)
	fmt.Printf("   (Use 'temporal workflow show --workflow-id %s' for full details)\n", workflowID)
}

func parseURLList(urlString string) []string {
	// Simple comma-separated parsing
	urls := make([]string, 0)
	for _, url := range splitString(urlString, ",") {
		if trimmed := trimSpace(url); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}

func splitString(s, sep string) []string {
	// Simple split implementation
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i < len(s)-len(sep)+1 && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	// Simple trim implementation
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func showHelp() {
	fmt.Println("🔧 CAIA Library CLI")
	fmt.Println("===================")
	fmt.Println("")
	fmt.Println("Usage: caia-cli [command] [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  ingest <url> [type]     - Ingest a single document")
	fmt.Println("  batch <url1,url2,url3>  - Batch ingest multiple documents")
	fmt.Println("  list                    - List recent workflows")
	fmt.Println("  show <workflow-id>      - Show workflow details")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  caia-cli ingest https://go.dev html")
	fmt.Println("  caia-cli batch https://go.dev,https://golang.org")
	fmt.Println("  caia-cli show cli-ingest-1234567890")
	fmt.Println("")
	fmt.Println("Requirements:")
	fmt.Println("  - Temporal server running on localhost:7233")
	fmt.Println("  - CAIA Library worker running")
	fmt.Println("")
}
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/pipeline"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/caiatech/govc"
	"github.com/rs/zerolog/log"
)

// GovcBackend implements StorageBackend using embedded govc library
type GovcBackend struct {
	// govc repository instance
	repo             *govc.Repository
	repoPath         string
	metricsCollector MetricsCollector
	
	// Document index for fast lookups
	docIndex         *DocumentIndex
	
	// Event bus for real-time processing
	eventBus         *pipeline.EventBus
}

// GovcConfig holds configuration for govc
type GovcConfig struct {
	MemoryMode bool          // Use pure memory mode
	Path       string        // Repository path (":memory:" for in-memory)
	Timeout    time.Duration // Operation timeout
}

// NewGovcBackend creates a new govc-based storage backend
func NewGovcBackend(repoName string, metrics MetricsCollector) (*GovcBackend, error) {
	// Default configuration - use memory mode for maximum performance
	config := &GovcConfig{
		MemoryMode: true,
		Path:       ":memory:",
		Timeout:    30 * time.Second,
	}

	return NewGovcBackendWithConfig(repoName, config, metrics)
}

// NewGovcBackendWithConfig creates a new govc backend with explicit configuration
func NewGovcBackendWithConfig(repoName string, config *GovcConfig, metrics MetricsCollector) (*GovcBackend, error) {
	var repo *govc.Repository
	var err error
	
	repoPath := config.Path
	if !config.MemoryMode && repoPath == "" {
		repoPath = filepath.Join("./data/govc", repoName)
	}
	
	if config.MemoryMode || repoPath == ":memory:" {
		// Create pure in-memory repository
		repo = govc.New()
		log.Info().
			Str("repo", repoName).
			Msg("Created in-memory govc repository")
	} else {
		// Try to open existing repository or create new one
		repo, err = govc.Open(repoPath)
		if err != nil {
			// Repository doesn't exist, create it
			repo, err = govc.Init(repoPath)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize govc repository: %w", err)
			}
			log.Info().
				Str("path", repoPath).
				Msg("Initialized new govc repository")
		} else {
			log.Info().
				Str("path", repoPath).
				Msg("Opened existing govc repository")
		}
	}
	
	// Create initial commit if repository is empty
	commits, err := repo.Log(1)
	if err != nil || len(commits) == 0 {
		// Create initial commit
		err = repo.WriteFile("README.md", []byte("# CAIA Library Document Repository\n"))
		if err == nil {
			_, err = repo.Commit("Initial commit")
			if err != nil {
				log.Warn().Err(err).Msg("Failed to create initial commit")
			}
		}
	}

	backend := &GovcBackend{
		repo:             repo,
		repoPath:         repoPath,
		metricsCollector: metrics,
		docIndex:         NewDocumentIndex(),
		eventBus:         pipeline.NewEventBus(1000, 4), // Large buffer, 4 workers
	}
	
	// Build initial index from existing documents
	if err := backend.buildIndex(); err != nil {
		log.Warn().Err(err).Msg("Failed to build initial document index")
	}
	
	return backend, nil
}

func (g *GovcBackend) StoreDocument(ctx context.Context, doc *document.Document) (string, error) {
	start := time.Now()
	
	// Validate document before storing
	if err := doc.Validate(); err != nil {
		g.recordMetric("store", start, false, err)
		return "", fmt.Errorf("document validation failed: %w", err)
	}
	
	// Prepare document paths
	docPath := doc.GitPath()
	
	// Store document metadata as JSON
	metadata, err := json.Marshal(map[string]interface{}{
		"id":         doc.ID,
		"source":     doc.Source,
		"created_at": doc.CreatedAt,
		"updated_at": doc.UpdatedAt,
		"metadata":   doc.Content.Metadata,
	})
	if err != nil {
		g.recordMetric("store", start, false, err)
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// Use govc's atomic multi-file update for efficiency
	files := make(map[string][]byte)
	files[fmt.Sprintf("%s/metadata.json", docPath)] = metadata
	files[fmt.Sprintf("%s/content.txt", docPath)] = []byte(doc.Content.Text)
	
	if len(doc.Content.Raw) > 0 {
		files[fmt.Sprintf("%s/raw", docPath)] = doc.Content.Raw
	}
	
	// Perform atomic commit with all files
	commit, err := g.repo.AtomicMultiFileUpdate(
		files,
		fmt.Sprintf("Add document %s", doc.ID),
	)
	if err != nil {
		g.recordMetric("store", start, false, err)
		return "", fmt.Errorf("failed to store document: %w", err)
	}
	
	commitHash := commit.Hash()
	
	// Add to index for fast retrieval
	metadataPath := fmt.Sprintf("%s/metadata.json", docPath)
	g.docIndex.Add(doc.ID, metadataPath, &DocumentMetadata{
		ID:        doc.ID,
		Path:      metadataPath,
		Type:      doc.Source.Type,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	})
	
	log.Debug().
		Str("document_id", doc.ID).
		Str("commit", commitHash).
		Msg("Document stored in govc repository")
	
	// Emit document added event for real-time processing
	event := pipeline.NewDocumentEvent(pipeline.EventDocumentAdded, doc)
	event.Metadata["commit_hash"] = commitHash
	event.Metadata["backend"] = "govc"
	
	if err := g.eventBus.Publish(event); err != nil {
		log.Warn().Err(err).Str("document_id", doc.ID).Msg("Failed to publish document event")
	}
	
	g.recordMetric("store", start, true, nil)
	return commitHash, nil
}

func (g *GovcBackend) GetDocument(ctx context.Context, id string) (*document.Document, error) {
	start := time.Now()
	
	// First check the index for O(1) lookup
	metadataPath, exists := g.docIndex.Get(id)
	
	if !exists {
		// Fallback to searching if not in index (shouldn't happen after initial build)
		searchPaths := []string{
			fmt.Sprintf("documents/*/*/*/%s/metadata.json", id),
			fmt.Sprintf("documents/*/*/%s/metadata.json", id),
			fmt.Sprintf("documents/*/%s/metadata.json", id),
		}
		
		// Also try paths based on how we store documents (GitPath format)
		now := time.Now()
		for _, docType := range []string{"text", "html", "pdf", "json", "markdown", "xml", "csv"} {
			// Try current month and previous months
			for i := 0; i < 12; i++ {
				checkDate := now.AddDate(0, -i, 0)
				datePath := checkDate.Format("2006/01")
				searchPaths = append(searchPaths, fmt.Sprintf("documents/%s/%s/%s/metadata.json", docType, datePath, id))
			}
		}
		
		for _, path := range searchPaths {
			// Try to read the file directly
			if _, err := g.repo.ReadFile(path); err == nil {
				metadataPath = path
				// Add to index for next time
				g.docIndex.Add(id, path, nil)
				break
			}
		}
	}
	
	if metadataPath == "" {
		err := fmt.Errorf("document not found: %s", id)
		g.recordMetric("get", start, false, err)
		return nil, err
	}
	
	// Extract document directory from metadata path
	docDir := filepath.Dir(metadataPath)
	
	// Read metadata
	metadataBytes, err := g.repo.ReadFile(metadataPath)
	if err != nil {
		g.recordMetric("get", start, false, err)
		// If metadata file cannot be read, remove from index and return error
		g.docIndex.Remove(id)
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		g.recordMetric("get", start, false, err)
		// Remove corrupted entry from index
		g.docIndex.Remove(id)
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}
	
	// Create document from metadata
	doc := &document.Document{
		ID: id,
		Content: document.Content{
			Metadata: make(map[string]string),
		},
	}
	
	// Parse source information
	if source, ok := metadata["source"].(map[string]interface{}); ok {
		if sourceType, ok := source["type"].(string); ok {
			doc.Source.Type = sourceType
		}
		if url, ok := source["url"].(string); ok {
			doc.Source.URL = url
		}
		if path, ok := source["path"].(string); ok {
			doc.Source.Path = path
		}
	}
	
	// Parse timestamps
	if createdAt, ok := metadata["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			doc.CreatedAt = t
		}
	}
	if updatedAt, ok := metadata["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			doc.UpdatedAt = t
		}
	}
	
	// Parse content metadata
	if contentMeta, ok := metadata["metadata"].(map[string]interface{}); ok {
		for k, v := range contentMeta {
			if str, ok := v.(string); ok {
				doc.Content.Metadata[k] = str
			}
		}
	}
	
	// Read text content
	contentPath := filepath.Join(docDir, "content.txt")
	if contentBytes, err := g.repo.ReadFile(contentPath); err == nil {
		doc.Content.Text = string(contentBytes)
	}
	
	// Read raw content if exists
	rawPath := filepath.Join(docDir, "raw")
	if rawBytes, err := g.repo.ReadFile(rawPath); err == nil {
		doc.Content.Raw = rawBytes
	}
	
	g.recordMetric("get", start, true, nil)
	return doc, nil
}

func (g *GovcBackend) MergeBranch(ctx context.Context, branchName string) error {
	start := time.Now()
	
	// Get current branch to restore after merge
	currentBranch, err := g.repo.CurrentBranch()
	if err != nil {
		g.recordMetric("merge", start, false, err)
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	
	// Perform merge using govc
	err = g.repo.Merge(branchName, currentBranch)
	if err != nil {
		g.recordMetric("merge", start, false, err)
		return fmt.Errorf("merge failed: %w", err)
	}
	
	log.Debug().
		Str("branch", branchName).
		Str("into", currentBranch).
		Msg("Branch merged in govc repository")
	
	g.recordMetric("merge", start, true, nil)
	return nil
}

func (g *GovcBackend) ListDocuments(ctx context.Context, filters map[string]string) ([]*document.Document, error) {
	start := time.Now()
	
	// List all files in the repository
	allFiles, err := g.repo.ListFiles()
	if err != nil {
		g.recordMetric("list", start, false, err)
		return []*document.Document{}, nil
	}
	
	// Find metadata.json files
	metadataPaths := []string{}
	for _, file := range allFiles {
		if filepath.Base(file) == "metadata.json" && strings.HasPrefix(file, "documents/") {
			metadataPaths = append(metadataPaths, file)
		}
	}
	
	if len(metadataPaths) == 0 {
		g.recordMetric("list", start, true, nil)
		return []*document.Document{}, nil // Return empty list if no documents
	}
	
	documents := make([]*document.Document, 0)
	
	for _, metadataPath := range metadataPaths {
		// Read metadata file
		metadataBytes, err := g.repo.ReadFile(metadataPath)
		if err != nil {
			continue
		}
		
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			continue
		}
		
		// Extract document ID
		id, ok := metadata["id"].(string)
		if !ok {
			continue
		}
		
		// Apply filters
		matches := true
		for key, value := range filters {
			switch key {
			case "type":
				if source, ok := metadata["source"].(map[string]interface{}); ok {
					if docType, ok := source["type"].(string); ok {
						if docType != value {
							matches = false
						}
					}
				}
			case "source":
				if source, ok := metadata["source"].(map[string]interface{}); ok {
					if url, ok := source["url"].(string); ok {
						if url != value {
							matches = false
						}
					}
				}
			}
			if !matches {
				break
			}
		}
		
		if matches {
			// Load full document
			doc, err := g.GetDocument(ctx, id)
			if err == nil && doc != nil {
				documents = append(documents, doc)
			}
		}
	}
	
	g.recordMetric("list", start, true, nil)
	return documents, nil
}

func (g *GovcBackend) Health(ctx context.Context) error {
	start := time.Now()
	
	// Check repository health by attempting to get current commit
	_, err := g.repo.CurrentCommit()
	if err != nil {
		g.recordMetric("health", start, false, err)
		return fmt.Errorf("repository unhealthy: %w", err)
	}
	
	g.recordMetric("health", start, true, nil)
	return nil
}

// GetMemoryStats returns current memory usage statistics
// This is specific to govc and helps with integration testing
func (g *GovcBackend) GetMemoryStats() map[string]interface{} {
	// Get repository memory statistics
	memStats := g.repo.GetMemoryUsage()
	
	return map[string]interface{}{
		"implementation":      "embedded-library",
		"govc_integrated":     true,
		"repo_path":           g.repoPath,
		"memory_mode":         g.repoPath == ":memory:",
		"total_objects":       memStats.TotalObjects,
		"total_bytes":         memStats.TotalBytes,
		"compacted_bytes":     memStats.CompactedBytes,
		"fragment_ratio":      memStats.FragmentRatio,
		"indexed_documents":   g.docIndex.Size(),
	}
}

func (g *GovcBackend) recordMetric(operation string, start time.Time, success bool, err error) {
	if g.metricsCollector != nil {
		g.metricsCollector.RecordMetric(StorageMetrics{
			OperationType: operation,
			Duration:      time.Since(start).Nanoseconds(),
			Success:       success,
			Backend:       "govc",
			Error:         err,
		})
	}
}

// buildIndex builds the document index from existing repository files
func (g *GovcBackend) buildIndex() error {
	// List all files in the repository
	allFiles, err := g.repo.ListFiles()
	if err != nil {
		// If listing fails, index will be built incrementally
		return nil
	}
	
	// Find all metadata.json files and add to index
	for _, file := range allFiles {
		if filepath.Base(file) == "metadata.json" && strings.HasPrefix(file, "documents/") {
			// Extract document ID from path
			dir := filepath.Dir(file)
			parts := strings.Split(dir, "/")
			if len(parts) > 0 {
				docID := parts[len(parts)-1]
				g.docIndex.Add(docID, file, nil)
			}
		}
	}
	
	log.Info().
		Int("indexed_documents", g.docIndex.Size()).
		Msg("Built document index")
	
	return nil
}

// GetEventBus returns the event bus for external subscribers
func (g *GovcBackend) GetEventBus() *pipeline.EventBus {
	return g.eventBus
}

// GetDocumentIndex returns the document index for direct access
// This allows optimized queries that leverage O(1) document lookups
func (g *GovcBackend) GetDocumentIndex() *DocumentIndex {
	return g.docIndex
}

// Close closes the backend and event bus
func (g *GovcBackend) Close() {
	if g.eventBus != nil {
		g.eventBus.Close()
	}
}
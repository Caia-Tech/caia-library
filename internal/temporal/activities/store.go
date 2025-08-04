package activities

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Caia-Tech/caia-library/internal/git"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
)

func StoreDocumentActivity(ctx context.Context, input workflows.StoreInput) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Storing document", "url", input.URL, "type", input.Type)

	// Get repository path from environment or default
	repoPath := os.Getenv("CAIA_REPO_PATH")
	if repoPath == "" {
		repoPath = "/tmp/caia-library-repo" // Default for testing
	}

	repo, err := git.NewRepository(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	// Create document
	doc := &document.Document{
		ID: uuid.New().String(),
		Source: document.Source{
			Type: input.Type,
			URL:  input.URL,
		},
		Content: document.Content{
			Raw:        input.Content,
			Text:       input.Text,
			Metadata:   input.Metadata,
			Embeddings: input.Embeddings,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	commitHash, err := repo.StoreDocument(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("failed to store document: %w", err)
	}

	logger.Info("Document stored successfully", "documentID", doc.ID, "commitHash", commitHash)
	return commitHash, nil
}
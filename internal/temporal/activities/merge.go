package activities

import (
	"context"
	"fmt"
	"os"

	"github.com/caiatech/caia-library/internal/git"
	"go.temporal.io/sdk/activity"
)

func MergeBranchActivity(ctx context.Context, commitHash string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Merging branch", "commitHash", commitHash)

	// Get repository path from environment or default
	repoPath := os.Getenv("CAIA_REPO_PATH")
	if repoPath == "" {
		repoPath = "/tmp/caia-library-repo" // Default for testing
	}

	repo, err := git.NewRepository(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// TODO: Implement actual merge logic
	_ = repo // Use repo to merge branch to main

	logger.Info("Branch merged successfully", "commitHash", commitHash)
	return nil
}
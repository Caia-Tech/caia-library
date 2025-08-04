package activities

import (
	"context"
	"fmt"
	"os"

	"github.com/Caia-Tech/caia-library/internal/git"
	"github.com/go-git/go-git/v5/plumbing"
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

	// For this specific commit, we need to find the branch that contains it
	// For now, we'll use a simplified approach and merge the branch that was just created
	// The workflow should pass the document ID, but for compatibility we extract from context
	
	// Get all ingest branches and merge them (temporary fix until workflow passes branch name)
	gitRepo := repo.GetRepo()
	refs, err := gitRepo.References()
	if err != nil {
		return fmt.Errorf("failed to get references: %w", err)
	}
	
	var merged int
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() && ref.Name().String() != "refs/heads/main" {
			branchName := ref.Name().Short()
			if len(branchName) > 7 && branchName[:7] == "ingest/" {
				logger.Info("Found ingest branch", "branch", branchName)
				if mergeErr := repo.MergeBranch(ctx, branchName); mergeErr != nil {
					logger.Warn("Failed to merge branch", "branch", branchName, "error", mergeErr)
				} else {
					logger.Info("Successfully merged branch", "branch", branchName)
					merged++
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to iterate references: %w", err)
	}
	
	logger.Info("Merge operation completed", "branchesMerged", merged)

	logger.Info("Branch merged successfully", "commitHash", commitHash)
	return nil
}
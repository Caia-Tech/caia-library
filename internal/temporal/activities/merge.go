package activities

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

func MergeBranchActivity(ctx context.Context, branchName string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Merging branch", "branch", branchName)

	if globalHybridStorage == nil {
		return fmt.Errorf("hybrid storage not initialized")
	}

	err := globalHybridStorage.MergeBranch(ctx, branchName)
	if err != nil {
		return fmt.Errorf("failed to merge branch: %w", err)
	}

	logger.Info("Branch merged successfully", "branch", branchName)
	return nil
}
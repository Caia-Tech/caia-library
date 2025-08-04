package activities

import (
	"context"
	"fmt"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"go.temporal.io/sdk/activity"
)

func ExtractTextActivity(ctx context.Context, input workflows.ExtractInput) (workflows.ExtractResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Extracting text", "type", input.Type, "contentSize", len(input.Content))

	engine := extractor.NewEngine()
	
	text, metadata, err := engine.Extract(ctx, input.Content, input.Type)
	if err != nil {
		return workflows.ExtractResult{}, fmt.Errorf("failed to extract text: %w", err)
	}

	logger.Info("Text extracted successfully", "textLength", len(text), "metadataCount", len(metadata))
	return workflows.ExtractResult{
		Text:     text,
		Metadata: metadata,
	}, nil
}
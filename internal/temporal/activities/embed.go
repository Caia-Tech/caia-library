package activities

import (
	"context"
	"fmt"

	"github.com/Caia-Tech/caia-library/pkg/embedder"
	"go.temporal.io/sdk/activity"
)

func GenerateEmbeddingsActivity(ctx context.Context, content []byte) ([]float32, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating embeddings", "contentSize", len(content))

	engine, err := embedder.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	embeddings, err := engine.Generate(ctx, string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	logger.Info("Embeddings generated successfully", "dimensions", len(embeddings))
	return embeddings, nil
}
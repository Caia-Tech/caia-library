package embedder

import (
	"context"
)

type Engine struct {
	embedder *AdvancedEmbedder
}

func NewEngine() (*Engine, error) {
	// Use advanced embedder with 384 dimensions (same as all-MiniLM-L6-v2)
	return &Engine{
		embedder: NewAdvancedEmbedder(384),
	}, nil
}

func (e *Engine) Generate(ctx context.Context, text string) ([]float32, error) {
	return e.embedder.Generate(ctx, text)
}
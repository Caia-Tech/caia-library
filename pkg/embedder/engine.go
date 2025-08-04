package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strings"
)

type Engine struct {
	// In production, this would use ONNX runtime or similar
	// For now, we'll use a simple hash-based approach
}

func NewEngine() (*Engine, error) {
	// TODO: Initialize ONNX runtime with a real model
	return &Engine{}, nil
}

func (e *Engine) Generate(ctx context.Context, text string) ([]float32, error) {
	// Simple embedding generation for demo purposes
	// In production, use a real embedding model via ONNX
	
	// Normalize text
	text = strings.ToLower(strings.TrimSpace(text))
	
	// Generate hash-based embeddings (384 dimensions to match common models)
	dimensions := 384
	embeddings := make([]float32, dimensions)
	
	// Create multiple hashes for different aspects
	words := strings.Fields(text)
	
	for i := 0; i < dimensions; i++ {
		// Combine word count, char count, and hash for each dimension
		seed := []byte(text)
		seed = append(seed, byte(i))
		
		hash := sha256.Sum256(seed)
		
		// Convert hash bytes to float
		value := binary.BigEndian.Uint32(hash[:4])
		
		// Normalize to [-1, 1] range
		embeddings[i] = (float32(value)/float32(math.MaxUint32))*2 - 1
		
		// Add some variation based on text features
		if i < len(words) {
			embeddings[i] *= float32(len(words[i])) / 10.0
		}
	}
	
	// Normalize the vector
	var sum float32
	for _, v := range embeddings {
		sum += v * v
	}
	
	norm := float32(math.Sqrt(float64(sum)))
	if norm > 0 {
		for i := range embeddings {
			embeddings[i] /= norm
		}
	}
	
	return embeddings, nil
}
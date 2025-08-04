package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Caia-Tech/caia-library/pkg/embedder"
)

func main() {
	// Create embedder
	engine, err := embedder.NewEngine()
	if err != nil {
		fmt.Printf("Error creating embedder: %v\n", err)
		os.Exit(1)
	}

	// Test texts
	texts := []string{
		"Machine learning is a subset of artificial intelligence.",
		"AI and machine learning are related fields in computer science.",
		"The weather today is sunny and warm.",
		"Caia Library uses Git for cryptographic provenance.",
		"Document intelligence systems need trustworthy data storage.",
	}

	// Generate embeddings
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := engine.Generate(context.Background(), text)
		if err != nil {
			fmt.Printf("Error generating embedding: %v\n", err)
			os.Exit(1)
		}
		embeddings[i] = emb
		fmt.Printf("Text %d: %s\n", i+1, text)
		fmt.Printf("Embedding (first 10 dims): %.3f\n\n", emb[:10])
	}

	// Calculate similarities
	fmt.Println("=== Similarity Matrix ===")
	fmt.Print("     ")
	for i := range texts {
		fmt.Printf("  %d   ", i+1)
	}
	fmt.Println()

	for i := range texts {
		fmt.Printf("%d    ", i+1)
		for j := range texts {
			similarity := embedder.CosineSimilarity(embeddings[i], embeddings[j])
			fmt.Printf("%.3f ", similarity)
		}
		fmt.Println()
	}

	fmt.Println("\nInterpretation:")
	fmt.Println("- Texts 1 & 2 (ML/AI topics) should have high similarity")
	fmt.Println("- Text 3 (weather) should have low similarity with others")
	fmt.Println("- Texts 4 & 5 (document systems) should have moderate similarity")
}
package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// AdvancedEmbedder generates more sophisticated embeddings without external dependencies
type AdvancedEmbedder struct {
	Dimensions int
	engine     *Engine
}

// NewAdvancedEmbedder creates a new advanced embedder
func NewAdvancedEmbedder(dimensions int) *AdvancedEmbedder {
	return &AdvancedEmbedder{
		Dimensions: dimensions,
	}
}

// Generate creates embeddings using advanced hashing techniques
func (e *AdvancedEmbedder) Generate(ctx context.Context, text string) ([]float32, error) {
	// Check for empty text
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}
	
	// Preprocess text
	text = normalizeText(text)
	
	// Extract features
	features := extractFeatures(text)
	
	// Generate embeddings
	embeddings := make([]float32, e.Dimensions)
	
	// Use multiple hash functions for different aspects
	for i := 0; i < e.Dimensions; i++ {
		// Combine multiple features
		value := float32(0)
		
		// Character n-grams
		if i < len(features.charTrigrams) {
			value += features.charTrigrams[i]
		}
		
		// Word frequencies
		if len(features.wordFreqs) > 0 {
			wordIdx := i % len(features.wordFreqs)
			value += features.wordFreqs[wordIdx] * 0.5
		}
		
		// Positional encoding
		position := float32(i) / float32(e.Dimensions)
		value += float32(math.Sin(float64(position * math.Pi)))
		
		// Hash-based component for uniqueness
		hash := sha256.Sum256([]byte(text + string(rune(i))))
		hashValue := binary.BigEndian.Uint32(hash[:4])
		value += (float32(hashValue)/float32(math.MaxUint32) - 0.5) * 0.3
		
		embeddings[i] = value
	}
	
	// Normalize to unit vector
	return normalize(embeddings), nil
}

// Features extracted from text
type textFeatures struct {
	charTrigrams []float32
	wordFreqs    []float32
	avgWordLen   float32
	uniqueRatio  float32
}

// extractFeatures analyzes text and extracts relevant features
func extractFeatures(text string) textFeatures {
	features := textFeatures{}
	
	// Character trigrams with TF-IDF-like scoring
	trigrams := make(map[string]int)
	runes := []rune(text)
	for i := 0; i < len(runes)-2; i++ {
		trigram := string(runes[i : i+3])
		trigrams[trigram]++
	}
	
	// Convert to sorted list for consistency
	var trigramScores []float32
	for _, count := range trigrams {
		// Simple IDF approximation based on trigram rarity
		idf := 1.0 / (1.0 + math.Log(float64(count)))
		score := float32(count) * float32(idf)
		trigramScores = append(trigramScores, score)
	}
	sort.Slice(trigramScores, func(i, j int) bool {
		return trigramScores[i] < trigramScores[j]
	})
	features.charTrigrams = trigramScores
	
	// Word frequency analysis
	words := tokenize(text)
	wordCounts := make(map[string]int)
	totalWords := 0
	totalLength := 0
	
	for _, word := range words {
		wordCounts[word]++
		totalWords++
		totalLength += len(word)
	}
	
	// Calculate word frequencies
	var wordFreqs []float32
	for _, count := range wordCounts {
		freq := float32(count) / float32(totalWords)
		wordFreqs = append(wordFreqs, freq)
	}
	sort.Slice(wordFreqs, func(i, j int) bool {
		return wordFreqs[i] < wordFreqs[j]
	})
	features.wordFreqs = wordFreqs
	
	// Average word length
	if totalWords > 0 {
		features.avgWordLen = float32(totalLength) / float32(totalWords)
	}
	
	// Unique word ratio
	if totalWords > 0 {
		features.uniqueRatio = float32(len(wordCounts)) / float32(totalWords)
	}
	
	return features
}

// normalizeText preprocesses text for embedding generation
func normalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)
	
	// Remove excessive whitespace
	text = strings.Join(strings.Fields(text), " ")
	
	return text
}

// tokenize splits text into words
func tokenize(text string) []string {
	var words []string
	var currentWord strings.Builder
	
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			currentWord.WriteRune(r)
		} else if currentWord.Len() > 0 {
			words = append(words, currentWord.String())
			currentWord.Reset()
		}
	}
	
	if currentWord.Len() > 0 {
		words = append(words, currentWord.String())
	}
	
	return words
}

// normalize converts embeddings to unit vector
func normalize(embeddings []float32) []float32 {
	// Calculate magnitude
	var sum float32
	for _, v := range embeddings {
		sum += v * v
	}
	
	// Avoid division by zero
	if sum == 0 {
		return embeddings
	}
	
	// Normalize
	magnitude := float32(math.Sqrt(float64(sum)))
	result := make([]float32, len(embeddings))
	for i, v := range embeddings {
		result[i] = v / magnitude
	}
	
	return result
}

// CosineSimilarity calculates similarity between two embeddings
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct float32
	for i := range a {
		dotProduct += a[i] * b[i]
	}
	
	// Assuming both vectors are normalized
	return dotProduct
}
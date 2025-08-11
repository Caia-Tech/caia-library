package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"github.com/Caia-Tech/caia-library/pkg/logging"
	"github.com/Caia-Tech/caia-library/pkg/pipeline"
)

type QualityContent struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata"`
	WordCount   int               `json:"word_count"`
	Quality     string            `json:"quality"`
	ExtractedAt string            `json:"extracted_at"`
}

type QualityDataset struct {
	Sources   []QualityContent `json:"sources"`
	Metadata  DatasetMetadata  `json:"metadata"`
	CreatedAt string           `json:"created_at"`
}

type DatasetMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TotalItems  int    `json:"total_items"`
	TotalWords  int    `json:"total_words"`
}

func main() {
	fmt.Println("🔧 HIGH-QUALITY CONTENT RE-SCRAPER")
	fmt.Println("==================================")
	fmt.Println("Properly extracting content with improved HTML parser")
	fmt.Println()

	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "warn"
	
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	// Test sources for quality extraction
	sources := []struct {
		URL   string
		Title string
	}{
		{
			URL:   "https://go.dev/doc/effective_go",
			Title: "Effective Go",
		},
		{
			URL:   "https://go.dev/doc/tutorial/getting-started",
			Title: "Getting Started with Go",
		},
		{
			URL:   "https://en.wikipedia.org/wiki/Machine_learning",
			Title: "Machine Learning",
		},
	}

	extractor := extractor.NewEngine()
	ctx := context.Background()
	
	var qualityContent []QualityContent
	totalWords := 0

	fmt.Println("📥 Extracting quality content...")
	for i, source := range sources {
		fmt.Printf("   [%d/%d] %s\n", i+1, len(sources), source.Title)
		
		// Fetch content
		content, err := fetchURL(source.URL)
		if err != nil {
			fmt.Printf("        ❌ Failed to fetch: %v\n", err)
			continue
		}

		// Extract with improved parser
		text, metadata, err := extractor.Extract(ctx, content, "html")
		if err != nil {
			fmt.Printf("        ❌ Failed to extract: %v\n", err)
			continue
		}

		// Quality check
		wordCount := len(strings.Fields(text))
		quality := "unknown"
		if wordCount > 2000 {
			quality = "high"
		} else if wordCount > 500 {
			quality = "medium"
		} else {
			quality = "low"
		}

		fmt.Printf("        ✅ Extracted: %d words (quality: %s)\n", wordCount, quality)
		
		// Show sample of extracted text
		sample := text
		if len(sample) > 200 {
			sample = sample[:200] + "..."
		}
		fmt.Printf("        Sample: %s\n", sample)

		qc := QualityContent{
			URL:         source.URL,
			Title:       source.Title,
			Content:     text,
			Metadata:    metadata,
			WordCount:   wordCount,
			Quality:     quality,
			ExtractedAt: time.Now().UTC().Format(time.RFC3339),
		}

		qualityContent = append(qualityContent, qc)
		totalWords += wordCount

		// Be respectful
		if i < len(sources)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	// Create dataset
	dataset := QualityDataset{
		Sources: qualityContent,
		Metadata: DatasetMetadata{
			Name:        "High-Quality Extracted Content",
			Description: "Properly extracted content with improved HTML parser",
			TotalItems:  len(qualityContent),
			TotalWords:  totalWords,
		},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Save dataset
	outputFile := "quality_extracted_content.json"
	fmt.Printf("\n💾 Saving to %s...\n", outputFile)
	
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("❌ Failed to create file: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(dataset); err != nil {
		fmt.Printf("❌ Failed to encode JSON: %v\n", err)
		return
	}

	fmt.Println("\n✅ QUALITY EXTRACTION COMPLETE")
	fmt.Printf("   • Total sources: %d\n", len(qualityContent))
	fmt.Printf("   • Total words: %d\n", totalWords)
	fmt.Printf("   • Average words/source: %d\n", totalWords/len(qualityContent))
	
	// Show quality distribution
	qualityDist := make(map[string]int)
	for _, qc := range qualityContent {
		qualityDist[qc.Quality]++
	}
	fmt.Println("   • Quality distribution:")
	for q, count := range qualityDist {
		fmt.Printf("     - %s: %d\n", q, count)
	}
}

func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "CAIA-Quality-Scraper/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	// Limit to 10MB
	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
	return io.ReadAll(limitedReader)
}
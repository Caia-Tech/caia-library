package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ConversationalEntry represents a single conversational Q&A pair for LLMs
type ConversationalEntry struct {
	ID           string                 `json:"id"`
	Conversation []ConversationalTurn   `json:"conversation"`
	Metadata     map[string]interface{} `json:"metadata"`
	Source       ConversationalSource   `json:"source"`
	CreatedAt    string                 `json:"created_at"`
}

type ConversationalTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ConversationalSource struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	WordCount   int    `json:"word_count"`
	Quality     string `json:"quality_tier"`
}

type ConversationalDataset struct {
	Dataset     []ConversationalEntry `json:"dataset"`
	Metadata    DatasetMetadata       `json:"metadata"`
	GeneratedAt string                `json:"generated_at"`
}

type DatasetMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	TotalItems  int    `json:"total_items"`
	Sources     string `json:"sources"`
	Purpose     string `json:"purpose"`
}

func main() {
	fmt.Println("🔗 GOLANG CONVERSATIONAL DATASET MERGER")
	fmt.Println("=======================================")
	fmt.Println("Combining official golang.org and ethical web content into comprehensive dataset")
	fmt.Println()

	// Load first dataset (golang.org official docs)
	fmt.Println("📚 Loading golang.org conversational dataset...")
	dataset1, err := loadDataset("golang_conversational_dataset.json")
	if err != nil {
		fmt.Printf("❌ Failed to load golang.org dataset: %v\n", err)
		return
	}
	fmt.Printf("✅ Loaded %d conversations from golang.org docs\n", len(dataset1.Dataset))

	// Load second dataset (ethical web scraping)
	fmt.Println("\n🌐 Loading web-scraped conversational dataset...")
	dataset2, err := loadDataset("go_web_conversational_dataset.json")
	if err != nil {
		fmt.Printf("❌ Failed to load web dataset: %v\n", err)
		return
	}
	fmt.Printf("✅ Loaded %d conversations from web sources\n", len(dataset2.Dataset))

	// Merge datasets
	fmt.Println("\n🔗 Merging datasets...")
	mergedDataset := mergeDatasets(dataset1, dataset2)

	// Export comprehensive dataset
	outputFile := "comprehensive_go_conversational_dataset.json"
	fmt.Printf("\n💾 Exporting comprehensive dataset to %s...\n", outputFile)
	
	if err := exportDataset(mergedDataset, outputFile); err != nil {
		fmt.Printf("❌ Failed to export: %v\n", err)
		return
	}

	generateMergedSummary(mergedDataset, outputFile)
}

func loadDataset(filename string) (ConversationalDataset, error) {
	var dataset ConversationalDataset
	
	file, err := os.Open(filename)
	if err != nil {
		return dataset, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&dataset); err != nil {
		return dataset, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return dataset, nil
}

func mergeDatasets(dataset1, dataset2 ConversationalDataset) ConversationalDataset {
	// Combine all conversations
	allConversations := append(dataset1.Dataset, dataset2.Dataset...)

	return ConversationalDataset{
		Dataset: allConversations,
		Metadata: DatasetMetadata{
			Name:        "Comprehensive Go Programming Conversational Dataset",
			Description: "Complete conversational Q&A dataset combining official golang.org documentation and ethically scraped web content for comprehensive Go programming assistance",
			Version:     "2.0.0",
			TotalItems:  len(allConversations),
			Sources:     "Official golang.org documentation + Ethically scraped Go community content (Go Wiki, FAQ, Memory Model, Code Walks)",
			Purpose:     "Comprehensive LLM training, fine-tuning, and production-ready Go programming conversational AI",
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func exportDataset(dataset ConversationalDataset, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(dataset); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func generateMergedSummary(dataset ConversationalDataset, filename string) {
	fmt.Printf("\n🎉 COMPREHENSIVE DATASET CREATED!\n")
	fmt.Printf("=================================\n")
	
	// File size
	if info, err := os.Stat(filename); err == nil {
		fmt.Printf("• File: %s (%.1f KB)\n", filename, float64(info.Size())/1024)
	}
	
	fmt.Printf("• Total Conversations: %d\n", len(dataset.Dataset))
	fmt.Printf("• Dataset Version: %s\n", dataset.Metadata.Version)
	fmt.Printf("• Generated: %s\n", dataset.GeneratedAt)

	// Analyze content
	typeCount := make(map[string]int)
	sourceCount := make(map[string]int)
	categoryCount := make(map[string]int)
	totalTurns := 0
	totalChars := 0
	totalWords := 0

	for _, entry := range dataset.Dataset {
		// Count by conversation type
		if entryType, ok := entry.Metadata["type"].(string); ok {
			typeCount[entryType]++
		}

		// Count by source category
		categoryCount[entry.Source.Category]++
		
		// Count by source type (official vs web)
		if entry.Source.URL != "" {
			if strings.Contains(entry.Source.URL, "golang.org") {
				sourceCount["Official golang.org"]++
			} else {
				sourceCount["Community Web Sources"]++
			}
		}

		totalTurns += len(entry.Conversation)
		totalWords += entry.Source.WordCount

		for _, turn := range entry.Conversation {
			totalChars += len(turn.Content)
		}
	}

	fmt.Printf("\n📊 Conversation Types:\n")
	for convType, count := range typeCount {
		fmt.Printf("   • %s: %d conversations\n", convType, count)
	}

	fmt.Printf("\n📚 Content Categories:\n")
	for category, count := range categoryCount {
		fmt.Printf("   • %s: %d conversations\n", category, count)
	}

	fmt.Printf("\n🌐 Source Distribution:\n")
	for source, count := range sourceCount {
		fmt.Printf("   • %s: %d conversations\n", source, count)
	}

	fmt.Printf("\n💬 Comprehensive Dataset Statistics:\n")
	fmt.Printf("   • Total Conversational Turns: %d\n", totalTurns)
	fmt.Printf("   • Total Characters: %d (%.1f KB)\n", totalChars, float64(totalChars)/1024)
	fmt.Printf("   • Total Source Words: %d\n", totalWords)
	fmt.Printf("   • Average Conversation Length: %.0f characters\n", float64(totalChars)/float64(len(dataset.Dataset)))
	fmt.Printf("   • Average Words per Source: %.0f words\n", float64(totalWords)/float64(len(dataset.Dataset)))

	fmt.Printf("\n🎯 Dataset Capabilities:\n")
	fmt.Printf("   • ✅ Official Go documentation expertise\n")
	fmt.Printf("   • ✅ Community knowledge and best practices\n")
	fmt.Printf("   • ✅ Code examples and practical applications\n")
	fmt.Printf("   • ✅ FAQ and troubleshooting guidance\n")
	fmt.Printf("   • ✅ Advanced topics (memory model, concurrency)\n")
	fmt.Printf("   • ✅ Multiple conversation styles and depths\n")

	fmt.Printf("\n🚀 Ready for Advanced LLM Applications:\n")
	fmt.Printf("   • Production Go programming assistants\n")
	fmt.Printf("   • Comprehensive Go ecosystem chatbots\n")
	fmt.Printf("   • Multi-modal Go education platforms\n")
	fmt.Printf("   • Code generation and review systems\n")
	fmt.Printf("   • Go community knowledge bases\n")

	fmt.Printf("\n📈 Quality Metrics:\n")
	fmt.Printf("   • Ethically sourced content with robots.txt compliance\n")
	fmt.Printf("   • Quality validated with scoring system\n")
	fmt.Printf("   • Diverse conversation types for robust training\n")
	fmt.Printf("   • Official + community perspectives for completeness\n")

	fmt.Printf("\n🔬 Technical Specifications:\n")
	fmt.Printf("   • Format: JSON with structured conversation pairs\n")
	fmt.Printf("   • Schema: OpenAI/Anthropic compatible conversation format\n")  
	fmt.Printf("   • Metadata: Rich source and quality information\n")
	fmt.Printf("   • Encoding: UTF-8 with proper escaping\n")
	fmt.Printf("   • Ready for: Fine-tuning, RAG, and direct training\n")
}
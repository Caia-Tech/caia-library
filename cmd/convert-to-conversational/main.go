package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/Caia-Tech/caia-library/pkg/logging"
	"github.com/Caia-Tech/caia-library/pkg/pipeline"
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
	Role    string `json:"role"`    // "user" or "assistant"
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

// ConversationalDataset represents the complete dataset
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
	Source      string `json:"source"`
	Purpose     string `json:"purpose"`
}

func main() {
	fmt.Println("🔄 GOLANG.ORG TO CONVERSATIONAL JSON CONVERTER")
	fmt.Println("==============================================")
	fmt.Println("Converting curated golang.org docs to LLM-friendly conversational JSON")
	fmt.Println()

	// Setup logging
	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "info"
	
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	logger := logging.GetPipelineLogger("conversational-converter", "main")
	logger.Info().Msg("Starting conversational JSON conversion")

	// Initialize storage to read curated docs
	fmt.Println("📚 Loading curated golang.org documentation...")
	storage, err := initializeStorage(config)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer storage.Close()

	// Load all golang.org documents
	ctx := context.Background()
	golangDocs, err := loadGolangDocuments(ctx, storage)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load golang documents")
	}

	fmt.Printf("✅ Loaded %d golang.org documents\n", len(golangDocs))

	// Convert to conversational format
	fmt.Println("\n🔄 Converting to conversational JSON format...")
	conversationalData := convertToConversational(golangDocs)

	// Export to JSON file
	outputFile := "golang_conversational_dataset.json"
	fmt.Printf("💾 Exporting to %s...\n", outputFile)
	
	if err := exportConversationalJSON(conversationalData, outputFile); err != nil {
		logger.Fatal().Err(err).Msg("Failed to export JSON")
	}

	// Generate summary
	generateSummary(conversationalData, outputFile)
	
	logger.Info().Int("conversations", len(conversationalData.Dataset)).Msg("Conversational JSON conversion completed")
}

func initializeStorage(config *pipeline.PipelineConfig) (*storage.HybridStorage, error) {
	metrics := storage.NewSimpleMetricsCollector()
	config.Storage.PrimaryBackend = "govc"
	
	return storage.NewHybridStorage(
		config.DataPaths.GitRepo,
		"golang-curation-storage", 
		config.Storage,
		metrics,
	)
}

func loadGolangDocuments(ctx context.Context, storage *storage.HybridStorage) ([]*document.Document, error) {
	// List all documents
	docs, err := storage.ListDocuments(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	var golangDocs []*document.Document
	
	// Filter for golang.org documents and load full content
	for _, doc := range docs {
		fullDoc, err := storage.GetDocument(ctx, doc.ID)
		if err != nil {
			continue
		}
		
		// Check if this is a golang.org document
		if source, ok := fullDoc.Content.Metadata["source"]; ok && source == "golang.org" {
			golangDocs = append(golangDocs, fullDoc)
		}
	}
	
	return golangDocs, nil
}

func convertToConversational(docs []*document.Document) ConversationalDataset {
	var conversations []ConversationalEntry
	
	for _, doc := range docs {
		// Create multiple conversational entries per document
		entries := createConversationalEntries(doc)
		conversations = append(conversations, entries...)
	}
	
	return ConversationalDataset{
		Dataset: conversations,
		Metadata: DatasetMetadata{
			Name:        "Golang.org Conversational Dataset",
			Description: "Conversational Q&A pairs derived from official Go documentation for LLM training and fine-tuning",
			Version:     "1.0.0",
			TotalItems:  len(conversations),
			Source:      "golang.org official documentation",
			Purpose:     "LLM training, fine-tuning, and Go programming assistance",
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func createConversationalEntries(doc *document.Document) []ConversationalEntry {
	var entries []ConversationalEntry
	
	title := doc.Content.Metadata["title"]
	url := doc.Content.Metadata["url"]
	category := doc.Content.Metadata["category"]
	description := doc.Content.Metadata["description"]
	qualityTier := doc.Content.Metadata["quality_tier"]
	
	wordCount := 0
	if wc := doc.Content.Metadata["word_count"]; wc != "" {
		fmt.Sscanf(wc, "%d", &wordCount)
	}
	
	source := ConversationalSource{
		URL:         url,
		Title:       title,
		Category:    category,
		Description: description,
		WordCount:   wordCount,
		Quality:     qualityTier,
	}

	// 1. Overview conversation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_overview_%d", sanitizeID(title), time.Now().Unix()),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("Can you explain what %s covers? I'm looking at the Go documentation.", title),
			},
			{
				Role:    "assistant", 
				Content: fmt.Sprintf("%s\n\nThis documentation covers: %s", description, createOverview(doc.Content.Text, title)),
			},
		},
		Metadata: map[string]interface{}{
			"type":     "overview",
			"section":  "introduction",
			"keywords": extractKeywords(title, description),
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 2. Main content conversation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_content_%d", sanitizeID(title), time.Now().Unix()),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("I need detailed information about %s. Can you walk me through the key concepts?", strings.ToLower(title)),
			},
			{
				Role:    "assistant",
				Content: createMainContent(doc.Content.Text, title),
			},
		},
		Metadata: map[string]interface{}{
			"type":      "detailed_explanation",
			"section":   "main_content",
			"keywords":  extractKeywords(title, doc.Content.Text),
			"length":    "comprehensive",
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 3. Code examples conversation (if contains code)
	if containsCode(doc.Content.Text) {
		entries = append(entries, ConversationalEntry{
			ID: fmt.Sprintf("%s_examples_%d", sanitizeID(title), time.Now().Unix()),
			Conversation: []ConversationalTurn{
				{
					Role:    "user",
					Content: fmt.Sprintf("Do you have code examples for %s? I learn better with practical examples.", strings.ToLower(title)),
				},
				{
					Role:    "assistant",
					Content: extractCodeExamples(doc.Content.Text, title),
				},
			},
			Metadata: map[string]interface{}{
				"type":     "code_examples",
				"section":  "examples",
				"keywords": append(extractKeywords(title, doc.Content.Text), "examples", "code", "tutorial"),
			},
			Source:    source,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	// 4. Best practices conversation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_practices_%d", sanitizeID(title), time.Now().Unix()),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("What are the best practices and important considerations for %s?", strings.ToLower(title)),
			},
			{
				Role:    "assistant",
				Content: extractBestPractices(doc.Content.Text, title),
			},
		},
		Metadata: map[string]interface{}{
			"type":     "best_practices",
			"section":  "guidelines",
			"keywords": append(extractKeywords(title, doc.Content.Text), "best practices", "guidelines", "recommendations"),
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	return entries
}

func createOverview(text, title string) string {
	// Extract first few sentences or paragraphs as overview
	sentences := strings.Split(text, ".")
	overview := ""
	for i, sentence := range sentences {
		if i >= 3 { // Take first 3 sentences
			break
		}
		if len(strings.TrimSpace(sentence)) > 10 {
			overview += strings.TrimSpace(sentence) + ". "
		}
	}
	
	if overview == "" {
		// Fallback to first 200 characters
		if len(text) > 200 {
			overview = text[:200] + "..."
		} else {
			overview = text
		}
	}
	
	return strings.TrimSpace(overview)
}

func createMainContent(text, title string) string {
	// Clean and format the main content for conversational style
	content := cleanContent(text)
	
	// Add conversational introduction
	intro := fmt.Sprintf("I'd be happy to explain %s in detail.\n\n", title)
	
	// Format content into readable sections
	sections := splitIntoSections(content)
	formattedContent := ""
	
	for i, section := range sections {
		if len(section) > 50 { // Skip very short sections
			if i > 0 {
				formattedContent += "\n\n"
			}
			formattedContent += formatSection(section)
		}
	}
	
	return intro + formattedContent
}

func extractCodeExamples(text, title string) string {
	// Extract code blocks and examples
	codeRegex := regexp.MustCompile(`(?s)(?:func\s+\w+|package\s+\w+|import\s*\(|var\s+\w+|type\s+\w+|const\s+\w+)[^.]*?(?:\n\s*\n|\n\s*}|\n\s*\)|\n\s*$)`)
	
	examples := codeRegex.FindAllString(text, -1)
	
	response := fmt.Sprintf("Here are practical code examples from %s:\n\n", title)
	
	for i, example := range examples {
		if len(example) > 20 && len(example) < 500 { // Reasonable size examples
			response += fmt.Sprintf("**Example %d:**\n```go\n%s\n```\n\n", i+1, strings.TrimSpace(example))
		}
		if i >= 4 { // Limit to 5 examples
			break
		}
	}
	
	if len(examples) == 0 {
		response += "The documentation includes various code patterns and examples throughout. Let me highlight the key programming concepts:\n\n"
		response += extractProgrammingConcepts(text)
	}
	
	return response
}

func extractBestPractices(text, title string) string {
	// Look for best practices, guidelines, recommendations
	keywords := []string{"should", "must", "avoid", "recommend", "best", "practice", "guideline", "important", "note", "warning"}
	
	sentences := strings.Split(text, ".")
	practices := []string{}
	
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) > 30 { // Skip very short sentences
			for _, keyword := range keywords {
				if strings.Contains(strings.ToLower(sentence), keyword) {
					practices = append(practices, "• "+sentence+".")
					break
				}
			}
		}
	}
	
	response := fmt.Sprintf("Here are the key best practices and important considerations for %s:\n\n", title)
	
	if len(practices) > 0 {
		for i, practice := range practices {
			response += practice + "\n\n"
			if i >= 7 { // Limit to 8 practices
				break
			}
		}
	} else {
		response += generateGenericPractices(title, text)
	}
	
	return response
}

func generateGenericPractices(title, text string) string {
	return fmt.Sprintf("Based on %s, here are important considerations:\n\n• Follow the patterns and conventions shown in the documentation\n• Pay attention to error handling and edge cases\n• Use the standard library approaches when available\n• Keep code readable and maintainable\n• Test your implementations thoroughly\n\nFor specific details, refer to the full documentation content.", title)
}

func extractKeywords(title, text string) []string {
	keywords := []string{}
	
	// Add title words
	titleWords := strings.Fields(strings.ToLower(title))
	for _, word := range titleWords {
		word = strings.Trim(word, ".,!?()[]{}:;\"'")
		if len(word) > 2 {
			keywords = append(keywords, word)
		}
	}
	
	// Add common Go keywords found in text
	goKeywords := []string{"func", "package", "import", "var", "const", "type", "struct", "interface", "go", "golang", "module", "dependency", "error", "test", "example"}
	
	textLower := strings.ToLower(text)
	for _, keyword := range goKeywords {
		if strings.Contains(textLower, keyword) {
			keywords = append(keywords, keyword)
		}
	}
	
	return keywords
}

func sanitizeID(s string) string {
	// Clean string for use as ID
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return strings.ToLower(reg.ReplaceAllString(s, "_"))
}

func containsCode(text string) bool {
	codeIndicators := []string{"func ", "package ", "import ", "var ", "const ", "type ", "struct {", "interface {", "go "}
	textLower := strings.ToLower(text)
	
	for _, indicator := range codeIndicators {
		if strings.Contains(textLower, indicator) {
			return true
		}
	}
	return false
}

func cleanContent(text string) string {
	// Remove extra whitespace and clean formatting
	lines := strings.Split(text, "\n")
	cleaned := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n")
}

func splitIntoSections(text string) []string {
	// Split text into logical sections
	paragraphs := strings.Split(text, "\n\n")
	sections := []string{}
	
	currentSection := ""
	for _, paragraph := range paragraphs {
		if len(paragraph) > 100 && currentSection != "" {
			sections = append(sections, currentSection)
			currentSection = paragraph
		} else {
			if currentSection != "" {
				currentSection += "\n\n" + paragraph
			} else {
				currentSection = paragraph
			}
		}
	}
	
	if currentSection != "" {
		sections = append(sections, currentSection)
	}
	
	return sections
}

func formatSection(section string) string {
	// Add conversational formatting
	if len(section) > 500 {
		return section[:500] + "...\n\n[Content continues with detailed explanations and examples]"
	}
	return section
}

func extractProgrammingConcepts(text string) string {
	concepts := []string{
		"Variable declaration and usage",
		"Function definitions and calls", 
		"Type definitions and interfaces",
		"Error handling patterns",
		"Package organization",
		"Import statements",
	}
	
	result := ""
	for _, concept := range concepts {
		result += "• " + concept + "\n"
	}
	
	return result
}

func exportConversationalJSON(data ConversationalDataset, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	
	return nil
}

func generateSummary(data ConversationalDataset, filename string) {
	fmt.Printf("\n🎉 CONVERSATIONAL JSON EXPORT COMPLETED!\n")
	fmt.Printf("========================================\n")
	fmt.Printf("• File: %s\n", filename)
	fmt.Printf("• Total Conversations: %d\n", len(data.Dataset))
	fmt.Printf("• Generated: %s\n", data.GeneratedAt)
	fmt.Printf("• Purpose: %s\n", data.Metadata.Purpose)
	
	// Count by type
	typeCount := make(map[string]int)
	totalTurns := 0
	
	for _, entry := range data.Dataset {
		if entryType, ok := entry.Metadata["type"].(string); ok {
			typeCount[entryType]++
		}
		totalTurns += len(entry.Conversation)
	}
	
	fmt.Printf("\n📊 Conversation Types:\n")
	for convType, count := range typeCount {
		fmt.Printf("   • %s: %d conversations\n", convType, count)
	}
	
	fmt.Printf("\n💬 Total Conversational Turns: %d\n", totalTurns)
	fmt.Printf("📁 Ready for LLM training and fine-tuning!\n")
}
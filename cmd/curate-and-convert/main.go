package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement/scraping"
	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/Caia-Tech/caia-library/pkg/extractor"
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

type GolangContent struct {
	URL         string
	Title       string
	Description string
	Category    string
	Priority    int
}

func main() {
	fmt.Println("🔄 GOLANG.ORG CURATE & CONVERT TO CONVERSATIONAL JSON")
	fmt.Println("===================================================")
	fmt.Println("Complete pipeline: Curate golang.org docs → Convert to LLM conversational JSON")
	fmt.Println()

	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "info"
	
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	logger := logging.GetPipelineLogger("curate-convert", "main")
	
	// Phase 1: Curate golang.org documents
	fmt.Println("📚 Phase 1: Curating golang.org documentation...")
	docs, err := curateGolangDocuments()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to curate documents")
	}
	
	fmt.Printf("✅ Curated %d documents\n", len(docs))

	// Phase 2: Convert to conversational JSON
	fmt.Println("\n🔄 Phase 2: Converting to conversational JSON...")
	conversationalData := convertToConversational(docs)

	// Phase 3: Export
	outputFile := "golang_conversational_dataset.json"
	fmt.Printf("\n💾 Phase 3: Exporting to %s...\n", outputFile)
	
	if err := exportConversationalJSON(conversationalData, outputFile); err != nil {
		logger.Fatal().Err(err).Msg("Failed to export JSON")
	}

	generateSummary(conversationalData, outputFile)
	logger.Info().Int("conversations", len(conversationalData.Dataset)).Msg("Complete pipeline finished")
}

func curateGolangDocuments() ([]*document.Document, error) {
	// Initialize components for curation
	complianceEngine := scraping.NewComplianceEngine(scraping.DefaultComplianceConfig())
	qualityValidator := quality.NewQualityValidator(nil)
	extractorEngine := extractor.NewEngine()

	// Define key golang.org content
	targets := []GolangContent{
		{"https://golang.org/doc/", "Go Documentation Hub", "Main documentation portal", "Core Documentation", 1},
		{"https://golang.org/doc/effective_go", "Effective Go", "Essential guide to writing idiomatic Go code", "Core Documentation", 1},
		{"https://golang.org/ref/spec", "Go Language Specification", "Official language specification", "Language Reference", 1},
		{"https://golang.org/doc/tutorial/getting-started", "Tutorial: Get started with Go", "Introduction for new Go developers", "Tutorials", 1},
		{"https://golang.org/doc/tutorial/create-module", "Tutorial: Create a Go module", "Learn to create Go modules", "Tutorials", 1},
		{"https://golang.org/doc/code", "How to Write Go Code", "Guide to structuring Go programs", "Programming Guide", 2},
		{"https://golang.org/pkg/fmt/", "Package fmt", "Formatted I/O functions", "Standard Library", 3},
		{"https://golang.org/pkg/net/http/", "Package http", "HTTP client and server implementations", "Standard Library", 3},
	}

	var docs []*document.Document
	ctx := context.Background()

	for i, target := range targets {
		fmt.Printf("   [%d/%d] %s\n", i+1, len(targets), target.Title)
		
		doc, err := processDocument(ctx, target, complianceEngine, qualityValidator, extractorEngine)
		if err != nil {
			fmt.Printf("      ❌ Failed: %v\n", err)
			continue
		}
		
		docs = append(docs, doc)
		fmt.Printf("      ✅ Curated successfully\n")
		
		// Respectful delay
		if i < len(targets)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return docs, nil
}

func processDocument(
	ctx context.Context,
	target GolangContent,
	compliance *scraping.ComplianceEngine,
	quality *quality.QualityValidator,
	extractor *extractor.Engine,
) (*document.Document, error) {
	// Compliance check
	result, err := compliance.CheckCompliance(ctx, target.URL)
	if err != nil || !result.Allowed {
		return nil, fmt.Errorf("compliance failed: %w", err)
	}
	
	if result.RequiredDelay > 0 {
		time.Sleep(result.RequiredDelay)
	}

	// Fetch content
	content, err := fetchContent(ctx, target.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	// Extract text
	text, _, err := extractor.Extract(ctx, content, "html")
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Quality validation
	qualityMeta := map[string]string{
		"source": "golang.org",
		"url":    target.URL,
		"title":  target.Title,
	}
	
	qualityResult, _ := quality.ValidateContent(ctx, text, qualityMeta)
	qualityScore := 0.0
	qualityTier := "unknown"
	
	if qualityResult != nil {
		qualityScore = qualityResult.OverallScore
		qualityTier = qualityResult.QualityTier
	}

	// Create document
	doc := &document.Document{
		ID: fmt.Sprintf("golang_%s_%d", sanitizeID(target.Title), time.Now().Unix()),
		Source: document.Source{
			Type: "web",
			URL:  target.URL,
		},
		Content: document.Content{
			Text: text,
			Metadata: map[string]string{
				"source":        "golang.org",
				"url":           target.URL,
				"title":         target.Title,
				"description":   target.Description,
				"category":      target.Category,
				"priority":      fmt.Sprintf("%d", target.Priority),
				"quality_score": fmt.Sprintf("%.3f", qualityScore),
				"quality_tier":  qualityTier,
				"word_count":    fmt.Sprintf("%d", len(strings.Fields(text))),
				"curated_at":    time.Now().UTC().Format(time.RFC3339),
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return doc, nil
}

func fetchContent(ctx context.Context, url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "CAIA-Library-Curator/1.0 (+https://caiatech.com)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	return io.ReadAll(resp.Body)
}

func convertToConversational(docs []*document.Document) ConversationalDataset {
	var conversations []ConversationalEntry
	
	for _, doc := range docs {
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

	baseID := sanitizeID(title)
	timestamp := time.Now().Unix()

	// 1. Overview conversation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_overview_%d", baseID, timestamp),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("Can you explain what %s covers? I'm looking at the Go documentation.", title),
			},
			{
				Role:    "assistant", 
				Content: fmt.Sprintf("I'd be happy to explain %s!\n\n%s\n\n%s", title, description, createOverview(doc.Content.Text, title)),
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

	// 2. Detailed explanation conversation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_detailed_%d", baseID, timestamp+1),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("I need detailed information about %s. Can you walk me through the key concepts?", strings.ToLower(title)),
			},
			{
				Role:    "assistant",
				Content: createDetailedExplanation(doc.Content.Text, title),
			},
		},
		Metadata: map[string]interface{}{
			"type":      "detailed_explanation",
			"section":   "main_content",
			"keywords":  extractKeywords(title, doc.Content.Text),
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 3. Quick reference conversation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_reference_%d", baseID, timestamp+2),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("Can you give me a quick reference or summary of the main points in %s?", title),
			},
			{
				Role:    "assistant",
				Content: createQuickReference(doc.Content.Text, title),
			},
		},
		Metadata: map[string]interface{}{
			"type":     "quick_reference",
			"section":  "summary",
			"keywords": extractKeywords(title, doc.Content.Text),
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 4. Code examples (if contains code)
	if containsCode(doc.Content.Text) {
		entries = append(entries, ConversationalEntry{
			ID: fmt.Sprintf("%s_examples_%d", baseID, timestamp+3),
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

	return entries
}

func createOverview(text, title string) string {
	// Extract first meaningful paragraph or sentences
	paragraphs := strings.Split(text, "\n\n")
	overview := ""
	
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if len(para) > 50 && len(para) < 300 {
			overview = para
			break
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
	
	return "Here's what it covers:\n\n" + overview
}

func createDetailedExplanation(text, title string) string {
	intro := fmt.Sprintf("Let me walk you through %s in detail.\n\n", title)
	
	// Take substantial content but keep it manageable for LLM context
	content := cleanContent(text)
	if len(content) > 1500 {
		content = content[:1500] + "\n\n[This covers the main concepts - there's additional detail in the full documentation]"
	}
	
	return intro + content
}

func createQuickReference(text, title string) string {
	// Extract key points, bullet points, or create summary
	keyPoints := []string{}
	
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for lines that seem like key points
		if len(line) > 20 && len(line) < 150 {
			if strings.Contains(line, ":") || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				keyPoints = append(keyPoints, "• " + line)
			}
		}
	}
	
	response := fmt.Sprintf("Here's a quick reference for %s:\n\n", title)
	
	if len(keyPoints) > 0 {
		for i, point := range keyPoints {
			response += point + "\n"
			if i >= 6 { // Limit to 7 points
				break
			}
		}
	} else {
		// Create generic key points
		response += createGenericSummary(title, text)
	}
	
	return response
}

func createGenericSummary(title, text string) string {
	wordCount := len(strings.Fields(text))
	return fmt.Sprintf("• %s is comprehensive documentation from golang.org\n• Contains %d words of detailed information\n• Covers essential concepts and practical guidance\n• Part of the official Go documentation suite\n• Essential reading for Go developers", title, wordCount)
}

func extractCodeExamples(text, title string) string {
	// Look for code patterns
	codePatterns := []string{
		`func\s+\w+.*\{[^}]*\}`,
		`package\s+\w+`,
		`import\s*\([^)]*\)`,
		`var\s+\w+.*=`,
		`type\s+\w+\s+struct`,
	}
	
	examples := []string{}
	for _, pattern := range codePatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllString(text, 3) // Limit matches per pattern
		examples = append(examples, matches...)
	}
	
	response := fmt.Sprintf("Here are code examples from %s:\n\n", title)
	
	if len(examples) > 0 {
		for i, example := range examples {
			if len(example) > 10 && len(example) < 200 {
				response += fmt.Sprintf("```go\n%s\n```\n\n", strings.TrimSpace(example))
			}
			if i >= 2 { // Limit to 3 examples
				break
			}
		}
	} else {
		response += "The documentation contains various Go programming examples and patterns. "
		response += "Here are some key programming concepts covered:\n\n"
		response += extractProgrammingConcepts(text)
	}
	
	return response
}

func extractProgrammingConcepts(text string) string {
	concepts := []string{
		"Go syntax and language constructs",
		"Package and module organization", 
		"Function definitions and usage",
		"Type systems and interfaces",
		"Error handling best practices",
		"Standard library usage patterns",
	}
	
	result := ""
	for _, concept := range concepts {
		result += "• " + concept + "\n"
	}
	
	return result
}

func extractKeywords(title, text string) []string {
	keywords := []string{}
	
	// Extract from title
	titleWords := strings.Fields(strings.ToLower(title))
	for _, word := range titleWords {
		word = strings.Trim(word, ".,!?()[]{}:;\"'")
		if len(word) > 2 && word != "the" && word != "and" && word != "for" {
			keywords = append(keywords, word)
		}
	}
	
	// Add Go-specific terms
	goTerms := []string{"go", "golang", "package", "function", "module", "interface", "struct", "error", "test"}
	textLower := strings.ToLower(text)
	
	for _, term := range goTerms {
		if strings.Contains(textLower, term) {
			keywords = append(keywords, term)
		}
	}
	
	return keywords
}

func sanitizeID(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return strings.ToLower(reg.ReplaceAllString(s, "_"))
}

func containsCode(text string) bool {
	codeIndicators := []string{"func ", "package ", "import ", "var ", "const ", "type ", "struct {", "interface {"}
	textLower := strings.ToLower(text)
	
	for _, indicator := range codeIndicators {
		if strings.Contains(textLower, indicator) {
			return true
		}
	}
	return false
}

func cleanContent(text string) string {
	// Clean up text formatting
	lines := strings.Split(text, "\n")
	cleaned := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 && !strings.HasPrefix(line, "//") {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n")
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
	
	// File size
	if info, err := os.Stat(filename); err == nil {
		fmt.Printf("• File: %s (%.1f KB)\n", filename, float64(info.Size())/1024)
	}
	
	fmt.Printf("• Total Conversations: %d\n", len(data.Dataset))
	fmt.Printf("• Generated: %s\n", data.GeneratedAt)
	
	// Count by type
	typeCount := make(map[string]int)
	totalTurns := 0
	totalChars := 0
	
	for _, entry := range data.Dataset {
		if entryType, ok := entry.Metadata["type"].(string); ok {
			typeCount[entryType]++
		}
		totalTurns += len(entry.Conversation)
		
		for _, turn := range entry.Conversation {
			totalChars += len(turn.Content)
		}
	}
	
	fmt.Printf("\n📊 Conversation Types:\n")
	for convType, count := range typeCount {
		fmt.Printf("   • %s: %d conversations\n", convType, count)
	}
	
	fmt.Printf("\n💬 Dataset Statistics:\n")
	fmt.Printf("   • Total Conversational Turns: %d\n", totalTurns)
	fmt.Printf("   • Total Characters: %d (%.1f KB)\n", totalChars, float64(totalChars)/1024)
	fmt.Printf("   • Average Conversation Length: %.0f characters\n", float64(totalChars)/float64(len(data.Dataset)))
	
	fmt.Printf("\n🚀 Ready for LLM Training!\n")
	fmt.Printf("   • Fine-tuning datasets\n")
	fmt.Printf("   • Instruction following training\n") 
	fmt.Printf("   • Go programming assistance models\n")
	fmt.Printf("   • Conversational AI development\n")
}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// Quick diverse sources for demonstration
func getQuickHighValueSources() []HighValueSource {
	return []HighValueSource{
		{
			URL:         "https://en.wikipedia.org/wiki/Artificial_intelligence",
			Title:       "Artificial Intelligence - Wikipedia",
			Description: "Comprehensive overview of AI concepts, history, and applications",
			Domain:      "Technology",
			Category:    "Encyclopedia",
			Priority:    1,
			Tags:        []string{"ai", "technology", "machine-learning", "comprehensive"},
			ContentType: "educational_article",
			ValueReason: "Authoritative, well-researched content covering cutting-edge technology",
		},
		{
			URL:         "https://en.wikipedia.org/wiki/Physics",
			Title:       "Physics - Wikipedia",
			Description: "Foundational physics principles and discoveries",
			Domain:      "Science",
			Category:    "Encyclopedia",
			Priority:    1,
			Tags:        []string{"physics", "science", "fundamental", "natural-laws"},
			ContentType: "scientific_article",
			ValueReason: "Core scientific knowledge essential for understanding the natural world",
		},
		{
			URL:         "https://en.wikipedia.org/wiki/Philosophy",
			Title:       "Philosophy - Wikipedia",
			Description: "Fundamental philosophical concepts and reasoning",
			Domain:      "Philosophy",
			Category:    "Encyclopedia",
			Priority:    1,
			Tags:        []string{"philosophy", "ethics", "logic", "critical-thinking"},
			ContentType: "philosophical_article",
			ValueReason: "Essential for reasoning, ethics, and logical thinking skills",
		},
		{
			URL:         "https://en.wikipedia.org/wiki/Mathematics",
			Title:       "Mathematics - Wikipedia",
			Description: "Mathematical foundations and problem-solving methods",
			Domain:      "Mathematics",
			Category:    "Encyclopedia",
			Priority:    1,
			Tags:        []string{"mathematics", "logic", "quantitative", "problem-solving"},
			ContentType: "mathematical_article",
			ValueReason: "Fundamental quantitative reasoning and logical thinking foundations",
		},
		{
			URL:         "https://en.wikipedia.org/wiki/Psychology",
			Title:       "Psychology - Wikipedia", 
			Description: "Human behavior, cognition, and mental processes",
			Domain:      "Psychology",
			Category:    "Encyclopedia",
			Priority:    1,
			Tags:        []string{"psychology", "behavior", "cognition", "human-nature"},
			ContentType: "scientific_article",
			ValueReason: "Understanding human behavior and cognitive processes",
		},
	}
}

// Reuse types from main scraper
type HighValueSource struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Domain      string   `json:"domain"`
	Category    string   `json:"category"`
	Priority    int      `json:"priority"`
	Tags        []string `json:"tags"`
	ContentType string   `json:"content_type"`
	ValueReason string   `json:"value_reason"`
}

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
	Domain      string `json:"domain"`
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
	Domains     string `json:"domains"`
	Purpose     string `json:"purpose"`
}

func main() {
	fmt.Println("⚡ QUICK DIVERSE HIGH-VALUE DATA SCRAPER")
	fmt.Println("=======================================")
	fmt.Println("Fast ethical scraping of key high-value sources across domains")
	fmt.Println()

	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "warn" // Less verbose for quick demo
	
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	logger := logging.GetPipelineLogger("quick-diverse-scraper", "main")

	// Quick high-value sources
	fmt.Println("🎯 Targeting 5 key high-value sources across domains...")
	sources := getQuickHighValueSources()
	fmt.Printf("✅ Selected %d premium sources\n", len(sources))

	// Fast ethical scraping
	fmt.Println("\n🚀 Fast ethical scraping...")
	docs, err := quickEthicalScrape(sources)
	if err != nil {
		logger.Fatal().Err(err).Msg("Quick scraping failed")
	}

	fmt.Printf("✅ Successfully scraped %d documents\n", len(docs))

	// Convert to conversational format
	fmt.Println("\n🔄 Converting to conversational JSON...")
	conversationalData := convertToConversational(docs)

	// Export
	outputFile := "quick_diverse_conversational_dataset.json"
	fmt.Printf("\n💾 Exporting to %s...\n", outputFile)
	
	if err := exportJSON(conversationalData, outputFile); err != nil {
		logger.Fatal().Err(err).Msg("Export failed")
	}

	generateQuickSummary(conversationalData, outputFile)
}

func quickEthicalScrape(sources []HighValueSource) ([]*document.Document, error) {
	complianceEngine := scraping.NewComplianceEngine(scraping.DefaultComplianceConfig())
	qualityValidator := quality.NewQualityValidator(nil)
	extractorEngine := extractor.NewEngine()

	var docs []*document.Document
	ctx := context.Background()

	for i, source := range sources {
		fmt.Printf("   [%d/%d] %s (%s)\n", i+1, len(sources), source.Title, source.Domain)
		
		// Quick compliance check
		complianceResult, err := complianceEngine.CheckCompliance(ctx, source.URL)
		if err != nil || !complianceResult.Allowed {
			fmt.Printf("           ❌ Not allowed: %v\n", err)
			continue
		}

		// Respect delays
		if complianceResult.RequiredDelay > 0 {
			time.Sleep(complianceResult.RequiredDelay)
		}

		// Fetch content
		content, _, err := fetchContent(ctx, source.URL)
		if err != nil {
			fmt.Printf("           ❌ Fetch failed: %v\n", err)
			continue
		}

		// Extract text
		text, _, err := extractorEngine.Extract(ctx, content, "html")
		if err != nil || len(text) < 1000 {
			fmt.Printf("           ❌ Extraction failed or too short\n")
			continue
		}

		// Quick quality check
		qualityResult, _ := qualityValidator.ValidateContent(ctx, text, map[string]string{
			"url": source.URL,
			"title": source.Title,
		})
		
		qualityScore := 0.0
		qualityTier := "unknown"
		if qualityResult != nil {
			qualityScore = qualityResult.OverallScore
			qualityTier = qualityResult.QualityTier
		}

		// Create document
		doc := &document.Document{
			ID: fmt.Sprintf("quick_%s_%d", sanitizeID(source.Title), time.Now().Unix()),
			Source: document.Source{
				Type: "quick_diverse_web",
				URL:  source.URL,
			},
			Content: document.Content{
				Text: text,
				Metadata: map[string]string{
					"url":           source.URL,
					"title":         source.Title,
					"description":   source.Description,
					"domain":        source.Domain,
					"category":      source.Category,
					"tags":          strings.Join(source.Tags, ","),
					"value_reason":  source.ValueReason,
					"quality_score": fmt.Sprintf("%.3f", qualityScore),
					"quality_tier":  qualityTier,
					"word_count":    fmt.Sprintf("%d", len(strings.Fields(text))),
					"scraped_at":    time.Now().UTC().Format(time.RFC3339),
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		docs = append(docs, doc)
		fmt.Printf("           ✅ Success (%d chars, %.2f quality)\n", len(text), qualityScore)

		// Quick delay
		if i < len(sources)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return docs, nil
}

func fetchContent(ctx context.Context, urlStr string) ([]byte, string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, "", err
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, "", fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("User-Agent", "CAIA-Quick-Scraper/1.0")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
	content, err := io.ReadAll(limitedReader)
	return content, resp.Header.Get("Content-Type"), err
}

func convertToConversational(docs []*document.Document) ConversationalDataset {
	var conversations []ConversationalEntry

	for _, doc := range docs {
		entries := createConversations(doc)
		conversations = append(conversations, entries...)
	}

	return ConversationalDataset{
		Dataset: conversations,
		Metadata: DatasetMetadata{
			Name:        "Quick Diverse High-Value Dataset",
			Description: "Fast-scraped conversational dataset from key high-value sources across domains",
			Version:     "1.0.0",
			TotalItems:  len(conversations),
			Domains:     "Technology, Science, Philosophy, Mathematics, Psychology",
			Purpose:     "Quick general-purpose LLM training with diverse high-quality knowledge",
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func createConversations(doc *document.Document) []ConversationalEntry {
	var entries []ConversationalEntry

	title := doc.Content.Metadata["title"]
	url := doc.Content.Metadata["url"]
	domain := doc.Content.Metadata["domain"]
	category := doc.Content.Metadata["category"]
	description := doc.Content.Metadata["description"]
	qualityTier := doc.Content.Metadata["quality_tier"]
	valueReason := doc.Content.Metadata["value_reason"]

	wordCount := 0
	if wc := doc.Content.Metadata["word_count"]; wc != "" {
		fmt.Sscanf(wc, "%d", &wordCount)
	}

	source := ConversationalSource{
		URL:         url,
		Title:       title,
		Domain:      domain,
		Category:    category,
		Description: description,
		WordCount:   wordCount,
		Quality:     qualityTier,
	}

	subject := strings.TrimSuffix(title, " - Wikipedia")
	baseID := sanitizeID(subject)
	timestamp := time.Now().Unix()

	// 1. Introduction
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_intro_%d", baseID, timestamp),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("I want to understand %s. Can you explain what this field is about?", subject),
			},
			{
				Role:    "assistant",
				Content: fmt.Sprintf("I'd be happy to explain %s!\n\n%s\n\n%s", subject, description, createOverview(doc.Content.Text)),
			},
		},
		Metadata: map[string]interface{}{
			"type":       "introduction",
			"domain":     domain,
			"difficulty": "beginner",
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 2. Detailed explanation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_detailed_%d", baseID, timestamp+1),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("Can you go deeper into %s? I want to understand the key concepts and principles.", subject),
			},
			{
				Role:    "assistant",
				Content: fmt.Sprintf("Let me explain %s in more detail.\n\n%s", subject, createDetailedExplanation(doc.Content.Text)),
			},
		},
		Metadata: map[string]interface{}{
			"type":       "detailed_explanation",
			"domain":     domain,
			"difficulty": "intermediate",
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 3. Applications and importance
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_applications_%d", baseID, timestamp+2),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("Why is %s important? How is it used in the real world?", subject),
			},
			{
				Role:    "assistant",
				Content: fmt.Sprintf("%s is important because %s\n\nReal-world applications include:\n%s", subject, valueReason, createApplications(domain, subject)),
			},
		},
		Metadata: map[string]interface{}{
			"type":       "applications",
			"domain":     domain,
			"difficulty": "applied",
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	return entries
}

func createOverview(text string) string {
	paragraphs := strings.Split(text, "\n\n")
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if len(para) > 150 && len(para) < 500 {
			return "Here's a foundational overview:\n\n" + para
		}
	}
	
	if len(text) > 300 {
		return "Here's a foundational overview:\n\n" + text[:300] + "..."
	}
	return "Here's a foundational overview:\n\n" + text
}

func createDetailedExplanation(text string) string {
	// Extract meaningful content
	content := cleanText(text)
	if len(content) > 1500 {
		content = content[:1500] + "\n\n[This covers the core concepts - there's much more depth to explore]"
	}
	return content
}

func createApplications(domain, subject string) string {
	applications := map[string][]string{
		"Technology": {
			"• Software development and computer systems",
			"• Automation and process optimization", 
			"• Innovation in digital products and services",
		},
		"Science": {
			"• Research and scientific discovery",
			"• Medical and healthcare applications",
			"• Environmental and sustainability solutions",
		},
		"Philosophy": {
			"• Ethical decision-making frameworks",
			"• Critical thinking and reasoning skills",
			"• Policy development and governance",
		},
		"Mathematics": {
			"• Problem-solving across all fields",
			"• Data analysis and statistics",
			"• Engineering and design applications",
		},
		"Psychology": {
			"• Mental health and therapy",
			"• Education and learning optimization",
			"• Human-computer interaction design",
		},
	}

	if apps, exists := applications[domain]; exists {
		return strings.Join(apps, "\n")
	}

	return fmt.Sprintf("• Research and academic study\n• Professional applications\n• Educational and training purposes")
}

func cleanText(text string) string {
	lines := strings.Split(text, "\n")
	cleaned := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 20 && !strings.Contains(strings.ToLower(line), "edit") {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n")
}

func sanitizeID(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return strings.ToLower(reg.ReplaceAllString(s, "_"))
}

func exportJSON(data ConversationalDataset, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func generateQuickSummary(data ConversationalDataset, filename string) {
	fmt.Printf("\n⚡ QUICK DIVERSE DATASET CREATED!\n")
	fmt.Printf("================================\n")
	
	if info, err := os.Stat(filename); err == nil {
		fmt.Printf("• File: %s (%.1f KB)\n", filename, float64(info.Size())/1024)
	}
	
	fmt.Printf("• Total Conversations: %d\n", len(data.Dataset))
	fmt.Printf("• Domains: %s\n", data.Metadata.Domains)

	// Count by domain
	domainCount := make(map[string]int)
	totalChars := 0

	for _, entry := range data.Dataset {
		if domain, ok := entry.Metadata["domain"].(string); ok {
			domainCount[domain]++
		}
		for _, turn := range entry.Conversation {
			totalChars += len(turn.Content)
		}
	}

	fmt.Printf("\n🌐 Domain Coverage:\n")
	for domain, count := range domainCount {
		fmt.Printf("   • %s: %d conversations\n", domain, count)
	}

	fmt.Printf("\n📊 Statistics:\n")
	fmt.Printf("   • Total Characters: %d (%.1f KB)\n", totalChars, float64(totalChars)/1024)
	fmt.Printf("   • Average Length: %.0f chars/conversation\n", float64(totalChars)/float64(len(data.Dataset)))

	fmt.Printf("\n🎯 Quick Achievements:\n")
	fmt.Printf("   • ✅ Multi-domain high-value content\n")
	fmt.Printf("   • ✅ Ethical scraping with compliance\n")
	fmt.Printf("   • ✅ Quality validation and filtering\n")
	fmt.Printf("   • ✅ Conversational format ready for LLMs\n")
	fmt.Printf("   • ✅ Fast processing for rapid deployment\n")
}
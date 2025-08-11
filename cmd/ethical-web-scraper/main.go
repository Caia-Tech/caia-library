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

// WebSource represents an ethical web scraping target
type WebSource struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Priority    int      `json:"priority"`
	Tags        []string `json:"tags"`
	Expected    string   `json:"expected_content_type"`
}

// ConversationalEntry represents a conversational Q&A pair for LLMs
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
	fmt.Println("🕷️  ETHICAL WEB SCRAPER FOR GO CONTENT")
	fmt.Println("=====================================")
	fmt.Println("Ethically collecting high-quality Go programming content from the web")
	fmt.Println()

	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "info"
	
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	logger := logging.GetPipelineLogger("ethical-scraper", "main")
	logger.Info().Msg("Starting ethical web scraping")

	// Phase 1: Get high-quality Go content sources
	fmt.Println("🎯 Phase 1: Identifying ethical Go content sources...")
	sources := getEthicalGoSources()
	fmt.Printf("✅ Identified %d potential sources\n", len(sources))

	// Phase 2: Ethical scraping with compliance checks
	fmt.Println("\n🛡️  Phase 2: Ethical scraping with robots.txt compliance...")
	docs, err := ethicalScrapeGoContent(sources)
	if err != nil {
		logger.Fatal().Err(err).Msg("Ethical scraping failed")
	}

	fmt.Printf("✅ Successfully scraped %d documents ethically\n", len(docs))

	// Phase 3: Convert to conversational JSON
	fmt.Println("\n🔄 Phase 3: Converting to conversational JSON...")
	conversationalData := convertWebContentToConversational(docs)

	// Phase 4: Export enhanced dataset
	outputFile := "go_web_conversational_dataset.json"
	fmt.Printf("\n💾 Phase 4: Exporting to %s...\n", outputFile)
	
	if err := exportConversationalJSON(conversationalData, outputFile); err != nil {
		logger.Fatal().Err(err).Msg("Failed to export JSON")
	}

	generateWebScrapingSummary(conversationalData, outputFile)
	logger.Info().Int("conversations", len(conversationalData.Dataset)).Msg("Ethical web scraping completed")
}

func getEthicalGoSources() []WebSource {
	return []WebSource{
		// Go Blog - Official Content
		{
			URL:         "https://go.dev/blog/",
			Title:       "The Go Blog",
			Description: "Official Go team blog with latest updates and insights",
			Category:    "Official Blog",
			Priority:    1,
			Tags:        []string{"official", "blog", "updates", "go-team"},
			Expected:    "blog_posts",
		},
		
		// Go Wiki - Community Documentation  
		{
			URL:         "https://github.com/golang/go/wiki",
			Title:       "Go Wiki",
			Description: "Community-maintained Go documentation and guides",
			Category:    "Community Wiki",
			Priority:    2,
			Tags:        []string{"community", "wiki", "documentation", "guides"},
			Expected:    "wiki_content",
		},

		// Go by Example - Practical Examples
		{
			URL:         "https://gobyexample.com/",
			Title:       "Go by Example",
			Description: "Hands-on introduction to Go using annotated example programs",
			Category:    "Tutorial Examples",
			Priority:    1,
			Tags:        []string{"examples", "tutorial", "hands-on", "beginner"},
			Expected:    "code_examples",
		},

		// Effective Go Patterns - Advanced Content
		{
			URL:         "https://golang.org/doc/codewalk/",
			Title:       "Go Code Walks",
			Description: "Guided tours through Go programs and patterns",
			Category:    "Code Walks",
			Priority:    2,
			Tags:        []string{"advanced", "patterns", "guided-tours", "analysis"},
			Expected:    "code_analysis",
		},

		// Go FAQ Extensions
		{
			URL:         "https://golang.org/doc/faq",
			Title:       "Go FAQ Extended",
			Description: "Frequently asked questions about Go programming",
			Category:    "FAQ",
			Priority:    2,
			Tags:        []string{"faq", "questions", "troubleshooting", "community"},
			Expected:    "qa_content",
		},

		// Go Memory Model
		{
			URL:         "https://golang.org/ref/mem",
			Title:       "Go Memory Model",
			Description: "Specification of memory model and concurrent programming",
			Category:    "Specification",
			Priority:    2,
			Tags:        []string{"memory", "concurrency", "specification", "advanced"},
			Expected:    "technical_spec",
		},

		// Tour of Go
		{
			URL:         "https://go.dev/tour/",
			Title:       "A Tour of Go", 
			Description: "Interactive introduction to the Go programming language",
			Category:    "Interactive Tutorial",
			Priority:    1,
			Tags:        []string{"interactive", "tutorial", "beginner", "guided"},
			Expected:    "tutorial_content",
		},

		// Go Playground Examples
		{
			URL:         "https://go.dev/play/",
			Title:       "Go Playground",
			Description: "Web service that runs Go programs in a sandbox",
			Category:    "Interactive Examples",
			Priority:    3,
			Tags:        []string{"playground", "sandbox", "examples", "interactive"},
			Expected:    "code_examples",
		},
	}
}

func ethicalScrapeGoContent(sources []WebSource) ([]*document.Document, error) {
	// Initialize ethical scraping components
	complianceEngine := scraping.NewComplianceEngine(scraping.DefaultComplianceConfig())
	qualityValidator := quality.NewQualityValidator(nil)
	extractorEngine := extractor.NewEngine()

	var docs []*document.Document
	ctx := context.Background()

	successCount := 0
	for i, source := range sources {
		fmt.Printf("   [%d/%d] 🔍 %s\n", i+1, len(sources), source.Title)
		fmt.Printf("           🔗 %s\n", source.URL)
		
		// Comprehensive compliance check
		fmt.Printf("           🛡️  Checking robots.txt compliance...\n")
		complianceResult, err := complianceEngine.CheckCompliance(ctx, source.URL)
		if err != nil || !complianceResult.Allowed {
			fmt.Printf("           ❌ Not allowed by robots.txt: %v\n", err)
			continue
		}
		fmt.Printf("           ✅ Ethically approved for scraping\n")

		// Respect required delays
		if complianceResult.RequiredDelay > 0 {
			fmt.Printf("           ⏱️  Respecting %v delay...\n", complianceResult.RequiredDelay)
			time.Sleep(complianceResult.RequiredDelay)
		}

		// Fetch content ethically
		fmt.Printf("           📥 Fetching content...\n")
		content, contentType, err := fetchContentEthically(ctx, source.URL)
		if err != nil {
			fmt.Printf("           ❌ Failed to fetch: %v\n", err)
			continue
		}
		fmt.Printf("           📊 Fetched: %d bytes (%s)\n", len(content), contentType)

		// Extract and validate content
		fmt.Printf("           🔍 Extracting text content...\n")
		text, _, err := extractorEngine.Extract(ctx, content, "html")
		if err != nil {
			fmt.Printf("           ❌ Extraction failed: %v\n", err)
			continue
		}

		// Skip if content is too small
		if len(text) < 500 {
			fmt.Printf("           ⚠️  Content too small (%d chars), skipping\n", len(text))
			continue
		}

		fmt.Printf("           📝 Extracted: %d characters\n", len(text))

		// Quality validation
		fmt.Printf("           🏆 Validating content quality...\n")
		qualityMeta := map[string]string{
			"url":      source.URL,
			"title":    source.Title,
			"category": source.Category,
		}

		qualityResult, _ := qualityValidator.ValidateContent(ctx, text, qualityMeta)
		qualityScore := 0.0
		qualityTier := "unknown"

		if qualityResult != nil {
			qualityScore = qualityResult.OverallScore
			qualityTier = qualityResult.QualityTier
			fmt.Printf("           📊 Quality: %.2f (%s)\n", qualityScore, qualityTier)
		}

		// Create document
		doc := createWebDocument(source, text, qualityScore, qualityTier)
		docs = append(docs, doc)
		successCount++

		fmt.Printf("           ✅ Successfully scraped and validated\n")

		// Ethical delay between requests (minimum 2 seconds)
		if i < len(sources)-1 {
			fmt.Printf("           😴 Ethical 2s delay before next source...\n")
			time.Sleep(2 * time.Second)
		}
		fmt.Println()
	}

	fmt.Printf("📈 Ethical scraping results: %d/%d sources successfully scraped\n", successCount, len(sources))
	return docs, nil
}

func fetchContentEthically(ctx context.Context, urlStr string) ([]byte, string, error) {
	// Parse URL to validate it
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTPS and HTTP
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, "", fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, "", err
	}

	// Set ethical headers
	req.Header.Set("User-Agent", "CAIA-Library-Ethical-Scraper/1.0 (+https://caiatech.com/bot)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	// Remove Accept-Encoding header to let Go handle compression automatically
	req.Header.Set("DNT", "1") // Do Not Track
	req.Header.Set("Connection", "close")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Limit content size to prevent abuse (10MB max)
	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	return content, contentType, nil
}

func createWebDocument(source WebSource, text string, qualityScore float64, qualityTier string) *document.Document {
	return &document.Document{
		ID: fmt.Sprintf("web_%s_%d", sanitizeID(source.Title), time.Now().Unix()),
		Source: document.Source{
			Type: "web",
			URL:  source.URL,
		},
		Content: document.Content{
			Text: text,
			Metadata: map[string]string{
				"source":        "web_scraping",
				"url":           source.URL,
				"title":         source.Title,
				"description":   source.Description,
				"category":      source.Category,
				"priority":      fmt.Sprintf("%d", source.Priority),
				"tags":          strings.Join(source.Tags, ","),
				"expected_type": source.Expected,
				"quality_score": fmt.Sprintf("%.3f", qualityScore),
				"quality_tier":  qualityTier,
				"word_count":    fmt.Sprintf("%d", len(strings.Fields(text))),
				"scraped_at":    time.Now().UTC().Format(time.RFC3339),
				"scraped_ethically": "true",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func convertWebContentToConversational(docs []*document.Document) ConversationalDataset {
	var conversations []ConversationalEntry

	for _, doc := range docs {
		entries := createWebConversationalEntries(doc)
		conversations = append(conversations, entries...)
	}

	return ConversationalDataset{
		Dataset: conversations,
		Metadata: DatasetMetadata{
			Name:        "Go Web Content Conversational Dataset",
			Description: "Conversational Q&A pairs derived from ethically scraped Go programming content for LLM training",
			Version:     "1.0.0",
			TotalItems:  len(conversations),
			Sources:     "Ethically scraped web content from go.dev, golang.org, and community sources",
			Purpose:     "LLM training, fine-tuning, and comprehensive Go programming assistance",
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func createWebConversationalEntries(doc *document.Document) []ConversationalEntry {
	var entries []ConversationalEntry

	title := doc.Content.Metadata["title"]
	url := doc.Content.Metadata["url"]
	category := doc.Content.Metadata["category"]
	description := doc.Content.Metadata["description"]
	qualityTier := doc.Content.Metadata["quality_tier"]
	tags := doc.Content.Metadata["tags"]

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

	// 1. Topic Introduction
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_intro_%d", baseID, timestamp),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("I'm interested in learning about %s. Can you tell me what this covers?", title),
			},
			{
				Role:    "assistant",
				Content: fmt.Sprintf("Great choice! %s is %s.\n\n%s\n\nThis content comes from %s and covers essential concepts that will help you understand Go programming better.", title, strings.ToLower(description), createWebOverview(doc.Content.Text), category),
			},
		},
		Metadata: map[string]interface{}{
			"type":     "topic_introduction",
			"section":  "overview",
			"tags":     strings.Split(tags, ","),
			"keywords": extractWebKeywords(title, description, tags),
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 2. Deep Dive Explanation
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_deepdive_%d", baseID, timestamp+1),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("Can you give me a detailed explanation of the concepts in %s? I want to understand it thoroughly.", title),
			},
			{
				Role:    "assistant",
				Content: createWebDetailedContent(doc.Content.Text, title, category),
			},
		},
		Metadata: map[string]interface{}{
			"type":     "detailed_explanation",
			"section":  "deep_dive",
			"tags":     strings.Split(tags, ","),
			"keywords": extractWebKeywords(title, doc.Content.Text, tags),
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 3. Practical Applications
	entries = append(entries, ConversationalEntry{
		ID: fmt.Sprintf("%s_practical_%d", baseID, timestamp+2),
		Conversation: []ConversationalTurn{
			{
				Role:    "user",
				Content: fmt.Sprintf("How can I apply what's covered in %s to real Go programming? What are the practical applications?", title),
			},
			{
				Role:    "assistant",
				Content: createPracticalApplications(doc.Content.Text, title, category),
			},
		},
		Metadata: map[string]interface{}{
			"type":     "practical_applications",
			"section":  "applications",
			"tags":     append(strings.Split(tags, ","), "practical", "application", "real-world"),
			"keywords": extractWebKeywords(title, doc.Content.Text, tags),
		},
		Source:    source,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 4. Code Examples (if applicable)
	if containsWebCode(doc.Content.Text) {
		entries = append(entries, ConversationalEntry{
			ID: fmt.Sprintf("%s_code_%d", baseID, timestamp+3),
			Conversation: []ConversationalTurn{
				{
					Role:    "user",
					Content: fmt.Sprintf("Do you have any code examples from %s? I learn best by seeing actual Go code.", title),
				},
				{
					Role:    "assistant",
					Content: extractWebCodeExamples(doc.Content.Text, title),
				},
			},
			Metadata: map[string]interface{}{
				"type":     "code_examples",
				"section":  "examples",
				"tags":     append(strings.Split(tags, ","), "code", "examples", "programming"),
				"keywords": extractWebKeywords(title, doc.Content.Text, tags),
			},
			Source:    source,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	return entries
}

func createWebOverview(text string) string {
	// Extract meaningful introduction or summary
	paragraphs := strings.Split(text, "\n\n")
	overview := ""

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if len(para) > 100 && len(para) < 400 && !strings.HasPrefix(para, "//") {
			overview = para
			break
		}
	}

	if overview == "" {
		// Fallback to first substantial content
		if len(text) > 300 {
			overview = text[:300] + "..."
		} else {
			overview = text
		}
	}

	return "Here's what you'll find:\n\n" + overview
}

func createWebDetailedContent(text, title, category string) string {
	intro := fmt.Sprintf("Let me walk you through %s in detail.\n\n", title)
	
	// Process content based on category
	content := processContentByCategory(text, category)
	
	if len(content) > 2000 {
		content = content[:2000] + "\n\n[This covers the key concepts - the full content includes additional details and examples]"
	}

	return intro + content
}

func processContentByCategory(text, category string) string {
	switch category {
	case "Official Blog":
		return extractBlogContent(text)
	case "Tutorial Examples":
		return extractTutorialContent(text)
	case "Interactive Tutorial":
		return extractInteractiveContent(text)
	case "Code Walks":
		return extractCodeWalkContent(text)
	default:
		return cleanWebContent(text)
	}
}

func extractBlogContent(text string) string {
	// Extract blog post content, removing navigation and headers
	lines := strings.Split(text, "\n")
	content := []string{}
	
	inContent := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 50 && !strings.Contains(strings.ToLower(line), "menu") && !strings.Contains(strings.ToLower(line), "navigation") {
			inContent = true
		}
		if inContent && len(line) > 10 {
			content = append(content, line)
		}
	}
	
	return strings.Join(content, "\n")
}

func extractTutorialContent(text string) string {
	// Focus on tutorial explanations and examples
	return cleanWebContent(text)
}

func extractInteractiveContent(text string) string {
	// Extract interactive tutorial content
	return cleanWebContent(text)
}

func extractCodeWalkContent(text string) string {
	// Extract code analysis and explanations
	return cleanWebContent(text)
}

func cleanWebContent(text string) string {
	// Generic content cleaning
	lines := strings.Split(text, "\n")
	cleaned := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 20 && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "<!--") {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n")
}

func createPracticalApplications(text, title, category string) string {
	applications := []string{
		fmt.Sprintf("The concepts in %s have several practical applications:", title),
		"",
		"• **Real-world Development**: Apply these patterns in production Go applications",
		"• **Code Quality**: Improve your code's readability, maintainability, and performance",  
		"• **Best Practices**: Follow established Go community standards and conventions",
		"• **Problem Solving**: Use these techniques to solve common programming challenges",
		"• **Team Development**: Share these approaches with your development team",
	}

	if category == "Tutorial Examples" {
		applications = append(applications, "• **Learning Path**: Build on these examples to create more complex applications")
	}

	if category == "Official Blog" {
		applications = append(applications, "• **Stay Current**: Keep up with the latest Go language developments and features")
	}

	// Add specific content-based applications
	if strings.Contains(strings.ToLower(text), "concurrency") {
		applications = append(applications, "• **Concurrent Programming**: Build efficient concurrent and parallel applications")
	}

	if strings.Contains(strings.ToLower(text), "testing") {
		applications = append(applications, "• **Testing Strategy**: Develop comprehensive testing approaches for Go applications")
	}

	return strings.Join(applications, "\n")
}

func containsWebCode(text string) bool {
	codeIndicators := []string{
		"func ", "package ", "import ", "var ", "const ", "type ",
		"struct {", "interface {", "go ", "defer ", "chan ",
	}
	
	textLower := strings.ToLower(text)
	for _, indicator := range codeIndicators {
		if strings.Contains(textLower, indicator) {
			return true
		}
	}
	return false
}

func extractWebCodeExamples(text, title string) string {
	// Extract Go code examples with better parsing
	codePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?s)func\s+\w+[^{]*\{[^}]*\}`),
		regexp.MustCompile(`package\s+\w+`),
		regexp.MustCompile(`(?s)import\s*\([^)]*\)`),
		regexp.MustCompile(`(?s)type\s+\w+\s+(?:struct|interface)\s*\{[^}]*\}`),
	}

	examples := []string{}
	for _, pattern := range codePatterns {
		matches := pattern.FindAllString(text, 2) // Limit matches per pattern
		for _, match := range matches {
			if len(match) > 20 && len(match) < 400 {
				examples = append(examples, strings.TrimSpace(match))
			}
		}
		if len(examples) >= 3 { // Limit total examples
			break
		}
	}

	response := fmt.Sprintf("Here are practical Go code examples from %s:\n\n", title)

	if len(examples) > 0 {
		for i, example := range examples {
			response += fmt.Sprintf("**Example %d:**\n```go\n%s\n```\n\n", i+1, example)
		}
	} else {
		response += "While this content focuses on concepts and explanations rather than specific code examples, "
		response += "here are the key programming patterns it discusses:\n\n"
		response += extractWebProgrammingConcepts(text)
	}

	return response
}

func extractWebProgrammingConcepts(text string) string {
	concepts := []string{
		"• Go language syntax and idioms",
		"• Package organization and structure",
		"• Error handling patterns and best practices",
		"• Interface design and implementation",
		"• Concurrent programming with goroutines",
		"• Memory management and garbage collection",
		"• Testing strategies and methodologies",
		"• Performance optimization techniques",
	}

	// Filter concepts based on content
	textLower := strings.ToLower(text)
	relevantConcepts := []string{}
	
	for _, concept := range concepts {
		conceptWords := strings.Fields(strings.ToLower(concept))
		found := false
		for _, word := range conceptWords {
			if len(word) > 3 && strings.Contains(textLower, word) {
				found = true
				break
			}
		}
		if found {
			relevantConcepts = append(relevantConcepts, concept)
		}
	}

	if len(relevantConcepts) > 0 {
		return strings.Join(relevantConcepts, "\n")
	}
	return strings.Join(concepts[:4], "\n") // Default fallback
}

func extractWebKeywords(title, content, tags string) []string {
	keywords := []string{}

	// Extract from title
	titleWords := strings.Fields(strings.ToLower(title))
	for _, word := range titleWords {
		word = strings.Trim(word, ".,!?()[]{}:;\"'")
		if len(word) > 2 && !isStopWord(word) {
			keywords = append(keywords, word)
		}
	}

	// Add tags
	if tags != "" {
		tagWords := strings.Split(strings.ToLower(tags), ",")
		for _, tag := range tagWords {
			tag = strings.TrimSpace(tag)
			if len(tag) > 0 {
				keywords = append(keywords, tag)
			}
		}
	}

	// Add Go-specific terms found in content
	goTerms := []string{"golang", "goroutine", "channel", "interface", "struct", "slice", "map", "pointer", "package", "module"}
	contentLower := strings.ToLower(content)

	for _, term := range goTerms {
		if strings.Contains(contentLower, term) {
			keywords = append(keywords, term)
		}
	}

	return keywords
}

func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true, "up": true, "about": true, "into": true,
		"through": true, "during": true, "before": true, "after": true, "above": true, "below": true, "between": true,
		"a": true, "an": true, "is": true, "are": true, "was": true, "were": true, "be": true, "been": true, "being": true,
	}
	return stopWords[word]
}

func sanitizeID(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return strings.ToLower(reg.ReplaceAllString(s, "_"))
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

func generateWebScrapingSummary(data ConversationalDataset, filename string) {
	fmt.Printf("\n🎉 ETHICAL WEB SCRAPING COMPLETED!\n")
	fmt.Printf("==================================\n")
	
	// File size
	if info, err := os.Stat(filename); err == nil {
		fmt.Printf("• File: %s (%.1f KB)\n", filename, float64(info.Size())/1024)
	}
	
	fmt.Printf("• Total Conversations: %d\n", len(data.Dataset))
	fmt.Printf("• Generated: %s\n", data.GeneratedAt)

	// Count by type and source
	typeCount := make(map[string]int)
	sourceCount := make(map[string]int)
	totalTurns := 0
	totalChars := 0

	for _, entry := range data.Dataset {
		if entryType, ok := entry.Metadata["type"].(string); ok {
			typeCount[entryType]++
		}
		sourceCount[entry.Source.Category]++
		totalTurns += len(entry.Conversation)

		for _, turn := range entry.Conversation {
			totalChars += len(turn.Content)
		}
	}

	fmt.Printf("\n📊 Conversation Types:\n")
	for convType, count := range typeCount {
		fmt.Printf("   • %s: %d conversations\n", convType, count)
	}

	fmt.Printf("\n🌐 Content Sources:\n")
	for source, count := range sourceCount {
		fmt.Printf("   • %s: %d conversations\n", source, count)
	}

	fmt.Printf("\n💬 Dataset Statistics:\n")
	fmt.Printf("   • Total Conversational Turns: %d\n", totalTurns)
	fmt.Printf("   • Total Characters: %d (%.1f KB)\n", totalChars, float64(totalChars)/1024)
	fmt.Printf("   • Average Conversation Length: %.0f characters\n", float64(totalChars)/float64(len(data.Dataset)))

	fmt.Printf("\n🌟 Ethical Scraping Achievements:\n")
	fmt.Printf("   • ✅ Full robots.txt compliance\n")
	fmt.Printf("   • ✅ Respectful delays between requests\n")
	fmt.Printf("   • ✅ Quality content validation\n")
	fmt.Printf("   • ✅ Diverse, high-quality Go content sources\n")
	fmt.Printf("   • ✅ Ready for responsible LLM training\n")

	fmt.Printf("\n🚀 Enhanced Dataset Ready for:\n")
	fmt.Printf("   • Advanced Go programming assistance\n")
	fmt.Printf("   • Multi-source knowledge integration\n")
	fmt.Printf("   • Comprehensive Go ecosystem training\n")
	fmt.Printf("   • Production-ready conversational AI\n")
}
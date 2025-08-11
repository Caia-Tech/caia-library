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
	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"github.com/Caia-Tech/caia-library/pkg/logging"
	"github.com/Caia-Tech/caia-library/pkg/pipeline"
)

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
	URL       string `json:"url"`
	Title     string `json:"title"`
	Domain    string `json:"domain"`
	Category  string `json:"category"`
	WordCount int    `json:"word_count"`
	Quality   string `json:"quality"`
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

type Source struct {
	URL      string
	Title    string
	Domain   string
	Category string
}

func main() {
	fmt.Println("🚀 HIGH-QUALITY CONTENT RE-SCRAPER")
	fmt.Println("==================================")
	fmt.Println("Re-scraping with proper content extraction")
	fmt.Println()

	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "warn"
	
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	// High-value sources to re-scrape
	sources := []Source{
		// Go Documentation
		{URL: "https://go.dev/doc/effective_go", Title: "Effective Go", Domain: "Programming", Category: "Go"},
		{URL: "https://go.dev/doc/tutorial/getting-started", Title: "Getting Started with Go", Domain: "Programming", Category: "Go"},
		{URL: "https://go.dev/doc/tutorial/create-module", Title: "Create a Go Module", Domain: "Programming", Category: "Go"},
		{URL: "https://go.dev/ref/spec", Title: "Go Language Specification", Domain: "Programming", Category: "Go"},
		
		// Computer Science & AI
		{URL: "https://en.wikipedia.org/wiki/Machine_learning", Title: "Machine Learning", Domain: "Technology", Category: "AI"},
		{URL: "https://en.wikipedia.org/wiki/Deep_learning", Title: "Deep Learning", Domain: "Technology", Category: "AI"},
		{URL: "https://en.wikipedia.org/wiki/Natural_language_processing", Title: "Natural Language Processing", Domain: "Technology", Category: "AI"},
		{URL: "https://en.wikipedia.org/wiki/Computer_science", Title: "Computer Science", Domain: "Technology", Category: "CS"},
		
		// Mathematics & Logic
		{URL: "https://en.wikipedia.org/wiki/Algorithm", Title: "Algorithms", Domain: "Mathematics", Category: "CS"},
		{URL: "https://en.wikipedia.org/wiki/Data_structure", Title: "Data Structures", Domain: "Mathematics", Category: "CS"},
		{URL: "https://en.wikipedia.org/wiki/Computational_complexity_theory", Title: "Computational Complexity", Domain: "Mathematics", Category: "CS"},
		
		// Software Engineering
		{URL: "https://en.wikipedia.org/wiki/Software_engineering", Title: "Software Engineering", Domain: "Engineering", Category: "Software"},
		{URL: "https://en.wikipedia.org/wiki/Design_pattern", Title: "Design Patterns", Domain: "Engineering", Category: "Software"},
		{URL: "https://en.wikipedia.org/wiki/Test-driven_development", Title: "Test-Driven Development", Domain: "Engineering", Category: "Software"},
		{URL: "https://en.wikipedia.org/wiki/Agile_software_development", Title: "Agile Development", Domain: "Engineering", Category: "Software"},
	}

	// Initialize components
	complianceEngine := scraping.NewComplianceEngine(scraping.DefaultComplianceConfig())
	extractorEngine := extractor.NewEngine()
	ctx := context.Background()

	var conversations []ConversationalEntry
	successCount := 0

	fmt.Printf("📥 Scraping %d high-value sources...\n", len(sources))
	
	for i, source := range sources {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(sources), source.Title)
		fmt.Printf("       URL: %s\n", source.URL)
		
		// Check compliance
		complianceResult, err := complianceEngine.CheckCompliance(ctx, source.URL)
		if err != nil || !complianceResult.Allowed {
			fmt.Printf("       ❌ Not allowed: %v\n", err)
			continue
		}

		// Respect crawl delay
		if complianceResult.RequiredDelay > 0 {
			fmt.Printf("       ⏱️  Waiting %.1fs (robots.txt delay)\n", complianceResult.RequiredDelay.Seconds())
			time.Sleep(complianceResult.RequiredDelay)
		}

		// Fetch content
		content, err := fetchURL(source.URL)
		if err != nil {
			fmt.Printf("       ❌ Fetch failed: %v\n", err)
			continue
		}

		// Extract with improved parser
		text, metadata, err := extractorEngine.Extract(ctx, content, "html")
		if err != nil {
			fmt.Printf("       ❌ Extraction failed: %v\n", err)
			continue
		}

		// Quality assessment
		wordCount := len(strings.Fields(text))
		quality := assessQuality(wordCount, text)
		
		fmt.Printf("       ✅ Extracted: %d words (quality: %s)\n", wordCount, quality)
		
		// Skip if too short
		if wordCount < 100 {
			fmt.Printf("       ⚠️  Skipping: content too short\n")
			continue
		}

		// Create conversational entries
		entries := createConversationalEntries(source, text, metadata, wordCount, quality)
		conversations = append(conversations, entries...)
		successCount++
		
		// Show progress
		fmt.Printf("       📝 Generated %d conversations\n", len(entries))
		
		// Be respectful between requests
		if i < len(sources)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	// Create dataset
	dataset := ConversationalDataset{
		Dataset: conversations,
		Metadata: DatasetMetadata{
			Name:        "High-Quality Re-scraped Dataset",
			Description: "Properly extracted content with meaningful conversations",
			Version:     "2.0.0",
			TotalItems:  len(conversations),
			Domains:     "Programming, Technology, Mathematics, Engineering",
			Purpose:     "High-quality LLM training with properly extracted content",
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Save dataset
	outputFile := "high_quality_conversational_dataset.json"
	fmt.Printf("\n💾 Saving to %s...\n", outputFile)
	
	if err := saveDataset(dataset, outputFile); err != nil {
		fmt.Printf("❌ Failed to save: %v\n", err)
		return
	}

	// Summary
	fmt.Println("\n✅ HIGH-QUALITY RE-SCRAPING COMPLETE")
	fmt.Printf("   • Sources processed: %d/%d\n", successCount, len(sources))
	fmt.Printf("   • Total conversations: %d\n", len(conversations))
	fmt.Printf("   • Average conversations/source: %.1f\n", float64(len(conversations))/float64(successCount))
	
	// Domain breakdown
	domainCount := make(map[string]int)
	for _, conv := range conversations {
		domainCount[conv.Source.Domain]++
	}
	
	fmt.Println("\n📊 Domain Distribution:")
	for domain, count := range domainCount {
		fmt.Printf("   • %s: %d conversations\n", domain, count)
	}
	
	// Quality distribution
	qualityCount := make(map[string]int)
	for _, conv := range conversations {
		qualityCount[conv.Source.Quality]++
	}
	
	fmt.Println("\n⭐ Quality Distribution:")
	for quality, count := range qualityCount {
		fmt.Printf("   • %s: %d conversations\n", quality, count)
	}
}

func fetchURL(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "CAIA-Library-Quality-Scraper/2.0 (Educational/Research)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	
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

func assessQuality(wordCount int, text string) string {
	// Assess based on multiple factors
	hasCodeExamples := strings.Contains(text, "func ") || strings.Contains(text, "function ") || 
		strings.Contains(text, "class ") || strings.Contains(text, "def ")
	hasStructure := strings.Count(text, "\n\n") > 10
	
	if wordCount > 5000 && hasStructure {
		return "excellent"
	} else if wordCount > 2000 || (wordCount > 1000 && hasCodeExamples) {
		return "high"
	} else if wordCount > 500 {
		return "medium"
	}
	return "low"
}

func createConversationalEntries(source Source, text string, metadata map[string]string, wordCount int, quality string) []ConversationalEntry {
	var entries []ConversationalEntry
	
	// Clean and prepare text
	cleanedText := cleanText(text)
	sections := extractSections(cleanedText)
	
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	baseID := sanitizeID(source.Title)
	
	sourceInfo := ConversationalSource{
		URL:       source.URL,
		Title:     source.Title,
		Domain:    source.Domain,
		Category:  source.Category,
		WordCount: wordCount,
		Quality:   quality,
	}
	
	// 1. Introduction conversation
	intro := createIntroduction(source.Title, sections)
	if intro != "" {
		entries = append(entries, ConversationalEntry{
			ID: fmt.Sprintf("%s_intro_%s", baseID, timestamp),
			Conversation: []ConversationalTurn{
				{
					Role:    "user",
					Content: fmt.Sprintf("Can you explain %s to me?", source.Title),
				},
				{
					Role:    "assistant",
					Content: intro,
				},
			},
			Metadata: map[string]interface{}{
				"type":       "introduction",
				"difficulty": "beginner",
				"domain":     source.Domain,
			},
			Source:    sourceInfo,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	
	// 2. Detailed explanation
	if len(sections) > 1 {
		detailed := createDetailedExplanation(source.Title, sections)
		entries = append(entries, ConversationalEntry{
			ID: fmt.Sprintf("%s_detailed_%s", baseID, timestamp),
			Conversation: []ConversationalTurn{
				{
					Role:    "user",
					Content: fmt.Sprintf("Can you go deeper into the key concepts of %s?", source.Title),
				},
				{
					Role:    "assistant",
					Content: detailed,
				},
			},
			Metadata: map[string]interface{}{
				"type":       "detailed_explanation",
				"difficulty": "intermediate",
				"domain":     source.Domain,
			},
			Source:    sourceInfo,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	
	// 3. Code examples (if applicable)
	if source.Domain == "Programming" || strings.Contains(text, "func ") || strings.Contains(text, "package ") {
		examples := extractCodeExamples(cleanedText)
		if examples != "" {
			entries = append(entries, ConversationalEntry{
				ID: fmt.Sprintf("%s_examples_%s", baseID, timestamp),
				Conversation: []ConversationalTurn{
					{
						Role:    "user",
						Content: fmt.Sprintf("Can you show me practical examples of %s?", source.Title),
					},
					{
						Role:    "assistant",
						Content: examples,
					},
				},
				Metadata: map[string]interface{}{
					"type":       "code_examples",
					"difficulty": "practical",
					"domain":     source.Domain,
				},
				Source:    sourceInfo,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
	
	// 4. Q&A based on content
	qa := createQAFromContent(source.Title, sections)
	if qa.Question != "" && qa.Answer != "" {
		entries = append(entries, ConversationalEntry{
			ID: fmt.Sprintf("%s_qa_%s", baseID, timestamp),
			Conversation: []ConversationalTurn{
				{
					Role:    "user",
					Content: qa.Question,
				},
				{
					Role:    "assistant",
					Content: qa.Answer,
				},
			},
			Metadata: map[string]interface{}{
				"type":       "question_answer",
				"difficulty": "applied",
				"domain":     source.Domain,
			},
			Source:    sourceInfo,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	
	return entries
}

func cleanText(text string) string {
	// Remove navigation artifacts
	lines := strings.Split(text, "\n")
	var cleaned []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip common navigation elements
		if strings.HasPrefix(line, "Toggle") ||
			strings.HasPrefix(line, "Main page") ||
			strings.HasPrefix(line, "Create account") ||
			strings.HasPrefix(line, "Log in") ||
			strings.HasPrefix(line, "Contents") ||
			strings.Contains(line, "navigation") ||
			strings.Contains(line, "sidebar") ||
			line == "" {
			continue
		}
		
		cleaned = append(cleaned, line)
	}
	
	return strings.Join(cleaned, "\n")
}

func extractSections(text string) []string {
	// Split into meaningful sections
	var sections []string
	current := strings.Builder{}
	lines := strings.Split(text, "\n")
	
	for _, line := range lines {
		current.WriteString(line + "\n")
		
		// Section boundary detection
		if len(current.String()) > 500 && (strings.HasPrefix(line, "#") || 
			line == "" || len(sections) < 5) {
			if section := strings.TrimSpace(current.String()); section != "" {
				sections = append(sections, section)
				current.Reset()
			}
		}
	}
	
	// Add remaining content
	if section := strings.TrimSpace(current.String()); section != "" {
		sections = append(sections, section)
	}
	
	return sections
}

func createIntroduction(title string, sections []string) string {
	if len(sections) == 0 {
		return ""
	}
	
	intro := fmt.Sprintf("Let me explain %s.\n\n", title)
	
	// Use first meaningful section
	firstSection := sections[0]
	if len(firstSection) > 1500 {
		firstSection = firstSection[:1500] + "..."
	}
	
	intro += firstSection
	return intro
}

func createDetailedExplanation(title string, sections []string) string {
	if len(sections) < 2 {
		return ""
	}
	
	explanation := fmt.Sprintf("Here's a deeper look at %s:\n\n", title)
	
	// Combine key sections
	for i := 1; i < len(sections) && i < 4; i++ {
		section := sections[i]
		if len(section) > 800 {
			section = section[:800] + "..."
		}
		explanation += section + "\n\n"
	}
	
	return strings.TrimSpace(explanation)
}

func extractCodeExamples(text string) string {
	var examples []string
	lines := strings.Split(text, "\n")
	inCode := false
	currentCode := strings.Builder{}
	
	for _, line := range lines {
		// Detect code blocks
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "    ") || 
			strings.Contains(line, "func ") || strings.Contains(line, "package ") {
			inCode = true
			currentCode.WriteString(line + "\n")
		} else if inCode && (line == "" || !strings.HasPrefix(line, " ")) {
			if code := currentCode.String(); len(code) > 50 {
				examples = append(examples, code)
			}
			currentCode.Reset()
			inCode = false
		} else if inCode {
			currentCode.WriteString(line + "\n")
		}
		
		// Limit examples
		if len(examples) >= 3 {
			break
		}
	}
	
	if len(examples) == 0 {
		return ""
	}
	
	result := "Here are some practical examples:\n\n"
	for i, example := range examples {
		result += fmt.Sprintf("Example %d:\n%s\n", i+1, example)
	}
	
	return result
}

type QA struct {
	Question string
	Answer   string
}

func createQAFromContent(title string, sections []string) QA {
	if len(sections) == 0 {
		return QA{}
	}
	
	// Generate contextual Q&A
	questions := []string{
		fmt.Sprintf("What are the key benefits of %s?", title),
		fmt.Sprintf("How does %s work in practice?", title),
		fmt.Sprintf("What should I know about %s?", title),
		fmt.Sprintf("Why is %s important?", title),
	}
	
	// Pick a question based on content
	question := questions[len(sections)%len(questions)]
	
	// Create answer from available sections
	answer := ""
	for _, section := range sections {
		if strings.Contains(strings.ToLower(section), "benefit") ||
			strings.Contains(strings.ToLower(section), "important") ||
			strings.Contains(strings.ToLower(section), "advantage") ||
			strings.Contains(strings.ToLower(section), "why") {
			answer = section
			break
		}
	}
	
	if answer == "" && len(sections) > 0 {
		answer = sections[len(sections)-1]
	}
	
	if len(answer) > 1000 {
		answer = answer[:1000] + "..."
	}
	
	return QA{
		Question: question,
		Answer:   answer,
	}
}

func sanitizeID(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	s = reg.ReplaceAllString(s, "_")
	s = strings.ToLower(s)
	s = strings.Trim(s, "_")
	return s
}

func saveDataset(dataset ConversationalDataset, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(dataset)
}
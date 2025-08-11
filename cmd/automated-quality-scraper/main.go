package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
)

type QualitySource struct {
	URL         string `json:"url"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Title       string `json:"title"`
	Quality     string `json:"quality"`
	Language    string `json:"language"`
	Expected    string `json:"expected_content"`
}

type ScrapedDocument struct {
	ID          string            `json:"id"`
	Source      QualitySource     `json:"source"`
	Content     string            `json:"content"`
	CleanText   string            `json:"clean_text"`
	WordCount   int               `json:"word_count"`
	CharCount   int               `json:"char_count"`
	Quality     float64           `json:"quality_score"`
	Metadata    map[string]string `json:"metadata"`
	ScrapedAt   time.Time         `json:"scraped_at"`
	ProcessedAt time.Time         `json:"processed_at"`
}

// Comprehensive quality training sources
var qualityTrainingSources = []QualitySource{
	// AI & Machine Learning - Foundational
	{URL: "https://en.wikipedia.org/wiki/Artificial_intelligence", Category: "AI_ML", Subcategory: "foundations", Title: "Artificial Intelligence", Quality: "high", Language: "en", Expected: "comprehensive_overview"},
	{URL: "https://en.wikipedia.org/wiki/Machine_learning", Category: "AI_ML", Subcategory: "foundations", Title: "Machine Learning", Quality: "high", Language: "en", Expected: "algorithms_theory"},
	{URL: "https://en.wikipedia.org/wiki/Deep_learning", Category: "AI_ML", Subcategory: "advanced", Title: "Deep Learning", Quality: "high", Language: "en", Expected: "neural_networks"},
	{URL: "https://en.wikipedia.org/wiki/Neural_network", Category: "AI_ML", Subcategory: "foundations", Title: "Neural Networks", Quality: "high", Language: "en", Expected: "network_architecture"},
	{URL: "https://en.wikipedia.org/wiki/Natural_language_processing", Category: "AI_ML", Subcategory: "nlp", Title: "Natural Language Processing", Quality: "high", Language: "en", Expected: "text_processing"},
	{URL: "https://en.wikipedia.org/wiki/Computer_vision", Category: "AI_ML", Subcategory: "vision", Title: "Computer Vision", Quality: "high", Language: "en", Expected: "image_processing"},
	
	// Programming & Software Engineering
	{URL: "https://go.dev/doc/effective_go", Category: "programming", Subcategory: "go_lang", Title: "Effective Go", Quality: "high", Language: "en", Expected: "best_practices"},
	{URL: "https://go.dev/doc/tutorial/getting-started", Category: "programming", Subcategory: "go_lang", Title: "Go Tutorial", Quality: "high", Language: "en", Expected: "hands_on_learning"},
	{URL: "https://golang.org/doc/", Category: "programming", Subcategory: "go_lang", Title: "Go Documentation", Quality: "high", Language: "en", Expected: "reference_material"},
	{URL: "https://en.wikipedia.org/wiki/Software_engineering", Category: "programming", Subcategory: "fundamentals", Title: "Software Engineering", Quality: "high", Language: "en", Expected: "methodology"},
	{URL: "https://en.wikipedia.org/wiki/Algorithm", Category: "programming", Subcategory: "fundamentals", Title: "Algorithms", Quality: "high", Language: "en", Expected: "computational_theory"},
	{URL: "https://en.wikipedia.org/wiki/Data_structure", Category: "programming", Subcategory: "fundamentals", Title: "Data Structures", Quality: "high", Language: "en", Expected: "organization_methods"},
	
	// Mathematics & Statistics
	{URL: "https://en.wikipedia.org/wiki/Mathematics", Category: "mathematics", Subcategory: "foundations", Title: "Mathematics", Quality: "high", Language: "en", Expected: "broad_overview"},
	{URL: "https://en.wikipedia.org/wiki/Statistics", Category: "mathematics", Subcategory: "statistics", Title: "Statistics", Quality: "high", Language: "en", Expected: "data_analysis"},
	{URL: "https://en.wikipedia.org/wiki/Linear_algebra", Category: "mathematics", Subcategory: "algebra", Title: "Linear Algebra", Quality: "high", Language: "en", Expected: "vector_operations"},
	{URL: "https://en.wikipedia.org/wiki/Calculus", Category: "mathematics", Subcategory: "analysis", Title: "Calculus", Quality: "high", Language: "en", Expected: "differential_integral"},
	{URL: "https://en.wikipedia.org/wiki/Probability", Category: "mathematics", Subcategory: "statistics", Title: "Probability Theory", Quality: "high", Language: "en", Expected: "random_processes"},
	{URL: "https://en.wikipedia.org/wiki/Discrete_mathematics", Category: "mathematics", Subcategory: "discrete", Title: "Discrete Mathematics", Quality: "high", Language: "en", Expected: "logic_combinatorics"},
	
	// Computer Science Fundamentals
	{URL: "https://en.wikipedia.org/wiki/Computer_science", Category: "computer_science", Subcategory: "foundations", Title: "Computer Science", Quality: "high", Language: "en", Expected: "field_overview"},
	{URL: "https://en.wikipedia.org/wiki/Computational_complexity_theory", Category: "computer_science", Subcategory: "theory", Title: "Complexity Theory", Quality: "high", Language: "en", Expected: "algorithmic_analysis"},
	{URL: "https://en.wikipedia.org/wiki/Database", Category: "computer_science", Subcategory: "systems", Title: "Database Systems", Quality: "high", Language: "en", Expected: "data_management"},
	{URL: "https://en.wikipedia.org/wiki/Operating_system", Category: "computer_science", Subcategory: "systems", Title: "Operating Systems", Quality: "high", Language: "en", Expected: "system_management"},
	
	// Technology & Innovation
	{URL: "https://en.wikipedia.org/wiki/Technology", Category: "technology", Subcategory: "general", Title: "Technology", Quality: "high", Language: "en", Expected: "innovation_overview"},
	{URL: "https://en.wikipedia.org/wiki/Information_technology", Category: "technology", Subcategory: "information", Title: "Information Technology", Quality: "high", Language: "en", Expected: "it_systems"},
	{URL: "https://en.wikipedia.org/wiki/Internet", Category: "technology", Subcategory: "networking", Title: "Internet", Quality: "high", Language: "en", Expected: "network_infrastructure"},
	{URL: "https://en.wikipedia.org/wiki/World_Wide_Web", Category: "technology", Subcategory: "web", Title: "World Wide Web", Quality: "high", Language: "en", Expected: "web_technologies"},
	
	// Physics & Sciences (for technical training)
	{URL: "https://en.wikipedia.org/wiki/Physics", Category: "science", Subcategory: "physics", Title: "Physics", Quality: "high", Language: "en", Expected: "natural_laws"},
	{URL: "https://en.wikipedia.org/wiki/Quantum_computing", Category: "science", Subcategory: "quantum", Title: "Quantum Computing", Quality: "high", Language: "en", Expected: "quantum_mechanics"},
	{URL: "https://en.wikipedia.org/wiki/Cryptography", Category: "science", Subcategory: "security", Title: "Cryptography", Quality: "high", Language: "en", Expected: "security_methods"},
	
	// Business & Economics (for well-rounded training)
	{URL: "https://en.wikipedia.org/wiki/Economics", Category: "business", Subcategory: "economics", Title: "Economics", Quality: "high", Language: "en", Expected: "economic_principles"},
	{URL: "https://en.wikipedia.org/wiki/Management", Category: "business", Subcategory: "management", Title: "Management", Quality: "high", Language: "en", Expected: "organizational_theory"},
}

func main() {
	fmt.Println("🤖 AUTOMATED QUALITY CONTENT SCRAPER")
	fmt.Println("====================================")
	fmt.Printf("Scraping %d high-quality sources for LLM training\n\n", len(qualityTrainingSources))

	// Create organized folder structure
	baseDir := "./training-content"
	if err := setupFolderStructure(baseDir); err != nil {
		log.Fatalf("❌ Failed to setup folder structure: %v", err)
	}

	// Initialize HTTP client with reasonable timeouts
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var allDocuments []ScrapedDocument
	categoryStats := make(map[string]int)
	successCount := 0
	
	fmt.Println("🔄 Starting automated scraping...")
	
	for i, source := range qualityTrainingSources {
		fmt.Printf("\n[%d/%d] Scraping: %s\n", i+1, len(qualityTrainingSources), source.Title)
		fmt.Printf("        URL: %s\n", source.URL)
		fmt.Printf("        Category: %s/%s\n", source.Category, source.Subcategory)
		
		document, err := scrapeDocument(client, source)
		if err != nil {
			fmt.Printf("        ❌ Failed: %v\n", err)
			continue
		}
		
		// Quality filtering
		if document.Quality < 0.6 { // Minimum quality threshold
			fmt.Printf("        ⚠️  Low quality score: %.2f - skipped\n", document.Quality)
			continue
		}
		
		// Save document to organized structure
		if err := saveDocument(baseDir, document); err != nil {
			fmt.Printf("        ❌ Save failed: %v\n", err)
			continue
		}
		
		allDocuments = append(allDocuments, *document)
		categoryStats[source.Category]++
		successCount++
		
		fmt.Printf("        ✅ Success! Quality: %.2f, Words: %d\n", document.Quality, document.WordCount)
		
		// Respectful rate limiting
		if i < len(qualityTrainingSources)-1 {
			time.Sleep(2 * time.Second)
		}
	}
	
	// Generate comprehensive summary
	generateSummary(baseDir, allDocuments, categoryStats, successCount)
	
	fmt.Printf("\n🎉 AUTOMATED SCRAPING COMPLETE!\n")
	fmt.Printf("📁 Content saved to: %s\n", baseDir)
	fmt.Printf("📊 Success rate: %d/%d (%.1f%%)\n", 
		successCount, len(qualityTrainingSources),
		float64(successCount)/float64(len(qualityTrainingSources))*100)
}

func setupFolderStructure(baseDir string) error {
	categories := []string{
		"AI_ML/foundations", "AI_ML/advanced", "AI_ML/nlp", "AI_ML/vision",
		"programming/go_lang", "programming/fundamentals",
		"mathematics/foundations", "mathematics/statistics", "mathematics/algebra", "mathematics/analysis", "mathematics/discrete",
		"computer_science/foundations", "computer_science/theory", "computer_science/systems",
		"technology/general", "technology/information", "technology/networking", "technology/web",
		"science/physics", "science/quantum", "science/security",
		"business/economics", "business/management",
	}
	
	// Create base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}
	
	// Create category subdirectories
	for _, category := range categories {
		categoryPath := filepath.Join(baseDir, category)
		if err := os.MkdirAll(categoryPath, 0755); err != nil {
			return err
		}
	}
	
	// Create additional directories
	additionalDirs := []string{"raw", "processed", "summaries", "indexes"}
	for _, dir := range additionalDirs {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0755); err != nil {
			return err
		}
	}
	
	fmt.Printf("📁 Organized folder structure created: %s\n", baseDir)
	return nil
}

func scrapeDocument(client *http.Client, source QualitySource) (*ScrapedDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Educational-Content-Collector/1.0)")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	content := string(body)
	cleanText := extractCleanText(content)
	
	document := &ScrapedDocument{
		ID:        uuid.New().String(),
		Source:    source,
		Content:   content,
		CleanText: cleanText,
		WordCount: len(strings.Fields(cleanText)),
		CharCount: len(cleanText),
		Quality:   calculateQualityScore(cleanText, source),
		Metadata: map[string]string{
			"content_type":   resp.Header.Get("Content-Type"),
			"content_length": fmt.Sprintf("%d", len(content)),
			"status_code":    fmt.Sprintf("%d", resp.StatusCode),
			"url":           source.URL,
		},
		ScrapedAt:   time.Now(),
		ProcessedAt: time.Now(),
	}
	
	return document, nil
}

func extractCleanText(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		// Fallback: basic HTML tag removal
		re := regexp.MustCompile(`<[^>]*>`)
		return re.ReplaceAllString(htmlContent, " ")
	}
	
	// Remove script, style, and nav elements
	doc.Find("script, style, nav, header, footer, aside").Remove()
	
	// Extract main content
	text := doc.Find("body").Text()
	
	// Clean up whitespace
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	
	return text
}

func calculateQualityScore(text string, source QualitySource) float64 {
	score := 0.0
	
	// Length quality (longer content generally better for training)
	wordCount := len(strings.Fields(text))
	if wordCount > 10000 {
		score += 0.3
	} else if wordCount > 5000 {
		score += 0.2
	} else if wordCount > 1000 {
		score += 0.1
	}
	
	// Educational content indicators
	educationalKeywords := []string{
		"definition", "concept", "theory", "principle", "method", "algorithm",
		"example", "explanation", "analysis", "research", "study", "development",
		"process", "system", "technique", "approach", "framework", "model",
	}
	
	textLower := strings.ToLower(text)
	keywordCount := 0
	for _, keyword := range educationalKeywords {
		if strings.Contains(textLower, keyword) {
			keywordCount++
		}
	}
	
	// Bonus for educational keywords
	score += float64(keywordCount) * 0.02
	
	// Source quality bonus
	if strings.Contains(source.URL, "wikipedia.org") {
		score += 0.2 // Wikipedia generally high quality
	} else if strings.Contains(source.URL, ".edu") {
		score += 0.3 // Educational institutions
	} else if strings.Contains(source.URL, "go.dev") || strings.Contains(source.URL, "golang.org") {
		score += 0.25 // Official documentation
	}
	
	// Technical depth indicators
	technicalTerms := []string{
		"implementation", "optimization", "architecture", "performance",
		"complexity", "efficiency", "scalability", "methodology",
	}
	
	techCount := 0
	for _, term := range technicalTerms {
		if strings.Contains(textLower, term) {
			techCount++
		}
	}
	score += float64(techCount) * 0.03
	
	// Normalize score to 0-1 range
	if score > 1.0 {
		score = 1.0
	}
	
	return score
}

func saveDocument(baseDir string, document *ScrapedDocument) error {
	// Create category-specific path
	categoryPath := filepath.Join(baseDir, document.Source.Category, document.Source.Subcategory)
	
	// Save raw content
	rawFile := filepath.Join(categoryPath, fmt.Sprintf("%s_raw.html", document.ID))
	if err := ioutil.WriteFile(rawFile, []byte(document.Content), 0644); err != nil {
		return err
	}
	
	// Save clean text
	textFile := filepath.Join(categoryPath, fmt.Sprintf("%s_text.txt", document.ID))
	if err := ioutil.WriteFile(textFile, []byte(document.CleanText), 0644); err != nil {
		return err
	}
	
	// Save metadata
	metadataFile := filepath.Join(categoryPath, fmt.Sprintf("%s_metadata.json", document.ID))
	metadataBytes, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(metadataFile, metadataBytes, 0644); err != nil {
		return err
	}
	
	// Also save to processed directory with better naming
	processedDir := filepath.Join(baseDir, "processed")
	filename := fmt.Sprintf("%s_%s_%s", 
		document.Source.Category,
		strings.ReplaceAll(document.Source.Title, " ", "_"),
		document.ID[:8])
		
	processedFile := filepath.Join(processedDir, filename+"_training.txt")
	trainingContent := fmt.Sprintf("Title: %s\nCategory: %s/%s\nURL: %s\nQuality: %.2f\nWords: %d\n\n%s",
		document.Source.Title,
		document.Source.Category,
		document.Source.Subcategory,
		document.Source.URL,
		document.Quality,
		document.WordCount,
		document.CleanText)
		
	return ioutil.WriteFile(processedFile, []byte(trainingContent), 0644)
}

func generateSummary(baseDir string, documents []ScrapedDocument, categoryStats map[string]int, successCount int) {
	summaryPath := filepath.Join(baseDir, "scraping_summary.json")
	
	totalWords := 0
	totalChars := 0
	avgQuality := 0.0
	
	categoryDetails := make(map[string]struct {
		Count      int     `json:"count"`
		TotalWords int     `json:"total_words"`
		AvgQuality float64 `json:"avg_quality"`
	})
	
	for _, doc := range documents {
		totalWords += doc.WordCount
		totalChars += doc.CharCount
		avgQuality += doc.Quality
		
		stats := categoryDetails[doc.Source.Category]
		stats.Count++
		stats.TotalWords += doc.WordCount
		stats.AvgQuality += doc.Quality
		categoryDetails[doc.Source.Category] = stats
	}
	
	if len(documents) > 0 {
		avgQuality /= float64(len(documents))
	}
	
	// Calculate average quality per category
	for category, stats := range categoryDetails {
		if stats.Count > 0 {
			stats.AvgQuality /= float64(stats.Count)
		}
		categoryDetails[category] = stats
	}
	
	summary := map[string]interface{}{
		"scraping_completed_at": time.Now(),
		"total_sources_attempted": len(qualityTrainingSources),
		"successful_scrapes": successCount,
		"success_rate": float64(successCount) / float64(len(qualityTrainingSources)) * 100,
		"total_documents": len(documents),
		"total_words": totalWords,
		"total_characters": totalChars,
		"average_quality_score": avgQuality,
		"category_breakdown": categoryDetails,
		"folder_structure": baseDir,
		"quality_threshold": 0.6,
	}
	
	summaryBytes, _ := json.MarshalIndent(summary, "", "  ")
	ioutil.WriteFile(summaryPath, summaryBytes, 0644)
	
	// Also create a human-readable report
	reportPath := filepath.Join(baseDir, "SCRAPING_REPORT.txt")
	report := fmt.Sprintf(`🤖 AUTOMATED QUALITY CONTENT SCRAPING REPORT
================================================

📊 OVERVIEW
-----------
• Sources Attempted: %d
• Successful Scrapes: %d
• Success Rate: %.1f%%
• Total Training Content: %s words (%s characters)
• Average Quality Score: %.2f/1.0

📁 FOLDER STRUCTURE
-------------------
All content organized in: %s
├── AI_ML/ (foundations, advanced, nlp, vision)
├── programming/ (go_lang, fundamentals)  
├── mathematics/ (foundations, statistics, algebra, analysis, discrete)
├── computer_science/ (foundations, theory, systems)
├── technology/ (general, information, networking, web)
├── science/ (physics, quantum, security)
├── business/ (economics, management)
├── processed/ (training-ready files)
├── summaries/ (analysis data)
└── indexes/ (search indexes)

📈 CATEGORY BREAKDOWN
---------------------
`,
		len(qualityTrainingSources),
		successCount,
		float64(successCount)/float64(len(qualityTrainingSources))*100,
		formatNumber(totalWords),
		formatNumber(totalChars),
		avgQuality,
		baseDir)
	
	for category, stats := range categoryDetails {
		report += fmt.Sprintf("• %s: %d docs, %s words, %.2f quality\n",
			strings.ToUpper(category), stats.Count, formatNumber(stats.TotalWords), stats.AvgQuality)
	}
	
	report += fmt.Sprintf(`
✅ TRAINING CONTENT READY
-------------------------
High-quality educational content collected and organized for:
• LLM Training and Fine-tuning
• Knowledge Base Construction
• Question-Answering Systems
• Educational AI Applications
• Technical Documentation Systems

🚀 All files ready for immediate use in ML training pipelines!
`)
	
	ioutil.WriteFile(reportPath, []byte(report), 0644)
	
	fmt.Printf("\n📋 Summary saved to: %s\n", summaryPath)
	fmt.Printf("📄 Report saved to: %s\n", reportPath)
}

func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
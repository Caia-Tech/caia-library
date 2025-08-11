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

	"github.com/Caia-Tech/caia-library/internal/procurement/scraping"
	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"github.com/Caia-Tech/caia-library/pkg/logging"
	"github.com/Caia-Tech/caia-library/pkg/pipeline"
)

func getPremiumSources() []map[string]string {
	return []map[string]string{
		// Core CS concepts - high value
		{"url": "https://en.wikipedia.org/wiki/Recursion_(computer_science)", "title": "Recursion", "domain": "CS"},
		{"url": "https://en.wikipedia.org/wiki/Object-oriented_programming", "title": "OOP", "domain": "Programming"},
		{"url": "https://en.wikipedia.org/wiki/Functional_programming", "title": "Functional Programming", "domain": "Programming"},
		{"url": "https://en.wikipedia.org/wiki/Design_Patterns", "title": "Design Patterns", "domain": "Engineering"},
		{"url": "https://en.wikipedia.org/wiki/SOLID", "title": "SOLID Principles", "domain": "Engineering"},
		
		// Essential algorithms
		{"url": "https://en.wikipedia.org/wiki/Merge_sort", "title": "Merge Sort", "domain": "Algorithms"},
		{"url": "https://en.wikipedia.org/wiki/Dijkstra%27s_algorithm", "title": "Dijkstra's Algorithm", "domain": "Algorithms"},
		{"url": "https://en.wikipedia.org/wiki/A*_search_algorithm", "title": "A* Search", "domain": "Algorithms"},
		
		// Modern tech
		{"url": "https://en.wikipedia.org/wiki/Blockchain", "title": "Blockchain", "domain": "Technology"},
		{"url": "https://en.wikipedia.org/wiki/Quantum_computing", "title": "Quantum Computing", "domain": "Technology"},
	}
}

func main() {
	fmt.Println("💎 PREMIUM QUALITY DATA EXTRACTOR")
	fmt.Println("=================================")
	fmt.Println()

	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "error"
	
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	sources := getPremiumSources()
	complianceEngine := scraping.NewComplianceEngine(scraping.DefaultComplianceConfig())
	extractorEngine := extractor.NewEngine()
	ctx := context.Background()

	dataset := map[string]interface{}{
		"conversations": []interface{}{},
		"metadata": map[string]interface{}{
			"name": "Premium Technical Knowledge",
			"quality": "exceptional",
			"formats": []string{
				"socratic_dialogue",
				"expert_explanation", 
				"practical_guide",
				"conceptual_breakdown",
				"implementation_tutorial",
				"comparative_analysis",
				"problem_solving",
				"real_world_application",
			},
		},
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}

	conversations := []interface{}{}
	totalWords := 0

	for i, source := range sources {
		fmt.Printf("[%d/%d] Processing %s...", i+1, len(sources), source["title"])
		
		// Check compliance
		complianceResult, err := complianceEngine.CheckCompliance(ctx, source["url"])
		if err != nil || !complianceResult.Allowed {
			fmt.Printf(" ❌ Blocked\n")
			continue
		}

		if complianceResult.RequiredDelay > 0 {
			time.Sleep(complianceResult.RequiredDelay)
		}

		// Fetch
		content, err := fetchURL(source["url"])
		if err != nil {
			fmt.Printf(" ❌ Failed\n")
			continue
		}

		// Extract
		text, _, err := extractorEngine.Extract(ctx, content, "html")
		if err != nil || len(text) < 500 {
			fmt.Printf(" ❌ Too short\n")
			continue
		}

		wordCount := len(strings.Fields(text))
		totalWords += wordCount

		// Generate premium conversations
		newConvs := generatePremiumConversations(source, text, wordCount)
		conversations = append(conversations, newConvs...)
		
		fmt.Printf(" ✅ %d words → %d conversations\n", wordCount, len(newConvs))
		
		if i < len(sources)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	dataset["conversations"] = conversations
	dataset["metadata"].(map[string]interface{})["total_conversations"] = len(conversations)
	dataset["metadata"].(map[string]interface{})["total_words"] = totalWords

	// Save
	outputFile := "premium_dataset.json"
	if err := saveJSON(dataset, outputFile); err != nil {
		fmt.Printf("❌ Save failed: %v\n", err)
		return
	}

	// Summary
	fmt.Println("\n✨ PREMIUM DATASET COMPLETE!")
	fmt.Printf("📊 Generated %d conversations from %d sources\n", len(conversations), len(sources))
	fmt.Printf("📝 Total words: %d\n", totalWords)
	fmt.Printf("💾 Saved to: %s\n", outputFile)
}

func generatePremiumConversations(source map[string]string, text string, wordCount int) []interface{} {
	conversations := []interface{}{}
	cleanedText := cleanContent(text)
	
	// 1. Socratic Dialogue
	conversations = append(conversations, map[string]interface{}{
		"format": "socratic_dialogue",
		"topic": source["title"],
		"dialogue": []map[string]string{
			{"role": "student", "content": fmt.Sprintf("What is %s really about at its core?", source["title"])},
			{"role": "teacher", "content": extractCore(cleanedText, source["title"])},
			{"role": "student", "content": "Why is this important to understand?"},
			{"role": "teacher", "content": extractImportance(cleanedText, source["title"])},
			{"role": "student", "content": "Can you give me a concrete example?"},
			{"role": "teacher", "content": extractExample(cleanedText, source["title"])},
		},
		"metadata": map[string]interface{}{
			"source": source["url"],
			"domain": source["domain"],
			"words": wordCount,
		},
	})

	// 2. Expert Explanation
	expertExplanation := extractExpertContent(cleanedText)
	if expertExplanation != "" {
		conversations = append(conversations, map[string]interface{}{
			"format": "expert_explanation",
			"topic": source["title"],
			"conversation": []map[string]string{
				{"role": "user", "content": fmt.Sprintf("Explain %s like I'm a software engineer", source["title"])},
				{"role": "assistant", "content": expertExplanation},
			},
			"metadata": map[string]interface{}{
				"level": "professional",
				"domain": source["domain"],
			},
		})
	}

	// 3. Practical Implementation
	if source["domain"] == "Algorithms" || source["domain"] == "Programming" {
		conversations = append(conversations, map[string]interface{}{
			"format": "implementation_guide",
			"topic": source["title"],
			"conversation": []map[string]string{
				{"role": "user", "content": fmt.Sprintf("How do I implement %s?", source["title"])},
				{"role": "assistant", "content": generateImplementationGuide(source["title"], cleanedText)},
			},
			"metadata": map[string]interface{}{
				"type": "code_focused",
				"domain": source["domain"],
			},
		})
	}

	// 4. Conceptual Breakdown
	conversations = append(conversations, map[string]interface{}{
		"format": "conceptual_breakdown",
		"topic": source["title"],
		"content": map[string]interface{}{
			"overview": extractOverview(cleanedText),
			"key_concepts": extractKeyConcepts(cleanedText),
			"applications": extractApplications(cleanedText),
			"considerations": extractConsiderations(cleanedText),
		},
		"metadata": map[string]interface{}{
			"structure": "hierarchical",
			"domain": source["domain"],
		},
	})

	// 5. Problem-Solving Approach
	conversations = append(conversations, map[string]interface{}{
		"format": "problem_solving",
		"topic": source["title"],
		"scenario": map[string]interface{}{
			"problem": fmt.Sprintf("How can %s solve real-world problems?", source["title"]),
			"approach": extractProblemSolving(cleanedText, source["title"]),
			"benefits": extractBenefits(cleanedText),
			"tradeoffs": extractTradeoffs(cleanedText),
		},
	})

	// 6. Comparative Analysis (if applicable)
	if strings.Contains(cleanedText, "compar") || strings.Contains(cleanedText, "versus") || 
	   strings.Contains(cleanedText, "alternative") {
		conversations = append(conversations, map[string]interface{}{
			"format": "comparative_analysis",
			"topic": source["title"],
			"comparison": extractComparison(cleanedText, source["title"]),
		})
	}

	return conversations
}

func cleanContent(text string) string {
	// Remove Wikipedia navigation and UI elements
	lines := strings.Split(text, "\n")
	var cleaned []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip navigation and meta content
		if strings.Contains(line, "Jump to") ||
		   strings.Contains(line, "From Wikipedia") ||
		   strings.Contains(line, "This article") ||
		   strings.Contains(line, "Categories:") ||
		   strings.Contains(line, "Navigation") ||
		   strings.Contains(line, "Edit links") ||
		   len(line) < 20 {
			continue
		}
		
		cleaned = append(cleaned, line)
	}
	
	return strings.Join(cleaned, "\n")
}

func extractCore(text, topic string) string {
	// Find the most fundamental explanation
	paragraphs := strings.Split(text, "\n\n")
	for _, p := range paragraphs {
		if len(p) > 100 && len(p) < 800 &&
		   (strings.Contains(strings.ToLower(p), "is a") ||
		    strings.Contains(strings.ToLower(p), "refers to") ||
		    strings.Contains(strings.ToLower(p), "defined as")) {
			return fmt.Sprintf("%s is fundamentally about: %s", topic, p)
		}
	}
	
	if len(paragraphs) > 0 && len(paragraphs[0]) > 50 {
		return paragraphs[0]
	}
	
	return fmt.Sprintf("%s is a fundamental concept in computer science and programming.", topic)
}

func extractImportance(text, topic string) string {
	keywords := []string{"important", "essential", "crucial", "fundamental", "key", "significant"}
	
	paragraphs := strings.Split(text, "\n\n")
	for _, p := range paragraphs {
		lower := strings.ToLower(p)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) && len(p) > 100 && len(p) < 600 {
				return p
			}
		}
	}
	
	return fmt.Sprintf("Understanding %s is crucial for building efficient, scalable, and maintainable software systems. It forms the foundation for solving complex computational problems.", topic)
}

func extractExample(text, topic string) string {
	// Look for example indicators
	exampleKeywords := []string{"example", "for instance", "such as", "consider", "suppose"}
	
	paragraphs := strings.Split(text, "\n\n")
	for _, p := range paragraphs {
		lower := strings.ToLower(p)
		for _, keyword := range exampleKeywords {
			if strings.Contains(lower, keyword) && len(p) > 50 && len(p) < 500 {
				return p
			}
		}
	}
	
	// Generate a generic example
	return fmt.Sprintf("For example, %s can be applied in scenarios where you need to optimize performance, manage complexity, or implement scalable solutions.", topic)
}

func extractExpertContent(text string) string {
	// Extract technical details
	sections := strings.Split(text, "\n\n")
	technical := []string{}
	
	for _, section := range sections {
		if len(section) > 200 && len(section) < 1000 &&
		   (strings.Contains(section, "algorithm") ||
		    strings.Contains(section, "complexity") ||
		    strings.Contains(section, "implementation") ||
		    strings.Contains(section, "performance") ||
		    strings.Contains(section, "technique")) {
			technical = append(technical, section)
			if len(technical) >= 3 {
				break
			}
		}
	}
	
	if len(technical) > 0 {
		return strings.Join(technical, "\n\n")
	}
	
	return ""
}

func generateImplementationGuide(topic, text string) string {
	guide := fmt.Sprintf("Here's how to implement %s:\n\n", topic)
	
	// Extract any code-like content
	if strings.Contains(text, "algorithm") || strings.Contains(text, "step") {
		guide += "1. Understand the core algorithm:\n"
		guide += extractAlgorithmSteps(text) + "\n\n"
	}
	
	guide += "2. Choose appropriate data structures\n"
	guide += "3. Handle edge cases and validation\n"
	guide += "4. Optimize for your use case\n"
	guide += "5. Test thoroughly with various inputs\n\n"
	
	// Add pseudocode
	guide += fmt.Sprintf("Basic structure:\n```\nfunction implement%s(input) {\n", strings.ReplaceAll(topic, " ", ""))
	guide += "    // Initialize\n"
	guide += "    // Process\n"
	guide += "    // Return result\n"
	guide += "}\n```"
	
	return guide
}

func extractAlgorithmSteps(text string) string {
	lines := strings.Split(text, "\n")
	steps := []string{}
	
	for _, line := range lines {
		if strings.Contains(line, "step") || 
		   strings.Contains(line, "first") ||
		   strings.Contains(line, "then") ||
		   strings.Contains(line, "finally") {
			if len(line) > 20 && len(line) < 200 {
				steps = append(steps, "• "+line)
				if len(steps) >= 5 {
					break
				}
			}
		}
	}
	
	if len(steps) > 0 {
		return strings.Join(steps, "\n")
	}
	
	return "• Analyze the problem\n• Design the solution\n• Implement core logic\n• Optimize and refine"
}

func extractOverview(text string) string {
	// Get first substantial paragraph
	paragraphs := strings.Split(text, "\n\n")
	for _, p := range paragraphs {
		if len(p) > 100 && len(p) < 500 {
			return p
		}
	}
	return "A comprehensive technical concept with wide applications."
}

func extractKeyConcepts(text string) []string {
	concepts := []string{}
	
	// Look for bullet points or listed items
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if (strings.HasPrefix(line, "•") || 
		    strings.HasPrefix(line, "-") ||
		    strings.HasPrefix(line, "*")) && len(line) > 10 {
			concepts = append(concepts, strings.TrimSpace(line[1:]))
			if len(concepts) >= 5 {
				break
			}
		}
	}
	
	if len(concepts) == 0 {
		// Generate generic concepts
		concepts = []string{
			"Core principles and foundations",
			"Implementation strategies",
			"Performance characteristics",
			"Common use cases",
			"Best practices",
		}
	}
	
	return concepts
}

func extractApplications(text string) string {
	keywords := []string{"application", "used in", "applied", "use case", "example"}
	
	for _, keyword := range keywords {
		idx := strings.Index(strings.ToLower(text), keyword)
		if idx != -1 && idx+500 < len(text) {
			return text[idx:idx+500]
		}
	}
	
	return "This concept has broad applications in software development, system design, and problem-solving."
}

func extractConsiderations(text string) string {
	keywords := []string{"consider", "important", "note", "caveat", "limitation"}
	
	for _, keyword := range keywords {
		idx := strings.Index(strings.ToLower(text), keyword)
		if idx != -1 && idx+400 < len(text) {
			return text[idx:idx+400]
		}
	}
	
	return "Consider performance implications, scalability requirements, and maintenance overhead when applying this concept."
}

func extractProblemSolving(text, topic string) string {
	if strings.Contains(text, "solve") || strings.Contains(text, "solution") {
		idx := strings.Index(strings.ToLower(text), "solv")
		if idx != -1 && idx+500 < len(text) {
			return text[idx:idx+500]
		}
	}
	
	return fmt.Sprintf("%s provides systematic approaches to complex problems by breaking them down into manageable components and applying proven techniques.", topic)
}

func extractBenefits(text string) string {
	keywords := []string{"benefit", "advantage", "improve", "enhance", "efficient"}
	
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(text), keyword) {
			idx := strings.Index(strings.ToLower(text), keyword)
			if idx != -1 && idx+300 < len(text) {
				return text[idx:idx+300]
			}
		}
	}
	
	return "Key benefits include improved performance, better maintainability, and enhanced scalability."
}

func extractTradeoffs(text string) string {
	keywords := []string{"tradeoff", "disadvantage", "limitation", "cost", "complexity"}
	
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(text), keyword) {
			idx := strings.Index(strings.ToLower(text), keyword)
			if idx != -1 && idx+300 < len(text) {
				return text[idx:idx+300]
			}
		}
	}
	
	return "Consider the tradeoffs between complexity and performance, as well as development time versus optimization."
}

func extractComparison(text, topic string) string {
	if strings.Contains(text, "compar") {
		idx := strings.Index(strings.ToLower(text), "compar")
		if idx != -1 && idx+500 < len(text) {
			return text[idx:idx+500]
		}
	}
	
	return fmt.Sprintf("%s offers unique advantages compared to alternative approaches, particularly in terms of efficiency and scalability.", topic)
}

func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", "CAIA-Premium-Extractor/1.0")
	req.Header.Set("Accept", "text/html")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	return io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
}

func saveJSON(data interface{}, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
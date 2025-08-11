package procurement_test

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WETRecord represents a single record from Common Crawl WET file
type WETRecord struct {
	URL         string
	Date        string
	ContentType string
	Content     string
	Length      int
}

func TestCommonCrawlIntegration(t *testing.T) {
	t.Run("Process Common Crawl Sample", func(t *testing.T) {
		// Use a small WET file (text-only format, much smaller than WARC)
		// This is a real Common Crawl file but we'll only process first few records
		// Updated to latest crawl
		wetURL := "https://data.commoncrawl.org/crawl-data/CC-MAIN-2024-46/segments/1730506990508.95/wet/CC-MAIN-20241105233631-20241106023631-00000.warc.wet.gz"
		
		t.Logf("Fetching Common Crawl WET file sample from: %s", wetURL)
		
		// Download with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		req, err := http.NewRequestWithContext(ctx, "GET", wetURL, nil)
		require.NoError(t, err)
		
		// Add range header to get only first 1MB
		req.Header.Set("Range", "bytes=0-1048576")
		
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		
		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Could not download Common Crawl sample: %v", err)
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			t.Skipf("Failed to download: HTTP %d", resp.StatusCode)
			return
		}
		
		// Process the compressed data
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			// Try processing first few KB of raw data if gzip fails
			t.Logf("Could not decompress, trying raw processing: %v", err)
			processRawWETSample(t, resp.Body)
			return
		}
		defer gzReader.Close()
		
		// Parse WET records
		records := parseWETRecords(t, gzReader, 5) // Only process first 5 records
		
		if len(records) == 0 {
			t.Skip("No records found in Common Crawl sample")
			return
		}
		
		// Validate content quality
		validator := quality.NewQualityValidator(nil)
		
		validCount := 0
		for i, record := range records {
			t.Logf("Record %d: URL=%s, Length=%d bytes", i+1, record.URL, record.Length)
			
			// Skip if content is too short
			if len(record.Content) < 100 {
				t.Logf("  Skipping: content too short (%d bytes)", len(record.Content))
				continue
			}
			
			// Validate content quality
			metadata := map[string]string{
				"source": "commoncrawl",
				"url":    record.URL,
				"type":   "web",
			}
			
			ctx := context.Background()
			result, err := validator.ValidateContent(ctx, record.Content, metadata)
			
			if err != nil {
				t.Logf("  Validation error: %v", err)
				continue
			}
			
			t.Logf("  Quality score: %.3f (tier: %s)", result.OverallScore, result.QualityTier)
			
			if result.OverallScore >= 0.5 {
				validCount++
			}
		}
		
		t.Logf("Processed %d records, %d passed quality threshold", len(records), validCount)
		
		// At least some content should pass quality checks
		assert.True(t, validCount > 0, "Should have at least one valid record")
	})
	
	t.Run("Test with Local Sample Data", func(t *testing.T) {
		// Create sample web content similar to Common Crawl
		sampleWebContent := []string{
			`Welcome to TechBlog: Understanding Distributed Systems Architecture

This comprehensive article explores the fundamentals of distributed systems and their critical importance in modern software architecture. 
Distributed systems enable applications to scale horizontally across multiple servers, providing significantly better performance, 
reliability, and fault tolerance compared to traditional monolithic architectures.

Key concepts that every software engineer should understand include consistency models (strong, eventual, and weak consistency), 
partition tolerance (the system's ability to continue operating during network failures), and availability trade-offs as described 
in the CAP theorem. These principles form the foundation of modern cloud computing infrastructure.

When designing distributed systems, engineers must carefully consider network latency, data replication strategies, consensus algorithms,
and failure handling mechanisms. Popular frameworks and technologies like Apache Kafka for event streaming, Redis for distributed caching,
Apache Zookeeper for coordination, and Kubernetes for container orchestration help manage the inherent complexity of distributed systems.

Modern microservices architectures rely heavily on distributed system principles to achieve horizontal scalability, service isolation,
and resilience in cloud environments. Companies like Netflix, Amazon, and Google have pioneered many of the patterns and practices
that are now considered industry standards for building reliable distributed systems at scale.`,
			
			`Shopping Cart Implementation Guide for E-Commerce Applications

Building a robust e-commerce shopping cart requires careful consideration of user experience, data persistence, and system architecture. 
This comprehensive guide covers essential aspects including session management, product inventory tracking, and checkout flow optimization
for modern web applications.

Key features that every shopping cart implementation should include are real-time inventory updates to prevent overselling, 
guest checkout options for reducing friction, saved cart functionality for returning customers, wishlist integration, 
and seamless integration with multiple payment gateways including credit cards, PayPal, and digital wallets. 

Security considerations are paramount in e-commerce applications. Implement CSRF protection for all state-changing operations,
secure payment token handling following PCI compliance standards, SSL/TLS encryption for all data transmission, and proper
input validation to prevent SQL injection and XSS attacks. Regular security audits and penetration testing are essential.

Performance optimization techniques like Redis caching for frequently accessed products, lazy loading of recommendations,
CDN integration for static assets, and database query optimization can significantly improve the user experience and
conversion rates. Consider implementing progressive web app features for mobile users to enhance performance further.`,
			
			`<html><body>
<h1>News Article</h1>
<p>Technology companies announced quarterly earnings today, showing strong growth in cloud services.</p>
<p>The market responded positively to the news, with tech stocks rising across the board.</p>
<p>Analysts predict continued growth in the sector through the next quarter.</p>
</body></html>`,
			
			`Random text that doesn't make much sense. Just words put together. No real meaning or structure here.
Some more random content. This is low quality content that should score poorly. Adding more text to meet 
minimum length requirements but still maintaining low quality. The quick brown fox jumps over the lazy dog.
This sentence contains every letter of the alphabet but doesn't add meaningful content to this document.
More filler text here just to reach word count. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
Adding random technical terms like blockchain, machine learning, and quantum computing without any context.
This paragraph continues with disconnected thoughts and no coherent narrative structure whatsoever.
The weather is nice today. Pizza is a popular food. Cars have four wheels. These are random facts.
Just filling space with more words to meet the arbitrary minimum word count requirement for validation.
This content should definitely score low on quality metrics due to lack of coherence and value.`,
		}
		
		validator := quality.NewQualityValidator(nil)
		
		results := make([]float64, 0)
		for i, content := range sampleWebContent {
			metadata := map[string]string{
				"source": "test_sample",
				"index":  fmt.Sprintf("%d", i),
				"type":   "web",
			}
			
			ctx := context.Background()
			result, err := validator.ValidateContent(ctx, content, metadata)
			
			if err != nil {
				t.Logf("Sample %d validation error: %v", i+1, err)
				continue
			}
			
			t.Logf("Sample %d quality score: %.3f (tier: %s)", i+1, result.OverallScore, result.QualityTier)
			results = append(results, result.OverallScore)
		}
		
		// Should have processed most samples (HTML one might fail)
		require.True(t, len(results) >= 3, "Should process at least 3 samples")
		
		// First two samples should score higher than the last one (random text)
		lastIndex := len(results) - 1
		assert.True(t, results[0] > results[lastIndex], 
			"Technical content (%.3f) should score higher than random text (%.3f)", 
			results[0], results[lastIndex])
		assert.True(t, results[1] > results[lastIndex], 
			"Shopping guide (%.3f) should score higher than random text (%.3f)", 
			results[1], results[lastIndex])
	})
}

func parseWETRecords(t *testing.T, reader io.Reader, maxRecords int) []WETRecord {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	
	records := make([]WETRecord, 0, maxRecords)
	currentRecord := &WETRecord{}
	inRecord := false
	contentLines := []string{}
	
	for scanner.Scan() && len(records) < maxRecords {
		line := scanner.Text()
		
		if strings.HasPrefix(line, "WARC/1.0") {
			// Start of new record
			if inRecord && len(contentLines) > 0 {
				// Save previous record
				currentRecord.Content = strings.Join(contentLines, "\n")
				currentRecord.Length = len(currentRecord.Content)
				if currentRecord.Length > 50 { // Skip tiny records
					records = append(records, *currentRecord)
				}
			}
			
			currentRecord = &WETRecord{}
			contentLines = []string{}
			inRecord = true
			
		} else if inRecord && strings.HasPrefix(line, "WARC-Target-URI:") {
			currentRecord.URL = strings.TrimSpace(strings.TrimPrefix(line, "WARC-Target-URI:"))
			
		} else if inRecord && strings.HasPrefix(line, "WARC-Date:") {
			currentRecord.Date = strings.TrimSpace(strings.TrimPrefix(line, "WARC-Date:"))
			
		} else if inRecord && line == "" && currentRecord.URL != "" {
			// Empty line after headers means content starts
			for scanner.Scan() && len(records) < maxRecords {
				contentLine := scanner.Text()
				if strings.HasPrefix(contentLine, "WARC/1.0") {
					// Next record started, reprocess this line
					line = contentLine
					break
				}
				contentLines = append(contentLines, contentLine)
				
				// Limit content size
				if len(contentLines) > 100 {
					break
				}
			}
		}
	}
	
	// Save last record if exists
	if inRecord && len(contentLines) > 0 {
		currentRecord.Content = strings.Join(contentLines, "\n")
		currentRecord.Length = len(currentRecord.Content)
		if currentRecord.Length > 50 {
			records = append(records, *currentRecord)
		}
	}
	
	if err := scanner.Err(); err != nil {
		t.Logf("Scanner error (may be partial read): %v", err)
	}
	
	return records
}

func processRawWETSample(t *testing.T, reader io.Reader) {
	// Try to read first few KB as text
	limitedReader := io.LimitReader(reader, 10*1024) // Read only 10KB
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		t.Logf("Could not read raw data: %v", err)
		return
	}
	
	t.Logf("Read %d bytes of raw data", len(data))
	
	// Try to find text content
	text := string(data)
	lines := strings.Split(text, "\n")
	
	contentFound := false
	for i, line := range lines {
		if len(line) > 50 && !strings.HasPrefix(line, "WARC") {
			t.Logf("Line %d: %s...", i, line[:50])
			contentFound = true
			if i > 10 {
				break
			}
		}
	}
	
	if !contentFound {
		t.Log("No readable text content found in sample")
	}
}

func BenchmarkCommonCrawlProcessing(b *testing.B) {
	// Create sample content similar to Common Crawl
	sampleContent := `
Tech News: Latest Developments in AI

Artificial intelligence continues to transform industries worldwide. Recent breakthroughs in large language models
have enabled new applications in healthcare, finance, and education. Companies are investing billions in AI research
and development, with focus on making systems more efficient and accessible.

Key trends include edge computing for AI, federated learning for privacy-preserving models, and multimodal systems
that can process text, images, and audio simultaneously. Regulatory frameworks are evolving to address ethical
concerns and ensure responsible AI deployment.

Industry experts predict that AI will become increasingly integrated into everyday applications, from smart homes
to autonomous vehicles. The challenge remains balancing innovation with safety and fairness considerations.
`
	
	validator := quality.NewQualityValidator(nil)
	metadata := map[string]string{
		"source": "benchmark",
		"type":   "web",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		result, err := validator.ValidateContent(ctx, sampleContent, metadata)
		if err != nil {
			b.Fatal(err)
		}
		if result.OverallScore < 0 {
			b.Fatal("Invalid score")
		}
	}
}
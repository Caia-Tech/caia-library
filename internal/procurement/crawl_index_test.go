package procurement_test

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CDXRecord represents a Common Crawl CDX index record
type CDXRecord struct {
	URLKey          string `json:"urlkey"`
	Timestamp       string `json:"timestamp"`
	URL             string `json:"url"`
	MIME            string `json:"mime"`
	MIMEDetected    string `json:"mime-detected"`
	Status          string `json:"status"`
	Digest          string `json:"digest"`
	Length          string `json:"length"`
	Offset          string `json:"offset"`
	Filename        string `json:"filename"`
	Languages       string `json:"languages"`
	Encoding        string `json:"encoding"`
}

func TestCommonCrawlIndex(t *testing.T) {
	t.Run("Process CDX Index Sample", func(t *testing.T) {
		// CDX index files are much smaller and contain metadata about crawled pages
		// This allows us to see what Common Crawl has without downloading large files
		cdxURL := "https://data.commoncrawl.org/cc-index/collections/CC-MAIN-2024-46/indexes/cdx-00000.gz"
		
		t.Logf("Fetching Common Crawl CDX index sample from: %s", cdxURL)
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		req, err := http.NewRequestWithContext(ctx, "GET", cdxURL, nil)
		require.NoError(t, err)
		
		// Get only first 100KB of the index
		req.Header.Set("Range", "bytes=0-102400")
		
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		
		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Could not download CDX index: %v", err)
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			t.Skipf("Failed to download CDX index: HTTP %d", resp.StatusCode)
			return
		}
		
		// Decompress the gzipped index
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			t.Skipf("Could not decompress CDX index: %v", err)
			return
		}
		defer gzReader.Close()
		
		// Parse CDX records
		scanner := bufio.NewScanner(gzReader)
		records := []CDXRecord{}
		lineCount := 0
		
		for scanner.Scan() && lineCount < 20 { // Only process first 20 records
			line := scanner.Text()
			lineCount++
			
			// CDX format is JSON lines
			var record CDXRecord
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				continue // Skip malformed lines
			}
			
			// Only interested in HTML content
			if strings.Contains(record.MIME, "html") {
				records = append(records, record)
			}
		}
		
		t.Logf("Found %d HTML records in first %d lines", len(records), lineCount)
		
		// Display sample of what's in Common Crawl
		for i, record := range records {
			if i >= 5 {
				break
			}
			t.Logf("Record %d:", i+1)
			t.Logf("  URL: %s", record.URL)
			t.Logf("  MIME: %s", record.MIME)
			t.Logf("  Status: %s", record.Status)
			t.Logf("  Languages: %s", record.Languages)
			t.Logf("  Timestamp: %s", record.Timestamp)
		}
		
		assert.True(t, len(records) > 0, "Should find at least some HTML records")
	})
	
	t.Run("Simulate Common Crawl Processing Pipeline", func(t *testing.T) {
		// Simulate what we would do with real Common Crawl data
		validator := quality.NewQualityValidator(nil)
		
		// Example URLs and content types from Common Crawl
		testCases := []struct {
			url         string
			mimeType    string
			content     string
			language    string
		}{
			{
				url:      "https://example.com/blog/tech-article",
				mimeType: "text/html",
				content: `The Future of Cloud Computing: Trends and Predictions

Cloud computing continues to evolve rapidly, transforming how businesses operate and deliver services to customers worldwide.
Major trends shaping the industry include serverless architectures, edge computing, and multi-cloud strategies that provide
flexibility and resilience. Organizations are increasingly adopting hybrid cloud models to balance security requirements with
scalability needs.

Artificial intelligence and machine learning workloads are driving demand for specialized cloud infrastructure, with providers
offering GPU-accelerated instances and managed AI services. Container orchestration platforms like Kubernetes have become
standard for deploying microservices at scale. Security remains a top priority, with zero-trust architectures and enhanced
encryption becoming mandatory for enterprise deployments.

Looking ahead, quantum computing services, sustainable green data centers, and improved data sovereignty solutions will define
the next generation of cloud platforms. Companies that embrace these technologies early will gain competitive advantages in
their respective markets.`,
				language: "en",
			},
			{
				url:      "https://example.de/wissenschaft/artikel",
				mimeType: "text/html",
				content: `Durchbruch in der Quantenphysik: Neue Erkenntnisse über Verschränkung

Wissenschaftler an der Technischen Universität haben einen bedeutenden Fortschritt im Verständnis der Quantenverschränkung
erzielt. Die Forschungsergebnisse zeigen, dass verschränkte Teilchen über größere Distanzen als bisher angenommen stabile
Verbindungen aufrechterhalten können. Diese Entdeckung hat weitreichende Implikationen für die Entwicklung von Quantencomputern
und sichere Kommunikationssysteme.

Das Forschungsteam verwendete innovative Messtechniken und supraleitende Materialien, um die Dekohärenzzeiten signifikant zu
verlängern. Die experimentellen Daten bestätigen theoretische Vorhersagen und öffnen neue Wege für praktische Anwendungen
der Quantentechnologie. Besonders vielversprechend sind die Möglichkeiten für unhackbare Verschlüsselungssysteme und die
Lösung komplexer Optimierungsprobleme.

Die Ergebnisse wurden in der renommierten Fachzeitschrift Nature Physics veröffentlicht und haben internationale Anerkennung
gefunden. Weitere Forschungen konzentrieren sich nun auf die Skalierung der Technologie für industrielle Anwendungen.`,
				language: "de",
			},
			{
				url:      "https://news.example.com/breaking/update",
				mimeType: "text/html",
				content: `Breaking: Major Tech Company Announces Revolutionary Product

In a surprise announcement today, the company unveiled its latest innovation that promises to transform the industry.
The new product combines cutting-edge technology with user-friendly design. Industry analysts predict significant market impact.
Early reviews have been overwhelmingly positive. The product will be available starting next quarter.
Competitors are already scrambling to respond. This could be a game-changer for the entire sector.
More details will be released at the upcoming conference. Investors responded positively to the news.
The stock price increased by 15% in after-hours trading.`,
				language: "en",
			},
		}
		
		results := []struct {
			url      string
			score    float64
			tier     string
			language string
		}{}
		
		for _, tc := range testCases {
			metadata := map[string]string{
				"source":   "commoncrawl",
				"url":      tc.url,
				"mime":     tc.mimeType,
				"language": tc.language,
			}
			
			ctx := context.Background()
			result, err := validator.ValidateContent(ctx, tc.content, metadata)
			
			if err != nil {
				t.Logf("Validation error for %s: %v", tc.url, err)
				continue
			}
			
			results = append(results, struct {
				url      string
				score    float64
				tier     string
				language string
			}{
				url:      tc.url,
				score:    result.OverallScore,
				tier:     result.QualityTier,
				language: tc.language,
			})
			
			t.Logf("URL: %s", tc.url)
			t.Logf("  Language: %s", tc.language)
			t.Logf("  Quality Score: %.3f", result.OverallScore)
			t.Logf("  Quality Tier: %s", result.QualityTier)
			t.Logf("  Confidence: %.2f", result.ConfidenceLevel)
		}
		
		// Verify we can process different types of content
		assert.True(t, len(results) >= 2, "Should process multiple content types")
		
		// Check that longer, more detailed content scores higher
		if len(results) >= 3 {
			// The tech article and scientific article should score higher than the brief news
			assert.True(t, results[0].score > results[2].score || results[1].score > results[2].score,
				"Detailed articles should score higher than brief news")
		}
		
		t.Logf("\nSummary of simulated Common Crawl processing:")
		t.Logf("Processed %d documents", len(results))
		
		avgScore := 0.0
		for _, r := range results {
			avgScore += r.score
		}
		if len(results) > 0 {
			avgScore /= float64(len(results))
			t.Logf("Average quality score: %.3f", avgScore)
		}
	})
}

func TestCrawlDataFiltering(t *testing.T) {
	// Test filtering logic for Common Crawl data
	validator := quality.NewQualityValidator(nil)
	
	// Types of content commonly found in Common Crawl
	contentSamples := map[string]string{
		"spam": "Buy cheap products now! Click here! Amazing deals! Best prices guaranteed! Order today! Limited time offer! Act now!",
		
		"navigation": "Home | About | Contact | Services | Blog | Privacy Policy | Terms of Service | Sitemap | FAQ | Support",
		
		"cookie_banner": "This website uses cookies to improve your experience. By continuing to browse, you accept our use of cookies. Accept | Decline | Learn More",
		
		"quality_content": `Understanding Database Indexing: A Comprehensive Guide
		
Database indexing is a crucial optimization technique that significantly improves query performance by creating data structures
that allow for faster data retrieval. Similar to an index in a book, database indexes provide quick access paths to data rows
without scanning entire tables. This guide explores different index types, their use cases, and best practices for implementation.

B-tree indexes are the most common type, organizing data in a balanced tree structure that maintains sorted data and allows for
efficient searching, insertion, and deletion operations. Hash indexes use hash functions for exact match queries but don't support
range queries. Bitmap indexes are optimal for columns with low cardinality, while full-text indexes enable efficient text searching.

When implementing indexes, consider the trade-offs between query performance and write overhead. Each index requires maintenance
during INSERT, UPDATE, and DELETE operations. Monitor index usage with database profiling tools and remove unused indexes to optimize
storage and write performance. Regular index maintenance, including rebuilding fragmented indexes, ensures optimal performance.`,
	}
	
	t.Run("Filter Low Quality Content", func(t *testing.T) {
		scores := make(map[string]float64)
		
		for contentType, content := range contentSamples {
			metadata := map[string]string{
				"source": "commoncrawl",
				"type":   contentType,
			}
			
			ctx := context.Background()
			result, err := validator.ValidateContent(ctx, content, metadata)
			
			if err != nil {
				// Short content will error, which is fine for filtering
				scores[contentType] = 0.0
				t.Logf("%s: Failed validation (filtered out)", contentType)
			} else {
				scores[contentType] = result.OverallScore
				t.Logf("%s: Score %.3f (tier: %s)", contentType, result.OverallScore, result.QualityTier)
			}
		}
		
		// Quality content should score highest
		assert.True(t, scores["quality_content"] > scores["spam"], 
			"Quality content should score higher than spam")
		
		// Navigation and cookie banners should score very low or fail
		assert.True(t, scores["navigation"] < 0.5, 
			"Navigation menus should score low")
		assert.True(t, scores["cookie_banner"] < 0.5, 
			"Cookie banners should score low")
		
		t.Logf("\nFiltering summary:")
		t.Logf("Content suitable for inclusion (score > 0.6):")
		for contentType, score := range scores {
			if score > 0.6 {
				t.Logf("  - %s: %.3f", contentType, score)
			}
		}
		
		t.Logf("Content to filter out (score <= 0.6):")
		for contentType, score := range scores {
			if score <= 0.6 {
				t.Logf("  - %s: %.3f", contentType, score)
			}
		}
	})
}

func BenchmarkCrawlDataProcessing(b *testing.B) {
	validator := quality.NewQualityValidator(nil)
	
	// Typical web content from Common Crawl
	content := `Artificial Intelligence in Healthcare: Transforming Patient Care

The integration of artificial intelligence in healthcare is revolutionizing diagnosis, treatment planning, and patient outcomes.
Machine learning algorithms analyze vast amounts of medical data to identify patterns invisible to human observers, enabling
earlier disease detection and more personalized treatment strategies. From radiology to pathology, AI systems augment physician
capabilities and reduce diagnostic errors.

Predictive analytics powered by AI help hospitals optimize resource allocation, reduce readmission rates, and improve operational
efficiency. Natural language processing extracts insights from unstructured clinical notes, while computer vision interprets
medical imaging with increasing accuracy. These technologies are particularly valuable in underserved areas where specialist
access is limited.

However, implementing AI in healthcare requires careful consideration of ethical implications, data privacy, and regulatory
compliance. Ensuring algorithm transparency, addressing bias in training data, and maintaining human oversight are critical
for responsible deployment. As the technology matures, collaboration between technologists and healthcare professionals will
be essential for realizing AI's full potential in improving patient care worldwide.`
	
	metadata := map[string]string{
		"source": "commoncrawl",
		"type":   "article",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		result, err := validator.ValidateContent(ctx, content, metadata)
		if err != nil {
			b.Fatal(err)
		}
		if result.OverallScore < 0 {
			b.Fatal("Invalid score")
		}
	}
}
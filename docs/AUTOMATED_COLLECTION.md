# Automated Data Collection & Curation Guide

## Overview

This guide details how to build automated, intelligent data collection pipelines with Caia Library, ensuring cryptographic provenance for every document.

## Architecture

```
Data Sources → Connectors → Temporal Workflows → Curation → Git Storage → Index
     ↑                              ↓                              ↓
     └──────── Schedulers ──────────┘                    Provenance Chain
```

## 1. Scheduled Ingestion System

### 1.1 Temporal Cron Workflows

```go
// workflows/scheduled_collector.go
type ScheduledCollectorWorkflow struct {
    Schedule     string              // Cron expression
    Sources      []CollectionSource
    CurationRules CurationConfig
}

type CollectionSource struct {
    Type        string // "rss", "web", "api", "s3"
    URL         string
    Credentials map[string]string
    Filters     []Filter
}

func (w *ScheduledCollectorWorkflow) Execute(ctx workflow.Context) error {
    // Run on schedule
    workflow.SetSchedule(ctx, w.Schedule)
    
    for _, source := range w.Sources {
        workflow.Go(ctx, func(ctx workflow.Context) {
            CollectFromSource(ctx, source)
        })
    }
    
    return nil
}
```

### 1.2 Source Configuration

```yaml
# config/sources.yaml
sources:
  - name: "ArXiv ML Papers"
    type: "rss"
    url: "http://export.arxiv.org/rss/cs.LG"
    schedule: "0 */6 * * *"  # Every 6 hours
    filters:
      - field: "title"
        contains: ["neural", "transformer", "llm"]
    
  - name: "SEC Filings"
    type: "api"
    url: "https://api.sec.gov/filings"
    schedule: "0 9 * * MON-FRI"  # Weekdays at 9 AM
    credentials:
      api_key: "${SEC_API_KEY}"
    
  - name: "Research PDFs"
    type: "s3"
    url: "s3://research-bucket/papers/"
    schedule: "0 0 * * SUN"  # Weekly on Sunday
    filters:
      - field: "modified"
        after: "${LAST_RUN}"
```

## 2. Intelligent Connectors

### 2.1 Web Scraper Connector

```go
// connectors/web_scraper.go
type WebScraperConnector struct {
    UserAgent     string
    RateLimit     time.Duration
    JavaScriptEnabled bool
}

func (w *WebScraperConnector) Collect(ctx context.Context, config CollectionSource) ([]Document, error) {
    // Initialize Playwright for JS-heavy sites
    if w.JavaScriptEnabled {
        pw, _ := playwright.Run()
        defer pw.Stop()
        
        browser, _ := pw.Chromium.Launch()
        defer browser.Close()
        
        page, _ := browser.NewPage()
        page.Goto(config.URL)
        
        // Wait for dynamic content
        page.WaitForLoadState(playwright.LoadStateNetworkIdle)
        
        content, _ := page.Content()
        return w.extractDocuments(content)
    }
    
    // Simple HTTP for static sites
    return w.fetchStatic(config.URL)
}

// Respect robots.txt
func (w *WebScraperConnector) checkRobots(url string) bool {
    robotsURL := getRobotsTxt(url)
    // Parse and check rules
    return true
}
```

### 2.2 RSS/Atom Feed Connector

```go
// connectors/rss_connector.go
type RSSConnector struct {
    SeenItems *bloom.BloomFilter  // Deduplication
}

func (r *RSSConnector) Collect(ctx context.Context, config CollectionSource) ([]Document, error) {
    parser := gofeed.NewParser()
    feed, err := parser.ParseURL(config.URL)
    if err != nil {
        return nil, err
    }
    
    var documents []Document
    
    for _, item := range feed.Items {
        // Check if we've seen this before
        itemHash := hash(item.GUID)
        if r.SeenItems.Test(itemHash) {
            continue
        }
        
        doc := Document{
            ID: generateID(),
            Source: Source{
                Type: "rss",
                URL:  item.Link,
            },
            Content: Content{
                Text: item.Description,
                Metadata: map[string]string{
                    "title":     item.Title,
                    "published": item.Published,
                    "feed":      feed.Title,
                },
            },
        }
        
        documents = append(documents, doc)
        r.SeenItems.Add(itemHash)
    }
    
    return documents, nil
}
```

### 2.3 Cloud Storage Connector

```go
// connectors/cloud_storage.go
type CloudStorageConnector struct {
    Provider string // "s3", "azure", "gcs"
    Client   interface{}
}

func (c *CloudStorageConnector) Collect(ctx context.Context, config CollectionSource) ([]Document, error) {
    switch c.Provider {
    case "s3":
        return c.collectFromS3(ctx, config)
    case "azure":
        return c.collectFromAzure(ctx, config)
    }
}

func (c *CloudStorageConnector) collectFromS3(ctx context.Context, config CollectionSource) ([]Document, error) {
    s3Client := c.Client.(*s3.Client)
    
    // List objects with pagination
    paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
        Bucket: aws.String(getBucket(config.URL)),
        Prefix: aws.String(getPrefix(config.URL)),
    })
    
    var documents []Document
    
    for paginator.HasMorePages() {
        output, err := paginator.NextPage(ctx)
        if err != nil {
            return nil, err
        }
        
        for _, obj := range output.Contents {
            // Check filters (modified date, size, etc.)
            if shouldProcess(obj, config.Filters) {
                doc := c.downloadAndCreateDocument(ctx, obj)
                documents = append(documents, doc)
            }
        }
    }
    
    return documents, nil
}
```

## 3. Curation Pipeline

### 3.1 Deduplication

```go
// curator/deduplication.go
type Deduplicator struct {
    LSH         *minhash.LSH
    Threshold   float64
}

func (d *Deduplicator) IsDuplicate(doc Document) (bool, string) {
    // Create MinHash signature
    mh := minhash.NewMinHash(128)
    tokens := tokenize(doc.Content.Text)
    
    for _, token := range tokens {
        mh.Push([]byte(token))
    }
    
    // Query LSH for similar documents
    similar := d.LSH.Query(mh)
    
    for _, candidate := range similar {
        similarity := mh.Jaccard(candidate.MinHash)
        if similarity > d.Threshold {
            return true, candidate.ID
        }
    }
    
    // Add to index if not duplicate
    d.LSH.Add(doc.ID, mh)
    return false, ""
}
```

### 3.2 Quality Scoring

```go
// curator/quality.go
type QualityScorer struct {
    MinLength      int
    MaxLength      int
    MinReadability float64
}

func (q *QualityScorer) Score(doc Document) float64 {
    score := 1.0
    
    // Length check
    length := len(doc.Content.Text)
    if length < q.MinLength || length > q.MaxLength {
        score *= 0.5
    }
    
    // Readability (Flesch-Kincaid)
    readability := calculateReadability(doc.Content.Text)
    if readability < q.MinReadability {
        score *= 0.7
    }
    
    // Language detection
    lang := detectLanguage(doc.Content.Text)
    if lang != "en" {
        score *= 0.8
    }
    
    // Check for spam indicators
    if containsSpamPatterns(doc.Content.Text) {
        score *= 0.3
    }
    
    return score
}
```

### 3.3 PII Detection & Redaction

```go
// curator/pii.go
type PIIDetector struct {
    Patterns map[string]*regexp.Regexp
}

func NewPIIDetector() *PIIDetector {
    return &PIIDetector{
        Patterns: map[string]*regexp.Regexp{
            "ssn":    regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
            "email":  regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
            "phone":  regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
            "credit": regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
        },
    }
}

func (p *PIIDetector) Redact(text string) (string, map[string]int) {
    detections := make(map[string]int)
    
    for piiType, pattern := range p.Patterns {
        matches := pattern.FindAllString(text, -1)
        if len(matches) > 0 {
            detections[piiType] = len(matches)
            text = pattern.ReplaceAllString(text, "[REDACTED-"+strings.ToUpper(piiType)+"]")
        }
    }
    
    return text, detections
}
```

## 4. Storage Optimization

### 4.1 Batch Processing

```go
// workflows/batch_ingestion.go
func BatchIngestionWorkflow(ctx workflow.Context, documents []Document) error {
    // Process in parallel batches
    batchSize := 10
    
    for i := 0; i < len(documents); i += batchSize {
        batch := documents[i:min(i+batchSize, len(documents))]
        
        workflow.Go(ctx, func(ctx workflow.Context) {
            ProcessBatch(ctx, batch)
        })
    }
    
    return nil
}

func ProcessBatch(ctx workflow.Context, batch []Document) error {
    // Single branch for entire batch
    branchName := fmt.Sprintf("batch-%s", generateBatchID())
    
    // Store all documents in one commit
    workflow.ExecuteActivity(ctx, StoreBatchActivity, batch, branchName)
    
    // Generate embeddings in parallel
    var futures []workflow.Future
    for _, doc := range batch {
        future := workflow.ExecuteActivity(ctx, GenerateEmbeddingsActivity, doc)
        futures = append(futures, future)
    }
    
    // Wait for all embeddings
    for _, future := range futures {
        future.Get(ctx, nil)
    }
    
    return nil
}
```

### 4.2 Git Optimization

```go
// storage/git_optimizer.go
type GitOptimizer struct {
    repo *git.Repository
}

func (g *GitOptimizer) Optimize() error {
    // Run git gc periodically
    cmd := exec.Command("git", "gc", "--aggressive", "--prune=now")
    cmd.Dir = g.repo.Path
    return cmd.Run()
}

func (g *GitOptimizer) CreateMonthlyArchive() error {
    // Tag old branches for archival
    cutoff := time.Now().AddDate(0, -1, 0)
    
    branches, _ := g.repo.Branches()
    for _, branch := range branches {
        if branch.CreatedAt.Before(cutoff) {
            tagName := fmt.Sprintf("archive/%s", branch.Name)
            g.repo.CreateTag(tagName, branch.Hash)
            g.repo.DeleteBranch(branch.Name)
        }
    }
    
    return nil
}
```

## 5. Monitoring & Observability

### 5.1 Collection Metrics

```go
// monitoring/metrics.go
var (
    documentsIngested = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "caia_documents_ingested_total",
            Help: "Total number of documents ingested",
        },
        []string{"source", "type"},
    )
    
    ingestionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "caia_ingestion_duration_seconds",
            Help: "Duration of document ingestion",
        },
        []string{"source"},
    )
    
    duplicatesDetected = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "caia_duplicates_detected_total",
            Help: "Total number of duplicate documents detected",
        },
    )
)
```

### 5.2 Alerting Rules

```yaml
# prometheus/alerts.yaml
groups:
  - name: caia_collection
    rules:
      - alert: IngestionFailureRate
        expr: rate(caia_ingestion_failures[5m]) > 0.1
        for: 10m
        annotations:
          summary: "High ingestion failure rate"
          
      - alert: NoNewDocuments
        expr: increase(caia_documents_ingested_total[1h]) == 0
        for: 2h
        annotations:
          summary: "No new documents in 2 hours"
          
      - alert: HighDuplicationRate
        expr: rate(caia_duplicates_detected_total[5m]) / rate(caia_documents_ingested_total[5m]) > 0.5
        for: 15m
        annotations:
          summary: "More than 50% of documents are duplicates"
```

## 6. Example: News Aggregation Pipeline

```go
// examples/news_pipeline.go
func SetupNewsAggregationPipeline() {
    sources := []CollectionSource{
        {
            Type: "rss",
            URL:  "https://news.ycombinator.com/rss",
            Schedule: "*/15 * * * *", // Every 15 minutes
        },
        {
            Type: "rss", 
            URL:  "https://lobste.rs/rss",
            Schedule: "*/30 * * * *", // Every 30 minutes
        },
        {
            Type: "web",
            URL:  "https://arxiv.org/list/cs.AI/recent",
            Schedule: "0 */6 * * *", // Every 6 hours
            JavaScriptEnabled: true,
        },
    }
    
    curation := CurationConfig{
        MinQualityScore: 0.7,
        Deduplication: DeduplicationConfig{
            Enabled:   true,
            Threshold: 0.85,
        },
        PIIRedaction: true,
        Categories: []string{"tech", "ai", "research"},
    }
    
    // Start the pipeline
    StartScheduledCollection(sources, curation)
}
```

## Best Practices

1. **Start Small**: Test with one source before scaling
2. **Monitor Everything**: Set up alerts before going to production
3. **Respect Rate Limits**: Be a good citizen of the web
4. **Version Control Config**: Store source configurations in Git
5. **Regular Maintenance**: Schedule Git optimization monthly
6. **Privacy First**: Always check for PII before storage
7. **Incremental Collection**: Track last successful run
8. **Graceful Degradation**: Handle source failures without stopping

## Conclusion

With these automated collection patterns, Caia Library becomes a self-sustaining knowledge repository that grows intelligently while maintaining perfect provenance for every document. The combination of Temporal's reliability and Git's immutability creates an unstoppable data collection machine.
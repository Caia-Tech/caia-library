# CAIA Library + govc Integration Plan
## Document Intelligence Pipeline with Memory-First Storage

### Architecture Overview

**Current State Analysis:**
- ✅ Temporal workflows for document processing (fetch → extract → embed → store → merge)  
- ✅ Git-native storage with go-git (384-dim embeddings, no API costs)
- ✅ PDF/HTML/Text extraction pipeline
- ✅ ONNX-based local embeddings (all-MiniLM-L6-v2)
- ✅ RESTful API with Fiber framework

**Target Architecture:**
```
┌─────────────────────────────────────────────────────────────────┐
│                    CAIA Library Architecture                    │
├─────────────────────────────────────────────────────────────────┤
│  REST API Layer (Fiber)                                        │
├─────────────────────────────────────────────────────────────────┤
│  Temporal Workflow Orchestration                               │
│  ┌──────────────────┬──────────────────┬──────────────────────┐ │
│  │  Document        │  Real-time       │  Content Cleaning    │ │
│  │  Ingestion       │  Processing      │  & Analysis         │ │
│  └──────────────────┴──────────────────┴──────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  Dual Storage Layer                                            │
│  ┌──────────────────┬──────────────────┬──────────────────────┐ │
│  │  govc            │  Traditional     │  Query Engine        │ │
│  │  (Memory-First)  │  Git (go-git)    │  (Hybrid)           │ │
│  │  - Hot Data      │  - Cold Storage  │  - Vector Search     │ │
│  │  - Pub/Sub       │  - Provenance    │  - Metadata Index   │ │
│  └──────────────────┴──────────────────┴──────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  Document Intelligence Engine                                  │
│  ┌──────────────────┬──────────────────┬──────────────────────┐ │
│  │  Content         │  Structure       │  Presentation        │ │
│  │  Cleaning        │  Analysis        │  Layer              │ │
│  │  (Rule-Based)    │  (Local Models)  │  (API + UI)         │ │
│  └──────────────────┴──────────────────┴──────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Phase 1: govc Integration Foundation (Weeks 1-3)

### Week 1: Dual Storage Layer Setup
**Objective:** Integrate govc as memory-first hot storage alongside existing Git persistence

**Implementation Details:**
```go
type HybridRepository struct {
    // Hot storage - govc memory-first for active documents
    govcClient   *govc.Client
    govcRepoName string
    
    // Cold storage - traditional git for persistence & provenance  
    gitRepo      *git.Repository
    
    // Configuration
    config       *HybridConfig
}

type HybridConfig struct {
    // Documents accessed within this time stay in govc memory
    HotDataTTL           time.Duration  // default: 24h
    
    // Max documents in memory before persistence
    MemoryThreshold      int            // default: 10000
    
    // Auto-sync interval between govc and git
    SyncInterval         time.Duration  // default: 5m
    
    // Use govc for reads when available
    PreferMemoryReads    bool          // default: true
}
```

**Storage Strategy:**
1. **Write Path**: All new documents go to both govc (memory) and git (disk)
2. **Read Path**: Check govc first, fallback to git if not found
3. **Persistence**: Background sync from govc → git every 5 minutes
4. **Eviction**: LRU eviction from govc memory based on access patterns

**Key Files to Modify:**
- `internal/git/repository.go` → `internal/storage/hybrid_repository.go`
- `internal/temporal/activities/store.go` → dual storage logic
- New: `internal/storage/govc_adapter.go`

### Week 2: Pub/Sub Event System
**Objective:** Implement real-time document processing with govc pub/sub

**Event Architecture:**
```go
type DocumentEvent struct {
    Type        EventType             `json:"type"`        // Created, Updated, Indexed
    DocumentID  string               `json:"document_id"`
    Timestamp   time.Time           `json:"timestamp"`
    Metadata    map[string]string   `json:"metadata"`
    Pipeline    PipelineStage       `json:"pipeline"`     // Fetch, Extract, Embed, etc.
}

type EventType string
const (
    DocumentCreated   EventType = "document.created"
    DocumentUpdated   EventType = "document.updated" 
    DocumentIndexed   EventType = "document.indexed"
    DocumentCleaned   EventType = "document.cleaned"
    PipelineProgress  EventType = "pipeline.progress"
)
```

**Pub/Sub Integration:**
- govc provides the pub/sub infrastructure
- Document processing activities publish events
- Subscribers can trigger additional workflows (indexing, analysis, etc.)
- Real-time dashboard updates via WebSocket

**Key Components:**
- New: `internal/events/publisher.go`
- New: `internal/events/subscriber.go` 
- Modified: All Temporal activities to publish events

### Week 3: Performance Optimization & Testing
**Objective:** Optimize dual storage and establish performance benchmarks

**Performance Targets:**
- Document retrieval: <10ms (govc memory) vs <100ms (git disk)
- Concurrent reads: 1000+ req/sec
- Memory usage: <100MB per 1GB repository
- govc→git sync latency: <5sec for 95th percentile

**Testing Framework:**
```go
func BenchmarkHybridStorage(b *testing.B) {
    // Compare govc vs git performance
    // Test concurrent read/write patterns
    // Memory usage profiling
    // Event throughput testing
}
```

## Phase 2: Document Intelligence Pipeline (Weeks 4-7)

### Week 4: Rule-Based Content Cleaning Engine
**Objective:** Implement automated content cleaning without LLM API costs

**Content Cleaning Rules:**
```go
type CleaningRule interface {
    Apply(content string, metadata map[string]string) (string, error)
    Priority() int
    ShouldApply(docType string) bool
}

// Academic paper cleaning
type AcademicPaperCleaner struct{}
func (a *AcademicPaperCleaner) Apply(content string, metadata map[string]string) (string, error) {
    // Remove headers/footers
    // Extract abstract, main content, references
    // Normalize citations
    // Remove figure/table references 
}

// Web content cleaning  
type WebContentCleaner struct{}
func (a *WebContentCleaner) Apply(content string, metadata map[string]string) (string, error) {
    // Remove navigation, ads, sidebars
    // Extract main article content
    // Clean HTML entities
    // Normalize whitespace
}
```

**Cleaning Pipeline:**
1. **Content Type Detection**: PDF vs HTML vs Text
2. **Rule Selection**: Based on source, content type, metadata
3. **Multi-pass Cleaning**: Structure → Content → Formatting
4. **Quality Scoring**: Confidence metrics for cleaning success

### Week 5: Structure Analysis Engine
**Objective:** Extract document structure and metadata using local models

**Structure Extraction:**
```go
type DocumentStructure struct {
    Title       string            `json:"title"`
    Authors     []string         `json:"authors"`
    Abstract    string           `json:"abstract"`
    Sections    []Section        `json:"sections"`
    References  []Reference      `json:"references"`
    Tables      []Table          `json:"tables"`
    Figures     []Figure         `json:"figures"`
    Keywords    []string         `json:"keywords"`
}

type StructureAnalyzer interface {
    AnalyzeStructure(content string, docType string) (*DocumentStructure, error)
}
```

**Analysis Approaches:**
- **RegEx-based**: For academic papers (title patterns, section headers)
- **HTML parsing**: For web content (semantic tags, schema.org)
- **Statistical**: For layout analysis (paragraph boundaries, font sizes)
- **Local NER models**: For entity extraction (authors, dates, locations)

### Week 6: Enhanced Query Engine
**Objective:** Fast document search combining vector similarity and structured queries

**Query Architecture:**
```go
type QueryEngine struct {
    vectorIndex    *VectorIndex     // In-memory embeddings index
    metadataIndex  *MetadataIndex   // Structured data index
    fullTextIndex  *FullTextIndex   // Text search index
    hybridStorage  *HybridRepository
}

type Query struct {
    // Vector similarity search
    EmbeddingQuery  []float32           `json:"embedding_query,omitempty"`
    
    // Structured filters
    Filters         map[string]string   `json:"filters,omitempty"`
    
    // Full text search
    TextQuery       string              `json:"text_query,omitempty"`
    
    // Hybrid parameters
    WeightVector    float64            `json:"weight_vector"`     // 0.0-1.0
    WeightText      float64            `json:"weight_text"`       // 0.0-1.0
    WeightMetadata  float64            `json:"weight_metadata"`   // 0.0-1.0
}
```

**Query Performance:**
- **Vector Search**: FAISS-like in-memory index for <50ms similarity search
- **Metadata Search**: B-tree index for structured queries <10ms
- **Hybrid Ranking**: Weighted combination of similarity scores

### Week 7: Real-Time Processing Pipeline Integration
**Objective:** Connect all components with event-driven processing

**Enhanced Workflow:**
```go
func EnhancedDocumentIngestionWorkflow(ctx workflow.Context, input DocumentInput) error {
    // 1. Parallel Fetch & Store in govc memory
    // 2. Parallel: Extract + Clean + Analyze Structure  
    // 3. Generate Embeddings from cleaned content
    // 4. Store in hybrid storage (govc + git)
    // 5. Index in query engine
    // 6. Publish completion event
    // 7. Trigger background workflows (presentation generation, etc.)
}
```

## Phase 3: Presentation Layer & Advanced Features (Weeks 8-10)

### Week 8: Document Presentation API
**Objective:** Generate clean, structured presentation of documents

**Presentation Formats:**
```go
type PresentationFormat string
const (
    FormatMarkdown    PresentationFormat = "markdown"
    FormatHTML        PresentationFormat = "html" 
    FormatJSON        PresentationFormat = "json"
    FormatSummary     PresentationFormat = "summary"
)

type PresentationEngine struct {
    cleaningRules   []CleaningRule
    structAnalyzer  StructureAnalyzer
    templates       map[PresentationFormat]*template.Template
}
```

**API Endpoints:**
```
GET /api/v1/documents/{id}/presentation?format=markdown&style=academic
GET /api/v1/documents/{id}/structure
GET /api/v1/documents/{id}/cleaned-content
POST /api/v1/documents/bulk-present
```

### Week 9: Advanced Analytics & Insights
**Objective:** Document intelligence beyond basic processing

**Analytics Features:**
- **Content Quality Scoring**: Readability, completeness, coherence
- **Topic Clustering**: Unsupervised grouping of similar documents  
- **Trend Analysis**: Content patterns over time
- **Duplicate Detection**: Near-duplicate document identification
- **Citation Networks**: Reference relationship mapping

### Week 10: Production Hardening & Monitoring
**Objective:** Production-ready deployment with comprehensive monitoring

**Monitoring Stack:**
- **Document Processing Metrics**: Success rates, processing times, queue depths
- **Storage Metrics**: govc memory usage, git sync latencies, query performance
- **Event Stream Health**: Pub/sub throughput, event processing delays
- **Quality Metrics**: Cleaning success rates, extraction accuracy

## Implementation Priority Matrix

| Feature                    | Impact | Effort | Priority |
|---------------------------|--------|--------|----------|
| govc Integration          | High   | Medium | P0       |
| Pub/Sub Events           | High   | Medium | P0       |
| Content Cleaning         | High   | Low    | P0       |
| Structure Analysis       | Medium | Medium | P1       |
| Enhanced Query Engine    | High   | High   | P1       |
| Presentation Layer       | Medium | Low    | P2       |
| Advanced Analytics       | Low    | High   | P3       |

## Technology Stack

**Storage & Performance:**
- **govc**: Memory-first Git for hot data and pub/sub
- **go-git**: Persistent storage and provenance 
- **ONNX Runtime**: Local embeddings (no API costs)

**Processing & Orchestration:**
- **Temporal**: Workflow orchestration (existing)
- **Fiber**: HTTP API framework (existing)
- **Custom rules engine**: Content cleaning

**Intelligence & Search:**
- **Local NLP models**: Structure analysis, NER
- **In-memory vector index**: Fast similarity search
- **Custom query engine**: Hybrid search capabilities

## Success Metrics

### Performance
- **Document Ingestion**: <30sec end-to-end for typical documents
- **Query Response**: <100ms for most document searches  
- **Memory Efficiency**: <50MB additional per 1000 documents
- **Concurrent Processing**: 100+ documents simultaneously

### Quality
- **Content Cleaning**: >90% user satisfaction on cleaned content
- **Structure Extraction**: >85% accuracy on academic papers
- **Search Relevance**: >80% user satisfaction on search results

### Cost Efficiency  
- **Zero LLM API costs**: All processing with local models
- **Reduced Infrastructure**: Memory-first approach reduces I/O
- **Operational Efficiency**: Automated pipeline reduces manual work

## Risk Mitigation

### Technical Risks
1. **govc Alpha Status**: Plan fallback to pure git if govc has stability issues
2. **Memory Pressure**: Implement intelligent eviction and monitoring
3. **Content Quality**: Extensive testing of cleaning rules across document types

### Operational Risks  
1. **Data Loss**: Dual storage ensures redundancy
2. **Performance Degradation**: Circuit breakers and fallback mechanisms
3. **Scaling Issues**: Horizontal scaling via repository sharding

## Getting Started

### Prerequisites
- Go 1.24+
- Temporal Server
- govc binary/library integration
- ONNX Runtime

### Implementation Order
1. **Week 1**: Set up development environment with govc integration
2. **Week 2**: Implement basic dual storage functionality  
3. **Week 3**: Add pub/sub events and basic monitoring
4. **Week 4**: Begin content cleaning rule implementation
5. **Continue sequentially** following the phase plan above

This plan provides a clear roadmap for transforming CAIA Library into a comprehensive document intelligence system while maintaining its core strengths of Git-native storage, Temporal orchestration, and cost-efficient local processing.
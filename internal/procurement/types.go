package procurement

import (
	"context"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
)

// ContentType defines the type of content to generate
type ContentType string

const (
	ContentTypeResearchAbstract ContentType = "research_abstract"
	ContentTypeTutorial         ContentType = "tutorial"
	ContentTypeDocumentation    ContentType = "documentation"
	ContentTypeCodeExample      ContentType = "code_example"
	ContentTypeEducational      ContentType = "educational"
	ContentTypeGeneral          ContentType = "general"
)

// Topic represents a content generation topic
type Topic struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Domain      string            `json:"domain"`
	Keywords    []string          `json:"keywords"`
	Priority    float64           `json:"priority"`
	Difficulty  string            `json:"difficulty"`
	Context     string            `json:"context"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// GenerationRequest represents a content generation request
type GenerationRequest struct {
	ID          string      `json:"id"`
	Topic       *Topic      `json:"topic"`
	ContentType ContentType `json:"content_type"`
	Priority    string      `json:"priority"`
	Template    string      `json:"template,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	RequestedBy string      `json:"requested_by"`
	CreatedAt   time.Time   `json:"created_at"`
}

// GenerationResult represents the result of content generation
type GenerationResult struct {
	RequestID        string            `json:"request_id"`
	Document         *document.Document `json:"document"`
	QualityScore     float64           `json:"quality_score"`
	ValidationResult *ValidationResult `json:"validation_result"`
	GenerationModel  string            `json:"generation_model"`
	ProcessingTime   time.Duration     `json:"processing_time"`
	Success          bool              `json:"success"`
	Error            string            `json:"error,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

// ValidationResult contains quality validation results
type ValidationResult struct {
	OverallScore      float64            `json:"overall_score"`
	DimensionScores   map[string]float64 `json:"dimension_scores"`
	QualityTier       string             `json:"quality_tier"`
	ConfidenceLevel   float64            `json:"confidence_level"`
	FactCheckResults  []FactCheckResult  `json:"fact_check_results"`
	TechnicalResults  *TechnicalValidation `json:"technical_results,omitempty"`
	ImprovementAreas  []string           `json:"improvement_areas"`
	ValidationTime    time.Duration      `json:"validation_time"`
}

// FactCheckResult represents the result of fact-checking a claim
type FactCheckResult struct {
	Claim       string    `json:"claim"`
	Verified    bool      `json:"verified"`
	Confidence  float64   `json:"confidence"`
	Sources     []string  `json:"sources"`
	Explanation string    `json:"explanation"`
	CheckedAt   time.Time `json:"checked_at"`
}

// TechnicalValidation represents technical content validation results
type TechnicalValidation struct {
	CodeBlocks       []CodeValidation `json:"code_blocks"`
	MathFormulas     []MathValidation `json:"math_formulas"`
	TechnicalAccuracy float64         `json:"technical_accuracy"`
	ExecutionResults []ExecutionResult `json:"execution_results"`
}

// CodeValidation represents code validation results
type CodeValidation struct {
	Language      string `json:"language"`
	Code          string `json:"code"`
	SyntaxValid   bool   `json:"syntax_valid"`
	Compilable    bool   `json:"compilable"`
	Executable    bool   `json:"executable"`
	BestPractices bool   `json:"best_practices"`
	SecuritySafe  bool   `json:"security_safe"`
	Errors        []string `json:"errors"`
}

// MathValidation represents mathematical formula validation
type MathValidation struct {
	Formula     string  `json:"formula"`
	Valid       bool    `json:"valid"`
	Confidence  float64 `json:"confidence"`
	Explanation string  `json:"explanation"`
}

// ExecutionResult represents code execution test results
type ExecutionResult struct {
	Language    string        `json:"language"`
	Code        string        `json:"code"`
	Input       string        `json:"input,omitempty"`
	Output      string        `json:"output"`
	Error       string        `json:"error,omitempty"`
	ExitCode    int           `json:"exit_code"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
}

// LLMProvider defines the interface for language model providers
type LLMProvider interface {
	Generate(ctx context.Context, request *GenerationRequest) (*GenerationResult, error)
	GetModelName() string
	GetCapabilities() []string
	EstimateCost(request *GenerationRequest) float64
	IsAvailable() bool
}

// QualityValidator defines the interface for content quality validation
type QualityValidator interface {
	ValidateContent(ctx context.Context, content string, metadata map[string]string) (*ValidationResult, error)
	ValidateCode(ctx context.Context, code string, language string) (*CodeValidation, error)
	ValidateMath(ctx context.Context, formula string) (*MathValidation, error)
	FactCheck(ctx context.Context, content string, topic *Topic) ([]FactCheckResult, error)
}

// ContentPlanner defines the interface for content planning
type ContentPlanner interface {
	PlanContent(ctx context.Context, domain string, count int) ([]*Topic, error)
	PrioritizeTopics(ctx context.Context, topics []*Topic) ([]*Topic, error)
	GenerateTopicSuggestions(ctx context.Context, existing []*Topic) ([]*Topic, error)
	AnalyzeContentGaps(ctx context.Context, domain string) (*GapAnalysis, error)
}

// GapAnalysis represents analysis of content gaps in the collection
type GapAnalysis struct {
	Domain           string            `json:"domain"`
	TotalDocuments   int               `json:"total_documents"`
	TopicCoverage    map[string]int    `json:"topic_coverage"`
	UnderservedTopics []string         `json:"underserved_topics"`
	TrendingTopics   []string          `json:"trending_topics"`
	PriorityAreas    []string          `json:"priority_areas"`
	Recommendations  []string          `json:"recommendations"`
	AnalyzedAt       time.Time         `json:"analyzed_at"`
}

// AttributionManager handles content attribution and metadata
type AttributionManager interface {
	GenerateAttribution(contentType ContentType, model string, topic *Topic) string
	EnrichMetadata(doc *document.Document, result *GenerationResult) error
	ValidateAttribution(doc *document.Document) error
}

// ProcurementMetrics tracks pipeline performance
type ProcurementMetrics struct {
	RequestsProcessed   int64         `json:"requests_processed"`
	RequestsSuccessful  int64         `json:"requests_successful"`
	RequestsFailed      int64         `json:"requests_failed"`
	AverageQualityScore float64       `json:"average_quality_score"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	ModelPerformance    map[string]*ModelMetrics `json:"model_performance"`
	QualityDistribution map[string]int `json:"quality_distribution"`
	TopicDistribution   map[string]int `json:"topic_distribution"`
	LastUpdated         time.Time     `json:"last_updated"`
}

// ModelMetrics tracks individual model performance
type ModelMetrics struct {
	ModelName         string        `json:"model_name"`
	RequestsHandled   int64         `json:"requests_handled"`
	SuccessRate       float64       `json:"success_rate"`
	AverageQuality    float64       `json:"average_quality"`
	AverageLatency    time.Duration `json:"average_latency"`
	CostPerRequest    float64       `json:"cost_per_request"`
	LastUsed          time.Time     `json:"last_used"`
}

// ContentTemplate represents a template for content generation
type ContentTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ContentType ContentType       `json:"content_type"`
	Template    string            `json:"template"`
	Variables   []string          `json:"variables"`
	Instructions string           `json:"instructions"`
	Examples    []string          `json:"examples"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// QualityTier represents content quality classification
type QualityTier string

const (
	QualityTierPremium QualityTier = "premium" // Score >= 0.9
	QualityTierHigh    QualityTier = "high"    // Score >= 0.8
	QualityTierMedium  QualityTier = "medium"  // Score >= 0.65
	QualityTierLow     QualityTier = "low"     // Score >= 0.5
	QualityTierReject  QualityTier = "reject"  // Score < 0.5
)

// Priority levels for content generation
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// ProcessingStatus represents the current status of a request
type ProcessingStatus string

const (
	StatusPending    ProcessingStatus = "pending"
	StatusProcessing ProcessingStatus = "processing"
	StatusValidating ProcessingStatus = "validating"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
	StatusRetrying   ProcessingStatus = "retrying"
)

// SyntheticContentService defines the main synthetic content generation service
type SyntheticContentService interface {
	GenerateContent(ctx context.Context, request *GenerationRequest) (*GenerationResult, error)
	GenerateBatch(ctx context.Context, requests []*GenerationRequest) ([]*GenerationResult, error)
	GetMetrics() *ProcurementMetrics
	GetModelPerformance(modelName string) (*ModelMetrics, error)
	UpdateConfiguration(config *ServiceConfig) error
}

// ServiceConfig represents service configuration
type ServiceConfig struct {
	MaxConcurrentRequests int                    `json:"max_concurrent_requests"`
	DefaultTimeout        time.Duration          `json:"default_timeout"`
	QualityThreshold      float64               `json:"quality_threshold"`
	EnabledModels         []string              `json:"enabled_models"`
	ModelWeights          map[string]float64    `json:"model_weights"`
	RetryPolicy           *RetryPolicy          `json:"retry_policy"`
	CostLimits           map[string]float64    `json:"cost_limits"`
	TemplateConfig       *TemplateConfig       `json:"template_config"`
}

// RetryPolicy defines retry behavior for failed requests
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	BaseDelay     time.Duration `json:"base_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
}

// TemplateConfig defines template management configuration
type TemplateConfig struct {
	TemplateDirectory string            `json:"template_directory"`
	DefaultTemplates  map[ContentType]string `json:"default_templates"`
	CustomTemplates   map[string]string `json:"custom_templates"`
	TemplateVariables map[string]interface{} `json:"template_variables"`
}

// ContentPlan represents a planned content structure
type ContentPlan struct {
	Topic           string      `json:"topic"`
	ContentType     ContentType `json:"content_type"`
	Sections        []string    `json:"sections"`
	Keywords        []string    `json:"keywords"`
	EstimatedLength int         `json:"estimated_length"`
	TargetAudience  string      `json:"target_audience"`
	Difficulty      string      `json:"difficulty"`
}

package quality

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/rs/zerolog/log"
)

// QualityValidator implements content quality validation
type QualityValidator struct {
	factChecker    *FactChecker
	codeValidator  *CodeValidator
	mathValidator  *MathValidator
	readabilityAnalyzer *ReadabilityAnalyzer
	
	// Configuration
	config *ValidationConfig
}

// ValidationConfig contains configuration for quality validation
type ValidationConfig struct {
	MinWordCount        int     `json:"min_word_count"`
	MaxWordCount        int     `json:"max_word_count"`
	MinSentenceCount    int     `json:"min_sentence_count"`
	ReadabilityThreshold float64 `json:"readability_threshold"`
	SpamThreshold       float64 `json:"spam_threshold"`
	
	// Quality dimension weights
	AccuracyWeight     float64 `json:"accuracy_weight"`
	CompletenessWeight float64 `json:"completeness_weight"`
	RelevanceWeight    float64 `json:"relevance_weight"`
	QualityWeight      float64 `json:"quality_weight"`
	UniquenessWeight   float64 `json:"uniqueness_weight"`
}

// DefaultValidationConfig returns default validation configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MinWordCount:        100,
		MaxWordCount:        50000,
		MinSentenceCount:    5,
		ReadabilityThreshold: 40.0,
		SpamThreshold:       0.3,
		AccuracyWeight:      0.3,
		CompletenessWeight:  0.2,
		RelevanceWeight:     0.2,
		QualityWeight:       0.15,
		UniquenessWeight:    0.15,
	}
}

// NewQualityValidator creates a new quality validator
func NewQualityValidator(config *ValidationConfig) *QualityValidator {
	if config == nil {
		config = DefaultValidationConfig()
	}
	
	return &QualityValidator{
		factChecker:         NewFactChecker(),
		codeValidator:       NewCodeValidator(),
		mathValidator:       NewMathValidator(),
		readabilityAnalyzer: NewReadabilityAnalyzer(),
		config:             config,
	}
}

// ValidateContent validates content quality across multiple dimensions
func (qv *QualityValidator) ValidateContent(ctx context.Context, content string, metadata map[string]string) (*procurement.ValidationResult, error) {
	start := time.Now()
	
	log.Debug().
		Int("content_length", len(content)).
		Str("content_type", metadata["content_type"]).
		Msg("Starting content quality validation")
	
	// Initialize result
	result := &procurement.ValidationResult{
		DimensionScores:  make(map[string]float64),
		FactCheckResults: make([]procurement.FactCheckResult, 0),
		ImprovementAreas: make([]string, 0),
	}
	
	// Validate basic content metrics
	basicMetrics, err := qv.validateBasicMetrics(content)
	if err != nil {
		return nil, fmt.Errorf("basic metrics validation failed: %w", err)
	}
	
	// Assess accuracy
	accuracyScore, factResults, err := qv.assessAccuracy(ctx, content, metadata)
	if err != nil {
		log.Warn().Err(err).Msg("Accuracy assessment failed, using default score")
		accuracyScore = 0.7 // Default neutral score
	}
	result.DimensionScores["accuracy"] = accuracyScore
	result.FactCheckResults = factResults
	
	// Assess completeness
	completenessScore := qv.assessCompleteness(content, metadata)
	result.DimensionScores["completeness"] = completenessScore
	
	// Assess relevance
	relevanceScore := qv.assessRelevance(content, metadata)
	result.DimensionScores["relevance"] = relevanceScore
	
	// Assess content quality
	qualityScore := qv.assessContentQuality(content, basicMetrics)
	result.DimensionScores["quality"] = qualityScore
	
	// Assess uniqueness (simplified - in production would check against database)
	uniquenessScore := qv.assessUniqueness(content)
	result.DimensionScores["uniqueness"] = uniquenessScore
	
	// Calculate overall score
	result.OverallScore = qv.calculateOverallScore(result.DimensionScores)
	result.QualityTier = qv.determineQualityTier(result.OverallScore)
	result.ConfidenceLevel = qv.calculateConfidenceLevel(result.DimensionScores, factResults)
	
	// Validate technical content if applicable
	if qv.containsTechnicalContent(content) {
		techValidation, err := qv.validateTechnicalContent(ctx, content)
		if err != nil {
			log.Warn().Err(err).Msg("Technical validation failed")
		} else {
			result.TechnicalResults = techValidation
		}
	}
	
	// Generate improvement suggestions
	result.ImprovementAreas = qv.generateImprovementSuggestions(result.DimensionScores, basicMetrics)
	
	result.ValidationTime = time.Since(start)
	
	log.Debug().
		Float64("overall_score", result.OverallScore).
		Str("quality_tier", result.QualityTier).
		Dur("validation_time", result.ValidationTime).
		Msg("Content quality validation completed")
	
	return result, nil
}

// validateBasicMetrics validates basic content metrics
func (qv *QualityValidator) validateBasicMetrics(content string) (*BasicMetrics, error) {
	metrics := &BasicMetrics{}
	
	// Word count
	words := strings.Fields(content)
	metrics.WordCount = len(words)
	
	if metrics.WordCount < qv.config.MinWordCount {
		return nil, fmt.Errorf("content too short: %d words (minimum: %d)", metrics.WordCount, qv.config.MinWordCount)
	}
	
	if metrics.WordCount > qv.config.MaxWordCount {
		return nil, fmt.Errorf("content too long: %d words (maximum: %d)", metrics.WordCount, qv.config.MaxWordCount)
	}
	
	// Sentence count
	sentences := qv.splitIntoSentences(content)
	metrics.SentenceCount = len(sentences)
	
	if metrics.SentenceCount < qv.config.MinSentenceCount {
		return nil, fmt.Errorf("too few sentences: %d (minimum: %d)", metrics.SentenceCount, qv.config.MinSentenceCount)
	}
	
	// Average sentence length
	if metrics.SentenceCount > 0 {
		metrics.AverageSentenceLength = float64(metrics.WordCount) / float64(metrics.SentenceCount)
	}
	
	// Character count
	metrics.CharacterCount = len(content)
	
	// Paragraph count
	paragraphs := strings.Split(content, "\n\n")
	metrics.ParagraphCount = len(paragraphs)
	
	return metrics, nil
}

// assessAccuracy evaluates content accuracy through fact-checking
func (qv *QualityValidator) assessAccuracy(ctx context.Context, content string, metadata map[string]string) (float64, []procurement.FactCheckResult, error) {
	// Extract factual claims
	claims := qv.extractFactualClaims(content)
	
	if len(claims) == 0 {
		// No factual claims to verify - assume neutral accuracy
		return 0.8, []procurement.FactCheckResult{}, nil
	}
	
	// Fact-check claims
	factResults := make([]procurement.FactCheckResult, 0)
	accuracyScores := make([]float64, 0)
	
	for _, claim := range claims {
		result, err := qv.factChecker.CheckFact(ctx, claim, metadata["domain"])
		if err != nil {
			log.Warn().Err(err).Str("claim", claim).Msg("Fact check failed")
			continue
		}
		
		factResults = append(factResults, *result)
		accuracyScores = append(accuracyScores, result.Confidence)
	}
	
	// Calculate overall accuracy score
	var totalAccuracy float64
	for _, score := range accuracyScores {
		totalAccuracy += score
	}
	
	accuracyScore := totalAccuracy / float64(len(accuracyScores))
	
	return accuracyScore, factResults, nil
}

// assessCompleteness evaluates content completeness
func (qv *QualityValidator) assessCompleteness(content string, metadata map[string]string) float64 {
	score := 0.0
	
	// Check for essential elements based on content type
	contentType := metadata["content_type"]
	
	switch contentType {
	case "research_abstract":
		score = qv.assessResearchCompletenessScore(content)
	case "tutorial":
		score = qv.assessTutorialCompletenessScore(content)
	case "documentation":
		score = qv.assessDocumentationCompletenessScore(content)
	default:
		score = qv.assessGeneralCompletenessScore(content)
	}
	
	// Check metadata completeness
	metadataScore := qv.assessMetadataCompleteness(metadata)
	
	// Combine content and metadata completeness
	return score*0.8 + metadataScore*0.2
}

func (qv *QualityValidator) assessResearchCompletenessScore(content string) float64 {
	requiredElements := []string{
		"abstract", "background", "methodology", "method", "results", "conclusion", "implications",
	}
	
	lowerContent := strings.ToLower(content)
	score := 0.0
	
	for _, element := range requiredElements {
		if strings.Contains(lowerContent, element) {
			score += 1.0
		}
	}
	
	return score / float64(len(requiredElements))
}

func (qv *QualityValidator) assessTutorialCompletenessScore(content string) float64 {
	requiredElements := []string{
		"overview", "prerequisites", "step", "example", "implementation",
	}
	
	lowerContent := strings.ToLower(content)
	score := 0.0
	
	for _, element := range requiredElements {
		if strings.Contains(lowerContent, element) {
			score += 1.0
		}
	}
	
	// Check for numbered steps
	stepPattern := regexp.MustCompile(`(?i)(step\s+\d+|^\d+\.)`)
	if stepPattern.MatchString(content) {
		score += 0.5
	}
	
	return math.Min(score/float64(len(requiredElements)), 1.0)
}

func (qv *QualityValidator) assessDocumentationCompletenessScore(content string) float64 {
	requiredElements := []string{
		"description", "usage", "example", "parameter",
	}
	
	lowerContent := strings.ToLower(content)
	score := 0.0
	
	for _, element := range requiredElements {
		if strings.Contains(lowerContent, element) {
			score += 1.0
		}
	}
	
	return score / float64(len(requiredElements))
}

func (qv *QualityValidator) assessGeneralCompletenessScore(content string) float64 {
	score := 0.0
	
	// Check for structure indicators
	if strings.Contains(content, "#") || strings.Contains(content, "##") {
		score += 0.3 // Has headings
	}
	
	if strings.Contains(strings.ToLower(content), "conclusion") ||
	   strings.Contains(strings.ToLower(content), "summary") {
		score += 0.3 // Has conclusion
	}
	
	if strings.Contains(strings.ToLower(content), "introduction") ||
	   strings.Contains(strings.ToLower(content), "overview") {
		score += 0.2 // Has introduction
	}
	
	// Check content depth
	paragraphs := strings.Split(content, "\n\n")
	if len(paragraphs) >= 3 {
		score += 0.2 // Has multiple paragraphs
	}
	
	return math.Min(score, 1.0)
}

// assessRelevance evaluates content relevance to the topic
func (qv *QualityValidator) assessRelevance(content string, metadata map[string]string) float64 {
	topic := metadata["topic"]
	keywords := strings.Split(metadata["keywords"], ",")
	
	if topic == "" {
		return 0.7 // Default neutral score
	}
	
	lowerContent := strings.ToLower(content)
	lowerTopic := strings.ToLower(topic)
	
	score := 0.0
	
	// Check if topic appears in content
	if strings.Contains(lowerContent, lowerTopic) {
		score += 0.4
	}
	
	// Check keyword presence
	keywordScore := 0.0
	validKeywords := 0
	
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		
		validKeywords++
		if strings.Contains(lowerContent, strings.ToLower(keyword)) {
			keywordScore += 1.0
		}
	}
	
	if validKeywords > 0 {
		keywordScore = keywordScore / float64(validKeywords)
		score += keywordScore * 0.4
	} else {
		score += 0.4 // No keywords to check
	}
	
	// Content focus score (topic density)
	topicMentions := strings.Count(lowerContent, lowerTopic)
	words := strings.Fields(content)
	if len(words) > 0 {
		topicDensity := float64(topicMentions) / float64(len(words))
		focusScore := math.Min(topicDensity*100, 1.0) // Cap at 1.0
		score += focusScore * 0.2
	}
	
	return math.Min(score, 1.0)
}

// assessContentQuality evaluates general content quality
func (qv *QualityValidator) assessContentQuality(content string, metrics *BasicMetrics) float64 {
	score := 0.0
	
	// Readability score
	readabilityScore := qv.readabilityAnalyzer.CalculateReadabilityScore(content)
	normalizedReadability := math.Max(0, math.Min(1, (readabilityScore-qv.config.ReadabilityThreshold)/50.0+0.5))
	score += normalizedReadability * 0.3
	
	// Grammar and language quality (simplified)
	languageScore := qv.assessLanguageQuality(content)
	score += languageScore * 0.3
	
	// Structure score
	structureScore := qv.assessStructureQuality(content)
	score += structureScore * 0.2
	
	// Length appropriateness
	lengthScore := qv.assessLengthAppropriateness(metrics)
	score += lengthScore * 0.2
	
	return math.Min(score, 1.0)
}

// assessUniqueness evaluates content uniqueness (simplified implementation)
func (qv *QualityValidator) assessUniqueness(content string) float64 {
	// In a real implementation, this would check against existing content database
	// For now, we'll do basic uniqueness checks
	
	score := 1.0
	
	// Check for common template language that suggests non-unique content
	templateIndicators := []string{
		"lorem ipsum",
		"placeholder text",
		"sample content",
		"example text",
		"template",
	}
	
	lowerContent := strings.ToLower(content)
	for _, indicator := range templateIndicators {
		if strings.Contains(lowerContent, indicator) {
			score -= 0.3
		}
	}
	
	// Check for excessive repetition
	words := strings.Fields(content)
	wordCounts := make(map[string]int)
	
	for _, word := range words {
		if len(word) > 3 { // Only count words longer than 3 characters
			wordCounts[strings.ToLower(word)]++
		}
	}
	
	// Calculate repetition penalty
	totalWords := len(words)
	excessiveRepetitions := 0
	
	for _, count := range wordCounts {
		if count > totalWords/20 { // More than 5% repetition
			excessiveRepetitions++
		}
	}
	
	if excessiveRepetitions > 0 {
		repetitionPenalty := float64(excessiveRepetitions) * 0.1
		score -= repetitionPenalty
	}
	
	return math.Max(score, 0.0)
}

// Helper methods

func (qv *QualityValidator) calculateOverallScore(dimensionScores map[string]float64) float64 {
	totalScore := 0.0
	
	totalScore += dimensionScores["accuracy"] * qv.config.AccuracyWeight
	totalScore += dimensionScores["completeness"] * qv.config.CompletenessWeight
	totalScore += dimensionScores["relevance"] * qv.config.RelevanceWeight
	totalScore += dimensionScores["quality"] * qv.config.QualityWeight
	totalScore += dimensionScores["uniqueness"] * qv.config.UniquenessWeight
	
	return math.Min(totalScore, 1.0)
}

func (qv *QualityValidator) determineQualityTier(score float64) string {
	switch {
	case score >= 0.9:
		return "premium"
	case score >= 0.8:
		return "high"
	case score >= 0.65:
		return "medium"
	case score >= 0.5:
		return "low"
	default:
		return "reject"
	}
}

func (qv *QualityValidator) calculateConfidenceLevel(dimensionScores map[string]float64, factResults []procurement.FactCheckResult) float64 {
	// Calculate confidence based on score consistency and fact-checking results
	scores := make([]float64, 0)
	for _, score := range dimensionScores {
		scores = append(scores, score)
	}
	
	// Calculate standard deviation of scores
	mean := 0.0
	for _, score := range scores {
		mean += score
	}
	mean /= float64(len(scores))
	
	variance := 0.0
	for _, score := range scores {
		variance += (score - mean) * (score - mean)
	}
	variance /= float64(len(scores))
	
	consistency := 1.0 - math.Min(math.Sqrt(variance), 1.0)
	
	// Factor in fact-checking confidence
	factConfidence := 1.0
	if len(factResults) > 0 {
		totalFactConfidence := 0.0
		for _, result := range factResults {
			totalFactConfidence += result.Confidence
		}
		factConfidence = totalFactConfidence / float64(len(factResults))
	}
	
	return (consistency*0.6 + factConfidence*0.4)
}

func (qv *QualityValidator) generateImprovementSuggestions(dimensionScores map[string]float64, metrics *BasicMetrics) []string {
	suggestions := make([]string, 0)
	
	if dimensionScores["accuracy"] < 0.7 {
		suggestions = append(suggestions, "Improve factual accuracy by verifying claims against authoritative sources")
	}
	
	if dimensionScores["completeness"] < 0.7 {
		suggestions = append(suggestions, "Add more comprehensive coverage of the topic")
	}
	
	if dimensionScores["relevance"] < 0.7 {
		suggestions = append(suggestions, "Better align content with the specified topic and keywords")
	}
	
	if dimensionScores["quality"] < 0.7 {
		suggestions = append(suggestions, "Improve readability and language quality")
	}
	
	if dimensionScores["uniqueness"] < 0.7 {
		suggestions = append(suggestions, "Reduce repetitive content and add more original insights")
	}
	
	if metrics.AverageSentenceLength > 25 {
		suggestions = append(suggestions, "Consider shortening sentences for better readability")
	}
	
	if metrics.ParagraphCount < 3 {
		suggestions = append(suggestions, "Break content into more paragraphs for better structure")
	}
	
	return suggestions
}

// BasicMetrics represents basic content metrics
type BasicMetrics struct {
	WordCount             int     `json:"word_count"`
	SentenceCount         int     `json:"sentence_count"`
	ParagraphCount        int     `json:"paragraph_count"`
	CharacterCount        int     `json:"character_count"`
	AverageSentenceLength float64 `json:"average_sentence_length"`
}

// Additional helper methods would go here...
func (qv *QualityValidator) splitIntoSentences(content string) []string {
	// Simple sentence splitting - in production would use more sophisticated NLP
	sentences := regexp.MustCompile(`[.!?]+\s+`).Split(content, -1)
	
	// Filter out very short sentences (likely not real sentences)
	realSentences := make([]string, 0)
	for _, sentence := range sentences {
		if len(strings.TrimSpace(sentence)) > 10 {
			realSentences = append(realSentences, sentence)
		}
	}
	
	return realSentences
}

func (qv *QualityValidator) extractFactualClaims(content string) []string {
	// Simplified claim extraction - in production would use NLP
	claims := make([]string, 0)
	
	// Look for sentences with numerical data, dates, or definitive statements
	sentences := qv.splitIntoSentences(content)
	
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		
		// Check for numerical claims
		if regexp.MustCompile(`\d+`).MatchString(sentence) {
			claims = append(claims, sentence)
			continue
		}
		
		// Check for definitive statements
		definitivePatterns := []string{
			"research shows", "studies indicate", "according to", "evidence suggests",
			"data reveals", "statistics show", "findings demonstrate",
		}
		
		lowerSentence := strings.ToLower(sentence)
		for _, pattern := range definitivePatterns {
			if strings.Contains(lowerSentence, pattern) {
				claims = append(claims, sentence)
				break
			}
		}
	}
	
	return claims
}

func (qv *QualityValidator) assessMetadataCompleteness(metadata map[string]string) float64 {
	essentialFields := []string{"title", "topic", "content_type"}
	optionalFields := []string{"keywords", "domain", "difficulty"}
	
	essentialScore := 0.0
	for _, field := range essentialFields {
		if value, exists := metadata[field]; exists && strings.TrimSpace(value) != "" {
			essentialScore += 1.0
		}
	}
	essentialScore /= float64(len(essentialFields))
	
	optionalScore := 0.0
	for _, field := range optionalFields {
		if value, exists := metadata[field]; exists && strings.TrimSpace(value) != "" {
			optionalScore += 1.0
		}
	}
	optionalScore /= float64(len(optionalFields))
	
	return essentialScore*0.8 + optionalScore*0.2
}

func (qv *QualityValidator) assessLanguageQuality(content string) float64 {
	score := 1.0
	
	// Check for common grammar issues (simplified)
	commonErrors := []string{
		" i ", " i'", "it's own", "their own", "your welcome", "alot",
	}
	
	lowerContent := strings.ToLower(content)
	for _, error := range commonErrors {
		if strings.Contains(lowerContent, error) {
			score -= 0.1
		}
	}
	
	return math.Max(score, 0.0)
}

func (qv *QualityValidator) assessStructureQuality(content string) float64 {
	score := 0.0
	
	// Check for headings
	if regexp.MustCompile(`#{1,6}\s+`).MatchString(content) {
		score += 0.4
	}
	
	// Check for lists
	if regexp.MustCompile(`^\s*[-*+]\s+`).MatchString(content) ||
	   regexp.MustCompile(`^\s*\d+\.\s+`).MatchString(content) {
		score += 0.3
	}
	
	// Check for code blocks
	if strings.Contains(content, "```") || strings.Contains(content, "`") {
		score += 0.2
	}
	
	// Check for proper paragraphing
	paragraphs := strings.Split(content, "\n\n")
	if len(paragraphs) >= 3 {
		score += 0.1
	}
	
	return math.Min(score, 1.0)
}

func (qv *QualityValidator) assessLengthAppropriateness(metrics *BasicMetrics) float64 {
	// Optimal range is different based on content type
	optimalMin := 500  // words
	optimalMax := 2000 // words
	
	if metrics.WordCount < optimalMin {
		return float64(metrics.WordCount) / float64(optimalMin)
	}
	
	if metrics.WordCount > optimalMax {
		excess := metrics.WordCount - optimalMax
		penalty := float64(excess) / float64(optimalMax)
		return math.Max(1.0-penalty, 0.5) // Don't penalize too harshly
	}
	
	return 1.0 // Within optimal range
}

func (qv *QualityValidator) containsTechnicalContent(content string) bool {
	// Check for code blocks, technical terms, etc.
	technicalIndicators := []string{
		"```", "function", "class", "import", "def ", "var ", "let ", "const ",
		"algorithm", "implementation", "API", "database", "server",
	}
	
	lowerContent := strings.ToLower(content)
	for _, indicator := range technicalIndicators {
		if strings.Contains(lowerContent, indicator) {
			return true
		}
	}
	
	return false
}

func (qv *QualityValidator) validateTechnicalContent(ctx context.Context, content string) (*procurement.TechnicalValidation, error) {
	validation := &procurement.TechnicalValidation{
		CodeBlocks:       make([]procurement.CodeValidation, 0),
		MathFormulas:     make([]procurement.MathValidation, 0),
		ExecutionResults: make([]procurement.ExecutionResult, 0),
	}
	
	// Extract and validate code blocks
	codeBlocks := qv.extractCodeBlocks(content)
	for _, block := range codeBlocks {
		codeValidation, err := qv.codeValidator.ValidateCode(ctx, block.Code, block.Language)
		if err != nil {
			log.Warn().Err(err).Msg("Code validation failed")
			continue
		}
		validation.CodeBlocks = append(validation.CodeBlocks, *codeValidation)
	}
	
	// Extract and validate mathematical formulas
	mathFormulas := qv.extractMathFormulas(content)
	for _, formula := range mathFormulas {
		mathValidation, err := qv.mathValidator.ValidateMath(ctx, formula)
		if err != nil {
			log.Warn().Err(err).Msg("Math validation failed")
			continue
		}
		validation.MathFormulas = append(validation.MathFormulas, *mathValidation)
	}
	
	// Calculate overall technical accuracy
	totalValidations := len(validation.CodeBlocks) + len(validation.MathFormulas)
	if totalValidations > 0 {
		validCount := 0
		for _, code := range validation.CodeBlocks {
			if code.SyntaxValid {
				validCount++
			}
		}
		for _, math := range validation.MathFormulas {
			if math.Valid {
				validCount++
			}
		}
		validation.TechnicalAccuracy = float64(validCount) / float64(totalValidations)
	} else {
		validation.TechnicalAccuracy = 1.0 // No technical content to validate
	}
	
	return validation, nil
}

// CodeBlock represents a code block extracted from content
type CodeBlock struct {
	Language string
	Code     string
}

func (qv *QualityValidator) extractCodeBlocks(content string) []CodeBlock {
	blocks := make([]CodeBlock, 0)
	
	// Match fenced code blocks
	codeBlockPattern := regexp.MustCompile("```(\\w*)\\n([\\s\\S]*?)\\n```")
	matches := codeBlockPattern.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) >= 3 {
			language := match[1]
			if language == "" {
				language = "text"
			}
			code := match[2]
			
			blocks = append(blocks, CodeBlock{
				Language: language,
				Code:     code,
			})
		}
	}
	
	return blocks
}

func (qv *QualityValidator) extractMathFormulas(content string) []string {
	formulas := make([]string, 0)
	
	// Simple math formula extraction (LaTeX-style)
	mathPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\$([^$]+)\$`),                    // Inline math
		regexp.MustCompile(`\$\$([^\$]+)\$\$`),               // Display math
		regexp.MustCompile(`\\begin\{equation\}(.*?)\\end\{equation\}`), // Equation environment
	}
	
	for _, pattern := range mathPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				formulas = append(formulas, strings.TrimSpace(match[1]))
			}
		}
	}
	
	return formulas
}

// ValidateCode validates a single code snippet
func (qv *QualityValidator) ValidateCode(ctx context.Context, code string, language string) (*procurement.CodeValidation, error) {
	return qv.codeValidator.ValidateCode(ctx, code, language)
}

// ValidateMath validates a mathematical formula
func (qv *QualityValidator) ValidateMath(ctx context.Context, formula string) (*procurement.MathValidation, error) {
	return qv.mathValidator.ValidateMath(ctx, formula)
}

// FactCheck performs fact-checking on content
func (qv *QualityValidator) FactCheck(ctx context.Context, content string, topic *procurement.Topic) ([]procurement.FactCheckResult, error) {
	claims := qv.extractFactualClaims(content)
	results := make([]procurement.FactCheckResult, 0)
	
	for _, claim := range claims {
		result, err := qv.factChecker.CheckFact(ctx, claim, topic.Domain)
		if err != nil {
			log.Warn().Err(err).Str("claim", claim).Msg("Fact check failed")
			continue
		}
		results = append(results, *result)
	}
	
	return results, nil
}
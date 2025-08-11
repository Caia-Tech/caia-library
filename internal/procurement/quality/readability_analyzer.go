package quality

import (
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"
)

// ReadabilityAnalyzer calculates readability scores for text content
type ReadabilityAnalyzer struct {
	config *ReadabilityConfig
}

// ReadabilityConfig configures readability analysis
type ReadabilityConfig struct {
	EnableFleschKincaid   bool    `json:"enable_flesch_kincaid"`
	EnableGunningFog      bool    `json:"enable_gunning_fog"`
	EnableSMOG            bool    `json:"enable_smog"`
	EnableARI             bool    `json:"enable_ari"`
	TargetReadabilityMin  float64 `json:"target_readability_min"`
	TargetReadabilityMax  float64 `json:"target_readability_max"`
}

// ReadabilityMetrics contains various readability measurements
type ReadabilityMetrics struct {
	FleschKincaidGrade    float64 `json:"flesch_kincaid_grade"`
	FleschReadingEase     float64 `json:"flesch_reading_ease"`
	GunningFogIndex       float64 `json:"gunning_fog_index"`
	SMOGIndex            float64 `json:"smog_index"`
	AutomatedReadabilityIndex float64 `json:"automated_readability_index"`
	
	// Basic metrics
	WordCount        int     `json:"word_count"`
	SentenceCount    int     `json:"sentence_count"`
	SyllableCount    int     `json:"syllable_count"`
	ComplexWordCount int     `json:"complex_word_count"`
	CharacterCount   int     `json:"character_count"`
	
	// Derived metrics
	AverageWordsPerSentence    float64 `json:"avg_words_per_sentence"`
	AverageSyllablesPerWord    float64 `json:"avg_syllables_per_word"`
	PercentageComplexWords     float64 `json:"percentage_complex_words"`
	
	// Overall assessment
	OverallScore    float64 `json:"overall_score"`
	ReadabilityLevel string  `json:"readability_level"`
	Recommendations []string `json:"recommendations"`
}

// NewReadabilityAnalyzer creates a new readability analyzer
func NewReadabilityAnalyzer() *ReadabilityAnalyzer {
	return &ReadabilityAnalyzer{
		config: &ReadabilityConfig{
			EnableFleschKincaid:  true,
			EnableGunningFog:     true,
			EnableSMOG:          true,
			EnableARI:           true,
			TargetReadabilityMin: 8.0,  // 8th grade level
			TargetReadabilityMax: 12.0, // 12th grade level
		},
	}
}

// CalculateReadabilityScore calculates a simple readability score
func (ra *ReadabilityAnalyzer) CalculateReadabilityScore(text string) float64 {
	metrics := ra.AnalyzeReadability(text)
	return metrics.OverallScore
}

// AnalyzeReadability performs comprehensive readability analysis
func (ra *ReadabilityAnalyzer) AnalyzeReadability(text string) *ReadabilityMetrics {
	if strings.TrimSpace(text) == "" {
		return &ReadabilityMetrics{
			OverallScore:     0.0,
			ReadabilityLevel: "unreadable",
			Recommendations:  []string{"Content is empty"},
		}
	}
	
	// Calculate basic metrics
	metrics := &ReadabilityMetrics{}
	metrics.WordCount = ra.countWords(text)
	metrics.SentenceCount = ra.countSentences(text)
	metrics.SyllableCount = ra.countSyllables(text)
	metrics.ComplexWordCount = ra.countComplexWords(text)
	metrics.CharacterCount = ra.countCharacters(text)
	
	// Calculate derived metrics
	if metrics.SentenceCount > 0 {
		metrics.AverageWordsPerSentence = float64(metrics.WordCount) / float64(metrics.SentenceCount)
	}
	
	if metrics.WordCount > 0 {
		metrics.AverageSyllablesPerWord = float64(metrics.SyllableCount) / float64(metrics.WordCount)
		metrics.PercentageComplexWords = (float64(metrics.ComplexWordCount) / float64(metrics.WordCount)) * 100
	}
	
	// Calculate readability scores
	if ra.config.EnableFleschKincaid {
		metrics.FleschKincaidGrade = ra.calculateFleschKincaidGrade(metrics)
		metrics.FleschReadingEase = ra.calculateFleschReadingEase(metrics)
	}
	
	if ra.config.EnableGunningFog {
		metrics.GunningFogIndex = ra.calculateGunningFogIndex(metrics)
	}
	
	if ra.config.EnableSMOG {
		metrics.SMOGIndex = ra.calculateSMOGIndex(metrics)
	}
	
	if ra.config.EnableARI {
		metrics.AutomatedReadabilityIndex = ra.calculateARI(metrics)
	}
	
	// Calculate overall score and assessment
	metrics.OverallScore = ra.calculateOverallScore(metrics)
	metrics.ReadabilityLevel = ra.determineReadabilityLevel(metrics.OverallScore)
	metrics.Recommendations = ra.generateRecommendations(metrics)
	
	log.Debug().
		Int("words", metrics.WordCount).
		Int("sentences", metrics.SentenceCount).
		Float64("flesch_kincaid", metrics.FleschKincaidGrade).
		Float64("overall_score", metrics.OverallScore).
		Str("level", metrics.ReadabilityLevel).
		Msg("Readability analysis completed")
	
	return metrics
}

// Basic text metric calculations

func (ra *ReadabilityAnalyzer) countWords(text string) int {
	// Split text into words and count non-empty ones
	words := strings.Fields(text)
	wordCount := 0
	
	for _, word := range words {
		// Remove punctuation and check if it's a real word
		cleanWord := ra.cleanWord(word)
		if len(cleanWord) > 0 {
			wordCount++
		}
	}
	
	return wordCount
}

func (ra *ReadabilityAnalyzer) countSentences(text string) int {
	// Count sentences by looking for sentence-ending punctuation
	sentenceEnders := regexp.MustCompile(`[.!?]+`)
	sentences := sentenceEnders.Split(text, -1)
	
	// Count non-empty sentences
	count := 0
	for _, sentence := range sentences {
		if len(strings.TrimSpace(sentence)) > 0 {
			count++
		}
	}
	
	// Ensure at least 1 sentence if there's content
	if count == 0 && len(strings.TrimSpace(text)) > 0 {
		count = 1
	}
	
	return count
}

func (ra *ReadabilityAnalyzer) countSyllables(text string) int {
	words := strings.Fields(text)
	totalSyllables := 0
	
	for _, word := range words {
		syllables := ra.countSyllablesInWord(ra.cleanWord(word))
		totalSyllables += syllables
	}
	
	return totalSyllables
}

func (ra *ReadabilityAnalyzer) countSyllablesInWord(word string) int {
	if len(word) == 0 {
		return 0
	}
	
	word = strings.ToLower(word)
	
	// Count vowel groups
	vowelGroups := 0
	prevWasVowel := false
	
	for _, char := range word {
		isVowel := strings.ContainsRune("aeiouy", char)
		if isVowel && !prevWasVowel {
			vowelGroups++
		}
		prevWasVowel = isVowel
	}
	
	// Adjust for common patterns
	if strings.HasSuffix(word, "e") {
		vowelGroups-- // Silent 'e'
	}
	
	if strings.HasSuffix(word, "le") && len(word) > 2 {
		if !strings.ContainsRune("aeiouy", rune(word[len(word)-3])) {
			vowelGroups++ // Words ending in consonant + 'le'
		}
	}
	
	// Ensure at least 1 syllable
	if vowelGroups <= 0 {
		vowelGroups = 1
	}
	
	return vowelGroups
}

func (ra *ReadabilityAnalyzer) countComplexWords(text string) int {
	words := strings.Fields(text)
	complexCount := 0
	
	for _, word := range words {
		cleanWord := ra.cleanWord(word)
		syllables := ra.countSyllablesInWord(cleanWord)
		
		// Words with 3+ syllables are considered complex
		// With some exceptions for common words
		if syllables >= 3 && !ra.isCommonComplexWord(cleanWord) {
			complexCount++
		}
	}
	
	return complexCount
}

func (ra *ReadabilityAnalyzer) countCharacters(text string) int {
	// Count letters and digits only
	count := 0
	for _, char := range text {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			count++
		}
	}
	return count
}

// Readability formula implementations

func (ra *ReadabilityAnalyzer) calculateFleschKincaidGrade(metrics *ReadabilityMetrics) float64 {
	if metrics.SentenceCount == 0 || metrics.WordCount == 0 {
		return 0.0
	}
	
	awl := metrics.AverageWordsPerSentence
	asw := metrics.AverageSyllablesPerWord
	
	return 0.39*awl + 11.8*asw - 15.59
}

func (ra *ReadabilityAnalyzer) calculateFleschReadingEase(metrics *ReadabilityMetrics) float64 {
	if metrics.SentenceCount == 0 || metrics.WordCount == 0 {
		return 0.0
	}
	
	awl := metrics.AverageWordsPerSentence
	asw := metrics.AverageSyllablesPerWord
	
	return 206.835 - 1.015*awl - 84.6*asw
}

func (ra *ReadabilityAnalyzer) calculateGunningFogIndex(metrics *ReadabilityMetrics) float64 {
	if metrics.SentenceCount == 0 || metrics.WordCount == 0 {
		return 0.0
	}
	
	awl := metrics.AverageWordsPerSentence
	pcw := metrics.PercentageComplexWords
	
	return 0.4 * (awl + pcw)
}

func (ra *ReadabilityAnalyzer) calculateSMOGIndex(metrics *ReadabilityMetrics) float64 {
	if metrics.SentenceCount == 0 {
		return 0.0
	}
	
	// SMOG formula: 1.043 * sqrt(complex_words * (30 / sentences)) + 3.1291
	complexWordsPer30Sentences := float64(metrics.ComplexWordCount) * (30.0 / float64(metrics.SentenceCount))
	
	return 1.043*math.Sqrt(complexWordsPer30Sentences) + 3.1291
}

func (ra *ReadabilityAnalyzer) calculateARI(metrics *ReadabilityMetrics) float64 {
	if metrics.SentenceCount == 0 || metrics.WordCount == 0 {
		return 0.0
	}
	
	charactersPerWord := float64(metrics.CharacterCount) / float64(metrics.WordCount)
	wordsPerSentence := float64(metrics.WordCount) / float64(metrics.SentenceCount)
	
	return 4.71*charactersPerWord + 0.5*wordsPerSentence - 21.43
}

// Overall assessment and recommendations

func (ra *ReadabilityAnalyzer) calculateOverallScore(metrics *ReadabilityMetrics) float64 {
	scores := []float64{}
	
	// Convert grade levels to scores (lower grade level = higher score)
	if ra.config.EnableFleschKincaid {
		// Convert Flesch-Kincaid grade to score (invert and normalize)
		fkScore := math.Max(0, 100-metrics.FleschKincaidGrade*5)
		scores = append(scores, fkScore)
	}
	
	if metrics.FleschReadingEase > 0 {
		// Flesch Reading Ease is already a score (0-100)
		scores = append(scores, metrics.FleschReadingEase)
	}
	
	if ra.config.EnableGunningFog {
		// Convert Gunning Fog to score
		gfScore := math.Max(0, 100-metrics.GunningFogIndex*5)
		scores = append(scores, gfScore)
	}
	
	if ra.config.EnableSMOG {
		// Convert SMOG to score
		smogScore := math.Max(0, 100-metrics.SMOGIndex*5)
		scores = append(scores, smogScore)
	}
	
	if ra.config.EnableARI {
		// Convert ARI to score
		ariScore := math.Max(0, 100-metrics.AutomatedReadabilityIndex*5)
		scores = append(scores, ariScore)
	}
	
	// Calculate average
	if len(scores) == 0 {
		return 50.0 // Default neutral score
	}
	
	total := 0.0
	for _, score := range scores {
		total += score
	}
	
	return total / float64(len(scores))
}

func (ra *ReadabilityAnalyzer) determineReadabilityLevel(overallScore float64) string {
	switch {
	case overallScore >= 90:
		return "very easy"
	case overallScore >= 80:
		return "easy"
	case overallScore >= 70:
		return "fairly easy"
	case overallScore >= 60:
		return "standard"
	case overallScore >= 50:
		return "fairly difficult"
	case overallScore >= 30:
		return "difficult"
	default:
		return "very difficult"
	}
}

func (ra *ReadabilityAnalyzer) generateRecommendations(metrics *ReadabilityMetrics) []string {
	var recommendations []string
	
	// Check sentence length
	if metrics.AverageWordsPerSentence > 20 {
		recommendations = append(recommendations, "Consider shortening sentences (average words per sentence is high)")
	}
	
	// Check complex words
	if metrics.PercentageComplexWords > 15 {
		recommendations = append(recommendations, "Consider simplifying vocabulary (high percentage of complex words)")
	}
	
	// Check syllable density
	if metrics.AverageSyllablesPerWord > 1.7 {
		recommendations = append(recommendations, "Consider using shorter words when possible")
	}
	
	// Check overall grade level
	if metrics.FleschKincaidGrade > ra.config.TargetReadabilityMax {
		recommendations = append(recommendations, "Content may be too difficult for target audience")
	}
	
	if metrics.FleschKincaidGrade < ra.config.TargetReadabilityMin {
		recommendations = append(recommendations, "Content may be overly simplified")
	}
	
	// Check content length
	if metrics.WordCount < 100 {
		recommendations = append(recommendations, "Content is quite short - consider expanding")
	}
	
	if metrics.SentenceCount < 3 {
		recommendations = append(recommendations, "Consider breaking content into more sentences")
	}
	
	// Positive feedback for good scores
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Content readability is well-balanced")
	}
	
	return recommendations
}

// Helper methods

func (ra *ReadabilityAnalyzer) cleanWord(word string) string {
	// Remove punctuation and convert to lowercase
	cleaned := strings.ToLower(word)
	
	// Remove common punctuation
	punctuation := `.,;:!?'"()[]{}—–-`
	for _, p := range punctuation {
		cleaned = strings.ReplaceAll(cleaned, string(p), "")
	}
	
	return strings.TrimSpace(cleaned)
}

func (ra *ReadabilityAnalyzer) isCommonComplexWord(word string) bool {
	// List of common complex words that shouldn't count against readability
	commonComplex := map[string]bool{
		"however":     true,
		"because":     true,
		"important":   true,
		"different":   true,
		"available":   true,
		"probably":    true,
		"understand":  true,
		"remember":    true,
		"everything":  true,
		"computer":    true,
		"technology":  true,
		"information": true,
		"business":    true,
		"government":  true,
		"community":   true,
		"university":  true,
		"education":   true,
		"development": true,
		"management":  true,
		"company":     true,
	}
	
	return commonComplex[word]
}

// GetReadabilityLevel returns a human-readable assessment of readability
func (ra *ReadabilityAnalyzer) GetReadabilityLevel(score float64) string {
	return ra.determineReadabilityLevel(score)
}

// IsTargetReadability checks if the content meets target readability requirements
func (ra *ReadabilityAnalyzer) IsTargetReadability(text string) bool {
	metrics := ra.AnalyzeReadability(text)
	return metrics.FleschKincaidGrade >= ra.config.TargetReadabilityMin && 
		   metrics.FleschKincaidGrade <= ra.config.TargetReadabilityMax
}
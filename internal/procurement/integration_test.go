package procurement_test

import (
	"context"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/Caia-Tech/caia-library/internal/procurement/synthetic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyntheticGenerationIntegration(t *testing.T) {
	// Create providers
	gpt4Provider := synthetic.NewGPT4Provider("test-api-key", "gpt-4")
	claudeProvider := synthetic.NewClaudeProvider("test-api-key", "claude-3")
	
	// Create quality validator
	validator := quality.NewQualityValidator(nil)
	
	// Create generation request
	request := &procurement.GenerationRequest{
		ID: "test-request-001",
		Topic: &procurement.Topic{
			ID:         "topic-001",
			Name:       "Machine Learning Fundamentals",
			Domain:     "technology",
			Keywords:   []string{"machine learning", "AI", "algorithms", "neural networks"},
			Priority:   0.8,
			Difficulty: "intermediate",
			Context:    "Educational content for computer science students",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		ContentType: procurement.ContentTypeTutorial,
		Priority:    "high",
		RequestedBy: "test-system",
		CreatedAt:   time.Now(),
	}
	
	t.Run("GPT-4 Content Generation", func(t *testing.T) {
		ctx := context.Background()
		
		result, err := gpt4Provider.Generate(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, request.ID, result.RequestID)
		assert.True(t, result.Success)
		assert.NotNil(t, result.Document)
		assert.NotEmpty(t, result.Document.Content.Text)
		
		// Verify document metadata
		assert.Equal(t, "synthetic", result.Document.Source.Type)
		assert.Equal(t, request.Topic.Name, result.Document.Content.Metadata["topic"])
		assert.Equal(t, string(request.ContentType), result.Document.Content.Metadata["content_type"])
	})
	
	t.Run("Claude Content Generation", func(t *testing.T) {
		ctx := context.Background()
		
		result, err := claudeProvider.Generate(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, request.ID, result.RequestID)
		assert.True(t, result.Success)
		assert.NotNil(t, result.Document)
		assert.NotEmpty(t, result.Document.Content.Text)
	})
	
	t.Run("Quality Validation", func(t *testing.T) {
		ctx := context.Background()
		
		// Generate content first
		result, err := gpt4Provider.Generate(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result.Document)
		
		// Validate the content
		validationResult, err := validator.ValidateContent(ctx, result.Document.Content.Text, result.Document.Content.Metadata)
		require.NoError(t, err)
		assert.NotNil(t, validationResult)
		
		// Check validation results
		assert.True(t, validationResult.OverallScore >= 0.0 && validationResult.OverallScore <= 1.0)
		assert.NotEmpty(t, validationResult.QualityTier)
		assert.True(t, validationResult.ConfidenceLevel >= 0.0 && validationResult.ConfidenceLevel <= 1.0)
		assert.NotNil(t, validationResult.DimensionScores)
		
		// Check dimension scores
		expectedDimensions := []string{"accuracy", "completeness", "relevance", "quality", "uniqueness"}
		for _, dimension := range expectedDimensions {
			score, exists := validationResult.DimensionScores[dimension]
			assert.True(t, exists, "Missing dimension: %s", dimension)
			assert.True(t, score >= 0.0 && score <= 1.0, "Invalid score for %s: %f", dimension, score)
		}
	})
	
	t.Run("Provider Capabilities", func(t *testing.T) {
		gpt4Capabilities := gpt4Provider.GetCapabilities()
		claudeCapabilities := claudeProvider.GetCapabilities()
		
		assert.NotEmpty(t, gpt4Capabilities)
		assert.NotEmpty(t, claudeCapabilities)
		
		// Both should support general writing
		assert.Contains(t, gpt4Capabilities, "general_writing")
		assert.Contains(t, claudeCapabilities, "general_writing")
	})
	
	t.Run("Cost Estimation", func(t *testing.T) {
		gpt4Cost := gpt4Provider.EstimateCost(request)
		claudeCost := claudeProvider.EstimateCost(request)
		
		assert.True(t, gpt4Cost > 0)
		assert.True(t, claudeCost > 0)
	})
	
	t.Run("Provider Availability", func(t *testing.T) {
		assert.True(t, gpt4Provider.IsAvailable())
		assert.True(t, claudeProvider.IsAvailable())
	})
}

func TestQualityValidationComponents(t *testing.T) {
	validator := quality.NewQualityValidator(nil)
	
	t.Run("Code Validation", func(t *testing.T) {
		ctx := context.Background()
		
		// Test valid Go code
		goCode := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}`
		
		result, err := validator.ValidateCode(ctx, goCode, "go")
		require.NoError(t, err)
		assert.True(t, result.SyntaxValid)
		assert.True(t, result.SecuritySafe)
		assert.Equal(t, "go", result.Language)
		
		// Test invalid code - missing closing parenthesis and import
		invalidCode := `package main
func main() {
    fmt.Println("Missing import"
}` 
		
		result, err = validator.ValidateCode(ctx, invalidCode, "go")
		require.NoError(t, err)
		
		// Should fail syntax validation due to missing parenthesis
		t.Logf("SyntaxValid: %v, Compilable: %v, Errors: %v", result.SyntaxValid, result.Compilable, result.Errors)
		assert.False(t, result.SyntaxValid)
		assert.NotEmpty(t, result.Errors)
	})
	
	t.Run("Math Validation", func(t *testing.T) {
		ctx := context.Background()
		
		// Test valid mathematical formula
		validFormula := "x^2 + 2x + 1 = (x + 1)^2"
		result, err := validator.ValidateMath(ctx, validFormula)
		require.NoError(t, err)
		assert.True(t, result.Valid)
		assert.True(t, result.Confidence > 0.5)
		
		// Test invalid formula
		invalidFormula := "1 / 0"
		result, err = validator.ValidateMath(ctx, invalidFormula)
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Explanation, "Division by zero")
	})
	
	t.Run("Fact Checking", func(t *testing.T) {
		ctx := context.Background()
		
		topic := &procurement.Topic{
			Domain: "technology",
			Name:   "Machine Learning",
		}
		
		content := "Machine learning is a subset of artificial intelligence. Research shows that neural networks can learn complex patterns from data."
		
		results, err := validator.FactCheck(ctx, content, topic)
		require.NoError(t, err)
		assert.NotEmpty(t, results)
		
		for _, result := range results {
			assert.NotEmpty(t, result.Claim)
			assert.True(t, result.Confidence >= 0.0 && result.Confidence <= 1.0)
		}
	})
}

func TestReadabilityAnalysis(t *testing.T) {
	analyzer := quality.NewReadabilityAnalyzer()
	
	t.Run("Simple Text Analysis", func(t *testing.T) {
		text := "This is a simple test. It has short sentences. The words are easy to understand."
		
		score := analyzer.CalculateReadabilityScore(text)
		assert.True(t, score >= 0.0 && score <= 100.0)
		
		metrics := analyzer.AnalyzeReadability(text)
		assert.NotNil(t, metrics)
		assert.True(t, metrics.WordCount > 0)
		assert.True(t, metrics.SentenceCount > 0)
		assert.NotEmpty(t, metrics.ReadabilityLevel)
	})
	
	t.Run("Complex Text Analysis", func(t *testing.T) {
		complexText := "The implementation of sophisticated machine learning algorithms necessitates comprehensive understanding of mathematical foundations, statistical methodologies, and computational complexity theory."
		
		score := analyzer.CalculateReadabilityScore(complexText)
		assert.True(t, score >= 0.0 && score <= 100.0)
		
		metrics := analyzer.AnalyzeReadability(complexText)
		assert.True(t, metrics.ComplexWordCount > 0)
		assert.True(t, metrics.PercentageComplexWords > 0)
	})
	
	t.Run("Empty Text", func(t *testing.T) {
		score := analyzer.CalculateReadabilityScore("")
		assert.Equal(t, 0.0, score)
		
		metrics := analyzer.AnalyzeReadability("")
		assert.Equal(t, "unreadable", metrics.ReadabilityLevel)
		assert.Contains(t, metrics.Recommendations[0], "empty")
	})
}

func BenchmarkContentGeneration(b *testing.B) {
	provider := synthetic.NewGPT4Provider("test-key", "gpt-4")
	
	request := &procurement.GenerationRequest{
		ID: "bench-request",
		Topic: &procurement.Topic{
			Name:     "Test Topic",
			Domain:   "technology",
			Keywords: []string{"test", "benchmark"},
		},
		ContentType: procurement.ContentTypeGeneral,
		CreatedAt:   time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := provider.Generate(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQualityValidation(b *testing.B) {
	validator := quality.NewQualityValidator(nil)
	
	text := "# Machine Learning Tutorial\n\n## Introduction\nMachine learning is a method of data analysis that automates analytical model building. It is a branch of artificial intelligence based on the idea that systems can learn from data, identify patterns and make decisions with minimal human intervention.\n\n## Key Concepts\n- **Supervised Learning**: Learning with labeled examples\n- **Unsupervised Learning**: Finding patterns in data without labels\n- **Reinforcement Learning**: Learning through interaction and feedback\n\n## Example Implementation\nHere's a simple example of linear regression:\n\n```python\nimport numpy as np\nfrom sklearn.linear_model import LinearRegression\n\n# Sample data\nX = np.array([[1], [2], [3], [4]])\ny = np.array([2, 4, 6, 8])\n\n# Create and train model\nmodel = LinearRegression()\nmodel.fit(X, y)\n\n# Make predictions\npredictions = model.predict([[5]])\nprint(predictions)  # Output: [10]\n```\n\nThis demonstrates the basic workflow of machine learning: prepare data, train model, make predictions."
	
	metadata := map[string]string{
		"title":        "Machine Learning Tutorial",
		"topic":        "Machine Learning", 
		"content_type": "tutorial",
		"domain":       "technology",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := validator.ValidateContent(ctx, text, metadata)
		if err != nil {
			b.Fatal(err)
		}
	}
}
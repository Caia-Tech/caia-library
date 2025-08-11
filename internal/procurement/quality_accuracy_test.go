package procurement_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityScoreAccuracy(t *testing.T) {
	validator := quality.NewQualityValidator(nil)
	require.NotNil(t, validator)

	t.Run("High Quality Content", func(t *testing.T) {
		highQualityContent := generateHighQualityContent()
		metadata := map[string]string{
			"type":     "article",
			"domain":   "technology",
			"language": "en",
		}

		result, err := validator.ValidateContent(context.Background(), highQualityContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// High quality content should score well
		assert.True(t, result.OverallScore >= 0.7, 
			"High quality content scored %.2f, expected >= 0.7", result.OverallScore)
		
		t.Logf("High quality content score: %.3f", result.OverallScore)
		t.Logf("Quality tier: %s, Confidence: %.2f", result.QualityTier, result.ConfidenceLevel)
		if len(result.DimensionScores) > 0 {
			t.Logf("Dimension scores: %+v", result.DimensionScores)
		}
	})

	t.Run("Technical Documentation", func(t *testing.T) {
		techContent := generateTechnicalDocumentation()
		metadata := map[string]string{
			"type":     "documentation",
			"domain":   "programming",
			"language": "en",
		}

		result, err := validator.ValidateContent(context.Background(), techContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Technical documentation should score reasonably well
		assert.True(t, result.OverallScore >= 0.6, 
			"Technical content scored %.2f, expected >= 0.6", result.OverallScore)
		
		t.Logf("Technical documentation score: %.3f", result.OverallScore)
	})

	t.Run("Code Examples", func(t *testing.T) {
		codeContent := generateCodeExampleContent()
		metadata := map[string]string{
			"type":     "code",
			"domain":   "programming",
			"language": "go",
		}

		result, err := validator.ValidateContent(context.Background(), codeContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Code content should be validated properly
		t.Logf("Code example score: %.3f", result.OverallScore)
		
		// Code might score differently due to different validation criteria
		assert.True(t, result.OverallScore >= 0.4, 
			"Code content scored %.2f, should be reasonable", result.OverallScore)
	})

	t.Run("Educational Content", func(t *testing.T) {
		educationalContent := generateEducationalContent()
		metadata := map[string]string{
			"type":     "tutorial",
			"domain":   "education",
			"language": "en",
		}

		result, err := validator.ValidateContent(context.Background(), educationalContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Educational content should score well
		assert.True(t, result.OverallScore >= 0.6, 
			"Educational content scored %.2f, expected >= 0.6", result.OverallScore)
		
		t.Logf("Educational content score: %.3f", result.OverallScore)
	})

	t.Run("Scientific Content", func(t *testing.T) {
		scientificContent := generateScientificContent()
		metadata := map[string]string{
			"type":     "research",
			"domain":   "science",
			"language": "en",
		}

		result, err := validator.ValidateContent(context.Background(), scientificContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Scientific content should score very well due to structured format
		assert.True(t, result.OverallScore >= 0.7, 
			"Scientific content scored %.2f, expected >= 0.7", result.OverallScore)
		
		t.Logf("Scientific content score: %.3f", result.OverallScore)
	})

	t.Run("Mediocre Content", func(t *testing.T) {
		mediocreContent := generateMediocreContent()
		metadata := map[string]string{
			"type":     "article",
			"domain":   "general",
			"language": "en",
		}

		result, err := validator.ValidateContent(context.Background(), mediocreContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Mediocre content should score in the middle range
		assert.True(t, result.OverallScore >= 0.3 && result.OverallScore <= 0.7, 
			"Mediocre content scored %.2f, expected 0.3-0.7", result.OverallScore)
		
		t.Logf("Mediocre content score: %.3f", result.OverallScore)
	})

	t.Run("Content with Errors", func(t *testing.T) {
		errorContent := generateContentWithErrors()
		metadata := map[string]string{
			"type":     "code",
			"domain":   "programming",
			"language": "go",
		}

		result, err := validator.ValidateContent(context.Background(), errorContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Content with errors should score lower
		assert.True(t, result.OverallScore <= 0.6, 
			"Erroneous content scored %.2f, expected <= 0.6", result.OverallScore)
		
		t.Logf("Content with errors score: %.3f", result.OverallScore)
	})

	t.Run("Repetitive Content", func(t *testing.T) {
		repetitiveContent := generateRepetitiveContent()
		metadata := map[string]string{
			"type":     "article",
			"domain":   "general",
			"language": "en",
		}

		result, err := validator.ValidateContent(context.Background(), repetitiveContent, metadata)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Repetitive content should score lower
		t.Logf("Repetitive content score: %.3f", result.OverallScore)
		t.Logf("Quality tier: %s", result.QualityTier)
		
		// Repetitive content should generally score lower
		assert.True(t, result.OverallScore <= 0.8, 
			"Repetitive content overall score %.2f should be <= 0.8", result.OverallScore)
	})
}

func generateHighQualityContent() string {
	return `
# Understanding Distributed Systems Architecture

Distributed systems are a fundamental aspect of modern software architecture, enabling applications to scale across multiple machines while maintaining reliability and performance. This comprehensive guide explores the core principles, patterns, and best practices for designing effective distributed systems.

## Introduction to Distributed Systems

A distributed system is a collection of independent computers that appears to its users as a single coherent system. The key characteristics that define distributed systems include:

- **Scalability**: The ability to handle increased workload by adding resources to the system
- **Reliability**: Continued operation despite failures in individual components
- **Availability**: The system remains operational over time
- **Consistency**: All nodes see the same data at the same time

## Core Architectural Patterns

### Microservices Architecture

Microservices decompose applications into small, independently deployable services. Each service:
- Focuses on a specific business capability
- Communicates through well-defined APIs
- Can be developed and deployed independently
- Uses its own data storage

### Event-Driven Architecture

Event-driven systems communicate through events, enabling loose coupling and high scalability. Key components include:
- Event producers that generate events
- Event brokers that route events
- Event consumers that process events

## Consistency Models

Distributed systems must choose between different consistency models:

1. **Strong Consistency**: All nodes see the same data simultaneously
2. **Eventual Consistency**: Nodes will eventually converge to the same state
3. **Weak Consistency**: No guarantees about when all nodes will be consistent

## Best Practices

To build robust distributed systems, consider these practices:

- Implement proper error handling and retry mechanisms
- Use circuit breakers to prevent cascade failures
- Design for idempotency to handle duplicate requests
- Monitor system health with comprehensive observability
- Plan for disaster recovery and backup strategies

## Conclusion

Distributed systems present unique challenges but offer significant benefits in terms of scalability and resilience. Understanding these fundamentals is essential for architects building modern applications that can handle the demands of today's digital landscape.

The key to success lies in making informed trade-offs between consistency, availability, and partition tolerance while implementing proper monitoring and operational practices.
`
}

func generateTechnicalDocumentation() string {
	return `
# HTTP Client Configuration Guide

## Overview

The HTTP client library provides configurable options for making HTTP requests with proper timeout handling, retry logic, and connection pooling.

## Basic Configuration

### Creating a Client

'''go
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
'''

### Making Requests

'''go
resp, err := client.Get("https://api.example.com/data")
if err != nil {
    return fmt.Errorf("request failed: %w", err)
}
defer resp.Body.Close()
'''

## Advanced Features

### Retry Logic

The client supports automatic retries with exponential backoff:

- Initial delay: 100ms
- Maximum delay: 30s
- Maximum attempts: 3

### Connection Pooling

Connection pools improve performance by reusing TCP connections:

- Maximum idle connections: 100
- Maximum idle connections per host: 10
- Idle connection timeout: 90 seconds

## Error Handling

Common error scenarios and their handling:

1. **Timeout Errors**: Increase timeout or implement retry logic
2. **Connection Errors**: Check network connectivity
3. **HTTP Errors**: Handle based on status code

## Configuration Examples

### Development Environment

'''go
config := ClientConfig{
    Timeout:     10 * time.Second,
    MaxRetries:  2,
    EnableDebug: true,
}
'''

### Production Environment

'''go
config := ClientConfig{
    Timeout:         30 * time.Second,
    MaxRetries:      3,
    EnableMetrics:   true,
    ConnectionPool:  100,
}
'''

This configuration provides reliable HTTP communication for production workloads while maintaining good performance characteristics.
`
}

func generateCodeExampleContent() string {
	return `
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"
)

// HTTPClient represents a configurable HTTP client
type HTTPClient struct {
    client  *http.Client
    baseURL string
    timeout time.Duration
}

// NewHTTPClient creates a new HTTP client with the specified configuration
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
    return &HTTPClient{
        client: &http.Client{
            Timeout: timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
        baseURL: baseURL,
        timeout: timeout,
    }
}

// Get performs a GET request to the specified endpoint
func (c *HTTPClient) Get(ctx context.Context, endpoint string) (*http.Response, error) {
    url := fmt.Sprintf("%s%s", c.baseURL, endpoint)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    
    req.Header.Set("User-Agent", "HTTPClient/1.0")
    req.Header.Set("Accept", "application/json")
    
    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("executing request: %w", err)
    }
    
    if resp.StatusCode >= 400 {
        resp.Body.Close()
        return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
    }
    
    return resp, nil
}

// Close closes the HTTP client and cleans up resources
func (c *HTTPClient) Close() {
    c.client.CloseIdleConnections()
}

func main() {
    client := NewHTTPClient("https://api.example.com", 30*time.Second)
    defer client.Close()
    
    ctx := context.Background()
    resp, err := client.Get(ctx, "/users/123")
    if err != nil {
        log.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()
    
    fmt.Printf("Response status: %s\n", resp.Status)
}
`
}

func generateEducationalContent() string {
	return `
# Introduction to Machine Learning: A Beginner's Guide

Machine learning is a subset of artificial intelligence that enables computers to learn and improve from experience without being explicitly programmed. This tutorial will introduce you to the fundamental concepts and help you understand how machine learning works.

## What is Machine Learning?

Machine learning algorithms build mathematical models based on training data to make predictions or decisions. Instead of following pre-programmed instructions, these systems learn patterns from data.

## Types of Machine Learning

### 1. Supervised Learning

In supervised learning, algorithms learn from labeled training data. Examples include:

- **Classification**: Predicting categories (email spam detection)
- **Regression**: Predicting continuous values (house price prediction)

### 2. Unsupervised Learning

Unsupervised learning finds patterns in data without labeled examples:

- **Clustering**: Grouping similar data points
- **Dimensionality Reduction**: Simplifying data while preserving information

### 3. Reinforcement Learning

Agents learn through interaction with an environment, receiving rewards or penalties for actions.

## Key Concepts

### Training and Testing

- **Training Set**: Data used to teach the algorithm
- **Test Set**: Data used to evaluate performance
- **Validation Set**: Data used for model selection and hyperparameter tuning

### Overfitting and Underfitting

- **Overfitting**: Model memorizes training data but fails on new data
- **Underfitting**: Model is too simple to capture underlying patterns

### Feature Engineering

The process of selecting and transforming variables for your model:

1. Feature selection: Choose relevant variables
2. Feature scaling: Normalize data ranges
3. Feature creation: Derive new variables from existing ones

## Common Algorithms

### Linear Regression

Finds the best line through data points to predict continuous values.

### Decision Trees

Creates a tree-like model of decisions and their possible consequences.

### Neural Networks

Inspired by biological neurons, these networks can learn complex patterns.

## Getting Started

To begin your machine learning journey:

1. Learn basic statistics and probability
2. Master a programming language (Python or R)
3. Study linear algebra and calculus fundamentals
4. Practice with datasets from Kaggle or UCI repository
5. Take online courses or tutorials

## Best Practices

- Start with simple algorithms before moving to complex ones
- Always validate your model on unseen data
- Understand your data through exploration and visualization
- Consider ethical implications of your models
- Document your process and results clearly

Machine learning is a powerful tool that can solve many real-world problems. With practice and patience, you can develop the skills needed to build effective machine learning solutions.
`
}

func generateScientificContent() string {
	return `
# The Impact of Climate Change on Arctic Sea Ice: A Comprehensive Analysis

## Abstract

This study examines the relationship between global climate change and Arctic sea ice extent from 1979 to 2024. Using satellite observations and climate models, we quantify the rate of sea ice decline and its implications for global climate systems. Our findings indicate a statistically significant decline of 13.1% per decade in September sea ice extent, with accelerating trends in recent years.

## Introduction

Arctic sea ice plays a crucial role in global climate regulation through its high albedo effect, reflecting solar radiation back to space. The Arctic region has experienced warming at twice the global average rate, a phenomenon known as Arctic amplification. Understanding the dynamics of sea ice loss is essential for predicting future climate scenarios and their socioeconomic impacts.

## Methodology

### Data Sources

Our analysis utilized the following datasets:

1. **Sea Ice Concentration**: NSIDC Sea Ice Index, providing daily sea ice extent from 1979-2024
2. **Temperature Data**: ERA5 reanalysis data for Arctic surface temperatures
3. **Climate Models**: CMIP6 ensemble predictions for future projections

### Statistical Analysis

We employed multiple linear regression models to analyze trends:

- **Dependent Variable**: Monthly sea ice extent (10⁶ km²)
- **Independent Variables**: Time, Arctic Oscillation Index, global temperature anomaly
- **Statistical Tests**: Mann-Kendall trend test, Sen's slope estimator

### Quality Control

Data underwent rigorous quality control procedures:
- Removal of anomalous values beyond 3 standard deviations
- Interpolation of missing values using cubic spline methods
- Cross-validation with independent observational datasets

## Results

### Historical Trends

The analysis reveals significant declining trends in Arctic sea ice:

- **September Minimum**: -13.1% per decade (p < 0.001)
- **March Maximum**: -2.8% per decade (p < 0.01)
- **Annual Average**: -5.4% per decade (p < 0.001)

### Spatial Patterns

Regional analysis shows heterogeneous patterns of ice loss:
- Beaufort Sea: Highest rate of decline (-18.2% per decade)
- Central Arctic: Moderate decline (-8.7% per decade)
- Canadian Archipelago: Minimal change (-1.3% per decade)

### Seasonal Variations

Ice loss exhibits strong seasonal dependence:
- Summer months show accelerated decline rates
- Winter recovery periods are becoming shorter
- First-year ice increasingly dominates total ice coverage

## Discussion

### Physical Mechanisms

Several feedback mechanisms contribute to observed trends:

1. **Albedo Feedback**: Reduced ice cover exposes dark ocean water, increasing heat absorption
2. **Ice-Albedo Feedback**: Thinner ice melts more easily, creating self-reinforcing cycle
3. **Atmospheric Heat Transport**: Enhanced meridional heat transport accelerates melting

### Climate Implications

Arctic sea ice loss has far-reaching consequences:
- Disruption of thermohaline circulation patterns
- Changes in regional weather patterns and storm tracks
- Acceleration of Greenland ice sheet melting
- Rising global sea levels

### Model Projections

CMIP6 models project continued ice loss under all emission scenarios:
- **Low Emissions (SSP1-2.6)**: Ice-free September by 2080
- **High Emissions (SSP5-8.5)**: Ice-free September by 2050
- **Uncertainty Range**: ±10 years depending on natural variability

## Limitations

This study has several limitations:
- Satellite record spans only 45 years
- Model uncertainties in cloud physics and precipitation
- Limited understanding of ice-ocean interaction processes

## Conclusions

Arctic sea ice extent has declined significantly over the satellite era, with accelerating trends in recent decades. The observed changes are consistent with climate model predictions and attributable to anthropogenic greenhouse gas emissions. Continued monitoring and improved process understanding are essential for reducing projection uncertainties.

These findings have important implications for climate policy, Arctic ecosystems, and indigenous communities dependent on sea ice. Immediate action on greenhouse gas emissions is necessary to limit future ice loss and its cascading effects on global climate systems.

## References

[Note: In a real scientific paper, this would contain actual citations to peer-reviewed literature]

## Acknowledgments

We thank the National Snow and Ice Data Center for providing sea ice data and the European Centre for Medium-Range Weather Forecasts for ERA5 reanalysis data. This research was supported by grants from the National Science Foundation and NASA.
`
}

func generateMediocreContent() string {
	return `
# Some Thoughts About Technology

Technology is everywhere these days. People use computers and phones all the time. It's pretty interesting how much things have changed over the years. I remember when we didn't have smartphones and had to use regular phones.

## The Internet

The internet is really useful. You can find information about almost anything. There are websites for news, shopping, social media, and entertainment. Many people spend hours online every day browsing different sites.

## Computers

Computers have gotten much faster over time. They used to be really slow and had limited memory. Now they can do many things at once and store lots of files. Most people have laptops or desktop computers at home.

## Mobile Devices

Smartphones are popular because you can carry them anywhere. They have cameras, games, apps, and internet access. Tablets are also common for reading and watching videos. These devices have touchscreens which makes them easy to use.

## Social Media

Social media platforms like Facebook, Twitter, and Instagram let people share photos and updates with friends. Some people post frequently while others just browse what others share. It's a way to stay connected with people you know.

## Online Shopping

You can buy almost anything online now. Websites like Amazon have millions of products. You just click what you want and it gets delivered to your house. This is convenient but some people still prefer shopping in stores.

## Gaming

Video games have improved a lot. The graphics are much better than before. People can play games on computers, consoles, phones, and tablets. Online games let you play with other people from around the world.

## Conclusion

Technology keeps changing and improving. It affects how we work, communicate, and spend our free time. Some changes are good while others might not be. It's important to use technology in moderation and not let it take over your life completely.

These are just some general observations about technology today. Different people have different opinions about how much technology they want to use in their daily lives.
`
}

func generateContentWithErrors() string {
	return `
package main

import (
    "fmt"
    "os
    "strconv"
)

func main() {
    // This function has several errors
    var x int = 5
    var y string = "10"
    
    // This will cause a runtime error
    result := x + y  // Cannot add int and string
    
    // Missing error handling
    num, _ := strconv.Atoi("not a number")
    
    // Division by zero
    division := 10 / 0
    
    // Array out of bounds
    arr := []int{1, 2, 3}
    value := arr[5]  // Index out of range
    
    // Unused variable
    unused := "this variable is not used"
    
    // Missing return statement
    func calculate() int {
        x := 5
        // Missing return
    }
    
    // Incorrect syntax
    if x = 5 {  // Should use == for comparison
        fmt.Println("x is 5")
    }
    
    // Memory leak potential
    for {
        data := make([]byte, 1000000)
        // data is never freed, potential memory leak
    }
    
    // Improper error handling
    file, err := os.Open("nonexistent.txt")
    fmt.Println(file)  // Using file without checking error
    
    fmt.Println(result, num, division, value)
}

// Function with logical error
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    // This implementation is inefficient and has exponential time complexity
    return fibonacci(n-1) + fibonacci(n-1)  // Should be fibonacci(n-2)
}

// SQL injection vulnerability example
func getUserData(userID string) string {
    query := "SELECT * FROM users WHERE id = '" + userID + "'"  // Vulnerable to SQL injection
    // Should use parameterized queries instead
    return query
}
`
}

func generateRepetitiveContent() string {
	content := `
# Article About Cats

Cats are popular pets that many people enjoy having in their homes. Cats are known for being independent animals that like to sleep during the day. Many cat owners appreciate how cats are relatively low-maintenance pets compared to dogs.

Cats come in many different breeds and colors. Some cats have long hair while other cats have short hair. Indoor cats typically live longer than outdoor cats because they face fewer dangers. Most cats enjoy playing with toys and sleeping in sunny spots.

`
	
	// Repeat the same paragraphs to create repetitive content
	repeated := strings.Repeat(content, 5)
	
	return repeated + `

Cat behavior is interesting to observe. Cats often purr when they are content and happy. Cats also like to scratch things to keep their claws sharp. Understanding cat behavior helps owners provide better care for their pets.

In conclusion, cats make wonderful companions for many people. The popularity of cats as pets continues to grow around the world. Cat ownership brings joy and companionship to millions of households globally.
`
}
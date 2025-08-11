package quality

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/rs/zerolog/log"
)

// MathValidator validates mathematical formulas and expressions
type MathValidator struct {
	config *MathValidationConfig
	parser *MathParser
}

// MathValidationConfig configures math validation behavior
type MathValidationConfig struct {
	EnableSymbolicValidation bool    `json:"enable_symbolic_validation"`
	MaxComplexity           int     `json:"max_complexity"`
	ConfidenceThreshold     float64 `json:"confidence_threshold"`
	SupportedNotations      []string `json:"supported_notations"`
}

// MathParser handles parsing and validation of mathematical expressions
type MathParser struct {
	// In a real implementation, this might use a proper math parsing library
	operators       map[string]int
	functions       map[string]bool
	constants       map[string]float64
	validationRules []ValidationRule
}

// ValidationRule represents a mathematical validation rule
type ValidationRule struct {
	Pattern     *regexp.Regexp
	Description string
	Severity    string
}

// NewMathValidator creates a new math validator
func NewMathValidator() *MathValidator {
	mv := &MathValidator{
		config: &MathValidationConfig{
			EnableSymbolicValidation: true,
			MaxComplexity:           100,
			ConfidenceThreshold:     0.6,
			SupportedNotations:      []string{"latex", "ascii", "unicode"},
		},
		parser: NewMathParser(),
	}
	
	return mv
}

// NewMathParser creates a new math parser
func NewMathParser() *MathParser {
	mp := &MathParser{
		operators: map[string]int{
			"+": 1, "-": 1,
			"*": 2, "/": 2, "%": 2,
			"^": 3, "**": 3,
		},
		functions: map[string]bool{
			"sin": true, "cos": true, "tan": true,
			"asin": true, "acos": true, "atan": true,
			"log": true, "ln": true, "exp": true,
			"sqrt": true, "abs": true,
			"floor": true, "ceil": true, "round": true,
			"max": true, "min": true,
			"sum": true, "prod": true,
		},
		constants: map[string]float64{
			"pi": math.Pi, "π": math.Pi,
			"e": math.E,
			"phi": 1.618033988749,
			"γ": 0.5772156649015329, // Euler-Mascheroni constant
		},
	}
	
	mp.setupValidationRules()
	return mp
}

// ValidateMath validates a mathematical formula or expression
func (mv *MathValidator) ValidateMath(ctx context.Context, formula string) (*procurement.MathValidation, error) {
	start := time.Now()
	
	log.Debug().
		Str("formula", formula).
		Msg("Starting math validation")
	
	result := &procurement.MathValidation{
		Formula: formula,
	}
	
	// Basic validation
	if strings.TrimSpace(formula) == "" {
		result.Valid = false
		result.Confidence = 0.0
		result.Explanation = "Empty formula"
		return result, nil
	}
	
	// Normalize formula
	normalizedFormula := mv.normalizeFormula(formula)
	
	// Detect notation type
	notation := mv.detectNotation(normalizedFormula)
	
	// Validate syntax
	syntaxValid, syntaxErrors := mv.validateSyntax(normalizedFormula, notation)
	
	// Calculate complexity
	complexity := mv.calculateComplexity(normalizedFormula)
	
	// Semantic validation
	semanticValid, semanticErrors := mv.validateSemantics(normalizedFormula)
	
	// Mathematical consistency check
	consistencyValid, consistencyErrors := mv.checkConsistency(normalizedFormula)
	
	// Calculate overall confidence
	confidence := mv.calculateConfidence(syntaxValid, semanticValid, consistencyValid, complexity)
	
	// Determine if formula is valid overall
	valid := syntaxValid && semanticValid && consistencyValid && confidence >= mv.config.ConfidenceThreshold
	
	// Build explanation
	explanation := mv.buildExplanation(syntaxValid, semanticValid, consistencyValid, 
		syntaxErrors, semanticErrors, consistencyErrors, notation, complexity)
	
	result.Valid = valid
	result.Confidence = confidence
	result.Explanation = explanation
	
	log.Debug().
		Str("formula", formula).
		Bool("valid", result.Valid).
		Float64("confidence", result.Confidence).
		Dur("duration", time.Since(start)).
		Msg("Math validation completed")
	
	return result, nil
}

// setupValidationRules initializes mathematical validation rules
func (mp *MathParser) setupValidationRules() {
	mp.validationRules = []ValidationRule{
		{
			Pattern:     regexp.MustCompile(`\b0/0\b`),
			Description: "Division by zero (indeterminate form)",
			Severity:    "error",
		},
		{
			Pattern:     regexp.MustCompile(`\d+\s*/\s*0\b`),
			Description: "Division by zero",
			Severity:    "error",
		},
		{
			Pattern:     regexp.MustCompile(`log\(0\)`),
			Description: "Logarithm of zero (undefined)",
			Severity:    "error",
		},
		{
			Pattern:     regexp.MustCompile(`log\(-\d+\)`),
			Description: "Logarithm of negative number (undefined in real numbers)",
			Severity:    "warning",
		},
		{
			Pattern:     regexp.MustCompile(`sqrt\(-\d+\)`),
			Description: "Square root of negative number (undefined in real numbers)",
			Severity:    "warning",
		},
		{
			Pattern:     regexp.MustCompile(`\(\s*\)`),
			Description: "Empty parentheses",
			Severity:    "error",
		},
		{
			Pattern:     regexp.MustCompile(`[+\-*/^]{2,}`),
			Description: "Consecutive operators",
			Severity:    "error",
		},
	}
}

// normalizeFormula cleans and normalizes the mathematical formula
func (mv *MathValidator) normalizeFormula(formula string) string {
	// Remove extra whitespace
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(formula), " ")
	
	// Convert common Unicode math symbols to ASCII equivalents
	replacements := map[string]string{
		"×": "*", "⋅": "*", "∙": "*",
		"÷": "/", 
		"²": "^2", "³": "^3",
		"√": "sqrt",
		"∞": "inf",
		"±": "+-",
		"≤": "<=", "≥": ">=",
		"≠": "!=", "≈": "≈",
	}
	
	for unicode, ascii := range replacements {
		normalized = strings.ReplaceAll(normalized, unicode, ascii)
	}
	
	return normalized
}

// detectNotation determines the mathematical notation being used
func (mv *MathValidator) detectNotation(formula string) string {
	// Check for LaTeX patterns
	if strings.Contains(formula, "\\") || strings.Contains(formula, "{") || strings.Contains(formula, "}") {
		return "latex"
	}
	
	// Check for Unicode math symbols
	unicodeMath := regexp.MustCompile(`[∑∏∫∞π√±×÷≤≥≠≈]`)
	if unicodeMath.MatchString(formula) {
		return "unicode"
	}
	
	// Default to ASCII
	return "ascii"
}

// validateSyntax checks the syntactic correctness of the formula
func (mv *MathValidator) validateSyntax(formula string, notation string) (bool, []string) {
	var errors []string
	
	// Check for balanced parentheses
	if !mv.checkBalancedParentheses(formula) {
		errors = append(errors, "Unbalanced parentheses")
	}
	
	// Check for proper operator usage
	if !mv.checkOperatorUsage(formula) {
		errors = append(errors, "Invalid operator usage")
	}
	
	// Check for valid function calls
	if !mv.checkFunctionCalls(formula) {
		errors = append(errors, "Invalid function calls")
	}
	
	// Apply validation rules
	for _, rule := range mv.parser.validationRules {
		if rule.Pattern.MatchString(formula) {
			errors = append(errors, rule.Description)
			if rule.Severity == "error" {
				return false, errors
			}
		}
	}
	
	return len(errors) == 0, errors
}

// validateSemantics checks the semantic correctness of the formula
func (mv *MathValidator) validateSemantics(formula string) (bool, []string) {
	var errors []string
	
	// Check for undefined variables (simplified)
	variables := mv.extractVariables(formula)
	for _, variable := range variables {
		if len(variable) > 1 && !mv.isKnownConstant(variable) && !mv.isKnownFunction(variable) {
			// This is a multi-character variable - should be properly defined
			// For now, we'll accept it but note it
		}
	}
	
	// Check for domain restrictions
	domainIssues := mv.checkDomainRestrictions(formula)
	errors = append(errors, domainIssues...)
	
	// Check for dimensional consistency (simplified)
	if !mv.checkDimensionalConsistency(formula) {
		// Note: This is a placeholder for more sophisticated dimensional analysis
	}
	
	return len(errors) == 0, errors
}

// checkConsistency performs consistency checks on the mathematical expression
func (mv *MathValidator) checkConsistency(formula string) (bool, []string) {
	var errors []string
	
	// Check for mathematical inconsistencies
	inconsistencies := mv.findInconsistencies(formula)
	errors = append(errors, inconsistencies...)
	
	// Check for redundancy
	if mv.hasRedundancy(formula) {
		errors = append(errors, "Contains redundant terms")
	}
	
	// Check for proper equation format (if it's an equation)
	if strings.Contains(formula, "=") && !mv.isValidEquation(formula) {
		errors = append(errors, "Invalid equation format")
	}
	
	return len(errors) == 0, errors
}

// calculateComplexity estimates the complexity of the mathematical expression
func (mv *MathValidator) calculateComplexity(formula string) int {
	complexity := 0
	
	// Count operators
	for operator := range mv.parser.operators {
		complexity += strings.Count(formula, operator)
	}
	
	// Count functions
	for function := range mv.parser.functions {
		complexity += strings.Count(formula, function) * 2 // Functions add more complexity
	}
	
	// Count parentheses (nested complexity)
	complexity += strings.Count(formula, "(")
	
	// Count variables and numbers
	variables := mv.extractVariables(formula)
	complexity += len(variables)
	
	numbers := mv.extractNumbers(formula)
	complexity += len(numbers)
	
	return complexity
}

// calculateConfidence computes overall confidence in the validation result
func (mv *MathValidator) calculateConfidence(syntaxValid, semanticValid, consistencyValid bool, complexity int) float64 {
	confidence := 0.0
	
	// Base confidence from validation results
	if syntaxValid {
		confidence += 0.4
	}
	if semanticValid {
		confidence += 0.3
	}
	if consistencyValid {
		confidence += 0.2
	}
	
	// Adjust for complexity
	complexityFactor := 1.0
	if complexity > mv.config.MaxComplexity {
		complexityFactor = 0.7 // Lower confidence for very complex expressions
	} else if complexity > mv.config.MaxComplexity/2 {
		complexityFactor = 0.9 // Slightly lower confidence for moderately complex
	}
	
	confidence *= complexityFactor
	
	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}
	
	return confidence
}

// Helper methods

func (mv *MathValidator) checkBalancedParentheses(formula string) bool {
	count := 0
	for _, char := range formula {
		switch char {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

func (mv *MathValidator) checkOperatorUsage(formula string) bool {
	// Check for operators at the beginning or end
	operators := "+-*/%^"
	if len(formula) > 0 {
		first := string(formula[0])
		last := string(formula[len(formula)-1])
		if strings.Contains(operators, first) && first != "-" { // Allow negative numbers
			return false
		}
		if strings.Contains(operators, last) {
			return false
		}
	}
	
	// Check for consecutive operators
	operatorPattern := regexp.MustCompile(`[+\-*/%^]{2,}`)
	return !operatorPattern.MatchString(formula)
}

func (mv *MathValidator) checkFunctionCalls(formula string) bool {
	// Check that all functions are followed by parentheses
	for function := range mv.parser.functions {
		functionPattern := regexp.MustCompile(function + `\b`)
		matches := functionPattern.FindAllStringIndex(formula, -1)
		for _, match := range matches {
			// Check if function is followed by '('
			nextPos := match[1]
			if nextPos < len(formula) {
				// Skip whitespace
				for nextPos < len(formula) && formula[nextPos] == ' ' {
					nextPos++
				}
				if nextPos >= len(formula) || formula[nextPos] != '(' {
					return false
				}
			} else {
				return false
			}
		}
	}
	return true
}

func (mv *MathValidator) extractVariables(formula string) []string {
	// Extract variable names (letters and combinations)
	variablePattern := regexp.MustCompile(`[a-zA-Z]+`)
	matches := variablePattern.FindAllString(formula, -1)
	
	// Filter out known functions and constants
	var variables []string
	for _, match := range matches {
		if !mv.isKnownFunction(match) && !mv.isKnownConstant(match) {
			variables = append(variables, match)
		}
	}
	
	return variables
}

func (mv *MathValidator) extractNumbers(formula string) []string {
	// Extract numeric values
	numberPattern := regexp.MustCompile(`\d+\.?\d*`)
	return numberPattern.FindAllString(formula, -1)
}

func (mv *MathValidator) isKnownFunction(name string) bool {
	return mv.parser.functions[name]
}

func (mv *MathValidator) isKnownConstant(name string) bool {
	_, exists := mv.parser.constants[name]
	return exists
}

func (mv *MathValidator) checkDomainRestrictions(formula string) []string {
	var errors []string
	
	// Check for logarithms of negative numbers or zero
	logPattern := regexp.MustCompile(`log\s*\(\s*([^)]+)\s*\)`)
	logMatches := logPattern.FindAllStringSubmatch(formula, -1)
	for _, match := range logMatches {
		if len(match) > 1 {
			arg := strings.TrimSpace(match[1])
			if value, err := strconv.ParseFloat(arg, 64); err == nil {
				if value <= 0 {
					errors = append(errors, fmt.Sprintf("Logarithm of non-positive value: log(%s)", arg))
				}
			}
		}
	}
	
	// Check for square roots of negative numbers
	sqrtPattern := regexp.MustCompile(`sqrt\s*\(\s*([^)]+)\s*\)`)
	sqrtMatches := sqrtPattern.FindAllStringSubmatch(formula, -1)
	for _, match := range sqrtMatches {
		if len(match) > 1 {
			arg := strings.TrimSpace(match[1])
			if value, err := strconv.ParseFloat(arg, 64); err == nil {
				if value < 0 {
					errors = append(errors, fmt.Sprintf("Square root of negative value: sqrt(%s)", arg))
				}
			}
		}
	}
	
	return errors
}

func (mv *MathValidator) checkDimensionalConsistency(formula string) bool {
	// Simplified dimensional analysis placeholder
	// In a full implementation, this would track units through the expression
	return true
}

func (mv *MathValidator) findInconsistencies(formula string) []string {
	var inconsistencies []string
	
	// Check for obvious mathematical errors
	if strings.Contains(formula, "1 = 0") || strings.Contains(formula, "0 = 1") {
		inconsistencies = append(inconsistencies, "Mathematical contradiction detected")
	}
	
	return inconsistencies
}

func (mv *MathValidator) hasRedundancy(formula string) bool {
	// Check for obvious redundancies like "x + 0" or "x * 1"
	redundancyPatterns := []string{
		`\+ 0\b`, `- 0\b`, `\* 1\b`, `/ 1\b`, `\^ 1\b`,
	}
	
	for _, pattern := range redundancyPatterns {
		matched, _ := regexp.MatchString(pattern, formula)
		if matched {
			return true
		}
	}
	
	return false
}

func (mv *MathValidator) isValidEquation(formula string) bool {
	parts := strings.Split(formula, "=")
	// Basic check: should have exactly two parts
	return len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != ""
}

func (mv *MathValidator) buildExplanation(syntaxValid, semanticValid, consistencyValid bool, 
	syntaxErrors, semanticErrors, consistencyErrors []string, notation string, complexity int) string {
	
	var parts []string
	
	parts = append(parts, fmt.Sprintf("Mathematical notation: %s", notation))
	parts = append(parts, fmt.Sprintf("Expression complexity: %d", complexity))
	
	if syntaxValid {
		parts = append(parts, "Syntax: Valid")
	} else {
		parts = append(parts, fmt.Sprintf("Syntax errors: %s", strings.Join(syntaxErrors, ", ")))
	}
	
	if semanticValid {
		parts = append(parts, "Semantics: Valid")
	} else {
		parts = append(parts, fmt.Sprintf("Semantic issues: %s", strings.Join(semanticErrors, ", ")))
	}
	
	if consistencyValid {
		parts = append(parts, "Consistency: Valid")
	} else {
		parts = append(parts, fmt.Sprintf("Consistency issues: %s", strings.Join(consistencyErrors, ", ")))
	}
	
	return strings.Join(parts, "; ")
}
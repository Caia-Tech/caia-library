package processing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
)

// CleaningRule represents a single content cleaning rule
type CleaningRule interface {
	Name() string
	Description() string
	Apply(content string) (string, error)
	Applicable(docType string) bool
}

// CleaningResult contains the results of content cleaning
type CleaningResult struct {
	OriginalLength  int                    `json:"original_length"`
	CleanedLength   int                    `json:"cleaned_length"`
	RulesApplied    []string               `json:"rules_applied"`
	BytesRemoved    int                    `json:"bytes_removed"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	Warnings        []string               `json:"warnings,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ContentCleaner applies rule-based cleaning to document content
type ContentCleaner struct {
	rules           []CleaningRule
	enabledRules    map[string]bool
	strictMode      bool
	preserveStructure bool
}

// NewContentCleaner creates a new content cleaner with default rules
func NewContentCleaner() *ContentCleaner {
	cleaner := &ContentCleaner{
		rules:           make([]CleaningRule, 0),
		enabledRules:    make(map[string]bool),
		strictMode:      false,
		preserveStructure: true,
	}
	
	// Add default cleaning rules
	cleaner.AddRule(&WhitespaceNormalizationRule{})
	cleaner.AddRule(&HTMLTagRemovalRule{})
	cleaner.AddRule(&URLCleaningRule{})
	cleaner.AddRule(&EmailObfuscationRule{})
	cleaner.AddRule(&NumberNormalizationRule{})
	cleaner.AddRule(&PunctuationCleaningRule{})
	cleaner.AddRule(&EncodingNormalizationRule{})
	cleaner.AddRule(&DuplicateLineRemovalRule{})
	
	// Enable all rules by default
	for _, rule := range cleaner.rules {
		cleaner.enabledRules[rule.Name()] = true
	}
	
	return cleaner
}

// AddRule adds a custom cleaning rule
func (cc *ContentCleaner) AddRule(rule CleaningRule) {
	cc.rules = append(cc.rules, rule)
	cc.enabledRules[rule.Name()] = true
}

// EnableRule enables a specific rule by name
func (cc *ContentCleaner) EnableRule(ruleName string) {
	cc.enabledRules[ruleName] = true
}

// DisableRule disables a specific rule by name  
func (cc *ContentCleaner) DisableRule(ruleName string) {
	cc.enabledRules[ruleName] = false
}

// SetStrictMode enables/disables strict cleaning mode
func (cc *ContentCleaner) SetStrictMode(strict bool) {
	cc.strictMode = strict
}

// SetPreserveStructure enables/disables structure preservation
func (cc *ContentCleaner) SetPreserveStructure(preserve bool) {
	cc.preserveStructure = preserve
}

// CleanDocument cleans the content of a document
func (cc *ContentCleaner) CleanDocument(ctx context.Context, doc *document.Document) (*CleaningResult, error) {
	if doc == nil || doc.Content.Text == "" {
		return &CleaningResult{
			OriginalLength: 0,
			CleanedLength:  0,
			RulesApplied:   []string{},
			ProcessingTime: 0,
		}, nil
	}
	
	start := time.Now()
	originalContent := doc.Content.Text
	cleanedContent := originalContent
	rulesApplied := []string{}
	warnings := []string{}
	
	// Apply enabled rules in sequence
	for _, rule := range cc.rules {
		if !cc.enabledRules[rule.Name()] {
			continue
		}
		
		// Check if rule is applicable to document type
		if !rule.Applicable(doc.Source.Type) {
			continue
		}
		
		// Apply rule with error handling
		before := cleanedContent
		after, err := rule.Apply(cleanedContent)
		if err != nil {
			warning := fmt.Sprintf("Rule %s failed: %v", rule.Name(), err)
			warnings = append(warnings, warning)
			if cc.strictMode {
				return nil, fmt.Errorf("cleaning failed in strict mode: %w", err)
			}
			continue
		}
		
		if after != before {
			cleanedContent = after
			rulesApplied = append(rulesApplied, rule.Name())
		}
	}
	
	// Update document with cleaned content
	doc.Content.Text = cleanedContent
	
	// Add cleaning metadata
	if doc.Content.Metadata == nil {
		doc.Content.Metadata = make(map[string]string)
	}
	doc.Content.Metadata["cleaned"] = "true"
	doc.Content.Metadata["cleaned_at"] = time.Now().Format(time.RFC3339)
	doc.Content.Metadata["rules_applied"] = strings.Join(rulesApplied, ",")
	
	result := &CleaningResult{
		OriginalLength: len(originalContent),
		CleanedLength:  len(cleanedContent),
		RulesApplied:   rulesApplied,
		BytesRemoved:   len(originalContent) - len(cleanedContent),
		ProcessingTime: time.Since(start),
		Warnings:       warnings,
		Metadata: map[string]interface{}{
			"original_length": len(originalContent),
			"cleaned_length":  len(cleanedContent),
			"compression_ratio": float64(len(cleanedContent)) / float64(len(originalContent)),
		},
	}
	
	return result, nil
}

// GetEnabledRules returns a list of currently enabled rules
func (cc *ContentCleaner) GetEnabledRules() []string {
	enabled := make([]string, 0)
	for _, rule := range cc.rules {
		if cc.enabledRules[rule.Name()] {
			enabled = append(enabled, rule.Name())
		}
	}
	return enabled
}

// GetAvailableRules returns all available rules with descriptions
func (cc *ContentCleaner) GetAvailableRules() map[string]string {
	rules := make(map[string]string)
	for _, rule := range cc.rules {
		rules[rule.Name()] = rule.Description()
	}
	return rules
}
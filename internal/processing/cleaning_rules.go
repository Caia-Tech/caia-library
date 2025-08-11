package processing

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// WhitespaceNormalizationRule normalizes whitespace characters
type WhitespaceNormalizationRule struct{}

func (r *WhitespaceNormalizationRule) Name() string {
	return "whitespace_normalization"
}

func (r *WhitespaceNormalizationRule) Description() string {
	return "Normalizes whitespace: removes excessive spaces, tabs, and newlines"
}

func (r *WhitespaceNormalizationRule) Applicable(docType string) bool {
	return true // Applies to all document types
}

func (r *WhitespaceNormalizationRule) Apply(content string) (string, error) {
	// Replace multiple consecutive whitespace with single space
	wsRegex := regexp.MustCompile(`\s+`)
	normalized := wsRegex.ReplaceAllString(content, " ")
	
	// Remove leading/trailing whitespace
	normalized = strings.TrimSpace(normalized)
	
	// Preserve paragraph breaks (double newlines become single newlines)
	paragraphRegex := regexp.MustCompile(`\n\s*\n`)
	normalized = paragraphRegex.ReplaceAllString(normalized, "\n\n")
	
	return normalized, nil
}

// HTMLTagRemovalRule removes HTML tags and entities
type HTMLTagRemovalRule struct{}

func (r *HTMLTagRemovalRule) Name() string {
	return "html_tag_removal"
}

func (r *HTMLTagRemovalRule) Description() string {
	return "Removes HTML tags and decodes HTML entities"
}

func (r *HTMLTagRemovalRule) Applicable(docType string) bool {
	return docType == "html" || docType == "text"
}

func (r *HTMLTagRemovalRule) Apply(content string) (string, error) {
	// Remove HTML tags
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := tagRegex.ReplaceAllString(content, " ")
	
	// Decode HTML entities
	cleaned = html.UnescapeString(cleaned)
	
	// Clean up extra spaces created by tag removal
	spaceRegex := regexp.MustCompile(`\s+`)
	cleaned = spaceRegex.ReplaceAllString(cleaned, " ")
	
	return strings.TrimSpace(cleaned), nil
}

// URLCleaningRule removes or normalizes URLs
type URLCleaningRule struct{}

func (r *URLCleaningRule) Name() string {
	return "url_cleaning"
}

func (r *URLCleaningRule) Description() string {
	return "Removes or normalizes URLs in text content"
}

func (r *URLCleaningRule) Applicable(docType string) bool {
	return true
}

func (r *URLCleaningRule) Apply(content string) (string, error) {
	// Match various URL patterns
	urlPatterns := []*regexp.Regexp{
		regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`),
		regexp.MustCompile(`www\.[^\s<>"{}|\\^` + "`" + `\[\]]+`),
		regexp.MustCompile(`ftp://[^\s<>"{}|\\^` + "`" + `\[\]]+`),
	}
	
	cleaned := content
	for _, urlRegex := range urlPatterns {
		// Replace URLs with [URL] placeholder or remove entirely
		cleaned = urlRegex.ReplaceAllString(cleaned, "[URL]")
	}
	
	return cleaned, nil
}

// EmailObfuscationRule removes or obfuscates email addresses
type EmailObfuscationRule struct{}

func (r *EmailObfuscationRule) Name() string {
	return "email_obfuscation"
}

func (r *EmailObfuscationRule) Description() string {
	return "Removes or obfuscates email addresses for privacy"
}

func (r *EmailObfuscationRule) Applicable(docType string) bool {
	return true
}

func (r *EmailObfuscationRule) Apply(content string) (string, error) {
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	
	// Replace emails with [EMAIL] placeholder
	cleaned := emailRegex.ReplaceAllString(content, "[EMAIL]")
	
	return cleaned, nil
}

// NumberNormalizationRule normalizes numbers and numerical data
type NumberNormalizationRule struct{}

func (r *NumberNormalizationRule) Name() string {
	return "number_normalization"
}

func (r *NumberNormalizationRule) Description() string {
	return "Normalizes numerical data and removes excessive precision"
}

func (r *NumberNormalizationRule) Applicable(docType string) bool {
	return docType == "text" || docType == "html" || docType == "json"
}

func (r *NumberNormalizationRule) Apply(content string) (string, error) {
	// Normalize floating point numbers (remove excessive decimal places)
	floatRegex := regexp.MustCompile(`\b\d+\.\d{4,}\b`)
	cleaned := floatRegex.ReplaceAllStringFunc(content, func(match string) string {
		// Keep only 2 decimal places for most numbers
		parts := strings.Split(match, ".")
		if len(parts) == 2 && len(parts[1]) > 2 {
			return parts[0] + "." + parts[1][:2]
		}
		return match
	})
	
	return cleaned, nil
}

// PunctuationCleaningRule cleans excessive punctuation
type PunctuationCleaningRule struct{}

func (r *PunctuationCleaningRule) Name() string {
	return "punctuation_cleaning"
}

func (r *PunctuationCleaningRule) Description() string {
	return "Removes excessive punctuation and normalizes quotation marks"
}

func (r *PunctuationCleaningRule) Applicable(docType string) bool {
	return true
}

func (r *PunctuationCleaningRule) Apply(content string) (string, error) {
	// Remove excessive punctuation (more than 3 consecutive)
	punctRegex := regexp.MustCompile(`[!?.,:;]{4,}`)
	cleaned := punctRegex.ReplaceAllStringFunc(content, func(match string) string {
		// Keep only the first 3 characters
		if len(match) > 3 {
			return match[:3]
		}
		return match
	})
	
	// Normalize quotation marks
	cleaned = strings.ReplaceAll(cleaned, "\u201c", "\"") // Left double quote
	cleaned = strings.ReplaceAll(cleaned, "\u201d", "\"") // Right double quote
	cleaned = strings.ReplaceAll(cleaned, "\u2018", "'")  // Left single quote
	cleaned = strings.ReplaceAll(cleaned, "\u2019", "'")
	
	return cleaned, nil
}

// EncodingNormalizationRule normalizes character encoding issues
type EncodingNormalizationRule struct{}

func (r *EncodingNormalizationRule) Name() string {
	return "encoding_normalization"
}

func (r *EncodingNormalizationRule) Description() string {
	return "Fixes common encoding issues and normalizes Unicode characters"
}

func (r *EncodingNormalizationRule) Applicable(docType string) bool {
	return true
}

func (r *EncodingNormalizationRule) Apply(content string) (string, error) {
	cleaned := content
	
	// Fix common UTF-8 encoding problems first
	cleaned = strings.ReplaceAll(cleaned, "â€™", "'")  // Broken encoding of right single quote
	cleaned = strings.ReplaceAll(cleaned, "â€œ", "\"") // Broken encoding of left double quote  
	cleaned = strings.ReplaceAll(cleaned, "â€", "\"")  // Broken encoding of right double quote
	cleaned = strings.ReplaceAll(cleaned, "Â ", " ")   // Broken encoding with non-breaking space
	cleaned = strings.ReplaceAll(cleaned, "Â", "")     // Stray broken encoding character
	
	// Remove BOM
	cleaned = strings.ReplaceAll(cleaned, "\uFEFF", "")
	
	// Remove stray non-breaking spaces and similar
	cleaned = strings.ReplaceAll(cleaned, "\u00A0", " ") // Non-breaking space
	cleaned = strings.ReplaceAll(cleaned, "\u200B", "") // Zero-width space
	
	// Fix smart quotes to regular quotes (using Unicode escape codes)
	cleaned = strings.ReplaceAll(cleaned, "\u2018", "'") // Left single quote
	cleaned = strings.ReplaceAll(cleaned, "\u2019", "'") // Right single quote
	cleaned = strings.ReplaceAll(cleaned, "\u201C", "\"") // Left double quote
	cleaned = strings.ReplaceAll(cleaned, "\u201D", "\"") // Right double quote
	
	// Fix dashes
	cleaned = strings.ReplaceAll(cleaned, "\u2013", "-") // En dash
	cleaned = strings.ReplaceAll(cleaned, "\u2014", "-") // Em dash
	
	// Remove or replace non-printable characters
	cleaned = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return -1 // Remove control characters except newlines and tabs
		}
		return r
	}, cleaned)
	
	return cleaned, nil
}

// DuplicateLineRemovalRule removes duplicate consecutive lines
type DuplicateLineRemovalRule struct{}

func (r *DuplicateLineRemovalRule) Name() string {
	return "duplicate_line_removal"
}

func (r *DuplicateLineRemovalRule) Description() string {
	return "Removes consecutive duplicate lines"
}

func (r *DuplicateLineRemovalRule) Applicable(docType string) bool {
	return true
}

func (r *DuplicateLineRemovalRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) <= 1 {
		return content, nil
	}
	
	result := make([]string, 0, len(lines))
	lastLine := ""
	
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != lastLine || trimmedLine == "" {
			result = append(result, line)
		}
		lastLine = trimmedLine
	}
	
	return strings.Join(result, "\n"), nil
}
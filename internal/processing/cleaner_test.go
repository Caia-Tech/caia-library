package processing

import (
	"context"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentCleanerBasic(t *testing.T) {
	cleaner := NewContentCleaner()
	
	// Test document with various content issues
	doc := &document.Document{
		ID: "test-cleaning-001",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/test.txt",
		},
		Content: document.Content{
			Text: `  This   is    a    test document   with    excessive    whitespace.

<p>It has <b>HTML</b> tags and &amp; entities.</p>

Check out this website: https://example.com/path?param=value

Contact me at test@example.com for more info!!!!!!

Some numbers: 3.14159265359 and 123.456789

"Smart quotes" and 'apostrophes' everywhere.

Duplicate line
Duplicate line
Another line
Another line

â€™s encoding problems and Â strange characters.`,
			Metadata: make(map[string]string),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	result, err := cleaner.CleanDocument(context.Background(), doc)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Check that content was cleaned
	assert.Less(t, result.CleanedLength, result.OriginalLength, "Content should be shorter after cleaning")
	assert.Greater(t, result.BytesRemoved, 0, "Some bytes should have been removed")
	assert.Greater(t, len(result.RulesApplied), 0, "Some rules should have been applied")
	
	// Check specific cleaning results
	cleanedContent := doc.Content.Text
	
	// Should not contain HTML tags
	assert.NotContains(t, cleanedContent, "<p>")
	assert.NotContains(t, cleanedContent, "<b>")
	assert.NotContains(t, cleanedContent, "&amp;")
	
	// URLs should be replaced
	assert.NotContains(t, cleanedContent, "https://example.com/path?param=value")
	assert.Contains(t, cleanedContent, "[URL]")
	
	// Emails should be replaced
	assert.NotContains(t, cleanedContent, "test@example.com")
	assert.Contains(t, cleanedContent, "[EMAIL]")
	
	// Excessive punctuation should be reduced
	assert.NotContains(t, cleanedContent, "!!!!!!")
	
	// Encoding issues should be fixed
	assert.NotContains(t, cleanedContent, "â€™")
	assert.NotContains(t, cleanedContent, "Â")
	
	// Metadata should be added
	assert.Equal(t, "true", doc.Content.Metadata["cleaned"])
	assert.NotEmpty(t, doc.Content.Metadata["cleaned_at"])
	assert.NotEmpty(t, doc.Content.Metadata["rules_applied"])
	
	t.Logf("Original length: %d, Cleaned length: %d", result.OriginalLength, result.CleanedLength)
	t.Logf("Bytes removed: %d", result.BytesRemoved)
	t.Logf("Rules applied: %v", result.RulesApplied)
	t.Logf("Processing time: %v", result.ProcessingTime)
}

func TestIndividualCleaningRules(t *testing.T) {
	tests := []struct {
		name     string
		rule     CleaningRule
		input    string
		expected string
	}{
		{
			name:     "WhitespaceNormalization",
			rule:     &WhitespaceNormalizationRule{},
			input:    "  Multiple   spaces    and\n\n\ntabs\t\t\there  ",
			expected: "Multiple spaces and tabs here",
		},
		{
			name:     "HTMLTagRemoval",
			rule:     &HTMLTagRemovalRule{},
			input:    "<p>Hello <b>world</b> &amp; friends!</p>",
			expected: "Hello world & friends!",
		},
		{
			name:     "URLCleaning",
			rule:     &URLCleaningRule{},
			input:    "Visit https://example.com or www.google.com",
			expected: "Visit [URL] or [URL]",
		},
		{
			name:     "EmailObfuscation",
			rule:     &EmailObfuscationRule{},
			input:    "Contact john.doe@example.com or jane+test@domain.org",
			expected: "Contact [EMAIL] or [EMAIL]",
		},
		{
			name:     "NumberNormalization",
			rule:     &NumberNormalizationRule{},
			input:    "Pi is 3.14159265359 approximately",
			expected: "Pi is 3.14 approximately",
		},
		{
			name:     "PunctuationCleaning",
			rule:     &PunctuationCleaningRule{},
			input:    "Really??????? \"Smart quotes\" and 'apostrophes'",
			expected: "Really??? \"Smart quotes\" and 'apostrophes'",
		},
		{
			name:     "EncodingNormalization",
			rule:     &EncodingNormalizationRule{},
			input:    "Thereâ€™s encoding problems with â€œquotesâ€",
			expected: "There's encoding problems with \"quotes\"",
		},
		{
			name: "DuplicateLineRemoval",
			rule: &DuplicateLineRemovalRule{},
			input: `Line 1
Line 2
Line 2
Line 3
Line 3
Line 4`,
			expected: `Line 1
Line 2
Line 3
Line 4`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.rule.Apply(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleaningRuleApplicability(t *testing.T) {
	rules := []CleaningRule{
		&WhitespaceNormalizationRule{},
		&HTMLTagRemovalRule{},
		&URLCleaningRule{},
		&EmailObfuscationRule{},
		&NumberNormalizationRule{},
		&PunctuationCleaningRule{},
		&EncodingNormalizationRule{},
		&DuplicateLineRemovalRule{},
	}
	
	testCases := []struct {
		docType  string
		expected map[string]bool
	}{
		{
			docType: "text",
			expected: map[string]bool{
				"whitespace_normalization": true,
				"html_tag_removal":         true,
				"url_cleaning":            true,
				"email_obfuscation":       true,
				"number_normalization":    true,
				"punctuation_cleaning":    true,
				"encoding_normalization":  true,
				"duplicate_line_removal":  true,
			},
		},
		{
			docType: "html",
			expected: map[string]bool{
				"html_tag_removal": true,
			},
		},
		{
			docType: "pdf",
			expected: map[string]bool{
				"number_normalization": false, // Only applies to text, html, json
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.docType, func(t *testing.T) {
			for _, rule := range rules {
				if expected, ok := tc.expected[rule.Name()]; ok {
					actual := rule.Applicable(tc.docType)
					assert.Equal(t, expected, actual, 
						"Rule %s applicability for %s", rule.Name(), tc.docType)
				}
			}
		})
	}
}

func TestContentCleanerConfiguration(t *testing.T) {
	cleaner := NewContentCleaner()
	
	// Test enabling/disabling rules
	cleaner.DisableRule("html_tag_removal")
	cleaner.DisableRule("url_cleaning")
	
	doc := &document.Document{
		ID: "config-test",
		Source: document.Source{Type: "html"},
		Content: document.Content{
			Text: "<p>Visit https://example.com</p>",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	result, err := cleaner.CleanDocument(context.Background(), doc)
	require.NoError(t, err)
	
	// HTML tags and URLs should still be present since those rules are disabled
	assert.Contains(t, doc.Content.Text, "<p>")
	assert.Contains(t, doc.Content.Text, "https://example.com")
	
	// But whitespace normalization should still work
	assert.NotContains(t, result.RulesApplied, "html_tag_removal")
	assert.NotContains(t, result.RulesApplied, "url_cleaning")
}

func TestEmptyAndNilDocument(t *testing.T) {
	cleaner := NewContentCleaner()
	
	// Test nil document
	result, err := cleaner.CleanDocument(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.OriginalLength)
	assert.Equal(t, 0, result.CleanedLength)
	
	// Test empty content
	doc := &document.Document{
		ID: "empty-test",
		Source: document.Source{Type: "text"},
		Content: document.Content{Text: ""},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	result, err = cleaner.CleanDocument(context.Background(), doc)
	require.NoError(t, err)
	assert.Equal(t, 0, result.OriginalLength)
	assert.Equal(t, 0, result.CleanedLength)
}

func BenchmarkContentCleaning(b *testing.B) {
	cleaner := NewContentCleaner()
	
	// Create a document with various content issues
	doc := &document.Document{
		ID: "benchmark-test",
		Source: document.Source{Type: "html"},
		Content: document.Content{
			Text: `<html><body><p>This is a <b>sample</b> HTML document with multiple issues.</p>
			
			<p>It contains URLs like https://example.com/path?param=value and emails like test@example.com.</p>
			
			<p>Numbers like 3.14159265359 and excessive punctuation!!!!!! are also present.</p>
			
			<p>There are â€™encoding problemsâ€ and    excessive     whitespace   everywhere.</p>
			
			</body></html>`,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Reset content for each iteration
		doc.Content.Text = `<html><body><p>This is a <b>sample</b> HTML document with multiple issues.</p>
			
			<p>It contains URLs like https://example.com/path?param=value and emails like test@example.com.</p>
			
			<p>Numbers like 3.14159265359 and excessive punctuation!!!!!! are also present.</p>
			
			<p>There are â€™encoding problemsâ€ and    excessive     whitespace   everywhere.</p>
			
			</body></html>`
		
		_, err := cleaner.CleanDocument(context.Background(), doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}
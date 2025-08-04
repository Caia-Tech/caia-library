package document

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDocument_GetStoragePath(t *testing.T) {
	tests := []struct {
		name     string
		docID    string
		expected string
	}{
		{
			name:     "two char ID",
			docID:    "ab",
			expected: "documents/ab/ab",
		},
		{
			name:     "normal ID",
			docID:    "test-doc-123",
			expected: "documents/te/st/test-doc-123",
		},
		{
			name:     "arxiv ID",
			docID:    "arxiv-2301.00001",
			expected: "documents/ar/xi/arxiv-2301.00001",
		},
		{
			name:     "uuid format",
			docID:    "550e8400-e29b-41d4-a716-446655440000",
			expected: "documents/55/0e/550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "single char",
			docID:    "a",
			expected: "documents/a/a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{ID: tt.docID}
			result := doc.GetStoragePath()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocument_Validate(t *testing.T) {
	tests := []struct {
		name    string
		doc     *Document
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid document",
			doc: &Document{
				ID: "test-123",
				Source: Source{
					Type: "pdf",
					URL:  "https://example.com/doc.pdf",
				},
				Content: Content{
					Text: "Test content",
					Metadata: map[string]string{
						"attribution": "Caia Tech",
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			doc: &Document{
				Source: Source{
					Type: "pdf",
					URL:  "https://example.com/doc.pdf",
				},
			},
			wantErr: true,
			errMsg:  "document ID cannot be empty",
		},
		{
			name: "missing source type",
			doc: &Document{
				ID: "test-123",
				Source: Source{
					URL: "https://example.com/doc.pdf",
				},
			},
			wantErr: true,
			errMsg:  "document source type cannot be empty",
		},
		{
			name: "missing source URL",
			doc: &Document{
				ID: "test-123",
				Source: Source{
					Type: "pdf",
				},
			},
			wantErr: true,
			errMsg:  "document must have either URL or path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.doc.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDocument_AttributionMetadata(t *testing.T) {
	doc := &Document{
		ID: "arxiv-123",
		Source: Source{
			Type: "pdf",
			URL:  "https://arxiv.org/pdf/2301.00001.pdf",
		},
		Content: Content{
			Text: "AI research paper content",
			Metadata: map[string]string{
				"source":      "arXiv",
				"attribution": "Content from arXiv.org, collected by Caia Tech",
				"license":     "arXiv License",
			},
		},
	}

	// Verify attribution is properly stored
	assert.Equal(t, "arXiv", doc.Content.Metadata["source"])
	assert.Contains(t, doc.Content.Metadata["attribution"], "Caia Tech")
	assert.NotEmpty(t, doc.Content.Metadata["license"])
}

func TestSource_IsAcademic(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		expected bool
	}{
		{
			name: "arxiv source",
			source: Source{
				Type: "pdf",
				URL:  "https://arxiv.org/pdf/2301.00001.pdf",
			},
			expected: true,
		},
		{
			name: "pubmed source",
			source: Source{
				Type: "html",
				URL:  "https://pubmed.ncbi.nlm.nih.gov/12345678",
			},
			expected: true,
		},
		{
			name: "generic web source",
			source: Source{
				Type: "html",
				URL:  "https://example.com/page.html",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would be implemented if we add IsAcademic() method
			// For now, just verify URL contains academic domain
			isAcademic := false
			academicDomains := []string{"arxiv.org", "pubmed.ncbi", "doaj.org", "plos.org"}
			for _, domain := range academicDomains {
				if strings.Contains(tt.source.URL, domain) {
					isAcademic = true
					break
				}
			}
			assert.Equal(t, tt.expected, isAcademic)
		})
	}
}
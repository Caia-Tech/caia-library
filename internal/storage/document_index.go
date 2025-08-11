package storage

import (
	"strings"
	"sync"
	"time"
)

// DocumentIndex maintains an in-memory index of document IDs to their storage paths
// This provides O(1) lookups instead of searching through the filesystem
type DocumentIndex struct {
	mu       sync.RWMutex
	index    map[string]string // docID -> path mapping
	metadata map[string]*DocumentMetadata
}

// DocumentMetadata caches frequently accessed document metadata
type DocumentMetadata struct {
	ID         string
	Path       string
	Type       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastAccess time.Time
}

// NewDocumentIndex creates a new document index
func NewDocumentIndex() *DocumentIndex {
	return &DocumentIndex{
		index:    make(map[string]string),
		metadata: make(map[string]*DocumentMetadata),
	}
}

// Add adds a document to the index
func (di *DocumentIndex) Add(docID, path string, meta *DocumentMetadata) {
	di.mu.Lock()
	defer di.mu.Unlock()
	
	di.index[docID] = path
	if meta != nil {
		meta.LastAccess = time.Now()
		di.metadata[docID] = meta
	}
}

// Get retrieves a document path from the index
func (di *DocumentIndex) Get(docID string) (string, bool) {
	di.mu.RLock()
	defer di.mu.RUnlock()
	
	path, exists := di.index[docID]
	if exists {
		if meta, ok := di.metadata[docID]; ok && meta != nil {
			meta.LastAccess = time.Now()
		}
	}
	return path, exists
}

// GetMetadata retrieves cached metadata for a document
func (di *DocumentIndex) GetMetadata(docID string) (*DocumentMetadata, bool) {
	di.mu.RLock()
	defer di.mu.RUnlock()
	
	meta, exists := di.metadata[docID]
	if exists && meta != nil {
		meta.LastAccess = time.Now()
	}
	return meta, exists
}

// Remove removes a document from the index
func (di *DocumentIndex) Remove(docID string) {
	di.mu.Lock()
	defer di.mu.Unlock()
	
	delete(di.index, docID)
	delete(di.metadata, docID)
}

// Size returns the number of documents in the index
func (di *DocumentIndex) Size() int {
	di.mu.RLock()
	defer di.mu.RUnlock()
	
	return len(di.index)
}

// Clear removes all entries from the index
func (di *DocumentIndex) Clear() {
	di.mu.Lock()
	defer di.mu.Unlock()
	
	di.index = make(map[string]string)
	di.metadata = make(map[string]*DocumentMetadata)
}

// RebuildFromPaths rebuilds the index from a list of file paths
func (di *DocumentIndex) RebuildFromPaths(paths []string) {
	di.mu.Lock()
	defer di.mu.Unlock()
	
	// Clear existing index
	di.index = make(map[string]string)
	di.metadata = make(map[string]*DocumentMetadata)
	
	// Extract document IDs from paths
	// Paths are in format: documents/{type}/{YYYY/MM}/{id}/metadata.json
	for _, path := range paths {
		docID := extractDocIDFromPath(path)
		if docID != "" {
			di.index[docID] = path
		}
	}
}

// GetAllDocuments returns all document metadata in the index
func (di *DocumentIndex) GetAllDocuments() []*DocumentMetadata {
	di.mu.RLock()
	defer di.mu.RUnlock()
	
	results := make([]*DocumentMetadata, 0, len(di.metadata))
	for docID, meta := range di.metadata {
		if meta != nil {
			results = append(results, meta)
		} else {
			// Create basic metadata if not available
			if path, exists := di.index[docID]; exists {
				basicMeta := &DocumentMetadata{
					ID:   docID,
					Path: path,
				}
				results = append(results, basicMeta)
			}
		}
	}
	
	// Also include documents that only have path info
	for docID, path := range di.index {
		if _, hasMetadata := di.metadata[docID]; !hasMetadata {
			basicMeta := &DocumentMetadata{
				ID:   docID,
				Path: path,
			}
			results = append(results, basicMeta)
		}
	}
	
	return results
}

// GetAllDocumentIDs returns all document IDs in the index
func (di *DocumentIndex) GetAllDocumentIDs() []string {
	di.mu.RLock()
	defer di.mu.RUnlock()
	
	ids := make([]string, 0, len(di.index))
	for docID := range di.index {
		ids = append(ids, docID)
	}
	return ids
}

// extractDocIDFromPath extracts document ID from a metadata file path
func extractDocIDFromPath(path string) string {
	// Path format: documents/{type}/{YYYY/MM}/{id}/metadata.json
	// We need to extract the {id} part
	
	if !strings.HasSuffix(path, "/metadata.json") {
		return ""
	}
	
	// Remove the /metadata.json suffix
	dir := strings.TrimSuffix(path, "/metadata.json")
	
	// Get the last component which should be the document ID
	parts := strings.Split(dir, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	
	return ""
}
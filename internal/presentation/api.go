package presentation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// API provides HTTP endpoints for document presentation
type API struct {
	renderer *Renderer
	storage  Storage
	config   *APIConfig
}

// APIConfig configures the presentation API
type APIConfig struct {
	Port            int    `json:"port"`
	Host            string `json:"host"`
	BasePath        string `json:"base_path"`
	EnableCORS      bool   `json:"enable_cors"`
	RateLimitPerMin int    `json:"rate_limit_per_min"`
	AuthRequired    bool   `json:"auth_required"`
}

// NewAPI creates a new presentation API
func NewAPI(renderer *Renderer, storage Storage, config *APIConfig) *API {
	if config == nil {
		config = &APIConfig{
			Port:            8080,
			Host:            "localhost",
			BasePath:        "/api/v1",
			EnableCORS:      true,
			RateLimitPerMin: 100,
			AuthRequired:    false,
		}
	}

	return &API{
		renderer: renderer,
		storage:  storage,
		config:   config,
	}
}

// Start starts the API server
func (api *API) Start() error {
	router := api.setupRoutes()

	// Add middleware
	handler := api.addMiddleware(router)

	addr := fmt.Sprintf("%s:%d", api.config.Host, api.config.Port)
	log.Info().Str("address", addr).Msg("Starting presentation API")

	return http.ListenAndServe(addr, handler)
}

// setupRoutes configures API routes
func (api *API) setupRoutes() *mux.Router {
	router := mux.NewRouter()
	base := router.PathPrefix(api.config.BasePath).Subrouter()

	// Document endpoints
	base.HandleFunc("/documents", api.listDocuments).Methods("GET")
	base.HandleFunc("/documents/{id}", api.getDocument).Methods("GET")
	base.HandleFunc("/documents/{id}/export", api.exportDocument).Methods("GET")
	
	// Search endpoints
	base.HandleFunc("/search", api.searchDocuments).Methods("GET", "POST")
	
	// Collection endpoints
	base.HandleFunc("/collections", api.listCollections).Methods("GET")
	base.HandleFunc("/collections/{name}", api.getCollection).Methods("GET")
	
	// Statistics endpoint
	base.HandleFunc("/statistics", api.getStatistics).Methods("GET")
	
	// Health check
	base.HandleFunc("/health", api.healthCheck).Methods("GET")

	return router
}

// addMiddleware adds middleware to the router
func (api *API) addMiddleware(router http.Handler) http.Handler {
	// CORS middleware
	if api.config.EnableCORS {
		router = api.corsMiddleware(router)
	}

	// Logging middleware
	router = api.loggingMiddleware(router)

	// Rate limiting would go here in production

	return router
}

// Handler implementations

func (api *API) listDocuments(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	params := r.URL.Query()
	
	pageSize, _ := strconv.Atoi(params.Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	
	pageNumber, _ := strconv.Atoi(params.Get("page"))
	if pageNumber <= 0 {
		pageNumber = 1
	}

	format := params.Get("format")
	if format == "" {
		format = "json"
	}

	// Get documents from storage
	docs, err := api.storage.List("", pageSize*(pageNumber-1), pageSize)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to list documents", err)
		return
	}

	// Render collection
	options := &CollectionOptions{
		RenderOptions: RenderOptions{
			Format:          OutputFormat(format),
			IncludeMetadata: params.Get("metadata") == "true",
			IncludeQuality:  params.Get("quality") == "true",
			MaxLength:       500,
		},
		PageSize:       pageSize,
		PageNumber:     pageNumber,
		ShowStatistics: params.Get("stats") == "true",
		SortBy:         params.Get("sort_by"),
		SortOrder:      params.Get("sort_order"),
		GroupBy:        params.Get("group_by"),
	}

	collection, err := api.renderer.RenderCollection(docs, options)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to render collection", err)
		return
	}

	api.sendJSON(w, collection)
}

func (api *API) getDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	docID := vars["id"]

	// Get document from storage
	doc, err := api.storage.Get(docID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			api.sendError(w, http.StatusNotFound, "Document not found", err)
		} else {
			api.sendError(w, http.StatusInternalServerError, "Failed to get document", err)
		}
		return
	}

	// Parse query parameters
	params := r.URL.Query()
	format := params.Get("format")
	if format == "" {
		format = "json"
	}

	// Render document
	options := &RenderOptions{
		Format:          OutputFormat(format),
		IncludeMetadata: params.Get("metadata") != "false",
		IncludeQuality:  params.Get("quality") == "true",
		MaxLength:       0, // No truncation for single document
	}

	// Check if HTML format is requested
	if format == "html" {
		options.Theme = params.Get("theme")
		if options.Theme == "" {
			options.Theme = "light"
		}
	}

	rendered, err := api.renderer.RenderDocument(doc, options)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to render document", err)
		return
	}

	// Send response based on format
	switch format {
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(rendered.Content))
	case "markdown":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Write([]byte(rendered.Content))
	case "plain":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(rendered.Content))
	default:
		api.sendJSON(w, rendered)
	}
}

func (api *API) exportDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	docID := vars["id"]

	// Get document from storage
	doc, err := api.storage.Get(docID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			api.sendError(w, http.StatusNotFound, "Document not found", err)
		} else {
			api.sendError(w, http.StatusInternalServerError, "Failed to get document", err)
		}
		return
	}

	// Get export format
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	// Export document
	data, err := api.renderer.ExportDocument(doc, ExportFormat(format))
	if err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid export format", err)
		return
	}

	// Set appropriate content type and headers
	switch ExportFormat(format) {
	case ExportJSON:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", docID))
	case ExportMarkdown:
		w.Header().Set("Content-Type", "text/markdown")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.md\"", docID))
	case ExportXML:
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.xml\"", docID))
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.bin\"", docID))
	}

	w.Write(data)
}

func (api *API) searchDocuments(w http.ResponseWriter, r *http.Request) {
	var query string

	if r.Method == "GET" {
		query = r.URL.Query().Get("q")
	} else {
		// POST request with JSON body
		var searchReq struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&searchReq); err != nil {
			api.sendError(w, http.StatusBadRequest, "Invalid request body", err)
			return
		}
		query = searchReq.Query
	}

	if query == "" {
		api.sendError(w, http.StatusBadRequest, "Query parameter is required", nil)
		return
	}

	// Simple search implementation - in production would use proper search engine
	allDocs, err := api.storage.List("", 0, 1000)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to search documents", err)
		return
	}

	// Filter documents containing query
	var matchedDocs []*Document
	scores := make(map[string]float64)
	queryLower := strings.ToLower(query)

	for _, doc := range allDocs {
		contentLower := strings.ToLower(doc.Content)
		if strings.Contains(contentLower, queryLower) {
			matchedDocs = append(matchedDocs, doc)
			// Simple scoring based on frequency
			count := strings.Count(contentLower, queryLower)
			scores[doc.ID] = float64(count) / float64(len(strings.Fields(doc.Content)))
		}
	}

	// Create search results
	results := &SearchResults{
		Query:      query,
		Documents:  matchedDocs,
		Scores:     scores,
		TotalHits:  len(matchedDocs),
		SearchTime: 100 * time.Millisecond, // Simulated
	}

	// Parse rendering options
	params := r.URL.Query()
	pageSize, _ := strconv.Atoi(params.Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	
	pageNumber, _ := strconv.Atoi(params.Get("page"))
	if pageNumber <= 0 {
		pageNumber = 1
	}

	options := &SearchOptions{
		CollectionOptions: CollectionOptions{
			RenderOptions: RenderOptions{
				Format:         OutputFormat(params.Get("format")),
				HighlightTerms: []string{query},
				MaxLength:      200,
			},
			PageSize:   pageSize,
			PageNumber: pageNumber,
		},
		ShowSnippets:     params.Get("snippets") != "false",
		SnippetLength:    150,
		ShowFacets:       params.Get("facets") == "true",
		HighlightMatches: params.Get("highlight") != "false",
	}

	// Render search results
	rendered, err := api.renderer.RenderSearch(results, options)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to render search results", err)
		return
	}

	api.sendJSON(w, rendered)
}

func (api *API) listCollections(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would list document collections/categories
	collections := []map[string]interface{}{
		{
			"name":        "synthetic",
			"description": "Synthetically generated content",
			"count":       0,
		},
		{
			"name":        "scraped",
			"description": "Web scraped content",
			"count":       0,
		},
		{
			"name":        "curated",
			"description": "Manually curated content",
			"count":       0,
		},
	}

	api.sendJSON(w, map[string]interface{}{
		"collections": collections,
		"total":       len(collections),
	})
}

func (api *API) getCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collectionName := vars["name"]

	// Filter documents by collection (using metadata)
	allDocs, err := api.storage.List("", 0, 1000)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to get collection", err)
		return
	}

	var collectionDocs []*Document
	for _, doc := range allDocs {
		if doc.Metadata != nil {
			if source, ok := doc.Metadata["source"].(string); ok && source == collectionName {
				collectionDocs = append(collectionDocs, doc)
			}
		}
	}

	// Parse query parameters
	params := r.URL.Query()
	pageSize, _ := strconv.Atoi(params.Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	
	pageNumber, _ := strconv.Atoi(params.Get("page"))
	if pageNumber <= 0 {
		pageNumber = 1
	}

	// Render collection
	options := &CollectionOptions{
		RenderOptions: RenderOptions{
			Format:          OutputFormat(params.Get("format")),
			IncludeMetadata: params.Get("metadata") == "true",
			MaxLength:       500,
		},
		PageSize:       pageSize,
		PageNumber:     pageNumber,
		ShowStatistics: true,
	}

	collection, err := api.renderer.RenderCollection(collectionDocs, options)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to render collection", err)
		return
	}

	api.sendJSON(w, collection)
}

func (api *API) getStatistics(w http.ResponseWriter, r *http.Request) {
	// Get all documents for statistics
	allDocs, err := api.storage.List("", 0, 10000)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to get statistics", err)
		return
	}

	stats := api.renderer.calculateStatistics(allDocs)
	
	api.sendJSON(w, map[string]interface{}{
		"statistics":      stats,
		"total_documents": len(allDocs),
		"timestamp":       time.Now(),
	})
}

func (api *API) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"services": map[string]string{
			"renderer": "operational",
			"storage":  "operational",
		},
	}

	api.sendJSON(w, health)
}

// Helper methods

func (api *API) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (api *API) sendError(w http.ResponseWriter, status int, message string, err error) {
	log.Error().Err(err).Str("message", message).Int("status", status).Msg("API error")
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	response := map[string]interface{}{
		"error":     message,
		"status":    status,
		"timestamp": time.Now(),
	}
	
	if err != nil {
		response["details"] = err.Error()
	}
	
	json.NewEncoder(w).Encode(response)
}

// Middleware implementations

func (api *API) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func (api *API) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.statusCode).
			Dur("duration", time.Since(start)).
			Msg("API request")
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
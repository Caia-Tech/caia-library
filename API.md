# Caia Library API Documentation

## Base URL
```
http://localhost:8080
```

## Endpoints

### Health Check
Check if the service is running.

```
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "caia-library"
}
```

### Ingest Document
Start a document ingestion workflow.

```
POST /api/v1/documents
```

**Request Body:**
```json
{
  "url": "https://example.com/document.pdf",
  "type": "pdf",
  "metadata": {
    "source": "web_scrape",
    "category": "research",
    "synthetic": "false"
  }
}
```

**Parameters:**
- `url` (required): The URL of the document to ingest
- `type` (required): Document type (`text`, `html`, `pdf`)
- `metadata` (optional): Key-value pairs for document metadata

**Response:**
```json
{
  "workflow_id": "ingest-abc123",
  "run_id": "xyz789"
}
```

### Get Workflow Status
Check the status of a document ingestion workflow.

```
GET /api/v1/workflows/{workflow_id}
```

**Response:**
```json
{
  "workflow_id": "ingest-abc123",
  "status": "Completed",
  "start_time": {
    "seconds": 1754263075,
    "nanos": 228693000
  },
  "close_time": {
    "seconds": 1754263075,
    "nanos": 274655000
  }
}
```

**Status Values:**
- `Running` - Workflow is still processing
- `Completed` - Successfully completed
- `Failed` - Failed with error
- `Terminated` - Manually terminated
- `Canceled` - Canceled
- `TimedOut` - Exceeded timeout

### Get Document (Not Yet Implemented)
Retrieve a specific document by ID.

```
GET /api/v1/documents/{document_id}
```

### List Documents (Not Yet Implemented)
List all documents with pagination.

```
GET /api/v1/documents?page=1&limit=20
```

## Error Responses

All errors follow this format:

```json
{
  "error": "Error message here"
}
```

**Common HTTP Status Codes:**
- `200` - Success
- `202` - Accepted (workflow started)
- `400` - Bad Request (invalid input)
- `404` - Not Found
- `500` - Internal Server Error

## Workflow Processing Stages

When you ingest a document, it goes through these stages:

1. **Fetch** - Download document from URL
2. **Extract** - Extract text content
3. **Embed** - Generate embeddings (parallel with extract)
4. **Store** - Save to Git repository
5. **Index** - Update search index
6. **Merge** - Merge branch to main

Each stage can be monitored via Temporal UI at http://localhost:8233
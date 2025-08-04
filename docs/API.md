# Caia Library API Documentation

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, the API does not require authentication. Authentication is planned for Phase 3 development.

## Endpoints

### Health Check

Check the service health status.

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "caia-library",
  "version": "0.1.0",
  "timestamp": "2024-03-14T10:30:00Z"
}
```

### Document Ingestion

#### Ingest Single Document

Start a document ingestion workflow for a single document.

```http
POST /api/v1/documents
```

**Request Body:**
```json
{
  "url": "https://example.com/document.pdf",
  "type": "pdf",
  "metadata": {
    "source": "manual",
    "category": "research"
  }
}
```

**Response:**
```json
{
  "workflow_id": "ingest-123e4567-e89b-12d3-a456-426614174000",
  "run_id": "run-123e4567-e89b-12d3-a456-426614174000"
}
```

#### Batch Ingestion

Ingest multiple documents in parallel.

```http
POST /api/v1/ingestion/batch
```

**Request Body:**
```json
{
  "documents": [
    {
      "url": "https://example.com/doc1.pdf",
      "type": "pdf",
      "metadata": {"category": "research"}
    },
    {
      "url": "https://example.com/doc2.html",
      "type": "html",
      "metadata": {"category": "news"}
    }
  ]
}
```

**Response:**
```json
{
  "workflow_id": "batch-123e4567-e89b-12d3-a456-426614174000",
  "run_id": "run-123e4567-e89b-12d3-a456-426614174000",
  "count": 2
}
```

### Scheduled Ingestion

Create a scheduled ingestion source that runs on a cron schedule.

```http
POST /api/v1/ingestion/scheduled
```

**Request Body:**
```json
{
  "name": "tech-news-rss",
  "type": "rss",
  "url": "https://news.ycombinator.com/rss",
  "schedule": "0 */6 * * *",
  "filters": ["AI", "machine learning", "data"],
  "metadata": {
    "category": "tech-news",
    "priority": "high"
  }
}
```

**Supported Types:**
- `rss` - RSS/Atom feeds
- `web` - Web page scraping
- `api` - JSON API endpoints

**Schedule Format:**
Uses standard cron expressions:
- `0 * * * *` - Every hour
- `0 */6 * * *` - Every 6 hours
- `0 0 * * *` - Daily at midnight
- `0 0 * * 1` - Weekly on Monday
- `0 0 1 * *` - Monthly on the 1st

**Response:**
```json
{
  "workflow_id": "scheduled-tech-news-rss-123e4567",
  "run_id": "run-123e4567-e89b-12d3-a456-426614174000",
  "schedule": "0 */6 * * *"
}
```

### Document Operations

#### Get Document

Retrieve a document by ID.

```http
GET /api/v1/documents/:id
```

**Response:**
```json
{
  "id": "123e4567e89b12d3a456426614174000",
  "source": {
    "type": "web",
    "url": "https://example.com/document"
  },
  "content": {
    "text": "Document text content...",
    "metadata": {
      "title": "Document Title",
      "author": "John Doe"
    }
  },
  "embeddings": [0.123, 0.456, ...],
  "created_at": "2024-03-14T10:30:00Z",
  "updated_at": "2024-03-14T10:30:00Z"
}
```

#### List Documents

List documents with pagination and filtering.

```http
GET /api/v1/documents?page=1&limit=20&type=pdf
```

**Query Parameters:**
- `page` (integer): Page number (default: 1)
- `limit` (integer): Items per page (default: 20, max: 100)
- `type` (string): Filter by document type

**Response:**
```json
{
  "documents": [
    {
      "id": "123e4567e89b12d3a456426614174000",
      "source": {
        "type": "pdf",
        "url": "https://example.com/doc.pdf"
      },
      "created_at": "2024-03-14T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 150
  }
}
```

### Workflow Status

Get the status of any workflow.

```http
GET /api/v1/workflows/:id
```

**Response:**
```json
{
  "workflow_id": "ingest-123e4567-e89b-12d3-a456-426614174000",
  "status": "Running",
  "start_time": "2024-03-14T10:30:00Z",
  "close_time": null,
  "error": null,
  "result": null
}
```

**Status Values:**
- `Running` - Workflow is currently executing
- `Completed` - Workflow completed successfully
- `Failed` - Workflow failed with error
- `Terminated` - Workflow was terminated
- `Canceled` - Workflow was canceled
- `ContinuedAsNew` - Workflow continued as new execution

## Error Responses

All errors follow a consistent format:

```json
{
  "error": "Error message",
  "details": "Additional error details (optional)"
}
```

**Common HTTP Status Codes:**
- `400` - Bad Request (invalid input)
- `404` - Not Found
- `500` - Internal Server Error

## Rate Limiting

Currently no rate limiting is implemented. Future versions will include:
- 100 requests per minute for document ingestion
- 1000 requests per minute for read operations

## Examples

### Example: Set up news monitoring

```bash
# Create RSS feed monitor for AI news
curl -X POST http://localhost:8080/api/v1/ingestion/scheduled \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ai-news",
    "type": "rss",
    "url": "https://news.ycombinator.com/rss",
    "schedule": "0 */2 * * *",
    "filters": ["AI", "GPT", "neural", "machine learning"],
    "metadata": {
      "category": "ai-research"
    }
  }'

# Monitor the workflow
curl http://localhost:8080/api/v1/workflows/scheduled-ai-news-xxxxx
```

### Example: Batch import documents

```bash
# Import multiple research papers
curl -X POST http://localhost:8080/api/v1/ingestion/batch \
  -H "Content-Type: application/json" \
  -d '{
    "documents": [
      {
        "url": "https://arxiv.org/pdf/2301.00001.pdf",
        "type": "pdf",
        "metadata": {"topic": "transformers"}
      },
      {
        "url": "https://arxiv.org/pdf/2301.00002.pdf",
        "type": "pdf",
        "metadata": {"topic": "vision"}
      }
    ]
  }'
```

## Docker Compose Deployment

The API is available when running the Docker Compose stack:

```bash
# Production
docker-compose up -d

# Development with hot reload
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

The API will be available at `http://localhost:8080`.
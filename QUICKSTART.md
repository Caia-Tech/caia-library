# Caia Library Quick Start Guide

## Prerequisites

- Go 1.20 or higher
- Git
- Temporal CLI (`brew install temporal` on macOS)

## Setup

1. **Clone and build:**
```bash
git clone https://github.com/caiatech/caia-library
cd caia-library
go build -o caia-server ./cmd/server
```

2. **Start Temporal:**
```bash
temporal server start-dev
```

3. **Initialize Git repository for document storage:**
```bash
mkdir -p /tmp/caia-library-repo
cd /tmp/caia-library-repo
git init --initial-branch=main
git config user.email "library@caiatech.com"
git config user.name "Caia Library"
echo "# Document Repository" > README.md
git add . && git commit -m "Initial commit"
```

4. **Start Caia Library server:**
```bash
export Caia_REPO_PATH=/tmp/caia-library-repo
export PORT=8080
./caia-server
```

## Basic Usage

### Health Check
```bash
curl http://localhost:8080/health
```

### Ingest a Text Document
```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/document.txt",
    "type": "text",
    "metadata": {
      "source": "example",
      "category": "test"
    }
  }'
```

### Check Workflow Status
```bash
# Use the workflow_id from the ingestion response
curl http://localhost:8080/api/v1/workflows/{workflow_id}
```

### View Stored Documents
```bash
cd /tmp/caia-library-repo
git log --oneline
find documents -type f -name "*.txt" | head -10
```

## Document Types Supported

- `text` - Plain text files
- `html` - HTML documents (tags automatically stripped)
- `pdf` - PDF documents (coming soon)

## Monitoring

- Temporal UI: http://localhost:8233
- Server logs show all processing activity
- Git history shows all document ingestions

## Next Steps

1. Set up production Git repository
2. Configure authentication (when implemented)
3. Add custom extractors for your document types
4. Integrate with your data pipelines
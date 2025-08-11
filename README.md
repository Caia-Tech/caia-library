# Caia Library

**Experimental Git-Native Document Intelligence System**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go)](https://go.dev/)
[![Temporal](https://img.shields.io/badge/Temporal-Workflows-black?logo=temporal)](https://temporal.io/)

## Overview

Caia Library is an experimental document management system that leverages Git's immutable history and cryptographic integrity to create auditable, versioned document intelligence pipelines. Built with Temporal workflows and designed for human-in-the-loop operations, it provides a foundation for building trustworthy AI data systems.

**Focus on Ethical Academic Sources**: Caia Library specializes in collecting from academic sources that explicitly allow programmatic access, with proper attribution to Caia Tech and full compliance with terms of service.

> ⚠️ **Experimental Software**: This project is under active development and APIs may change. Use in production at your own risk.

## Key Features

### 🔒 Cryptographic Provenance
- Every document ingestion creates an immutable Git commit
- Complete audit trail of when, how, and from where data was collected
- Cryptographic proof of data integrity via Git's SHA-1 hashing
- No possibility of silent data corruption or tampering

### 🤖 Automated Intelligence Pipelines
- Temporal-based workflow orchestration for reliable processing
- Parallel text extraction and embedding generation
- Automatic retry logic for transient failures
- Extensible architecture for adding ML models and processors

### 🎓 Ethical Academic Collection
- **Only sources that allow programmatic access**: arXiv, PubMed Central, DOAJ, PLOS
- **Strict rate limiting**: Respects each source's API limits
- **Full attribution**: Every document credits both the source and Caia Tech
- **Transparent identification**: Clear User-Agent with contact information

### 📅 Scheduled & Batch Ingestion
- Cron-based scheduled collection from academic sources
- Batch processing for importing multiple documents efficiently
- Automatic deduplication to prevent redundant processing
- Configurable filters for targeted data collection

### 🚀 Easy Deployment
- Production-ready Docker Compose configuration
- Kubernetes manifests for cloud deployments
- Development mode with hot reload
- Built-in health checks and monitoring

### 👥 Human-in-the-Loop Design
- Git branches allow review before merging to main
- Clear commit messages document each ingestion
- Manual intervention points for quality control
- Transparent processing history

### 📊 Advanced Document Processing
- PDF text extraction with OCR support (planned)
- HTML content cleaning and metadata extraction
- 384-dimensional embeddings without external dependencies
- Extensible extractor and embedder interfaces

### 🔍 Git Query Language (GQL)
- **SQL-like syntax** for querying documents in Git
- **Attribution tracking** queries to ensure compliance
- **Time-travel queries** through Git history
- **Performance optimized** using Git's efficient storage

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   REST API  │────▶│   Temporal   │────▶│     Git     │
│  (Fiber)    │     │  Workflows   │     │ Repository  │
└─────────────┘     └──────────────┘     └─────────────┘
                            │
                    ┌───────┴────────┐
                    │                │
              ┌─────▼─────┐   ┌─────▼─────┐
              │   Text    │   │ Embedding │
              │ Extractor │   │ Generator │
              └───────────┘   └───────────┘
```

## Features

- **Git as Primary Database**: Leverages Git's distributed, immutable design
- **Document Versioning**: Full history of all document changes
- **Parallel Processing**: Simultaneous text extraction and embedding generation
- **Multiple Format Support**: Text, HTML, PDF
- **Scheduled Ingestion**: Automated collection from RSS, APIs, and websites
- **Batch Processing**: Import multiple documents efficiently
- **RESTful API**: Comprehensive HTTP interface for all operations
- **Workflow Tracking**: Monitor processing status via Temporal
- **Docker & Kubernetes**: Production-ready deployment options

## Quick Start

### Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/caiatech/caia-library
cd caia-library

# Start all services
docker-compose up -d

# Check service health
curl http://localhost:8080/health
```

### Development Mode

```bash
# Run with hot reload
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

### Manual Installation

```bash
# Install dependencies
go mod download

# Start Temporal
temporal server start-dev

# Run the server
go run ./cmd/server
```

## Usage

### Ingest a Document

```bash
# Collect an arXiv paper with proper attribution
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://arxiv.org/pdf/2301.00001.pdf",
    "type": "pdf",
    "metadata": {
      "source": "arXiv",
      "attribution": "Content from arXiv.org, collected by Caia Tech",
      "ethical_compliance": "true"
    }
  }'

# Set up scheduled arXiv collection (daily)
curl -X POST http://localhost:8080/api/v1/ingestion/scheduled \
  -H "Content-Type: application/json" \
  -d '{
    "name": "arxiv",
    "type": "arxiv",
    "url": "http://export.arxiv.org/api/query",
    "schedule": "0 2 * * *",
    "filters": ["cs.AI", "cs.LG"],
    "metadata": {
      "attribution": "Caia Tech"
    }
  }'
```

### Check Workflow Status

```bash
curl http://localhost:8080/api/v1/workflows/{workflow_id}
```

### Query Documents with Git Query Language

```bash
# Find all arXiv papers
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT FROM documents WHERE source = \"arXiv\" ORDER BY created_at DESC"}'

# Check attribution compliance
curl http://localhost:8080/api/v1/stats/attribution

# Search by content
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT FROM documents WHERE title ~ \"machine learning\" LIMIT 20"}'
```

## Data Integrity & Provenance

Every document in Caia Library maintains:

1. **Source Attribution**: Original URL or path
2. **Processing Timeline**: Timestamps for each stage
3. **Transformation History**: What was extracted/generated
4. **Error Documentation**: Any failures during processing
5. **Human Annotations**: Review notes and quality markers

## Documentation

- [API Reference](docs/API.md) - Complete API documentation
- [Git Query Language](docs/GIT_QUERY_LANGUAGE.md) - SQL-like queries for document discovery
- [Deployment Guide](docs/DEPLOYMENT.md) - Production deployment instructions
- [Ethical Scraping](docs/ETHICAL_SCRAPING.md) - Academic source compliance guide
- [Development Roadmap](docs/ROADMAP.md) - 10-week feature roadmap
- [Automated Collection](docs/AUTOMATED_COLLECTION.md) - Setting up data pipelines

## Future Roadmap

### Phase 1 (Completed)
- ✅ PDF support with basic detection
- ✅ Advanced embeddings (384-dimensional)
- ✅ Docker Compose deployment
- ✅ Scheduled ingestion workflows

### Phase 2 (Completed)
- ✅ Git Query Language for document discovery
- ✅ ONNX Runtime integration
- ✅ Full PDF text extraction with ledongthuc/pdf
- ✅ Git merge functionality with fast-forward support
- ✅ Concurrent operation safety with mutex protection

### Phase 3 (In Progress)
- [ ] Authentication & rate limiting
- [ ] Input validation and SSRF protection
- [ ] Monitoring and alerting integration
- [ ] Production hardening and optimization

### Phase 4 (Planned)
- [ ] Semantic search capabilities
- [ ] Multi-modal embeddings
- [ ] Differential privacy options
- [ ] Federated learning support

## Contributing

This is experimental software. Contributions welcome, but expect breaking changes.

## Security Considerations

- All documents stored in plaintext in Git
- No built-in encryption (use git-crypt if needed)
- Authentication not yet implemented
- API rate limiting not yet implemented
- SSRF protection not yet implemented

## Philosophy

Caia Library embraces the principle that AI systems should be:
- **Auditable**: Every decision traceable to source data
- **Reproducible**: Same inputs always produce same outputs
- **Transparent**: Clear visibility into data transformations
- **Correctable**: Errors can be identified and fixed
- **Attributable**: All data sources properly credited

By using Git as our foundation, we ensure these properties are not just features, but fundamental guarantees of the system architecture.

---

## Author

**Marvin Tutt**  
Chief Executive Officer, Caia Tech  
[owner@caiatech.com](mailto:owner@caiatech.com)

Built with 🧠 by Caia Tech - Experimental Intelligence Infrastructure
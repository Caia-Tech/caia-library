# CAIA Library

**Experimental Git-Native Document Intelligence System**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go)](https://go.dev/)
[![Temporal](https://img.shields.io/badge/Temporal-Workflows-black?logo=temporal)](https://temporal.io/)

## Overview

CAIA Library is an experimental document management system that leverages Git's immutable history and cryptographic integrity to create auditable, versioned document intelligence pipelines. Built with Temporal workflows and designed for human-in-the-loop operations, it provides a foundation for building trustworthy AI data systems.

> ⚠️ **Experimental Software**: This project is under active development and APIs may change. Use in production at your own risk.

## Key Strengths

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

### 👥 Human-in-the-Loop Design
- Git branches allow review before merging to main
- Clear commit messages document each ingestion
- Manual intervention points for quality control
- Transparent processing history

### 📊 Synthetic Data Generation Support
- Proper attribution tracking via metadata
- Source URL preservation for all documents
- Timestamp and processing metadata
- Clear tagging system for synthetic vs. real data

### 🎯 Hallucination & Error Tracking
- Failed ingestions tracked in Temporal workflow history
- Error states preserved for debugging
- Separate branches prevent bad data from polluting main
- Retry attempts logged with full error context

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
- **Multiple Format Support**: Text, HTML, PDF (coming soon)
- **RESTful API**: Simple HTTP interface for document ingestion
- **Workflow Tracking**: Monitor processing status via Temporal

## Installation

```bash
# Clone the repository
git clone https://github.com/caiatech/caia-library
cd caia-library

# Install dependencies
go mod download

# Start Temporal (required)
temporal server start-dev

# Build and run
go build -o caia-server ./cmd/server
./caia-server
```

## Usage

### Ingest a Document

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/document.pdf",
    "type": "pdf",
    "metadata": {
      "source": "web_scrape",
      "synthetic": "false"
    }
  }'
```

### Check Workflow Status

```bash
curl http://localhost:8080/api/v1/workflows/{workflow_id}
```

## Data Integrity & Provenance

Every document in CAIA Library maintains:

1. **Source Attribution**: Original URL or path
2. **Processing Timeline**: Timestamps for each stage
3. **Transformation History**: What was extracted/generated
4. **Error Documentation**: Any failures during processing
5. **Human Annotations**: Review notes and quality markers

## Future Roadmap

- [ ] ONNX Runtime integration for local embeddings
- [ ] PDF extraction via pdfcpu
- [ ] Structured data extraction
- [ ] Semantic search capabilities
- [ ] Differential privacy options
- [ ] Federated learning support

## Contributing

This is experimental software. Contributions welcome, but expect breaking changes.

## Security Considerations

- All documents stored in plaintext in Git
- No built-in encryption (use git-crypt if needed)
- Authentication not yet implemented
- Rate limiting not yet implemented

## Philosophy

CAIA Library embraces the principle that AI systems should be:
- **Auditable**: Every decision traceable to source data
- **Reproducible**: Same inputs always produce same outputs
- **Transparent**: Clear visibility into data transformations
- **Correctable**: Errors can be identified and fixed
- **Attributable**: All data sources properly credited

By using Git as our foundation, we ensure these properties are not just features, but fundamental guarantees of the system architecture.

---

## Author

**Marvin Tutt**  
Chief Executive Officer, CAIA Tech  
[owner@caiatech.com](mailto:owner@caiatech.com)

Built with 🧠 by CAIA Tech - Experimental Intelligence Infrastructure
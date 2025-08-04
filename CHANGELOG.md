# Changelog

All notable changes to Caia Library will be documented in this file.

## [Unreleased]

### Added
- Concurrent operation safety with mutex protection for Git operations
- Fast-forward merge support for optimal Git operations
- Comprehensive unit tests for core functionality
- Integration tests for complete workflows
- PDF text extraction using github.com/ledongthuc/pdf
- ONNX Runtime integration for embeddings generation
- Git Query Language (GQL) for document discovery
- Academic source collectors with proper attribution
- Retry policies with maximum 3 attempts to prevent infinite loops

### Fixed
- Git merge "clean working tree" error when merging branches
- Race conditions in concurrent document processing
- Test compilation errors with correct type usage
- Import path inconsistencies throughout codebase

### Changed
- Renamed all instances of "CAIA" to "Caia" for consistency
- Improved merge logic to handle fast-forward merges when possible
- Updated Temporal activity retry policies to prevent resource exhaustion

### Security
- Added non-retryable error types to prevent infinite retry loops
- Implemented proper content-type validation for documents

## [0.1.0] - Initial Release

### Added
- Git-native document storage with immutable history
- Temporal workflow orchestration
- REST API with Fiber framework
- Docker Compose deployment configuration
- Kubernetes manifests for cloud deployment
- Basic PDF and text document support
- 384-dimensional embeddings generation
- Scheduled ingestion workflows
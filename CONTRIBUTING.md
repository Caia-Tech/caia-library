# Contributing to CAIA Library

First off, thanks for taking the time to contribute! 🎉

## Code of Conduct

Be excellent to each other. We're building infrastructure for trustworthy AI - let's be trustworthy humans.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues. When you create a bug report, include:

- **Clear title and description**
- **Steps to reproduce**
- **Expected vs actual behavior**
- **Logs/screenshots if applicable**
- **Environment details** (OS, Go version, etc.)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- **Use a clear title**
- **Provide a detailed description**
- **Explain why this enhancement would be useful**
- **List any alternatives you've considered**

### Pull Requests

1. Fork the repo and create your branch from `main`
2. If you've added code, add tests
3. Ensure the test suite passes: `go test ./...`
4. Make sure your code follows Go conventions: `go fmt ./...`
5. Issue that pull request!

## Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/caia-library
cd caia-library

# Add upstream remote
git remote add upstream https://github.com/caiatech/caia-library

# Install dependencies
go mod download

# Run tests
go test ./...

# Run with race detector
go test -race ./...
```

## Project Structure

```
caia-library/
├── cmd/server/         # Server entry point
├── internal/           # Private application code
│   ├── api/           # HTTP handlers
│   ├── git/           # Git operations
│   └── temporal/      # Workflows & activities
├── pkg/               # Public packages
│   ├── document/      # Document types
│   ├── extractor/     # Text extraction
│   └── embedder/      # Embedding generation
└── test/              # Integration tests
```

## Adding New Document Types

1. Add extractor in `pkg/extractor/`
2. Register in `NewEngine()` map
3. Add tests
4. Update API documentation

## Testing

- Unit tests: `go test ./pkg/...`
- Integration tests: `INTEGRATION_TEST=true go test ./test/integration/...`
- Always test with Temporal running: `temporal server start-dev`

## Commit Message Format

We use conventional commits:

```
feat: add PDF extraction support
fix: handle empty documents correctly
docs: update API examples
test: add integration tests for HTML extraction
refactor: simplify embedding generation
```

## Questions?

Feel free to open an issue for any questions. We're building something unique here - your perspective matters!
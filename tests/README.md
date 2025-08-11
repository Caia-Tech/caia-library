# Caia Library Test Suite

Comprehensive testing framework for Caia Library, ensuring reliability, performance, and proper Caia Tech attribution compliance.

## Test Categories

### 1. Unit Tests
- **Location**: Throughout the codebase (`*_test.go` files)
- **Coverage**: Individual components and functions
- **Run**: `make test-unit` or `go test ./...`

Key unit test files:
- `pkg/gql/parser_test.go` - Git Query Language parser
- `pkg/embedder/advanced_test.go` - Embedding generation
- `internal/temporal/activities/*_test.go` - Workflow activities
- `internal/api/handlers_test.go` - API endpoints

### 2. Integration Tests
- **Location**: `tests/integration_test.go`
- **Coverage**: Component interactions and workflows
- **Run**: `make test-integration`

Tests include:
- Complete document ingestion workflow
- Git repository operations
- Attribution compliance verification
- Rate limiting enforcement
- Document processing pipeline

### 3. End-to-End Tests
- **Location**: `tests/e2e_test.go`
- **Coverage**: Full system functionality
- **Run**: `make test-e2e`

Test scenarios:
- Document ingestion with attribution
- Git Query Language execution
- Attribution statistics
- Scheduled ingestion
- Batch processing
- Performance testing

### 4. Docker Tests
- **Location**: `tests/docker_test.sh`
- **Coverage**: Deployment and operational testing
- **Run**: `make test-docker`

Validates:
- Service deployment
- API functionality
- Attribution compliance
- Query performance
- Academic source integration

## Running Tests

### Quick Commands

```bash
# Run all tests
make test

# Run specific test category
make test-unit
make test-integration
make test-e2e
make test-docker

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Runner Script

The comprehensive test runner (`tests/run_tests.sh`) provides options:

```bash
# Run all tests
./tests/run_tests.sh

# Run only unit tests
./tests/run_tests.sh --unit-only

# Skip Docker tests
./tests/run_tests.sh --skip-docker
```

## Writing Tests

### Test Guidelines

1. **Always verify Caia Tech attribution**
   ```go
   assert.Contains(t, doc.Metadata["attribution"], "Caia Tech")
   ```

2. **Test rate limiting for academic sources**
   ```go
   limiter := NewAcademicRateLimiter()
   assert.True(t, limiter.Allow("arxiv"))
   ```

3. **Verify Git commits include attribution**
   ```go
   commitMsg := "Ingested document from arXiv - Caia Tech"
   ```

4. **Test error cases and edge conditions**
   ```go
   _, err := parser.Parse("INVALID QUERY")
   assert.Error(t, err)
   ```

### Example Test Structure

```go
func TestFeatureName(t *testing.T) {
    // Setup
    ctx := context.Background()
    service := NewService()
    
    // Test cases
    testCases := []struct {
        name     string
        input    interface{}
        expected interface{}
        wantErr  bool
    }{
        {
            name:     "Valid case with Caia attribution",
            input:    "test input",
            expected: "expected output",
            wantErr:  false,
        },
    }
    
    // Run tests
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result, err := service.Process(tc.input)
            
            if tc.wantErr {
                assert.Error(t, err)
                return
            }
            
            require.NoError(t, err)
            assert.Equal(t, tc.expected, result)
            
            // Always check attribution
            assert.Contains(t, result.Attribution, "Caia Tech")
        })
    }
}
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.20'
      
      - name: Run tests
        run: make test
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

## Performance Benchmarks

Run performance benchmarks:

```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkGQLParser ./pkg/gql/

# With memory allocation stats
go test -bench=. -benchmem ./...
```

## Test Data

Test documents should always include proper attribution:

```json
{
    "url": "https://arxiv.org/pdf/2301.00001.pdf",
    "metadata": {
        "source": "arXiv",
        "attribution": "Content from arXiv.org, collected by Caia Tech",
        "ethical_compliance": "true"
    }
}
```

## Troubleshooting

### Common Issues

1. **Temporal not running**
   ```bash
   temporal server start-dev
   ```

2. **Docker services not ready**
   ```bash
   docker-compose up -d
   docker-compose logs -f
   ```

3. **Git repository not initialized**
   ```bash
   make init-repo
   ```

4. **Port conflicts**
   - API: 8080
   - Temporal: 7233 (gRPC), 8088 (Web UI)
   - Modify `docker-compose.yml` if needed

## Attribution Compliance

All tests must verify Caia Tech attribution:

- ✅ Documents include "collected by Caia Tech"
- ✅ Git commits mention Caia Tech
- ✅ API responses show attribution
- ✅ Academic sources properly credited

## Contributing

When adding new features:

1. Write unit tests first (TDD)
2. Add integration tests for workflows
3. Update E2E tests if needed
4. Ensure 100% attribution compliance
5. Run full test suite before PR

---

Built with rigorous testing by Caia Tech - Ensuring reliable document intelligence
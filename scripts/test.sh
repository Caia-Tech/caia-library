#!/bin/bash
# Caia Library Test Runner

set -e

echo "=========================================="
echo "Caia Library Unit Test Suite"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if running in CI
if [ -n "$CI" ]; then
    echo "Running in CI mode..."
    COVERAGE_FLAG="-coverprofile=coverage.out"
else
    COVERAGE_FLAG=""
fi

# Function to run tests for a package
run_tests() {
    local package=$1
    local name=$2
    
    echo -e "${YELLOW}Testing ${name}...${NC}"
    if go test -v -race -short ${COVERAGE_FLAG} ${package}; then
        echo -e "${GREEN}✓ ${name} tests passed${NC}"
    else
        echo -e "${RED}✗ ${name} tests failed${NC}"
        exit 1
    fi
    echo ""
}

# Run tests for each major component
echo "1. Testing Academic Collector (Ethical Scraping)"
run_tests "./internal/temporal/activities" "Academic Collector"

echo "2. Testing Rate Limiter"
run_tests "./pkg/ratelimit" "Rate Limiter"

echo "3. Testing Git Repository"
run_tests "./internal/git" "Git Repository"

echo "4. Testing Document Embedder"
run_tests "./pkg/embedder" "Document Embedder"

echo "5. Testing API Handlers"
run_tests "./internal/api" "API Handlers"

echo "6. Testing Document Package"
run_tests "./pkg/document" "Document Package"

echo "7. Testing Extractors"
run_tests "./pkg/extractor" "Text Extractors"

# Summary
echo "=========================================="
echo -e "${GREEN}All tests passed!${NC}"
echo "=========================================="

# Generate coverage report if not in CI
if [ -z "$CI" ] && [ -f "coverage.out" ]; then
    echo ""
    echo "Generating coverage report..."
    go tool cover -html=coverage.out -o coverage.html
    echo "Coverage report available at: coverage.html"
fi

# Run specific academic scraping tests
echo ""
echo "Running focused academic scraping tests..."
echo "=========================================="
go test -v -race ./internal/temporal/activities -run "TestAcademicCollector.*Attribution"
go test -v -race ./pkg/ratelimit -run "TestAcademicRateLimiter.*"

echo ""
echo -e "${GREEN}✓ All ethical scraping tests passed${NC}"
echo "  - Proper Caia Tech attribution verified"
echo "  - Rate limiting enforcement confirmed"
echo "  - Academic source compliance checked"
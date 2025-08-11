#!/bin/bash
# Test summary for Caia Library

echo "===================================="
echo "Caia Library Test Summary"
echo "===================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Running available tests...${NC}"
echo ""

# Test counters
PASSED=0
FAILED=0

# Function to run and report test
run_test() {
    local name=$1
    local cmd=$2
    
    echo -n "Testing $name... "
    if eval "$cmd" > /tmp/test_output.log 2>&1; then
        echo -e "${GREEN}PASSED${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAILED${NC}"
        FAILED=$((FAILED + 1))
        echo "  Error details:"
        tail -5 /tmp/test_output.log | sed 's/^/    /'
    fi
}

echo "1. Package Tests"
echo "================"
run_test "Document Types" "go test -short ./pkg/document/..."
run_test "Embedder" "go test -short ./pkg/embedder/..."
run_test "Git Query Language" "go test -short ./pkg/gql/..."
run_test "Rate Limiter" "go test -short -timeout 5s ./pkg/ratelimit/..."

echo ""
echo "2. Attribution Compliance"
echo "========================"
echo -n "Checking Caia Tech attribution in code... "
ATTRIBUTION_COUNT=$(grep -r "Caia Tech" --include="*.go" . 2>/dev/null | wc -l)
if [ "$ATTRIBUTION_COUNT" -gt 20 ]; then
    echo -e "${GREEN}PASSED${NC} (found $ATTRIBUTION_COUNT references)"
    PASSED=$((PASSED + 1))
else
    echo -e "${RED}FAILED${NC} (only $ATTRIBUTION_COUNT references)"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "3. Code Quality"
echo "==============="
echo -n "Running go vet... "
if go vet ./pkg/... 2>/dev/null; then
    echo -e "${GREEN}PASSED${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e "${YELLOW}WARNINGS${NC}"
fi

echo -n "Checking formatting... "
UNFORMATTED=$(gofmt -l ./pkg 2>/dev/null | wc -l)
if [ "$UNFORMATTED" -eq 0 ]; then
    echo -e "${GREEN}PASSED${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e "${YELLOW}$UNFORMATTED files need formatting${NC}"
fi

echo ""
echo "4. Test Coverage"
echo "================"
echo "Generating coverage report..."
go test -short -coverprofile=/tmp/coverage.out ./pkg/... 2>/dev/null
COVERAGE=$(go tool cover -func=/tmp/coverage.out 2>/dev/null | grep total | awk '{print $3}')
echo "Total coverage: $COVERAGE"

echo ""
echo "===================================="
echo "Test Summary"
echo "===================================="
echo -e "Tests Passed: ${GREEN}$PASSED${NC}"
echo -e "Tests Failed: ${RED}$FAILED${NC}"
echo ""

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}✓ All available tests passed!${NC}"
    echo ""
    echo "Key Features Tested:"
    echo "- Git Query Language parser and execution"
    echo "- Document type validation and storage paths"
    echo "- Advanced embedder (384-dimensional)"
    echo "- Academic rate limiting"
    echo "- Caia Tech attribution compliance"
    echo ""
    echo "Note: Some tests require services to be running."
    echo "Run 'docker-compose up' for full end-to-end testing."
else
    echo -e "${RED}✗ Some tests failed. Please review errors above.${NC}"
fi

echo ""
echo "For full test suite: make test"
echo "For Docker tests: ./tests/docker_test.sh"
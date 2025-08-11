#!/bin/bash
# Comprehensive test runner for Caia Library

set -e

echo "========================================"
echo "Caia Library Comprehensive Test Runner"
echo "========================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test categories
UNIT_TESTS=true
INTEGRATION_TESTS=true
E2E_TESTS=true
DOCKER_TESTS=true

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --unit-only)
            INTEGRATION_TESTS=false
            E2E_TESTS=false
            DOCKER_TESTS=false
            ;;
        --integration-only)
            UNIT_TESTS=false
            E2E_TESTS=false
            DOCKER_TESTS=false
            ;;
        --e2e-only)
            UNIT_TESTS=false
            INTEGRATION_TESTS=false
            DOCKER_TESTS=false
            ;;
        --docker-only)
            UNIT_TESTS=false
            INTEGRATION_TESTS=false
            E2E_TESTS=false
            ;;
        --skip-docker)
            DOCKER_TESTS=false
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--unit-only|--integration-only|--e2e-only|--docker-only|--skip-docker]"
            exit 1
            ;;
    esac
    shift
done

# Function to run tests
run_test_suite() {
    local name=$1
    local command=$2
    
    echo -e "${YELLOW}Running $name...${NC}"
    echo "================================"
    
    if eval "$command"; then
        echo -e "${GREEN}✓ $name passed${NC}"
        return 0
    else
        echo -e "${RED}✗ $name failed${NC}"
        return 1
    fi
    echo ""
}

# Track failures
FAILED=0

# Run unit tests
if [ "$UNIT_TESTS" = true ]; then
    echo ""
    echo "1. UNIT TESTS"
    echo "============="
    
    # Run all unit tests with coverage
    if ! run_test_suite "Unit Tests" "go test -v -race -coverprofile=coverage.out ./..."; then
        FAILED=$((FAILED + 1))
    fi
    
    # Generate coverage report
    echo -e "${YELLOW}Generating coverage report...${NC}"
    go tool cover -html=coverage.out -o coverage.html
    total_coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    echo -e "Total coverage: ${GREEN}$total_coverage${NC}"
    echo ""
fi

# Run integration tests
if [ "$INTEGRATION_TESTS" = true ]; then
    echo ""
    echo "2. INTEGRATION TESTS"
    echo "===================="
    
    # Start Temporal for integration tests
    echo -e "${YELLOW}Starting Temporal server...${NC}"
    temporal server start-dev > /tmp/temporal.log 2>&1 &
    TEMPORAL_PID=$!
    
    # Wait for Temporal
    sleep 5
    
    # Run integration tests
    if ! run_test_suite "Integration Tests" "go test -v -tags=integration ./tests/integration_test.go"; then
        FAILED=$((FAILED + 1))
    fi
    
    # Stop Temporal
    kill $TEMPORAL_PID 2>/dev/null || true
fi

# Run E2E tests
if [ "$E2E_TESTS" = true ]; then
    echo ""
    echo "3. END-TO-END TESTS"
    echo "==================="
    
    # Check if services are running
    if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${YELLOW}Services already running${NC}"
    else
        echo -e "${YELLOW}Starting services for E2E tests...${NC}"
        docker-compose up -d
        
        # Wait for services
        echo -n "Waiting for services to be ready..."
        attempts=0
        while [ $attempts -lt 30 ]; do
            if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
                echo -e " ${GREEN}ready${NC}"
                break
            fi
            echo -n "."
            sleep 2
            attempts=$((attempts + 1))
        done
    fi
    
    # Run E2E tests
    if ! run_test_suite "E2E Tests" "go test -v -tags=e2e ./tests/e2e_test.go"; then
        FAILED=$((FAILED + 1))
    fi
fi

# Run Docker tests
if [ "$DOCKER_TESTS" = true ]; then
    echo ""
    echo "4. DOCKER TESTS"
    echo "==============="
    
    if ! run_test_suite "Docker Tests" "./tests/docker_test.sh"; then
        FAILED=$((FAILED + 1))
    fi
fi

# Linting and formatting
echo ""
echo "5. CODE QUALITY CHECKS"
echo "======================"

# Run gofmt
echo -e "${YELLOW}Checking code formatting...${NC}"
if ! gofmt -l . | grep -q .; then
    echo -e "${GREEN}✓ Code is properly formatted${NC}"
else
    echo -e "${RED}✗ Code formatting issues found${NC}"
    echo "Run 'gofmt -w .' to fix"
    FAILED=$((FAILED + 1))
fi

# Run go vet
echo -e "${YELLOW}Running go vet...${NC}"
if go vet ./...; then
    echo -e "${GREEN}✓ go vet passed${NC}"
else
    echo -e "${RED}✗ go vet found issues${NC}"
    FAILED=$((FAILED + 1))
fi

# Run golint (if available)
if command -v golint &> /dev/null; then
    echo -e "${YELLOW}Running golint...${NC}"
    if golint ./... | grep -q .; then
        echo -e "${YELLOW}⚠ golint found suggestions${NC}"
    else
        echo -e "${GREEN}✓ golint passed${NC}"
    fi
fi

# Attribution compliance check
echo ""
echo "6. ATTRIBUTION COMPLIANCE"
echo "========================="

echo -e "${YELLOW}Checking Caia Tech attribution in code...${NC}"
attribution_count=$(grep -r "Caia Tech" --include="*.go" . | wc -l)
if [ "$attribution_count" -gt 10 ]; then
    echo -e "${GREEN}✓ Found $attribution_count Caia Tech attributions${NC}"
else
    echo -e "${RED}✗ Insufficient Caia Tech attributions (found $attribution_count)${NC}"
    FAILED=$((FAILED + 1))
fi

# Final report
echo ""
echo "========================================"
echo "TEST SUMMARY"
echo "========================================"

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "Caia Library is ready for deployment."
    echo "Remember to always include proper attribution to Caia Tech."
    exit 0
else
    echo -e "${RED}✗ $FAILED test suite(s) failed${NC}"
    echo ""
    echo "Please fix the failing tests before deployment."
    exit 1
fi
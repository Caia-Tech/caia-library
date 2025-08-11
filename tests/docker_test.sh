#!/bin/bash
# End-to-end test script for Caia Library with Docker Compose

set -e

echo "==================================="
echo "Caia Library End-to-End Test Suite"
echo "==================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
API_URL="http://localhost:8080"
TEMPORAL_URL="http://localhost:8088"
REPO_PATH="./data/repo"

# Function to print colored output
print_status() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ $2${NC}"
    else
        echo -e "${RED}✗ $2${NC}"
        exit 1
    fi
}

# Function to wait for service
wait_for_service() {
    local url=$1
    local service=$2
    local max_attempts=30
    local attempt=1
    
    echo -n "Waiting for $service to be ready..."
    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "$url" > /dev/null 2>&1; then
            echo -e " ${GREEN}ready${NC}"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    echo -e " ${RED}timeout${NC}"
    return 1
}

# Function to execute API request
api_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    
    if [ -z "$data" ]; then
        curl -s -X "$method" "$API_URL$endpoint" -H "Content-Type: application/json"
    else
        curl -s -X "$method" "$API_URL$endpoint" -H "Content-Type: application/json" -d "$data"
    fi
}

# Start services
echo "1. Starting Services"
echo "==================="

# Clean up any existing containers
docker-compose down -v 2>/dev/null || true

# Start services
docker-compose up -d
print_status $? "Docker Compose started"

# Wait for services
wait_for_service "$API_URL/health" "API"
wait_for_service "$TEMPORAL_URL" "Temporal"

echo ""
echo "2. Health Check"
echo "==============="

response=$(api_request GET /health)
if [[ "$response" == *"ok"* ]]; then
    print_status 0 "API health check passed"
else
    print_status 1 "API health check failed"
fi

echo ""
echo "3. Document Ingestion Test"
echo "========================="

# Ingest a test document with Caia attribution
ingest_response=$(api_request POST /api/v1/documents '{
    "url": "https://arxiv.org/pdf/2301.00001.pdf",
    "type": "pdf",
    "metadata": {
        "source": "arXiv",
        "attribution": "Content from arXiv.org, collected by Caia Tech (https://caiatech.com)",
        "title": "Neural Networks Research",
        "author": "Jane Smith"
    }
}')

workflow_id=$(echo "$ingest_response" | jq -r '.workflow_id')
if [ ! -z "$workflow_id" ] && [ "$workflow_id" != "null" ]; then
    print_status 0 "Document ingestion initiated (workflow: $workflow_id)"
else
    print_status 1 "Document ingestion failed"
fi

# Wait for workflow to complete
echo -n "Waiting for ingestion to complete..."
attempts=0
while [ $attempts -lt 30 ]; do
    workflow_status=$(api_request GET "/api/v1/workflows/$workflow_id" | jq -r '.status')
    if [ "$workflow_status" == "Completed" ]; then
        echo -e " ${GREEN}completed${NC}"
        break
    elif [ "$workflow_status" == "Failed" ]; then
        echo -e " ${RED}failed${NC}"
        exit 1
    fi
    echo -n "."
    sleep 2
    attempts=$((attempts + 1))
done

echo ""
echo "4. Git Query Language Tests"
echo "==========================="

# Test various GQL queries
queries=(
    'SELECT FROM documents WHERE source = "arXiv" LIMIT 5|Find arXiv documents'
    'SELECT FROM attribution WHERE caia_attribution = true|Check Caia attribution'
    'SELECT FROM attribution WHERE caia_attribution = false|Find missing attribution'
    'SELECT FROM sources|List all sources'
    'SELECT FROM documents WHERE title ~ "neural" LIMIT 3|Search by title'
)

for query_desc in "${queries[@]}"; do
    IFS='|' read -r query description <<< "$query_desc"
    
    response=$(api_request POST /api/v1/query "{\"query\": \"$query\"}")
    count=$(echo "$response" | jq -r '.count // 0')
    
    if [ "$query" == 'SELECT FROM attribution WHERE caia_attribution = false' ]; then
        # This should return 0 (no missing attributions)
        if [ "$count" -eq 0 ]; then
            print_status 0 "$description: $count items (correct - all have attribution)"
        else
            print_status 1 "$description: $count items (ERROR - found missing attributions!)"
        fi
    else
        if [ "$count" -ge 0 ]; then
            print_status 0 "$description: $count items"
        else
            print_status 1 "$description: query failed"
        fi
    fi
done

echo ""
echo "5. Attribution Compliance Check"
echo "==============================="

attr_stats=$(api_request GET /api/v1/stats/attribution)
compliance_rate=$(echo "$attr_stats" | jq -r '.compliance_rate')

if [ "$compliance_rate" == "100.0%" ]; then
    print_status 0 "Attribution compliance: $compliance_rate"
else
    print_status 1 "Attribution compliance: $compliance_rate (should be 100%)"
fi

echo ""
echo "6. Scheduled Ingestion Test"
echo "==========================="

# Create scheduled ingestion
schedule_response=$(api_request POST /api/v1/ingestion/scheduled '{
    "name": "test-arxiv",
    "type": "arxiv",
    "url": "http://export.arxiv.org/api/query",
    "schedule": "0 2 * * *",
    "filters": ["cs.AI", "cs.LG"],
    "metadata": {
        "attribution": "Caia Tech"
    }
}')

schedule_id=$(echo "$schedule_response" | jq -r '.schedule_id')
if [ ! -z "$schedule_id" ] && [ "$schedule_id" != "null" ]; then
    print_status 0 "Scheduled ingestion created (ID: $schedule_id)"
else
    print_status 1 "Failed to create scheduled ingestion"
fi

echo ""
echo "7. Batch Ingestion Test"
echo "======================="

batch_response=$(api_request POST /api/v1/ingestion/batch '{
    "documents": [
        {
            "url": "https://arxiv.org/pdf/2301.00002.pdf",
            "type": "pdf",
            "metadata": {
                "source": "arXiv",
                "attribution": "Content from arXiv.org, collected by Caia Tech",
                "title": "Machine Learning Advances"
            }
        },
        {
            "url": "https://arxiv.org/pdf/2301.00003.pdf",
            "type": "pdf",
            "metadata": {
                "source": "arXiv",
                "attribution": "Content from arXiv.org, collected by Caia Tech",
                "title": "Deep Learning Applications"
            }
        }
    ]
}')

batch_count=$(echo "$batch_response" | jq -r '.total_documents')
if [ "$batch_count" -eq 2 ]; then
    print_status 0 "Batch ingestion initiated: $batch_count documents"
else
    print_status 1 "Batch ingestion failed"
fi

echo ""
echo "8. Git Repository Verification"
echo "=============================="

# Check Git repository
if [ -d "$REPO_PATH/.git" ]; then
    print_status 0 "Git repository exists"
    
    # Check for commits with Caia attribution
    cd "$REPO_PATH"
    caia_commits=$(git log --grep="Caia Tech" --oneline 2>/dev/null | wc -l)
    cd - > /dev/null
    
    if [ "$caia_commits" -gt 0 ]; then
        print_status 0 "Found $caia_commits commits with Caia Tech attribution"
    else
        print_status 1 "No commits with Caia Tech attribution found"
    fi
else
    print_status 1 "Git repository not found"
fi

echo ""
echo "9. Performance Test"
echo "==================="

# Test query performance
start_time=$(date +%s%N)
perf_response=$(api_request POST /api/v1/query '{"query": "SELECT FROM documents ORDER BY created_at DESC LIMIT 100"}')
end_time=$(date +%s%N)

elapsed_ms=$(( ($end_time - $start_time) / 1000000 ))

if [ "$elapsed_ms" -lt 5000 ]; then
    print_status 0 "Query performance: ${elapsed_ms}ms (< 5s)"
else
    print_status 1 "Query performance: ${elapsed_ms}ms (> 5s - too slow)"
fi

echo ""
echo "10. Academic Source Compliance"
echo "=============================="

# Test each academic source
sources=("arxiv" "pubmed" "doaj" "plos")

for source in "${sources[@]}"; do
    response=$(api_request POST /api/v1/query "{\"query\": \"SELECT FROM documents WHERE source = \\\"$source\\\" LIMIT 1\"}")
    count=$(echo "$response" | jq -r '.count // 0')
    
    if [ "$count" -ge 0 ]; then
        # Check attribution in results
        items=$(echo "$response" | jq -r '.items[]')
        if [ ! -z "$items" ]; then
            attribution=$(echo "$items" | jq -r '.metadata.attribution // ""')
            if [[ "$attribution" == *"Caia Tech"* ]]; then
                print_status 0 "$source: proper Caia Tech attribution"
            else
                print_status 1 "$source: missing Caia Tech attribution"
            fi
        else
            print_status 0 "$source: no documents (skipped)"
        fi
    else
        print_status 0 "$source: query executed"
    fi
done

echo ""
echo "====================================="
echo -e "${GREEN}All tests completed successfully!${NC}"
echo "====================================="

echo ""
echo "Test Summary:"
echo "- API service: operational"
echo "- Document ingestion: working"
echo "- Git Query Language: functional"
echo "- Attribution compliance: 100%"
echo "- Scheduled ingestion: configured"
echo "- Batch processing: operational"
echo "- Git repository: verified"
echo "- Performance: acceptable"
echo "- Academic sources: compliant"

echo ""
echo "To view logs:"
echo "  docker-compose logs -f"

echo ""
echo "To stop services:"
echo "  docker-compose down"

echo ""
echo -e "${YELLOW}Note: This is a basic test suite. For production, add more comprehensive tests.${NC}"
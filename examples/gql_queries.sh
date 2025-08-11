#!/bin/bash
# Example Git Query Language (GQL) queries for Caia Library

API_URL="http://localhost:8080/api/v1"

echo "Caia Library Git Query Language Examples"
echo "========================================"
echo ""

# Helper function to execute query
run_query() {
    local query="$1"
    local description="$2"
    
    echo "Query: $description"
    echo "GQL: $query"
    echo "Response:"
    curl -s -X POST ${API_URL}/query \
        -H "Content-Type: application/json" \
        -d "{\"query\": \"$query\"}" | jq '.'
    echo ""
    echo "---"
    echo ""
}

# 1. Get example queries
echo "1. Available Query Examples"
echo "=========================="
curl -s ${API_URL}/query/examples | jq '.'
echo ""

# 2. Find all documents from arXiv
run_query \
    'SELECT FROM documents WHERE source = "arXiv" LIMIT 5' \
    "Find arXiv papers (limit 5)"

# 3. Search for AI/ML papers
run_query \
    'SELECT FROM documents WHERE title ~ "artificial intelligence" OR title ~ "machine learning" LIMIT 10' \
    "Search for AI/ML papers"

# 4. Recent documents (last 7 days)
WEEK_AGO=$(date -d '7 days ago' '+%Y-%m-%d' 2>/dev/null || date -v -7d '+%Y-%m-%d')
run_query \
    "SELECT FROM documents WHERE created_at > \"$WEEK_AGO\" ORDER BY created_at DESC" \
    "Documents from the last 7 days"

# 5. Check attribution compliance
run_query \
    'SELECT FROM attribution WHERE caia_attribution = true' \
    "Sources with proper Caia Tech attribution"

# 6. Find sources missing attribution
run_query \
    'SELECT FROM attribution WHERE caia_attribution = false' \
    "Sources MISSING Caia Tech attribution (should be empty!)"

# 7. List all document sources
run_query \
    'SELECT FROM sources' \
    "All document sources with counts"

# 8. Top authors
run_query \
    'SELECT FROM authors ORDER BY count DESC LIMIT 10' \
    "Top 10 authors by document count"

# 9. Documents by specific author
run_query \
    'SELECT FROM documents WHERE authors ~ "Smith" LIMIT 5' \
    "Papers by authors named Smith"

# 10. Attribution statistics
echo "10. Attribution Compliance Statistics"
echo "===================================="
curl -s ${API_URL}/stats/attribution | jq '.'
echo ""

# Advanced queries
echo "Advanced Query Examples"
echo "======================"
echo ""

# Complex multi-condition query
run_query \
    'SELECT FROM documents WHERE source = "arXiv" AND title ~ "neural" AND created_at > "2024-01-01" ORDER BY created_at DESC LIMIT 10' \
    "Recent arXiv papers about neural networks"

# Programmatic query building example
echo "Programmatic Query Building (Go example):"
echo "----------------------------------------"
cat << 'EOF'
query := gql.NewQueryBuilder(gql.QueryDocuments).
    Where("source", gql.OpEquals, "arXiv").
    Where("caia_attribution", gql.OpEquals, true).
    OrderBy("created_at", true).
    Limit(20).
    Build()
// Result: SELECT FROM documents WHERE source = "arXiv" AND caia_attribution = true ORDER BY created_at DESC LIMIT 20
EOF
echo ""

# Performance tips
echo "Performance Tips"
echo "================"
echo "1. Always use LIMIT to avoid loading entire repository"
echo "2. Add specific filters to narrow results"
echo "3. Use indexed fields (source, created_at) for faster queries"
echo "4. Time-based queries should use narrow date ranges"
echo ""

# Attribution reminder
echo "Caia Tech Attribution Policy"
echo "==========================="
echo "All documents collected by Caia Library MUST include:"
echo '- Source attribution (e.g., "Content from arXiv.org")'
echo '- Caia Tech attribution: "collected by Caia Tech (https://caiatech.com)"'
echo "- Compliance with source terms of service"
echo ""
echo "Use GQL to monitor and ensure 100% attribution compliance!"
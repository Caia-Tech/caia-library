#!/bin/bash

echo "=== Caia Library Full Test Suite ==="

# Setup
echo -e "\n1. Setting up test environment..."
mkdir -p /tmp/caia-library-repo
cd /tmp/caia-library-repo
rm -rf .git
git init --initial-branch=main
git config user.email "library@caiatech.com"
git config user.name "Caia Library"
echo "# Caia Library Repository" > README.md
git add README.md
git commit -m "Initial commit"
cd -

# Start the server
echo -e "\n2. Starting Caia Library server..."
export Caia_REPO_PATH=/tmp/caia-library-repo
export PORT=8092
./caia-server &
SERVER_PID=$!

echo "Server started with PID: $SERVER_PID"
echo "Waiting for server to be ready..."
sleep 5

# Test health endpoint
echo -e "\n3. Testing health endpoint..."
curl -s http://localhost:8092/health | jq .

# Create a test text file
echo -e "\n4. Creating test content..."
echo "This is a test document for Caia Library. It contains important information about testing." > /tmp/test-doc.txt
python3 -m http.server 8888 --directory /tmp &
HTTP_PID=$!
sleep 2

# Test document ingestion with local file
echo -e "\n5. Testing document ingestion (text file)..."
RESPONSE=$(curl -s -X POST http://localhost:8092/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "http://localhost:8888/test-doc.txt",
    "type": "text",
    "metadata": {
      "source": "test",
      "category": "testing"
    }
  }')

echo "$RESPONSE" | jq .
WORKFLOW_ID=$(echo "$RESPONSE" | jq -r .workflow_id)

# Wait for workflow to complete
echo -e "\n6. Waiting for workflow to complete..."
sleep 5

# Check workflow status
echo -e "\n7. Checking workflow status..."
curl -s http://localhost:8092/api/v1/workflows/$WORKFLOW_ID | jq .

# Test HTML document ingestion
echo -e "\n8. Creating HTML test content..."
cat > /tmp/test-doc.html << EOF
<!DOCTYPE html>
<html>
<head><title>Test Document</title></head>
<body>
<h1>Caia Library Test</h1>
<p>This is an HTML document for testing the extraction capabilities.</p>
<script>console.log('This should be removed');</script>
<style>body { color: black; }</style>
</body>
</html>
EOF

echo -e "\n9. Testing HTML document ingestion..."
HTML_RESPONSE=$(curl -s -X POST http://localhost:8092/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "http://localhost:8888/test-doc.html",
    "type": "html",
    "metadata": {
      "source": "test",
      "format": "html"
    }
  }')

echo "$HTML_RESPONSE" | jq .
HTML_WORKFLOW_ID=$(echo "$HTML_RESPONSE" | jq -r .workflow_id)

# Wait and check
sleep 5
echo -e "\n10. Checking HTML workflow status..."
curl -s http://localhost:8092/api/v1/workflows/$HTML_WORKFLOW_ID | jq .

# Check Git repository
echo -e "\n11. Checking Git repository contents..."
cd /tmp/caia-library-repo
echo "Git branches:"
git branch -a
echo -e "\nGit log:"
git log --oneline --all --graph | head -20
echo -e "\nRepository structure:"
find . -type f -not -path './.git/*' | head -20

# Check if documents were stored
echo -e "\n12. Checking stored documents..."
if [ -d "documents" ]; then
  echo "Documents directory exists!"
  find documents -type f | head -10
else
  echo "No documents directory found yet. Checking branches..."
  git branch -r | while read branch; do
    echo "Checking branch: $branch"
    git ls-tree -r "$branch" | grep documents || true
  done
fi

# Test invalid URL
echo -e "\n13. Testing error handling (invalid URL)..."
curl -s -X POST http://localhost:8092/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "http://invalid-url-that-does-not-exist.com/doc.pdf",
    "type": "pdf"
  }' | jq .

# List documents endpoint
echo -e "\n14. Testing list documents endpoint..."
curl -s http://localhost:8092/api/v1/documents | jq .

# Cleanup
echo -e "\n15. Cleaning up..."
kill $HTTP_PID 2>/dev/null
kill $SERVER_PID 2>/dev/null

# Show Temporal workflow history
echo -e "\n16. Checking Temporal workflows..."
temporal workflow list --fields long | head -20 || echo "Temporal CLI not available for listing"

echo -e "\n=== Test Suite Complete ==="
cd -
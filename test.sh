#!/bin/bash

echo "Starting CAIA Library server..."

# Create a test Git repository
mkdir -p /tmp/caia-library-repo
cd /tmp/caia-library-repo
git init --initial-branch=main
git config user.email "library@caiatech.com"
git config user.name "CAIA Library"
echo "# CAIA Library Repository" > README.md
git add README.md
git commit -m "Initial commit"
cd -

# Start the server
export CAIA_REPO_PATH=/tmp/caia-library-repo
export PORT=8091
./caia-server &
SERVER_PID=$!

echo "Server started with PID: $SERVER_PID"
echo "Waiting for server to be ready..."
sleep 3

# Test health endpoint
echo "Testing health endpoint..."
curl -s http://localhost:8091/health | jq .

# Test document ingestion
echo -e "\nTesting document ingestion..."
curl -s -X POST http://localhost:8091/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/test.txt",
    "type": "text",
    "metadata": {
      "source": "test"
    }
  }' | jq .

# Kill the server
echo -e "\nStopping server..."
kill $SERVER_PID

echo "Test complete!"
#!/bin/bash
# Example: Ingest a document into Caia Library

# Check if server is running
if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "❌ Caia Library server is not running on port 8080"
    echo "   Start it with: ./caia-server"
    exit 1
fi

echo "📄 Ingesting a text document..."

# Ingest a document
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://raw.githubusercontent.com/caiatech/caia-library/main/README.md",
    "type": "text",
    "metadata": {
      "source": "github",
      "project": "caia-library",
      "category": "documentation"
    }
  }')

echo "Response: $RESPONSE"

# Extract workflow ID
WORKFLOW_ID=$(echo "$RESPONSE" | grep -o '"workflow_id":"[^"]*' | cut -d'"' -f4)

if [ -z "$WORKFLOW_ID" ]; then
    echo "❌ Failed to start ingestion workflow"
    exit 1
fi

echo "✅ Started workflow: $WORKFLOW_ID"
echo ""
echo "Waiting for completion..."
sleep 3

# Check workflow status
STATUS=$(curl -s "http://localhost:8080/api/v1/workflows/$WORKFLOW_ID" | jq -r .status)
echo "📊 Workflow status: $STATUS"

# View in Temporal UI
echo ""
echo "🔍 View details in Temporal UI:"
echo "   http://localhost:8233/namespaces/default/workflows/$WORKFLOW_ID"
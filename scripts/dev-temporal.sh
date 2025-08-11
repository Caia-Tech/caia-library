#!/bin/bash

# CAIA Library Temporal Development Script
# Usage: ./scripts/dev-temporal.sh [command]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_FILE="$PROJECT_ROOT/temporal-dev.log"
PID_FILE="$PROJECT_ROOT/temporal-dev.pid"

cd "$PROJECT_ROOT"

case "${1:-help}" in
    start)
        echo "🚀 Starting Temporal dev server..."
        if [ -f "$PID_FILE" ]; then
            if ps -p $(cat "$PID_FILE") > /dev/null 2>&1; then
                echo "✅ Temporal server already running (PID: $(cat "$PID_FILE"))"
                exit 0
            else
                rm -f "$PID_FILE"
            fi
        fi
        
        # Start Temporal server in background
        nohup temporal server start-dev --headless > "$LOG_FILE" 2>&1 &
        echo $! > "$PID_FILE"
        
        echo "   Waiting for server to start..."
        sleep 3
        
        if ps -p $(cat "$PID_FILE") > /dev/null 2>&1; then
            echo "✅ Temporal server started (PID: $(cat "$PID_FILE"))"
            echo "   Server: localhost:7233"
            echo "   Logs: $LOG_FILE"
        else
            echo "❌ Failed to start Temporal server"
            cat "$LOG_FILE"
            exit 1
        fi
        ;;
        
    stop)
        echo "🛑 Stopping Temporal dev server..."
        if [ -f "$PID_FILE" ]; then
            if ps -p $(cat "$PID_FILE") > /dev/null 2>&1; then
                kill $(cat "$PID_FILE")
                rm -f "$PID_FILE"
                echo "✅ Temporal server stopped"
            else
                echo "⚠️  Temporal server not running"
                rm -f "$PID_FILE"
            fi
        else
            echo "⚠️  No PID file found"
        fi
        
        # Clean up any other temporal processes
        pkill -f "temporal server start-dev" || true
        ;;
        
    status)
        echo "📊 Temporal server status:"
        if [ -f "$PID_FILE" ]; then
            if ps -p $(cat "$PID_FILE") > /dev/null 2>&1; then
                echo "✅ Server running (PID: $(cat "$PID_FILE"))"
                echo "   Checking connectivity..."
                if temporal workflow list > /dev/null 2>&1; then
                    echo "✅ Server accessible at localhost:7233"
                else
                    echo "❌ Server not accessible"
                fi
            else
                echo "❌ Server not running (stale PID file)"
                rm -f "$PID_FILE"
            fi
        else
            echo "❌ Server not running"
        fi
        ;;
        
    logs)
        echo "📋 Temporal server logs:"
        if [ -f "$LOG_FILE" ]; then
            tail -f "$LOG_FILE"
        else
            echo "❌ No log file found"
        fi
        ;;
        
    test)
        echo "🧪 Running Temporal integration tests..."
        if ! [ -f "$PID_FILE" ] || ! ps -p $(cat "$PID_FILE") > /dev/null 2>&1; then
            echo "⚠️  Starting Temporal server first..."
            ./scripts/dev-temporal.sh start
            sleep 2
        fi
        
        echo "   Running comprehensive test suite..."
        go run cmd/temporal-test/main.go
        ;;
        
    workflows)
        echo "📋 Recent workflows:"
        temporal workflow list
        ;;
        
    workflow)
        if [ -z "$2" ]; then
            echo "❌ Usage: $0 workflow <workflow-id>"
            exit 1
        fi
        echo "🔍 Workflow details: $2"
        temporal workflow show --workflow-id "$2"
        ;;
        
    trigger)
        echo "🔄 Triggering document ingestion workflow..."
        URL="${2:-https://httpbin.org/html}"
        TYPE="${3:-html}"
        
        if ! [ -f "$PID_FILE" ] || ! ps -p $(cat "$PID_FILE") > /dev/null 2>&1; then
            echo "❌ Temporal server not running. Start with: $0 start"
            exit 1
        fi
        
        WORKFLOW_ID="manual-ingestion-$(date +%s)"
        echo "   Starting workflow: $WORKFLOW_ID"
        echo "   URL: $URL"
        echo "   Type: $TYPE"
        
        # Create a simple trigger command
        go run -c "
package main
import (
    \"context\"
    \"fmt\"
    \"log\"
    \"github.com/Caia-Tech/caia-library/internal/temporal/workflows\"
    \"go.temporal.io/sdk/client\"
)
func main() {
    c, err := client.Dial(client.Options{})
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    input := workflows.DocumentInput{
        URL: \"$URL\",
        Type: \"$TYPE\",
        Metadata: map[string]string{
            \"triggered_by\": \"dev-script\",
        },
    }
    
    we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
        ID: \"$WORKFLOW_ID\",
        TaskQueue: \"caia-library\",
    }, workflows.DocumentIngestionWorkflow, input)
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf(\"Workflow started: %s\\n\", we.GetID())
}" || echo "   💡 Use 'go run cmd/temporal-test/main.go' to test workflow execution"
        ;;
        
    server)
        echo "🌐 Opening Temporal Web UI..."
        open "http://localhost:8233" || echo "   Open http://localhost:8233 in your browser"
        ;;
        
    clean)
        echo "🧹 Cleaning up Temporal data..."
        ./scripts/dev-temporal.sh stop
        rm -f "$LOG_FILE" "$PID_FILE"
        rm -rf temporal-data
        echo "✅ Cleanup complete"
        ;;
        
    help|*)
        echo "🔧 CAIA Library Temporal Development Script"
        echo "==========================================="
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  start     - Start Temporal dev server"
        echo "  stop      - Stop Temporal dev server"
        echo "  status    - Check server status"
        echo "  logs      - Show server logs"
        echo "  test      - Run integration tests"
        echo "  workflows - List recent workflows"
        echo "  workflow  - Show workflow details <workflow-id>"
        echo "  trigger   - Trigger workflow [url] [type]"
        echo "  server    - Open Temporal Web UI"
        echo "  clean     - Clean up all data"
        echo "  help      - Show this help"
        echo ""
        echo "Examples:"
        echo "  $0 start                    # Start server"
        echo "  $0 test                     # Run tests"
        echo "  $0 trigger https://go.dev html  # Trigger workflow"
        echo "  $0 workflows                # List workflows"
        echo ""
        ;;
esac
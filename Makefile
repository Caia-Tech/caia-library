.PHONY: all build test test-unit test-integration test-coverage clean docker-build docker-up docker-down lint fmt

# Variables
BINARY_NAME=caia-server
GO=go
GOTEST=$(GO) test
GOCOVER=$(GO) tool cover
DOCKER_COMPOSE=docker-compose

# Build the application
all: build

build:
	$(GO) build -o $(BINARY_NAME) ./cmd/server

# Run all tests
test: test-unit

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -race -short ./...

# Run integration tests (requires Temporal)
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -race -run Integration ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCOVER) -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run specific package tests
test-pkg:
	@echo "Testing specific package: $(PKG)"
	$(GOTEST) -v -race ./$(PKG)/...

# Run academic collector tests
test-academic:
	@echo "Testing academic collector..."
	$(GOTEST) -v -race ./internal/temporal/activities -run Academic
	$(GOTEST) -v -race ./pkg/ratelimit -run Academic

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GO) clean
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Docker commands
docker-build:
	@echo "Building Docker image..."
	docker build -t caia-library:latest .

docker-up:
	@echo "Starting services with Docker Compose..."
	$(DOCKER_COMPOSE) up -d

docker-down:
	@echo "Stopping services..."
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f caia

# Development with hot reload
dev:
	@echo "Starting development environment..."
	$(DOCKER_COMPOSE) -f docker-compose.yml -f docker-compose.dev.yml up

# Lint the code
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	goimports -w .

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Generate mocks
mocks:
	@echo "Generating mocks..."
	mockery --all --output ./mocks

# Run temporal server for testing
temporal-dev:
	@echo "Starting Temporal dev server..."
	temporal server start-dev

# Setup academic ingestion examples
setup-academic:
	@echo "Setting up academic source collectors..."
	./examples/academic_ingestion.sh

# Quick test for CI
ci-test:
	@echo "Running CI tests..."
	$(GOTEST) -v -race -short -coverprofile=coverage.out ./...

# Help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the application"
	@echo "  make test           - Run all unit tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-academic  - Run academic collector tests"
	@echo "  make bench          - Run benchmarks"
	@echo "  make docker-up      - Start services with Docker Compose"
	@echo "  make docker-down    - Stop Docker Compose services"
	@echo "  make dev            - Start development environment"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo "  make clean          - Clean build artifacts"
# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o caia-server ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates git

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/caia-server .

# Create directories
RUN mkdir -p /data/repo /data/models

# Initialize git repository
RUN git config --global user.email "library@caiatech.com" && \
    git config --global user.name "Caia Library" && \
    cd /data/repo && \
    git init --initial-branch=main && \
    echo "# Caia Library Document Repository" > README.md && \
    git add README.md && \
    git commit -m "Initial commit"

# Expose port
EXPOSE 8080

# Environment variables
ENV CAIA_REPO_PATH=/data/repo
ENV PORT=8080
ENV TEMPORAL_HOST=temporal:7233

# Run the application
CMD ["./caia-server"]
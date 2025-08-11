# govc Service Integration Guide

## Overview

CAIA Library now integrates with govc as a REST API service for high-performance Git operations. govc provides a memory-first Git implementation that delivers significant performance improvements over traditional disk-based Git operations.

## Architecture Change

**Important**: govc is deployed as a separate HTTP service, not an embedded library. This means:
- Network communication overhead between CAIA Library and govc service
- Authentication and service discovery requirements
- Potential for network-related failures requiring fallback mechanisms

## Service Requirements

### 1. Running govc Service

govc must be running and accessible before starting CAIA Library:

```bash
# Start govc service (default port 8080)
govc server --port 8080 --memory-mode

# Or with authentication
govc server --port 8080 --jwt-secret your-secret-key
```

### 2. Environment Configuration

Configure CAIA Library to connect to govc service:

```bash
# Required: govc service URL
export GOVC_SERVICE_URL=http://localhost:8080

# Optional: Authentication token (JWT or API key)
export GOVC_AUTH_TOKEN=your-jwt-token
# or
export GOVC_API_KEY=your-api-key

# Optional: Repository name (default: caia-documents)
export GOVC_REPO_NAME=my-repo

# Optional: HTTP timeout (default: 30s)
export GOVC_TIMEOUT=60s

# Optional: Enable govc as primary backend
export CAIA_USE_GOVC=true
```

### 3. Docker Deployment

For production deployment with Docker:

```yaml
version: '3.8'
services:
  govc:
    image: caiatech/govc:latest
    ports:
      - "8080:8080"
    environment:
      - GOVC_MEMORY_MODE=true
      - GOVC_JWT_SECRET=${JWT_SECRET}
    volumes:
      - govc-data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  caia-library:
    image: caiatech/caia-library:latest
    depends_on:
      govc:
        condition: service_healthy
    environment:
      - GOVC_SERVICE_URL=http://govc:8080
      - GOVC_AUTH_TOKEN=${JWT_TOKEN}
    ports:
      - "8088:8088"

volumes:
  govc-data:
```

## Performance Characteristics

### Expected Performance

With govc service (actual measurements will vary based on network latency):
- **Write operations**: ~10-50ms per document (network + processing)
- **Read operations**: ~5-20ms per document (network + retrieval)
- **Throughput**: 100-500 documents/second depending on network

### Performance Comparison

| Operation | Traditional Git | govc Service | govc (In-Memory Mock) |
|-----------|----------------|--------------|----------------------|
| Write     | ~10ms          | ~20ms*       | ~12μs                |
| Read      | ~126μs         | ~10ms*       | ~2μs                 |
| Merge     | ~100ms         | ~30ms*       | ~5μs                 |

*Includes network latency

## Integration Architecture

```
┌─────────────────┐     REST API      ┌──────────────┐
│                 │ ◄───────────────► │              │
│  CAIA Library   │                    │ govc Service │
│                 │                    │              │
└────────┬────────┘                    └──────┬───────┘
         │                                     │
         │ Fallback                           │
         ▼                                     ▼
┌─────────────────┐                    ┌──────────────┐
│  Git Backend    │                    │ Memory Store │
│   (Fallback)    │                    │ (High Perf)  │
└─────────────────┘                    └──────────────┘
```

## Fallback Mechanism

CAIA Library implements automatic fallback to handle govc service unavailability:

1. **Primary**: Attempts govc service operations
2. **Fallback**: Falls back to in-memory store if service is unavailable
3. **Recovery**: Periodically checks service health and switches back when available

## Testing Integration

### 1. Unit Tests (Mock Mode)

Run without govc service:
```bash
go test ./...
```

### 2. Integration Tests (Service Required)

With govc service running:
```bash
# Start govc service
govc server --port 8080 --memory-mode &

# Run integration tests
GOVC_SERVICE_URL=http://localhost:8080 go test -run TestGovcServiceIntegration
```

### 3. Performance Tests

```bash
GOVC_SERVICE_URL=http://localhost:8080 go test -run TestGovcServicePerformance
```

## Monitoring

### Health Check Endpoint

CAIA Library exposes govc backend status:
```bash
curl http://localhost:8088/api/v1/storage/stats
```

Response includes:
```json
{
  "govc": {
    "govc_integrated": true,
    "service_url": "http://localhost:8080",
    "fallback_mode": false,
    "documents_in_memory": 42
  }
}
```

### Metrics

Monitor govc integration performance:
```bash
curl http://localhost:8088/api/v1/storage/metrics
```

## Troubleshooting

### Service Connection Issues

If CAIA Library cannot connect to govc:

1. **Check service is running**:
   ```bash
   curl http://localhost:8080/health
   ```

2. **Verify environment configuration**:
   ```bash
   echo $GOVC_SERVICE_URL
   echo $GOVC_AUTH_TOKEN
   ```

3. **Check logs** for connection errors:
   ```bash
   # CAIA Library logs will show:
   # "govc service unavailable, using memory fallback"
   ```

### Authentication Failures

If receiving 401/403 errors:

1. **Verify token is set**:
   ```bash
   export GOVC_AUTH_TOKEN=your-token
   ```

2. **Check token format** (JWT vs API key)

3. **Verify token permissions** on govc service

### Performance Issues

If experiencing slow operations:

1. **Check network latency**:
   ```bash
   ping govc-service-host
   ```

2. **Monitor govc service load**:
   ```bash
   curl http://localhost:8080/api/v1/metrics
   ```

3. **Consider connection pooling** adjustments

## Migration Path

### Phase 1: Dual Operation (Current)
- govc as optional service
- Automatic fallback to Git backend
- Performance testing and validation

### Phase 2: Primary Backend
- govc as primary storage
- Git backend for backup only
- Batch migration tools

### Phase 3: Full Integration
- govc-only operation
- Optimized batch operations
- Direct memory access patterns

## Security Considerations

1. **Authentication**: Always use JWT tokens in production
2. **Network Security**: Use TLS for govc service connections
3. **Access Control**: Implement repository-level permissions
4. **Audit Logging**: Enable govc audit logs for compliance

## Next Steps

1. **Deploy govc service** in your environment
2. **Configure CAIA Library** with service URL
3. **Run integration tests** to verify connectivity
4. **Monitor performance** metrics
5. **Optimize** based on workload patterns
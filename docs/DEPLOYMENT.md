# Caia Library Deployment Guide

## Quick Start with Docker Compose

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- Git
- 4GB+ available RAM
- 10GB+ available disk space

### Production Deployment

1. Clone the repository:
```bash
git clone https://github.com/Caia-Tech/caia-library.git
cd caia-library
```

2. Create data directories:
```bash
mkdir -p data/repo data/models
```

3. Start the services:
```bash
docker-compose up -d
```

4. Verify services are running:
```bash
docker-compose ps
```

5. Check logs:
```bash
docker-compose logs -f caia
```

### Development Setup

For development with hot reload:

```bash
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

This enables:
- Hot reload on code changes
- Temporal UI at http://localhost:8233
- Debug logging

## Service URLs

After deployment, services are available at:

- **Caia Library API**: http://localhost:8080
- **Temporal UI**: http://localhost:8233 (dev only)
- **MinIO Console**: http://localhost:9001
  - Username: `minioadmin`
  - Password: `minioadmin`
- **PostgreSQL**: localhost:5432
  - Database: `temporal`
  - Username: `temporal`
  - Password: `temporal`
- **Redis**: localhost:6379

## Configuration

### Environment Variables

Create a `.env` file for custom configuration:

```env
# API Server
PORT=8080
CORS_ORIGINS=http://localhost:3000,https://myapp.com

# Temporal
TEMPORAL_HOST=temporal:7233
TEMPORAL_NAMESPACE=default

# Git Repository
Caia_REPO_PATH=/data/repo
GIT_USER_NAME=Caia Library
GIT_USER_EMAIL=library@caiatech.com

# Storage (Optional)
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=caia-documents

# Redis Cache (Optional)
REDIS_URL=redis://redis:6379
```

### Volume Mounts

The following volumes are persisted:

- `./data/repo` - Git repository for documents
- `./data/models` - ML models (future use)
- `postgres-data` - Temporal database
- `minio-data` - Object storage
- `redis-data` - Cache data

## Production Considerations

### 1. Concurrency

The system uses mutex protection for Git operations to prevent race conditions. When running multiple workers:

- All Git operations are serialized through a mutex
- Document ingestion can be parallelized across workers
- Merge operations are thread-safe

### 2. Security

#### Network Isolation
```yaml
# docker-compose.override.yml
services:
  temporal:
    networks:
      - internal
  
  postgres:
    networks:
      - internal
    ports: []  # Remove external port

networks:
  internal:
    internal: true
```

#### Authentication
- Implement API key authentication
- Use environment-specific secrets
- Enable TLS/SSL

### 2. Scaling

#### Horizontal Scaling
```yaml
# docker-compose.scale.yml
services:
  caia:
    deploy:
      replicas: 3
    environment:
      - WORKER_ID=${HOSTNAME}
```

Run with:
```bash
docker-compose -f docker-compose.yml -f docker-compose.scale.yml up -d --scale caia=3
```

#### Resource Limits
```yaml
# docker-compose.prod.yml
services:
  caia:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

### 3. Monitoring

#### Health Checks
```yaml
services:
  caia:
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

#### Logging
```yaml
services:
  caia:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### 4. Backup

#### Automated Backups
```bash
#!/bin/bash
# backup.sh
BACKUP_DIR="/backups/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# Backup Git repository
docker run --rm -v caia-library_repo:/data alpine \
  tar -czf - /data > $BACKUP_DIR/repo.tar.gz

# Backup PostgreSQL
docker-compose exec -T postgres pg_dump -U temporal temporal \
  > $BACKUP_DIR/temporal.sql

# Backup MinIO
docker run --rm -v caia-library_minio-data:/data alpine \
  tar -czf - /data > $BACKUP_DIR/minio.tar.gz
```

Add to crontab:
```bash
0 2 * * * /path/to/backup.sh
```

## Cloud Deployments

### AWS ECS

1. Build and push image:
```bash
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin $ECR_REGISTRY
docker build -t caia-library .
docker tag caia-library:latest $ECR_REGISTRY/caia-library:latest
docker push $ECR_REGISTRY/caia-library:latest
```

2. Use provided task definition:
```json
{
  "family": "caia-library",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "1024",
  "memory": "2048",
  "containerDefinitions": [
    {
      "name": "caia",
      "image": "${ECR_REGISTRY}/caia-library:latest",
      "environment": [
        {"name": "TEMPORAL_HOST", "value": "temporal.internal:7233"}
      ]
    }
  ]
}
```

### Kubernetes

See [k8s/](../k8s/) directory for Kubernetes manifests:

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/
```

### Azure Container Instances

```bash
az container create \
  --resource-group caia-rg \
  --name caia-library \
  --image caiatech/caia-library:latest \
  --cpu 2 \
  --memory 4 \
  --environment-variables \
    TEMPORAL_HOST=temporal:7233 \
    Caia_REPO_PATH=/data/repo \
  --azure-file-volume-account-name caiastorage \
  --azure-file-volume-share-name caia-data \
  --azure-file-volume-mount-path /data
```

## Troubleshooting

### Common Issues

1. **Temporal connection failed**
   ```bash
   docker-compose logs temporal
   # Wait for "Temporal server started successfully"
   ```

2. **Git repository errors**
   ```bash
   docker-compose exec caia sh
   cd /data/repo
   git status
   git config --list
   ```

3. **Out of memory**
   - Increase Docker memory limit
   - Reduce worker concurrency
   - Enable swap

4. **Port conflicts**
   ```bash
   # Check port usage
   netstat -tulpn | grep -E '8080|7233|5432'
   
   # Change ports in docker-compose.yml
   ```

### Debug Mode

Enable debug logging:
```yaml
services:
  caia:
    environment:
      - LOG_LEVEL=debug
      - TEMPORAL_LOG_LEVEL=debug
```

### Performance Tuning

1. **PostgreSQL**
   ```sql
   -- Connect to postgres
   docker-compose exec postgres psql -U temporal
   
   -- Tune for Temporal
   ALTER SYSTEM SET max_connections = 200;
   ALTER SYSTEM SET shared_buffers = '256MB';
   ALTER SYSTEM SET effective_cache_size = '1GB';
   ```

2. **Redis**
   ```bash
   docker-compose exec redis redis-cli
   CONFIG SET maxmemory 1gb
   CONFIG SET maxmemory-policy allkeys-lru
   ```

3. **Worker Optimization**
   ```go
   // Adjust in main.go
   worker.Options{
       MaxConcurrentActivityExecutionSize: 20,
       MaxConcurrentWorkflowTaskExecutionSize: 20,
       WorkerActivitiesPerSecond: 100,
   }
   ```

## Maintenance

### Updates

```bash
# Pull latest changes
git pull origin main

# Rebuild and restart
docker-compose build
docker-compose up -d
```

### Cleanup

```bash
# Remove stopped containers
docker-compose rm -f

# Clean up volumes (WARNING: Data loss)
docker-compose down -v

# Prune system
docker system prune -a
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/Caia-Tech/caia-library/issues
- Documentation: https://github.com/Caia-Tech/caia-library/docs
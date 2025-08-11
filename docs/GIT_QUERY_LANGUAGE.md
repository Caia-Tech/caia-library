# Git Query Language (GQL) Documentation

## Overview

Caia Library's Git Query Language (GQL) provides a SQL-like interface for querying documents stored in the Git repository. It enables powerful searches across document metadata, attribution tracking, and source analysis.

## Features

- **SQL-like syntax** for familiar querying
- **Git-based execution** leveraging repository history
- **Attribution tracking** to ensure Caia Tech compliance
- **Time-travel queries** through Git history
- **Performance optimization** with Git's efficient storage

## Syntax

```sql
SELECT FROM <type> 
WHERE <conditions> 
ORDER BY <field> [ASC|DESC] 
LIMIT <n>
```

### Query Types

- `documents` - Query document metadata and content
- `attribution` - Track attribution compliance
- `sources` - Analyze document sources
- `authors` - Find documents by author

### Operators

- `=` - Exact match
- `!=` - Not equal
- `~` - Contains (substring match)
- `>` - Greater than
- `<` - Less than
- `exists` - Field exists
- `not exists` - Field doesn't exist

## Examples

### Document Queries

#### Find all arXiv papers
```sql
SELECT FROM documents WHERE source = "arXiv" ORDER BY created_at DESC
```

#### Search by title
```sql
SELECT FROM documents WHERE title ~ "machine learning" LIMIT 20
```

#### Recent documents
```sql
SELECT FROM documents WHERE created_at > "2024-03-01" ORDER BY created_at DESC
```

#### Documents by author
```sql
SELECT FROM documents WHERE authors ~ "John Doe" OR author = "John Doe"
```

### Attribution Queries

#### Check Caia Tech attribution compliance
```sql
SELECT FROM attribution WHERE caia_attribution = true
```

#### Find sources missing attribution
```sql
SELECT FROM attribution WHERE caia_attribution = false
```

#### Attribution by source
```sql
SELECT FROM attribution WHERE source = "arXiv"
```

### Source Analysis

#### List all sources with document counts
```sql
SELECT FROM sources
```

#### Find active sources
```sql
SELECT FROM sources WHERE count > 10
```

### Author Analysis

#### Top authors by document count
```sql
SELECT FROM authors ORDER BY count DESC LIMIT 20
```

#### Prolific authors
```sql
SELECT FROM authors WHERE count > 5
```

## API Usage

### Execute Query

```bash
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SELECT FROM documents WHERE source = \"arXiv\" LIMIT 10"
  }'
```

**Response:**
```json
{
  "type": "documents",
  "count": 10,
  "items": [
    {
      "id": "arxiv-2301.00001",
      "source": "arXiv",
      "url": "https://arxiv.org/pdf/2301.00001.pdf",
      "title": "Advances in Neural Networks",
      "author": "Jane Smith",
      "created_at": "2024-03-14T10:30:00Z",
      "metadata": {
        "attribution": "Content from arXiv.org, collected by Caia Tech",
        "license": "arXiv License"
      },
      "commit_hash": "abc123..."
    }
  ],
  "elapsed_ms": 45,
  "query": "SELECT FROM documents WHERE source = \"arXiv\" LIMIT 10"
}
```

### Get Query Examples

```bash
curl http://localhost:8080/api/v1/query/examples
```

### Check Attribution Stats

```bash
curl http://localhost:8080/api/v1/stats/attribution
```

**Response:**
```json
{
  "total_sources": 5,
  "compliant_sources": 5,
  "compliance_rate": "100.0%",
  "attribution_text": "Content collected by Caia Tech (https://caiatech.com)",
  "policy": "All documents must include proper attribution to both source and Caia Tech"
}
```

## Query Builder (Programmatic)

```go
import "github.com/Caia-Tech/caia-library/pkg/gql"

// Build a query programmatically
query := gql.NewQueryBuilder(gql.QueryDocuments).
    Where("source", gql.OpEquals, "arXiv").
    Where("created_at", gql.OpGreater, "2024-03-01").
    OrderBy("created_at", true).
    Limit(50).
    Build()

// Execute the query
result, err := executor.Execute(ctx, query)
```

## Advanced Features

### Time-based Queries

Query documents collected in specific time ranges:

```sql
SELECT FROM documents 
WHERE created_at > "2024-03-01" 
  AND created_at < "2024-03-31"
ORDER BY created_at DESC
```

### Multi-condition Filters

Combine multiple conditions with AND:

```sql
SELECT FROM documents 
WHERE source = "arXiv" 
  AND title ~ "neural" 
  AND authors ~ "Smith"
LIMIT 20
```

### Attribution Compliance Tracking

Monitor Caia Tech attribution across all sources:

```sql
SELECT FROM attribution 
WHERE caia_attribution = true 
ORDER BY document_count DESC
```

## Performance Considerations

1. **Use LIMIT** - Always limit results to avoid loading entire repository
2. **Index fields** - Common query fields are optimized
3. **Specific filters** - More specific queries run faster
4. **Time ranges** - Narrow time ranges improve performance

## Git Integration

GQL leverages Git's features:

- **Immutable history** - Queries can access any point in time
- **Efficient storage** - Git's compression reduces query overhead
- **Distributed queries** - Can query local or remote repositories
- **Audit trail** - Every query result includes commit hash

## Security

- Queries are read-only
- No modification operations supported
- Respects Git repository permissions
- Rate limiting applies to query endpoints

## Future Enhancements

- `OR` operator support
- Aggregate functions (COUNT, SUM, AVG)
- JOIN operations between types
- Full-text search in document content
- Query result caching
- GraphQL interface

## Examples for Common Use Cases

### Daily Report of New Documents
```sql
SELECT FROM documents 
WHERE created_at > "2024-03-14" 
ORDER BY created_at DESC
```

### Verify Attribution Compliance
```sql
SELECT FROM attribution 
WHERE source = "arXiv" 
  AND caia_attribution = false
```

### Find Similar Documents
```sql
SELECT FROM documents 
WHERE title ~ "transformer architecture" 
  OR abstract ~ "transformer architecture"
LIMIT 50
```

### Author Collaboration Network
```sql
SELECT FROM authors 
WHERE count > 3 
ORDER BY count DESC
```

## Error Handling

Common errors and solutions:

- **Parse error** - Check query syntax
- **Unknown field** - Verify field name exists
- **Type mismatch** - Ensure value matches field type
- **Timeout** - Reduce result limit or add more filters

---

Git Query Language makes Caia Library's document collection searchable while maintaining the integrity and attribution requirements that are core to Caia Tech's mission.
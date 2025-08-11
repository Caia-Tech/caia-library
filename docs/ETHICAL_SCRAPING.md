# Ethical Academic Scraping Guide

## Overview

Caia Library implements ethical scraping practices for academic sources, ensuring compliance with terms of service and proper attribution to Caia Tech.

## Principles

### 1. **Only Access Allowed Sources**
We only collect from sources that explicitly permit programmatic access:
- arXiv (via official API)
- PubMed Central (open access subset)
- Directory of Open Access Journals (DOAJ)
- PLOS (Creative Commons content)
- CORE (with proper API key)
- Semantic Scholar (with rate limits)

### 2. **Respect Rate Limits**
Each source has specific rate limits that we strictly enforce:
```
arXiv:            1 request per 3 seconds
PubMed:           3 requests per second
DOAJ:             1 request per second
PLOS:             1 request per second
Semantic Scholar: 100 requests per 5 minutes
```

### 3. **Clear Attribution**
Every collected document includes:
- Source attribution (e.g., "Content from arXiv.org")
- Caia Tech attribution
- Collection timestamp
- User agent identification
- License information

### 4. **Transparent Bot Identification**
Our User-Agent clearly identifies us:
```
Caia-Library/1.0 (https://github.com/Caia-Tech/caia-library; library@caiatech.com) Academic-Research-Bot
```

## Implementation

### Academic Collector

The `AcademicCollectorActivities` ensures ethical collection:

```go
// Every document includes full attribution
doc := workflows.CollectedDocument{
    Metadata: map[string]string{
        "source":           "arXiv",
        "attribution":      "Content from arXiv.org, collected by Caia Tech",
        "license":          "arXiv License",
        "collection_agent": userAgent,
        "ethical_notice":   "Collected in compliance with arXiv Terms of Use",
    },
}
```

### Rate Limiting

The `AcademicRateLimiter` enforces ethical limits:

```go
// Wait for rate limit before making request
err := rateLimiter.WaitForSource(ctx, "arxiv")
if err != nil {
    return err
}

// Make request...

// Record success or error
if err != nil {
    rateLimiter.RecordError("arxiv", err)
} else {
    rateLimiter.RecordSuccess("arxiv")
}
```

### Exponential Backoff

On errors, we implement exponential backoff:
- 3 errors: 30 second backoff
- 4 errors: 60 second backoff
- 5+ errors: Up to 5 minute backoff

## Configuration

### Setting Up Academic Sources

```bash
# Schedule arXiv collection (daily at 2 AM UTC)
curl -X POST http://localhost:8080/api/v1/ingestion/scheduled \
  -H "Content-Type: application/json" \
  -d '{
    "name": "arxiv",
    "type": "arxiv",
    "url": "http://export.arxiv.org/api/query",
    "schedule": "0 2 * * *",
    "filters": ["cs.AI", "cs.LG"],
    "metadata": {
      "attribution": "Caia Tech",
      "ethical_compliance": "true"
    }
  }'
```

### Supported Filters

#### arXiv Categories
- `cs.AI` - Artificial Intelligence
- `cs.LG` - Machine Learning
- `cs.CL` - Computation and Language
- `cs.CV` - Computer Vision
- `cs.RO` - Robotics
- `stat.ML` - Machine Learning (Statistics)

#### PubMed Keywords
- Any medical/biological terms
- Boolean operators supported
- MeSH terms recommended

## Compliance Checklist

Before adding a new source:

- [ ] Check robots.txt
- [ ] Read terms of service
- [ ] Verify API availability
- [ ] Confirm open access policy
- [ ] Implement rate limiting
- [ ] Add proper attribution
- [ ] Test with small batches
- [ ] Monitor error rates

## Attribution Template

All collected documents must include:

```json
{
  "source": "SOURCE_NAME",
  "source_url": "https://original.url",
  "attribution": "Content from SOURCE, collected by Caia Tech (https://caiatech.com)",
  "license": "LICENSE_TYPE",
  "collection_time": "2024-03-14T10:30:00Z",
  "collection_agent": "Caia-Library/1.0 (...)",
  "ethical_notice": "Collected in compliance with SOURCE Terms of Use"
}
```

## Monitoring

### Rate Limit Stats

```bash
# Check rate limiter statistics
curl http://localhost:8080/api/v1/stats/rate-limits
```

### Collection Metrics

- Requests per source per hour
- Error rates by source
- Backoff incidents
- Success/failure ratios

## Legal Compliance

### Terms We Follow

1. **arXiv Terms of Use**: https://arxiv.org/help/api/tou
2. **NCBI Terms**: https://www.ncbi.nlm.nih.gov/home/about/policies/
3. **DOAJ Terms**: https://doaj.org/terms
4. **PLOS Terms**: https://www.plos.org/terms-of-use

### What We Don't Do

- ❌ Scrape sources without permission
- ❌ Ignore rate limits
- ❌ Hide our identity
- ❌ Redistribute without attribution
- ❌ Access paywalled content
- ❌ Violate copyright

## Contact

For questions about our scraping practices:
- Email: library@caiatech.com
- GitHub: https://github.com/Caia-Tech/caia-library/issues

## Updates

This policy is regularly reviewed. Last updated: March 2024

---

**Remember**: Ethical scraping protects both the sources we rely on and the reputation of Caia Tech. When in doubt, err on the side of caution.
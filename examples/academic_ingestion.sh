#!/bin/bash
# Example: Setting up ethical academic source ingestion for Caia Library

# Base URL for API
API_URL="http://localhost:8080/api/v1"

echo "Setting up ethical academic source collectors for Caia Library..."
echo "All collectors include proper attribution to Caia Tech"
echo ""

# 1. arXiv AI/ML Papers (Daily at 2 AM UTC)
echo "1. Creating arXiv AI/ML collector..."
curl -X POST ${API_URL}/ingestion/scheduled \
  -H "Content-Type: application/json" \
  -d '{
    "name": "arxiv",
    "type": "arxiv",
    "url": "http://export.arxiv.org/api/query",
    "schedule": "0 2 * * *",
    "filters": ["cs.AI", "cs.LG", "cs.CL", "stat.ML"],
    "metadata": {
      "collection_type": "academic",
      "attribution": "Caia Tech",
      "ethical_compliance": "true",
      "rate_limited": "true"
    }
  }'
echo ""

# 2. PubMed Central Open Access (Weekly on Sundays)
echo "2. Creating PubMed Central collector..."
curl -X POST ${API_URL}/ingestion/scheduled \
  -H "Content-Type: application/json" \
  -d '{
    "name": "pubmed",
    "type": "pubmed",
    "url": "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/",
    "schedule": "0 3 * * 0",
    "filters": ["artificial intelligence", "machine learning", "neural networks"],
    "metadata": {
      "collection_type": "academic",
      "attribution": "Caia Tech",
      "subset": "open_access",
      "ethical_compliance": "true"
    }
  }'
echo ""

# 3. DOAJ Open Access Journals (Twice weekly)
echo "3. Creating DOAJ collector..."
curl -X POST ${API_URL}/ingestion/scheduled \
  -H "Content-Type: application/json" \
  -d '{
    "name": "doaj",
    "type": "doaj",
    "url": "https://doaj.org/api/v2/",
    "schedule": "0 4 * * 1,4",
    "filters": ["computer science", "data science", "artificial intelligence"],
    "metadata": {
      "collection_type": "academic",
      "attribution": "Caia Tech",
      "license": "open_access",
      "ethical_compliance": "true"
    }
  }'
echo ""

# 4. PLOS ONE (Weekly)
echo "4. Creating PLOS collector..."
curl -X POST ${API_URL}/ingestion/scheduled \
  -H "Content-Type: application/json" \
  -d '{
    "name": "plos",
    "type": "plos",
    "url": "https://api.plos.org/",
    "schedule": "0 5 * * 2",
    "filters": ["computational", "algorithm", "machine learning"],
    "metadata": {
      "collection_type": "academic",
      "attribution": "Caia Tech",
      "license": "CC-BY",
      "ethical_compliance": "true"
    }
  }'
echo ""

# 5. Test immediate ingestion from arXiv
echo "5. Testing immediate arXiv ingestion..."
curl -X POST ${API_URL}/documents \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://arxiv.org/pdf/2301.00001.pdf",
    "type": "pdf",
    "metadata": {
      "source": "arXiv",
      "attribution": "Content from arXiv.org, collected by Caia Tech",
      "collection_agent": "Caia-Library/1.0",
      "ethical_notice": "Collected in compliance with arXiv Terms of Use"
    }
  }'
echo ""

echo "Academic source collectors configured!"
echo ""
echo "Monitor collection at: http://localhost:8080/api/v1/workflows"
echo "View Temporal UI at: http://localhost:8233"
echo ""
echo "All collections include:"
echo "- Proper attribution to Caia Tech"
echo "- Compliance with source terms of service"
echo "- Rate limiting to respect server resources"
echo "- Clear identification via User-Agent"
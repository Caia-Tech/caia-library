# Web Scraping & Data Collection Pipeline

## Overview

An ethical, scalable, and compliant web scraping pipeline for collecting high-quality documents from public sources while respecting robots.txt, rate limits, and legal requirements. Designed to integrate seamlessly with CAIA Library's existing architecture.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Discovery     │    │   Compliance    │    │   Extraction    │
│   Engine        │    │   Engine        │    │   Engine        │
│                 │    │                 │    │                 │
│ • Site Crawling │    │ • Robots.txt    │    │ • Content Parse │
│ • URL Queue     │    │ • Rate Limits   │    │ • Metadata Ext. │
│ • Priority Mgmt │    │ • ToS Check     │    │ • Structure Det.│
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────┬───────────┴──────────┬───────────┘
                     │                      │
        ┌─────────────▼─────────────┐    ┌───▼────────────────┐
        │    Content Validation     │    │   Storage &        │
        │    & Quality Assurance    │    │   Attribution      │
        │                          │    │                    │
        │ • Duplicate Detection    │    │ • Source Tracking  │
        │ • Quality Scoring        │    │ • Metadata Rich.   │
        │ • Legal Compliance       │    │ • Format Standard. │
        └─────────────┬────────────┘    └───┬────────────────┘
                     │                      │
                     └──────────┬───────────┘
                               │
                  ┌─────────────▼─────────────┐
                  │    CAIA Library          │
                  │    Integration           │
                  │                         │
                  │ • Document Storage      │
                  │ • Event Publishing      │
                  │ • Search Integration    │
                  └───────────────────────────┘
```

## Target Sources & Strategies

### Academic & Research Sources

#### arXiv.org
- **Content**: Research papers, preprints
- **API Access**: Yes (preferred method)
- **Rate Limits**: 1 request per 3 seconds
- **Attribution**: "Paper from arXiv.org, collected by CAIA Tech"
- **Compliance**: Open access, attribution required

#### PubMed/PMC
- **Content**: Medical and life science literature
- **API Access**: NCBI E-utilities
- **Rate Limits**: 10 requests/second with API key
- **Attribution**: "Content from PubMed, collected by CAIA Tech"
- **Compliance**: Public domain, attribution encouraged

#### IEEE Xplore
- **Content**: Engineering and technology papers
- **API Access**: IEEE Xplore API
- **Rate Limits**: 200 queries/day (free tier)
- **Attribution**: "Paper from IEEE Xplore, collected by CAIA Tech"
- **Compliance**: Subscription content - metadata only

### Technical Documentation

#### GitHub Documentation
- **Content**: READMEs, wikis, documentation
- **API Access**: GitHub API v4 (GraphQL)
- **Rate Limits**: 5000 requests/hour
- **Attribution**: "Documentation from GitHub, collected by CAIA Tech"
- **Compliance**: Respect repository licenses

#### Developer Documentation Sites
- **Content**: API docs, tutorials, guides
- **API Access**: Varies by site
- **Rate Limits**: Site-specific
- **Attribution**: Per-site attribution requirements
- **Compliance**: ToS compliance checking

#### Stack Overflow (Public Data)
- **Content**: Questions, answers, code examples
- **API Access**: Stack Exchange API
- **Rate Limits**: 300 requests/day
- **Attribution**: "Content from Stack Overflow, collected by CAIA Tech"
- **Compliance**: CC BY-SA license

### News & Information

#### Government Publications
- **Content**: Reports, documentation, data
- **API Access**: data.gov APIs
- **Rate Limits**: Generally unrestricted
- **Attribution**: "Government publication, collected by CAIA Tech"
- **Compliance**: Public domain

#### Educational Institution Sites
- **Content**: Course materials, research
- **API Access**: Limited
- **Rate Limits**: Conservative approach
- **Attribution**: "Educational content, collected by CAIA Tech"
- **Compliance**: Educational fair use

## Compliance Framework

### Legal Compliance Engine

#### Terms of Service Checking
```python
class ToSChecker:
    def __init__(self):
        self.tos_cache = {}
        self.legal_patterns = self.load_legal_patterns()
    
    async def check_compliance(self, domain):
        tos_url = await self.find_tos_url(domain)
        if not tos_url:
            return {'compliant': False, 'reason': 'No ToS found'}
        
        tos_text = await self.fetch_tos(tos_url)
        analysis = self.analyze_tos(tos_text)
        
        return {
            'compliant': analysis['allows_scraping'],
            'restrictions': analysis['restrictions'],
            'attribution_required': analysis['attribution_required'],
            'commercial_use': analysis['commercial_use_allowed'],
            'last_checked': datetime.utcnow().isoformat()
        }
    
    def analyze_tos(self, tos_text):
        # Pattern matching for common restrictions
        restrictions = []
        
        if re.search(r'no.*scrapin|no.*crawl|prohibit.*automat', tos_text, re.I):
            restrictions.append('automated_access_prohibited')
        
        if re.search(r'no.*commercial|non.commercial', tos_text, re.I):
            restrictions.append('commercial_use_restricted')
        
        attribution_required = bool(re.search(r'attribut|credit|cite', tos_text, re.I))
        
        return {
            'allows_scraping': len(restrictions) == 0,
            'restrictions': restrictions,
            'attribution_required': attribution_required,
            'commercial_use_allowed': 'commercial_use_restricted' not in restrictions
        }
```

#### Robots.txt Compliance
```python
import urllib.robotparser

class RobotsChecker:
    def __init__(self):
        self.robots_cache = {}
        
    async def can_fetch(self, url, user_agent='*'):
        domain = self.extract_domain(url)
        
        if domain not in self.robots_cache:
            await self.load_robots(domain)
        
        rp = self.robots_cache.get(domain)
        if not rp:
            return True  # No robots.txt found, assume allowed
        
        return rp.can_fetch(user_agent, url)
    
    async def get_crawl_delay(self, domain, user_agent='*'):
        if domain not in self.robots_cache:
            await self.load_robots(domain)
        
        rp = self.robots_cache.get(domain)
        if rp:
            return rp.crawl_delay(user_agent) or 1  # Default 1 second
        return 1
    
    async def load_robots(self, domain):
        robots_url = f"https://{domain}/robots.txt"
        try:
            rp = urllib.robotparser.RobotFileParser()
            rp.set_url(robots_url)
            rp.read()
            self.robots_cache[domain] = rp
        except Exception as e:
            logger.warning(f"Could not load robots.txt for {domain}: {e}")
            self.robots_cache[domain] = None
```

### Rate Limiting & Politeness

#### Adaptive Rate Limiter
```python
class AdaptiveRateLimiter:
    def __init__(self):
        self.domain_limits = {}
        self.request_history = {}
        
    async def acquire(self, domain):
        now = time.time()
        
        # Initialize domain if new
        if domain not in self.domain_limits:
            await self.initialize_domain_limits(domain)
        
        # Check if we can make request
        last_request = self.request_history.get(domain, 0)
        required_delay = self.domain_limits[domain]['delay']
        
        time_since_last = now - last_request
        if time_since_last < required_delay:
            await asyncio.sleep(required_delay - time_since_last)
        
        self.request_history[domain] = time.time()
        
    async def initialize_domain_limits(self, domain):
        # Start conservative, adjust based on response
        base_delay = await self.get_robots_delay(domain)
        
        self.domain_limits[domain] = {
            'delay': max(base_delay, 1.0),  # Minimum 1 second
            'max_concurrent': 1,            # Conservative start
            'success_count': 0,
            'error_count': 0,
            'last_adjustment': time.time()
        }
    
    def adjust_rate_for_domain(self, domain, success):
        limits = self.domain_limits[domain]
        now = time.time()
        
        if success:
            limits['success_count'] += 1
            # Gradually decrease delay if consistently successful
            if limits['success_count'] > 10 and limits['error_count'] == 0:
                limits['delay'] = max(limits['delay'] * 0.9, 0.5)
        else:
            limits['error_count'] += 1
            # Increase delay on errors
            limits['delay'] = min(limits['delay'] * 1.5, 10.0)
        
        # Reset counters periodically
        if now - limits['last_adjustment'] > 3600:  # 1 hour
            limits['success_count'] = 0
            limits['error_count'] = 0
            limits['last_adjustment'] = now
```

## Content Extraction Engine

### Multi-Format Content Parser

#### HTML Content Extraction
```python
from bs4 import BeautifulSoup
from readability import Document
import trafilatura

class ContentExtractor:
    def __init__(self):
        self.extractors = {
            'trafilatura': self.extract_with_trafilatura,
            'readability': self.extract_with_readability,
            'custom': self.extract_with_custom_rules
        }
    
    async def extract_content(self, html, url):
        results = {}
        
        # Try multiple extraction methods
        for method, extractor in self.extractors.items():
            try:
                results[method] = await extractor(html, url)
            except Exception as e:
                logger.warning(f"Extraction method {method} failed for {url}: {e}")
                results[method] = None
        
        # Select best result
        best_result = self.select_best_extraction(results, url)
        
        if best_result:
            return self.enhance_content(best_result, url)
        else:
            raise ValueError(f"All extraction methods failed for {url}")
    
    async def extract_with_trafilatura(self, html, url):
        content = trafilatura.extract(
            html, 
            include_comments=False,
            include_tables=True,
            include_formatting=True,
            url=url
        )
        
        metadata = trafilatura.extract_metadata(html, url=url)
        
        return {
            'text': content,
            'title': metadata.title,
            'author': metadata.author,
            'date': metadata.date,
            'description': metadata.description,
            'method': 'trafilatura'
        }
    
    async def extract_with_readability(self, html, url):
        doc = Document(html)
        
        return {
            'text': doc.summary(),
            'title': doc.title(),
            'method': 'readability'
        }
    
    async def extract_with_custom_rules(self, html, url):
        soup = BeautifulSoup(html, 'html.parser')
        
        # Remove unwanted elements
        for tag in soup(['script', 'style', 'nav', 'header', 'footer', 'aside']):
            tag.decompose()
        
        # Extract main content
        main_content = self.find_main_content(soup)
        
        return {
            'text': main_content.get_text(separator=' ', strip=True),
            'title': self.extract_title(soup),
            'author': self.extract_author(soup),
            'date': self.extract_date(soup),
            'method': 'custom'
        }
```

#### Metadata Extraction
```python
class MetadataExtractor:
    def __init__(self):
        self.schema_extractors = {
            'article': self.extract_article_schema,
            'technicalarticle': self.extract_technical_article_schema,
            'research': self.extract_research_schema
        }
    
    async def extract_metadata(self, soup, url):
        metadata = {
            'url': url,
            'domain': self.extract_domain(url),
            'extracted_at': datetime.utcnow().isoformat()
        }
        
        # Extract from meta tags
        metadata.update(self.extract_meta_tags(soup))
        
        # Extract from JSON-LD structured data
        metadata.update(self.extract_json_ld(soup))
        
        # Extract from Open Graph tags
        metadata.update(self.extract_open_graph(soup))
        
        # Extract from schema.org microdata
        metadata.update(self.extract_microdata(soup))
        
        return metadata
    
    def extract_meta_tags(self, soup):
        meta_data = {}
        
        # Standard meta tags
        for tag in soup.find_all('meta'):
            name = tag.get('name') or tag.get('property') or tag.get('itemprop')
            content = tag.get('content')
            
            if name and content:
                meta_data[name.lower()] = content
        
        return meta_data
    
    def extract_json_ld(self, soup):
        json_ld_data = {}
        
        for script in soup.find_all('script', type='application/ld+json'):
            try:
                data = json.loads(script.string)
                if isinstance(data, dict):
                    json_ld_data.update(data)
                elif isinstance(data, list):
                    for item in data:
                        if isinstance(item, dict):
                            json_ld_data.update(item)
            except json.JSONDecodeError:
                continue
        
        return json_ld_data
```

## Quality Validation & Deduplication

### Content Quality Assessment
```python
class ContentQualityValidator:
    def __init__(self):
        self.quality_thresholds = {
            'min_word_count': 100,
            'max_word_count': 50000,
            'min_sentence_count': 5,
            'readability_score': 40,  # Flesch Reading Ease
            'spam_score_threshold': 0.3
        }
    
    async def validate_content(self, content, metadata):
        scores = {}
        
        # Basic content metrics
        scores['word_count'] = self.calculate_word_count_score(content)
        scores['readability'] = self.calculate_readability_score(content)
        scores['structure'] = self.assess_content_structure(content)
        scores['language_quality'] = self.assess_language_quality(content)
        scores['spam_detection'] = self.detect_spam_content(content)
        
        # Metadata quality
        scores['metadata_completeness'] = self.assess_metadata_quality(metadata)
        
        overall_score = self.calculate_overall_quality(scores)
        
        return {
            'quality_score': overall_score,
            'detailed_scores': scores,
            'passes_threshold': overall_score >= 0.7,
            'quality_issues': self.identify_quality_issues(scores)
        }
    
    def calculate_readability_score(self, text):
        try:
            from textstat import flesch_reading_ease
            score = flesch_reading_ease(text)
            # Normalize to 0-1 scale
            return max(0, min(1, (score + 100) / 200))
        except:
            return 0.5  # Default neutral score
    
    def detect_spam_content(self, text):
        spam_indicators = [
            r'click here',
            r'buy now',
            r'limited time',
            r'act now',
            r'free.*money',
            r'make.*money.*fast'
        ]
        
        spam_count = sum(
            len(re.findall(pattern, text.lower()))
            for pattern in spam_indicators
        )
        
        # Normalize by text length
        spam_ratio = spam_count / max(len(text.split()), 1)
        return max(0, 1 - spam_ratio * 10)  # Penalty for spam content
```

### Duplicate Detection
```python
import hashlib
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity

class DuplicateDetector:
    def __init__(self, similarity_threshold=0.85):
        self.similarity_threshold = similarity_threshold
        self.vectorizer = TfidfVectorizer(
            max_features=5000,
            stop_words='english',
            ngram_range=(1, 2)
        )
        self.existing_hashes = set()
        self.existing_vectors = []
        self.existing_ids = []
    
    async def is_duplicate(self, content, document_id):
        # Quick exact duplicate check
        content_hash = self.generate_content_hash(content)
        if content_hash in self.existing_hashes:
            return True, 'exact_duplicate', 1.0
        
        # Semantic similarity check
        if len(self.existing_vectors) > 0:
            similarity = await self.calculate_similarity(content)
            if similarity > self.similarity_threshold:
                return True, 'similar_content', similarity
        
        # Add to existing content tracking
        self.add_content(content, document_id, content_hash)
        return False, 'unique', 0.0
    
    def generate_content_hash(self, content):
        # Create hash of normalized content
        normalized = re.sub(r'\s+', ' ', content.lower().strip())
        return hashlib.sha256(normalized.encode()).hexdigest()
    
    async def calculate_similarity(self, content):
        try:
            # Vectorize new content
            new_vector = self.vectorizer.transform([content])
            
            # Calculate similarity with existing content
            similarities = cosine_similarity(new_vector, self.existing_vectors)
            max_similarity = similarities.max()
            
            return float(max_similarity)
        except:
            return 0.0
    
    def add_content(self, content, document_id, content_hash):
        self.existing_hashes.add(content_hash)
        
        # Update vectorizer and vectors periodically
        if len(self.existing_ids) % 100 == 0:
            self.update_similarity_index()
```

## Storage & Attribution

### Document Creation
```python
class ScrapedDocumentCreator:
    def __init__(self, attribution_templates):
        self.attribution_templates = attribution_templates
    
    def create_document(self, content, metadata, source_info):
        attribution = self.generate_attribution(source_info)
        
        return document.Document(
            id=self.generate_document_id(metadata['url']),
            source=document.Source(
                type=source_info['source_type'],
                url=metadata['url'],
                attribution=attribution
            ),
            content=document.Content(
                text=content,
                metadata={
                    **metadata,
                    'scraped': 'true',
                    'scraped_at': datetime.utcnow().isoformat(),
                    'attribution': attribution,
                    'source_compliance': source_info.get('compliance_status'),
                    'quality_score': str(source_info.get('quality_score', 'N/A')),
                    'extraction_method': source_info.get('extraction_method'),
                    'content_language': self.detect_language(content)
                }
            ),
            created_at=datetime.utcnow(),
            updated_at=datetime.utcnow()
        )
    
    def generate_attribution(self, source_info):
        domain = source_info['domain']
        source_type = source_info['source_type']
        
        template = self.attribution_templates.get(domain) or \
                  self.attribution_templates.get(source_type) or \
                  "Content from {domain}, collected by CAIA Tech"
        
        return template.format(
            domain=domain,
            source_type=source_type,
            collection_date=datetime.utcnow().strftime('%Y-%m-%d')
        )
```

## Performance & Scaling

### Distributed Scraping Architecture
```python
import aiohttp
from asyncio import Queue
import asyncio

class DistributedScraper:
    def __init__(self, max_workers=50, max_concurrent_per_domain=2):
        self.max_workers = max_workers
        self.max_concurrent_per_domain = max_concurrent_per_domain
        self.url_queue = Queue()
        self.result_queue = Queue()
        self.domain_semaphores = {}
        self.rate_limiter = AdaptiveRateLimiter()
        
    async def scrape_batch(self, urls):
        # Add URLs to queue
        for url in urls:
            await self.url_queue.put(url)
        
        # Start worker tasks
        workers = [
            asyncio.create_task(self.worker(worker_id))
            for worker_id in range(self.max_workers)
        ]
        
        # Wait for completion
        await self.url_queue.join()
        
        # Cancel workers
        for worker in workers:
            worker.cancel()
        
        # Collect results
        results = []
        while not self.result_queue.empty():
            results.append(await self.result_queue.get())
        
        return results
    
    async def worker(self, worker_id):
        async with aiohttp.ClientSession() as session:
            while True:
                try:
                    url = await asyncio.wait_for(self.url_queue.get(), timeout=1.0)
                except asyncio.TimeoutError:
                    continue
                
                try:
                    result = await self.scrape_url(session, url)
                    await self.result_queue.put(result)
                except Exception as e:
                    logger.error(f"Worker {worker_id} failed to scrape {url}: {e}")
                finally:
                    self.url_queue.task_done()
    
    async def scrape_url(self, session, url):
        domain = self.extract_domain(url)
        
        # Domain-level concurrency control
        semaphore = self.get_domain_semaphore(domain)
        async with semaphore:
            # Rate limiting
            await self.rate_limiter.acquire(domain)
            
            # Compliance checks
            if not await self.check_compliance(url, domain):
                raise ValueError(f"Compliance check failed for {url}")
            
            # Make request
            async with session.get(url, timeout=30) as response:
                if response.status == 200:
                    content = await response.text()
                    return await self.process_content(content, url, response.headers)
                else:
                    raise ValueError(f"HTTP {response.status} for {url}")
```

### Monitoring & Analytics

#### Scraping Metrics Dashboard
```python
class ScrapingMetrics:
    def __init__(self):
        self.metrics = {
            'urls_processed': 0,
            'urls_successful': 0,
            'urls_failed': 0,
            'content_extracted': 0,
            'duplicates_detected': 0,
            'compliance_violations': 0,
            'average_processing_time': 0,
            'domain_performance': {},
            'quality_scores': []
        }
    
    def update_metrics(self, result):
        self.metrics['urls_processed'] += 1
        
        if result['success']:
            self.metrics['urls_successful'] += 1
            self.metrics['content_extracted'] += 1
            self.metrics['quality_scores'].append(result['quality_score'])
        else:
            self.metrics['urls_failed'] += 1
        
        if result.get('duplicate'):
            self.metrics['duplicates_detected'] += 1
        
        if result.get('compliance_violation'):
            self.metrics['compliance_violations'] += 1
        
        domain = result['domain']
        if domain not in self.metrics['domain_performance']:
            self.metrics['domain_performance'][domain] = {
                'success_rate': 0,
                'average_quality': 0,
                'total_processed': 0
            }
        
        self.update_domain_metrics(domain, result)
```

## Implementation Timeline

### Phase 1: Foundation (Week 1)
- Implement compliance checking (robots.txt, ToS)
- Build basic content extraction pipeline
- Create rate limiting and politeness systems
- Set up monitoring and logging

### Phase 2: Quality & Scale (Week 2)
- Advanced content quality validation
- Duplicate detection and deduplication
- Multi-format extraction support
- Distributed scraping architecture

### Phase 3: Integration (Week 3)
- CAIA Library integration
- Event pipeline integration
- Attribution and metadata systems
- Performance optimization

### Phase 4: Production (Week 4)
- Production deployment with monitoring
- Legal compliance validation
- Community feedback integration
- Continuous improvement systems

This web scraping pipeline will provide CAIA Library with ethically sourced, high-quality content from diverse web sources while maintaining strict compliance and attribution standards.
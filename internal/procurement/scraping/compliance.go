package scraping

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ComplianceEngine manages legal and ethical compliance for web scraping
type ComplianceEngine struct {
	robotsCache   map[string]*RobotsData
	robotsMu      sync.RWMutex
	tosCache      map[string]*ToSData
	tosMu         sync.RWMutex
	client        *http.Client
	config        *ComplianceConfig
}

// ComplianceConfig configures compliance checking behavior
type ComplianceConfig struct {
	RespectRobotsTxt     bool          `json:"respect_robots_txt"`
	CheckTermsOfService  bool          `json:"check_terms_of_service"`
	CacheTimeout         time.Duration `json:"cache_timeout"`
	UserAgent            string        `json:"user_agent"`
	MaxConcurrentChecks  int           `json:"max_concurrent_checks"`
	RequestDelay         time.Duration `json:"request_delay"`
	EnableWhitelist      bool          `json:"enable_whitelist"`
	WhitelistedDomains   []string      `json:"whitelisted_domains"`
	BlacklistedDomains   []string      `json:"blacklisted_domains"`
}

// RobotsData represents parsed robots.txt data
type RobotsData struct {
	URL         string            `json:"url"`
	UserAgents  map[string]*Agent `json:"user_agents"`
	Sitemaps    []string          `json:"sitemaps"`
	CrawlDelay  time.Duration     `json:"crawl_delay"`
	LastFetched time.Time         `json:"last_fetched"`
	Valid       bool              `json:"valid"`
}

// Agent represents robots.txt rules for a specific user agent
type Agent struct {
	Name         string        `json:"name"`
	Allow        []string      `json:"allow"`
	Disallow     []string      `json:"disallow"`
	CrawlDelay   time.Duration `json:"crawl_delay"`
	RequestRate  string        `json:"request_rate"`
}

// ToSData represents Terms of Service analysis
type ToSData struct {
	URL               string            `json:"url"`
	AllowsAutomation  bool              `json:"allows_automation"`
	RequiresAttribution bool            `json:"requires_attribution"`
	CommercialUse     bool              `json:"commercial_use"`
	RateLimits        map[string]string `json:"rate_limits"`
	LastAnalyzed      time.Time         `json:"last_analyzed"`
	Confidence        float64           `json:"confidence"`
}

// ComplianceResult represents the result of a compliance check
type ComplianceResult struct {
	URL                string        `json:"url"`
	Domain             string        `json:"domain"`
	Allowed            bool          `json:"allowed"`
	RobotsCompliant    bool          `json:"robots_compliant"`
	ToSCompliant       bool          `json:"tos_compliant"`
	RequiredDelay      time.Duration `json:"required_delay"`
	AttributionNeeded  bool          `json:"attribution_needed"`
	Restrictions       []string      `json:"restrictions"`
	Recommendations    []string      `json:"recommendations"`
	CheckedAt          time.Time     `json:"checked_at"`
}

// NewComplianceEngine creates a new compliance engine
func NewComplianceEngine(config *ComplianceConfig) *ComplianceEngine {
	if config == nil {
		config = DefaultComplianceConfig()
	}
	
	return &ComplianceEngine{
		robotsCache: make(map[string]*RobotsData),
		tosCache:    make(map[string]*ToSData),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: config,
	}
}

// DefaultComplianceConfig returns default compliance configuration
func DefaultComplianceConfig() *ComplianceConfig {
	return &ComplianceConfig{
		RespectRobotsTxt:    true,
		CheckTermsOfService: true,
		CacheTimeout:        24 * time.Hour,
		UserAgent:           "CAIA-Library/1.0 (+https://caia.tech/bot)",
		MaxConcurrentChecks: 5,
		RequestDelay:        1 * time.Second,
		EnableWhitelist:     false,
		WhitelistedDomains: []string{
			"arxiv.org",
			"github.com",
			"stackoverflow.com",
			"docs.python.org",
			"golang.org",
		},
		BlacklistedDomains: []string{
			"facebook.com",
			"twitter.com",
			"instagram.com",
			"tiktok.com",
		},
	}
}

// CheckCompliance performs comprehensive compliance checking for a URL
func (ce *ComplianceEngine) CheckCompliance(ctx context.Context, targetURL string) (*ComplianceResult, error) {
	start := time.Now()
	
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	
	domain := parsedURL.Host
	
	log.Debug().
		Str("url", targetURL).
		Str("domain", domain).
		Msg("Starting compliance check")
	
	result := &ComplianceResult{
		URL:           targetURL,
		Domain:        domain,
		CheckedAt:     time.Now(),
		Restrictions:  make([]string, 0),
		Recommendations: make([]string, 0),
	}
	
	// Check domain whitelist/blacklist first
	if ce.config.EnableWhitelist {
		if !ce.isDomainWhitelisted(domain) {
			result.Allowed = false
			result.Restrictions = append(result.Restrictions, "Domain not in whitelist")
			return result, nil
		}
	}
	
	if ce.isDomainBlacklisted(domain) {
		result.Allowed = false
		result.Restrictions = append(result.Restrictions, "Domain is blacklisted")
		return result, nil
	}
	
	// Check robots.txt compliance
	robotsCompliant, robotsDelay, err := ce.checkRobotsCompliance(ctx, targetURL)
	if err != nil {
		log.Warn().Err(err).Str("url", targetURL).Msg("Failed to check robots.txt compliance")
		robotsCompliant = false // Err on the side of caution
	}
	
	result.RobotsCompliant = robotsCompliant
	result.RequiredDelay = robotsDelay
	
	if !robotsCompliant {
		result.Restrictions = append(result.Restrictions, "Blocked by robots.txt")
	}
	
	// Check Terms of Service compliance (if enabled)
	tosCompliant := true
	if ce.config.CheckTermsOfService {
		tosData, err := ce.checkToSCompliance(ctx, domain)
		if err != nil {
			log.Warn().Err(err).Str("domain", domain).Msg("Failed to check ToS compliance")
		} else {
			tosCompliant = tosData.AllowsAutomation
			result.AttributionNeeded = tosData.RequiresAttribution
			
			if !tosCompliant {
				result.Restrictions = append(result.Restrictions, "Terms of Service prohibit automation")
			}
			
			if tosData.RequiresAttribution {
				result.Recommendations = append(result.Recommendations, "Attribution required")
			}
		}
	}
	
	result.ToSCompliant = tosCompliant
	result.Allowed = robotsCompliant && tosCompliant
	
	// Add general recommendations
	if result.RequiredDelay > 0 {
		result.Recommendations = append(result.Recommendations, 
			fmt.Sprintf("Respect crawl delay of %v", result.RequiredDelay))
	}
	
	result.Recommendations = append(result.Recommendations, 
		"Use respectful crawling practices", "Monitor for rate limiting")
	
	log.Debug().
		Str("url", targetURL).
		Bool("allowed", result.Allowed).
		Bool("robots_compliant", result.RobotsCompliant).
		Bool("tos_compliant", result.ToSCompliant).
		Dur("required_delay", result.RequiredDelay).
		Dur("check_duration", time.Since(start)).
		Msg("Compliance check completed")
	
	return result, nil
}

// checkRobotsCompliance checks if scraping a URL is allowed by robots.txt
func (ce *ComplianceEngine) checkRobotsCompliance(ctx context.Context, targetURL string) (bool, time.Duration, error) {
	if !ce.config.RespectRobotsTxt {
		return true, 0, nil
	}
	
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return false, 0, err
	}
	
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	robotsURL := fmt.Sprintf("%s/robots.txt", baseURL)
	
	// Check cache first
	ce.robotsMu.RLock()
	robotsData, exists := ce.robotsCache[baseURL]
	ce.robotsMu.RUnlock()
	
	if exists && time.Since(robotsData.LastFetched) < ce.config.CacheTimeout {
		return ce.isPathAllowed(robotsData, parsedURL.Path), robotsData.CrawlDelay, nil
	}
	
	// Fetch robots.txt
	robotsData, err = ce.fetchRobotsTxt(ctx, robotsURL)
	if err != nil {
		// If robots.txt doesn't exist or is inaccessible, assume allowed
		log.Debug().Err(err).Str("robots_url", robotsURL).Msg("Could not fetch robots.txt, assuming allowed")
		return true, ce.config.RequestDelay, nil
	}
	
	// Cache the result
	ce.robotsMu.Lock()
	ce.robotsCache[baseURL] = robotsData
	ce.robotsMu.Unlock()
	
	allowed := ce.isPathAllowed(robotsData, parsedURL.Path)
	delay := robotsData.CrawlDelay
	if delay == 0 {
		delay = ce.config.RequestDelay
	}
	
	return allowed, delay, nil
}

// fetchRobotsTxt fetches and parses robots.txt
func (ce *ComplianceEngine) fetchRobotsTxt(ctx context.Context, robotsURL string) (*RobotsData, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("User-Agent", ce.config.UserAgent)
	
	resp, err := ce.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("robots.txt returned status %d", resp.StatusCode)
	}
	
	// Parse robots.txt (simplified implementation)
	robotsData := &RobotsData{
		URL:         robotsURL,
		UserAgents:  make(map[string]*Agent),
		Sitemaps:    make([]string, 0),
		LastFetched: time.Now(),
		Valid:       true,
	}
	
	// Read and parse content (simplified parser)
	buf := make([]byte, 64*1024) // 64KB limit for robots.txt
	n, _ := resp.Body.Read(buf)
	content := string(buf[:n])
	
	ce.parseRobotsTxt(robotsData, content)
	
	return robotsData, nil
}

// parseRobotsTxt parses robots.txt content (simplified implementation)
func (ce *ComplianceEngine) parseRobotsTxt(robotsData *RobotsData, content string) {
	lines := strings.Split(content, "\n")
	var currentAgent *Agent
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		
		field := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		
		switch field {
		case "user-agent":
			currentAgent = &Agent{
				Name:     value,
				Allow:    make([]string, 0),
				Disallow: make([]string, 0),
			}
			robotsData.UserAgents[value] = currentAgent
			
		case "disallow":
			if currentAgent != nil {
				currentAgent.Disallow = append(currentAgent.Disallow, value)
			}
			
		case "allow":
			if currentAgent != nil {
				currentAgent.Allow = append(currentAgent.Allow, value)
			}
			
		case "crawl-delay":
			if currentAgent != nil {
				if delay, err := time.ParseDuration(value + "s"); err == nil {
					currentAgent.CrawlDelay = delay
					if robotsData.CrawlDelay == 0 || delay > robotsData.CrawlDelay {
						robotsData.CrawlDelay = delay
					}
				}
			}
			
		case "sitemap":
			robotsData.Sitemaps = append(robotsData.Sitemaps, value)
			
		case "request-rate":
			if currentAgent != nil {
				currentAgent.RequestRate = value
			}
		}
	}
}

// isPathAllowed checks if a path is allowed by robots.txt
func (ce *ComplianceEngine) isPathAllowed(robotsData *RobotsData, path string) bool {
	userAgent := ce.config.UserAgent
	
	// Check specific user agent first
	if agent, exists := robotsData.UserAgents[userAgent]; exists {
		return ce.checkAgentRules(agent, path)
	}
	
	// Check wildcard user agent
	if agent, exists := robotsData.UserAgents["*"]; exists {
		return ce.checkAgentRules(agent, path)
	}
	
	// If no rules found, assume allowed
	return true
}

// checkAgentRules checks agent-specific allow/disallow rules
func (ce *ComplianceEngine) checkAgentRules(agent *Agent, path string) bool {
	// Check explicit allow rules first
	for _, allowRule := range agent.Allow {
		if ce.matchesPattern(allowRule, path) {
			return true
		}
	}
	
	// Check disallow rules
	for _, disallowRule := range agent.Disallow {
		if ce.matchesPattern(disallowRule, path) {
			return false
		}
	}
	
	// Default to allowed
	return true
}

// matchesPattern checks if a path matches a robots.txt pattern
func (ce *ComplianceEngine) matchesPattern(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	
	// Exact match
	if pattern == path {
		return true
	}
	
	// Prefix match
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	
	// Simple prefix match for paths ending with /
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	
	return strings.HasPrefix(path, pattern)
}

// checkToSCompliance checks Terms of Service compliance (simplified implementation)
func (ce *ComplianceEngine) checkToSCompliance(ctx context.Context, domain string) (*ToSData, error) {
	// Check cache first
	ce.tosMu.RLock()
	tosData, exists := ce.tosCache[domain]
	ce.tosMu.RUnlock()
	
	if exists && time.Since(tosData.LastAnalyzed) < ce.config.CacheTimeout {
		return tosData, nil
	}
	
	// For now, implement a simple heuristic-based approach
	// In production, this would integrate with ToS analysis services
	tosData = &ToSData{
		URL:              fmt.Sprintf("https://%s/terms", domain),
		AllowsAutomation: ce.getDomainPolicy(domain),
		RequiresAttribution: ce.requiresAttribution(domain),
		CommercialUse:    false, // Conservative default
		LastAnalyzed:     time.Now(),
		Confidence:       0.7, // Medium confidence for heuristic
	}
	
	// Cache the result
	ce.tosMu.Lock()
	ce.tosCache[domain] = tosData
	ce.tosMu.Unlock()
	
	return tosData, nil
}

// getDomainPolicy returns automation policy for known domains
func (ce *ComplianceEngine) getDomainPolicy(domain string) bool {
	// Known automation-friendly domains
	friendlyDomains := map[string]bool{
		"arxiv.org":        true,
		"github.com":       true,
		"stackoverflow.com": true,
		"docs.python.org":  true,
		"golang.org":       true,
		"wikipedia.org":    true,
		"archive.org":      true,
	}
	
	// Check exact match
	if policy, exists := friendlyDomains[domain]; exists {
		return policy
	}
	
	// Check subdomain patterns
	for friendlyDomain := range friendlyDomains {
		if strings.HasSuffix(domain, "."+friendlyDomain) {
			return friendlyDomains[friendlyDomain]
		}
	}
	
	// Conservative default for unknown domains
	return false
}

// requiresAttribution checks if domain requires attribution
func (ce *ComplianceEngine) requiresAttribution(domain string) bool {
	attributionRequired := map[string]bool{
		"arxiv.org":     true,
		"wikipedia.org": true,
		"archive.org":   true,
	}
	
	return attributionRequired[domain]
}

// isDomainWhitelisted checks if domain is in whitelist
func (ce *ComplianceEngine) isDomainWhitelisted(domain string) bool {
	for _, whitelisted := range ce.config.WhitelistedDomains {
		if domain == whitelisted || strings.HasSuffix(domain, "."+whitelisted) {
			return true
		}
	}
	return false
}

// isDomainBlacklisted checks if domain is in blacklist
func (ce *ComplianceEngine) isDomainBlacklisted(domain string) bool {
	for _, blacklisted := range ce.config.BlacklistedDomains {
		if domain == blacklisted || strings.HasSuffix(domain, "."+blacklisted) {
			return true
		}
	}
	return false
}

// GetCachedRobotsData returns cached robots.txt data for a domain
func (ce *ComplianceEngine) GetCachedRobotsData(domain string) *RobotsData {
	ce.robotsMu.RLock()
	defer ce.robotsMu.RUnlock()
	return ce.robotsCache[domain]
}

// ClearExpiredCache removes expired entries from all caches
func (ce *ComplianceEngine) ClearExpiredCache() (int, int) {
	robotsCleared := 0
	tosCleared := 0
	
	ce.robotsMu.Lock()
	for domain, data := range ce.robotsCache {
		if time.Since(data.LastFetched) > ce.config.CacheTimeout {
			delete(ce.robotsCache, domain)
			robotsCleared++
		}
	}
	ce.robotsMu.Unlock()
	
	ce.tosMu.Lock()
	for domain, data := range ce.tosCache {
		if time.Since(data.LastAnalyzed) > ce.config.CacheTimeout {
			delete(ce.tosCache, domain)
			tosCleared++
		}
	}
	ce.tosMu.Unlock()
	
	if robotsCleared > 0 || tosCleared > 0 {
		log.Info().
			Int("robots_cleared", robotsCleared).
			Int("tos_cleared", tosCleared).
			Msg("Expired compliance cache entries cleared")
	}
	
	return robotsCleared, tosCleared
}
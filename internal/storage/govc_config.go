package storage

import (
	"os"
	"time"
)

// GetGovcConfig returns govc configuration from environment variables
func GetGovcConfig() *GovcConfig {
	config := &GovcConfig{
		MemoryMode: true, // Default to memory mode for performance
		Path:       ":memory:",
		Timeout:    30 * time.Second,
	}

	// Override with environment variables if set
	if mode := os.Getenv("GOVC_MEMORY_MODE"); mode == "false" || mode == "0" {
		config.MemoryMode = false
		config.Path = os.Getenv("GOVC_REPO_PATH")
		if config.Path == "" {
			config.Path = "./data/govc"
		}
	}

	if timeout := os.Getenv("GOVC_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Timeout = d
		}
	}

	return config
}

// IsGovcEnabled checks if govc should be used as primary backend
func IsGovcEnabled() bool {
	// Check explicit enable flag
	if enabled := os.Getenv("CAIA_USE_GOVC"); enabled == "true" || enabled == "1" {
		return true
	}

	// Check if govc service URL is configured
	if url := os.Getenv("GOVC_SERVICE_URL"); url != "" {
		return true
	}

	return false
}

// GetGovcRepoName returns the repository name to use for govc
func GetGovcRepoName() string {
	if name := os.Getenv("GOVC_REPO_NAME"); name != "" {
		return name
	}
	return "caia-documents"
}
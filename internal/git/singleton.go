package git

import (
	"sync"
)

var (
	safeRepoInstance *SafeRepository
	safeRepoOnce     sync.Once
	safeRepoMutex    sync.Mutex
)

// GetSafeRepository returns a singleton SafeRepository instance
// This ensures all Git operations go through the same mutex-protected instance
func GetSafeRepository(path string) (*SafeRepository, error) {
	var err error
	safeRepoOnce.Do(func() {
		safeRepoInstance, err = NewSafeRepository(path)
	})
	if err != nil {
		return nil, err
	}
	return safeRepoInstance, nil
}

// ResetSingleton resets the singleton instance (useful for testing)
func ResetSingleton() {
	safeRepoMutex.Lock()
	defer safeRepoMutex.Unlock()
	safeRepoInstance = nil
	safeRepoOnce = sync.Once{}
}
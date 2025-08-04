package git

import (
	"sync"
)

// Global mutex for Git operations to prevent race conditions
// Git operations are not thread-safe, especially when working with the same repository
var gitMutex sync.Mutex

// WithGitLock executes a function with the Git mutex locked
func WithGitLock(fn func() error) error {
	gitMutex.Lock()
	defer gitMutex.Unlock()
	return fn()
}
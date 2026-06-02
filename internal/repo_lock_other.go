//go:build !unix

package internal

import (
	"fmt"
	"os"
)

// Platforms without syscall.Flock support still open/close the lock file so
// lock-path errors are surfaced consistently.
type repoMutationLock struct {
	file *os.File
	path string
}

func acquireRepoMutationLock(lockPath string) (repoMutationLock, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return repoMutationLock{}, fmt.Errorf("opening %s: %w", lockPath, err)
	}
	return repoMutationLock{
		file: file,
		path: lockPath,
	}, nil
}

func (l repoMutationLock) release() error {
	if l.file == nil {
		return nil
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", l.path, err)
	}
	return nil
}

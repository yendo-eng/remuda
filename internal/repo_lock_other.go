//go:build !unix

package internal

import (
	"os"

	pkgerrors "github.com/pkg/errors"
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
		return repoMutationLock{}, pkgerrors.Wrapf(err, "opening %s", lockPath)
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
		return pkgerrors.Wrapf(err, "closing %s", l.path)
	}
	return nil
}

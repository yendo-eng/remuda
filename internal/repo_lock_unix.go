//go:build unix

package internal

import (
	"errors"
	"fmt"
	"math"
	"os"
	"syscall"
)

type repoMutationLock struct {
	file *os.File
	path string
}

func acquireRepoMutationLock(lockPath string) (repoMutationLock, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return repoMutationLock{}, fmt.Errorf("opening %s: %w", lockPath, err)
	}

	fd, err := fileDescriptorInt(file)
	if err != nil {
		_ = file.Close()
		return repoMutationLock{}, fmt.Errorf("locking %s: %w", lockPath, err)
	}

	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return repoMutationLock{}, fmt.Errorf("locking %s: %w", lockPath, err)
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

	fd, err := fileDescriptorInt(l.file)
	if err != nil {
		closeErr := l.file.Close()
		if closeErr == nil {
			return fmt.Errorf("unlocking %s: %w", l.path, err)
		}
		return fmt.Errorf("unlock/close %s: %w", l.path, errors.Join(err, closeErr))
	}

	unlockErr := syscall.Flock(fd, syscall.LOCK_UN)
	closeErr := l.file.Close()
	if unlockErr == nil {
		if closeErr != nil {
			return fmt.Errorf("closing %s: %w", l.path, closeErr)
		}
		return nil
	}
	if closeErr == nil {
		return fmt.Errorf("unlocking %s: %w", l.path, unlockErr)
	}

	return fmt.Errorf("unlock/close %s: %w", l.path, errors.Join(unlockErr, closeErr))
}

func fileDescriptorInt(file *os.File) (int, error) {
	fd := file.Fd()
	if fd > uintptr(math.MaxInt) {
		return 0, fmt.Errorf("file descriptor %d exceeds int range", fd)
	}
	return int(fd), nil
}

//go:build unix

package internal

import (
	"errors"

	"math"
	"os"
	"syscall"

	pkgerrors "github.com/pkg/errors"
)

type repoMutationLock struct {
	file *os.File
	path string
}

func acquireRepoMutationLock(lockPath string) (repoMutationLock, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return repoMutationLock{}, pkgerrors.Wrapf(err, "opening %s", lockPath)
	}

	fd, err := fileDescriptorInt(file)
	if err != nil {
		_ = file.Close()
		return repoMutationLock{}, pkgerrors.Wrapf(err, "locking %s", lockPath)
	}

	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return repoMutationLock{}, pkgerrors.Wrapf(err, "locking %s", lockPath)
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
			return pkgerrors.Wrapf(err, "unlocking %s", l.path)
		}
		return pkgerrors.Wrapf(errors.Join(err, closeErr), "unlock/close %s", l.path)
	}

	unlockErr := syscall.Flock(fd, syscall.LOCK_UN)
	closeErr := l.file.Close()
	if unlockErr == nil {
		if closeErr != nil {
			return pkgerrors.Wrapf(closeErr, "closing %s", l.path)
		}
		return nil
	}
	if closeErr == nil {
		return pkgerrors.Wrapf(unlockErr, "unlocking %s", l.path)
	}

	return pkgerrors.Wrapf(errors.Join(unlockErr, closeErr), "unlock/close %s", l.path)
}

func fileDescriptorInt(file *os.File) (int, error) {
	fd := file.Fd()
	if fd > uintptr(math.MaxInt) {
		return 0, pkgerrors.Errorf("file descriptor %d exceeds int range", fd)
	}
	return int(fd), nil
}

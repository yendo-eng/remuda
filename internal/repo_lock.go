package internal

import (
	"path/filepath"

	pkgerrors "github.com/pkg/errors"
)

func withRepoMutationLock(baseDir string, fn func() error) (err error) {
	lockPath := filepath.Join(baseDir, ".repo_cache.lock")
	lock, err := acquireRepoMutationLock(lockPath)
	if err != nil {
		return pkgerrors.Wrap(err, "acquiring repo mutation lock")
	}

	defer func() {
		unlockErr := lock.release()
		if unlockErr == nil {
			return
		}
		if err == nil {
			err = pkgerrors.Wrap(unlockErr, "releasing repo mutation lock")
			return
		}
		err = pkgerrors.Wrapf(err, "additionally, releasing repo mutation lock: %s", unlockErr.Error())
	}()

	return fn()
}

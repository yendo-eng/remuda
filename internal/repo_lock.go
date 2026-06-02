package internal

import (
	"fmt"
	"path/filepath"
)

func withRepoMutationLock(baseDir string, fn func() error) (err error) {
	lockPath := filepath.Join(baseDir, ".repo_cache.lock")
	lock, err := acquireRepoMutationLock(lockPath)
	if err != nil {
		return fmt.Errorf("acquiring repo mutation lock: %w", err)
	}

	defer func() {
		unlockErr := lock.release()
		if unlockErr == nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("releasing repo mutation lock: %w", unlockErr)
			return
		}
		err = fmt.Errorf("%w; additionally, releasing repo mutation lock: %s", err, unlockErr.Error())
	}()

	return fn()
}

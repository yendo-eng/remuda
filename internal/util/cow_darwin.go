//go:build darwin

package util

import (
	"errors"
	"os"

	pkgerrors "github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// cloneFile clones src to dst with clonefile(2). clonefile refuses to write an
// existing destination, so a colliding dst is removed first to keep CopyDir's
// overwrite semantics.
func cloneFile(src, dst string) error {
	err := unix.Clonefile(src, dst, 0)
	if errors.Is(err, unix.EEXIST) {
		if err := os.Remove(dst); err != nil {
			return err
		}
		err = unix.Clonefile(src, dst, 0)
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EXDEV), errors.Is(err, unix.EINVAL):
		return errCoWUnsupported
	}
	return pkgerrors.Wrap(err, "clonefile")
}

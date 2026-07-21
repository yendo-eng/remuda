package util

import (
	"errors"

	pkgerrors "github.com/pkg/errors"
)

// errCoWUnsupported reports that the platform or the filesystem cannot clone a
// file copy-on-write, so the caller must copy its bytes instead.
var errCoWUnsupported = pkgerrors.New("copy-on-write clone unsupported")

// CoWCopyDir copies src into dst like CopyDir, except that regular files are
// cloned copy-on-write (clonefile on macOS, FICLONE reflinks on Linux) so the
// copy shares blocks with the source until one of them is written. That needs
// a capable filesystem (APFS, btrfs, XFS with reflink=1, bcachefs) and both
// paths on the same volume; when cloning is unavailable the tree is copied
// byte-for-byte instead. Reports whether files were cloned.
func CoWCopyDir(src, dst string) (bool, error) {
	cloned := false
	supported := true
	err := copyTree(src, dst, func(src, dst string) error {
		if supported {
			err := cloneFile(src, dst)
			if err == nil {
				cloned = true
				return nil
			}
			if !errors.Is(err, errCoWUnsupported) {
				return err
			}
			supported = false
		}
		return CopyFile(src, dst)
	})
	return cloned, err
}

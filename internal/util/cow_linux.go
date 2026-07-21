//go:build linux

package util

import (
	"errors"
	"os"

	pkgerrors "github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// cloneFile reflinks src to dst with the FICLONE ioctl, supported by btrfs,
// XFS with reflink=1 and bcachefs.
func cloneFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()&os.ModePerm)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		// dst is an empty placeholder at this point; drop it so the byte-copy
		// fallback starts from a clean slate.
		_ = os.Remove(dst)
		switch {
		case errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EXDEV), errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOTTY):
			return errCoWUnsupported
		}
		return pkgerrors.Wrap(err, "ficlone")
	}
	return nil
}

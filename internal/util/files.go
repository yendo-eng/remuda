package util

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
)

// CopyFile copies a regular file from src to dst, creating dst with the same
// permissions as src when possible.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	mode := info.Mode() & os.ModePerm

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// CopyDir copies the contents of src into dst, preserving permissions.
// Destination is created if it does not exist. Existing contents are
// overwritten when files collide. Sockets are skipped and symlinks are
// recreated as symlinks.
func CopyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &fs.PathError{Op: "copydir", Path: src, Err: pkgerrors.Errorf("not a directory")}
	}
	if err := os.MkdirAll(dst, info.Mode()&os.ModePerm); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			//nolint:gosec // G122: source tree and destination root are controlled by caller.
			if err := os.Symlink(linkTarget, target); err != nil {
				return err
			}
			return nil
		}
		if mode&os.ModeSocket != 0 {
			return nil
		}
		if mode.IsDir() {
			return os.MkdirAll(target, mode&os.ModePerm)
		}
		if mode.IsRegular() {
			return CopyFile(path, target)
		}
		return &fs.PathError{Op: "copydir", Path: path, Err: fs.ErrInvalid}
	})
}

// SplitWorkspacePath returns org, repo, folder given a workspace path under base.
func SplitWorkspacePath(base, ws string) (org, repo, folder string) {
	rel, err := filepath.Rel(base, ws)
	if err != nil {
		return
	}
	// Convert to slash form and split into segments.
	rel = filepath.ToSlash(rel)
	segs := []string{}
	for _, s := range strings.Split(rel, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	if len(segs) >= 3 {
		org, repo, folder = segs[0], segs[1], segs[2]
	}
	return
}

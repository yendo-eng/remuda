package util

import (
	"io/fs"
	"path/filepath"
)

// DirSize returns the sum of file sizes under root.
//
// Symlink targets are not included; the symlink itself is not counted.
func DirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Do not follow symlinks; removing the workspace only removes the link itself.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

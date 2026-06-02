package cli

import (
	"path/filepath"
	"strings"
)

func resolvePathFromWorkingDir(path, workingDir string) string {
	if path == "" || workingDir == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workingDir, path)
}

func absPathFromContext(path string, kctx Context) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~") {
		home, homeErr := homeDirFromContext(kctx)
		if expanded, err := expandHomePath(path, home, homeErr); err == nil && expanded != "" {
			path = expanded
		}
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	workingDir := workingDirFromContext(kctx)
	if workingDir == "" {
		if abs, err := filepath.Abs(path); err == nil && abs != "" {
			return abs
		}
		return filepath.Clean(path)
	}
	joined := filepath.Join(workingDir, path)
	if filepath.IsAbs(joined) {
		return filepath.Clean(joined)
	}
	if abs, err := filepath.Abs(joined); err == nil && abs != "" {
		return abs
	}
	return filepath.Clean(joined)
}

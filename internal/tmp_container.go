package internal

import (
	"fmt"
	"strings"
)

// tmpContainerMountError returns a clear, actionable error when a --tmp worktree
// cannot be reliably bind-mounted into a Docker container, or nil when the mount
// should work.
//
// On macOS, os.TempDir() resolves under /var/folders, which Docker Desktop does
// not include in its default file-sharing set, so mounting a temp worktree fails
// with an opaque error deep inside `docker run`. We catch that case up front and
// tell the user how to fix it: share the path in Docker Desktop, or point
// REMUDA_TMP_DIR at a directory under their home (which Docker Desktop shares by
// default). On other platforms bind-mounting temp paths works, so this is a
// no-op.
func tmpContainerMountError(workspaceAbs, tmpBaseDir, home, goos string) error {
	if !strings.EqualFold(strings.TrimSpace(goos), "darwin") {
		return nil
	}
	if strings.TrimSpace(tmpBaseDir) == "" {
		return nil
	}
	// Only relevant for worktrees that actually live under the temp root.
	if !pathWithin(tmpBaseDir, workspaceAbs) {
		return nil
	}
	// Docker Desktop shares the user's home directory by default, so a temp root
	// relocated under $HOME (via REMUDA_TMP_DIR) is mountable.
	if strings.TrimSpace(home) != "" && pathWithin(home, workspaceAbs) {
		return nil
	}
	return fmt.Errorf(
		"--tmp worktree %q is under the OS temp dir, which Docker Desktop does not share by default on macOS; "+
			"either add the path under Docker Desktop > Settings > Resources > File Sharing, "+
			"or set REMUDA_TMP_DIR to a directory under your home (e.g. ~/.remuda/tmp) and retry",
		workspaceAbs,
	)
}

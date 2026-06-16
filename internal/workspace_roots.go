package internal

import (
	"path/filepath"
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
)

// workspaceRoots returns the directory roots under which Remuda worktrees may
// live, in resolution priority order: the persistent repos base dir first, then
// the OS-temp root used by --tmp sessions (when configured). Session-derived
// workspace lookups probe these in order.
func (k Remuda) workspaceRoots() []string {
	roots := []string{k.Config.ReposBaseDir}
	if tmp := strings.TrimSpace(k.Config.TmpBaseDir); tmp != "" {
		roots = append(roots, tmp)
	}
	return roots
}

// splitWorkspaceAnyRoot returns the org/repo/folder segments for a workspace path
// relative to whichever configured root contains it. The persistent .repo_cache
// always lives under ReposBaseDir regardless of the matched root, so callers
// computing a cache path should combine the returned org/repo with ReposBaseDir.
func (k Remuda) splitWorkspaceAnyRoot(workspace string) (org, repo, folder string) {
	for _, root := range k.workspaceRoots() {
		o, r, f := util.SplitWorkspacePath(root, workspace)
		if o != "" && r != "" && f != "" {
			return o, r, f
		}
	}
	return "", "", ""
}

// cacheDirForWorkspace returns the persistent .repo_cache directory associated
// with a worktree. The cache always lives under ReposBaseDir even when the
// worktree itself is a --tmp checkout under the OS-temp root.
func (k Remuda) cacheDirForWorkspace(workspace string) string {
	org, repo, _ := k.splitWorkspaceAnyRoot(workspace)
	if org == "" || repo == "" {
		return ""
	}
	return filepath.Join(k.Config.ReposBaseDir, org, repo, ".repo_cache")
}

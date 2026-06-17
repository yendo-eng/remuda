package internal

import (
	"fmt"
	"os"
	"path"
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
		if !pathWithin(root, workspace) {
			continue
		}
		o, r, f := util.SplitWorkspacePath(root, workspace)
		if o != "" && r != "" && f != "" {
			return o, r, f
		}
	}
	return "", "", ""
}

// pathWithin reports whether target resolves to a location inside base.
func pathWithin(base, target string) bool {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

// cacheDirForWorkspace returns the persistent .repo_cache directory associated
// with a worktree. The cache always lives under ReposBaseDir even when the
// worktree itself is a --tmp checkout under the OS-temp root.
func (k Remuda) cacheDirForWorkspace(workspace string) string {
	org, repo, _ := k.splitWorkspaceAnyRoot(workspace)
	if org != "" && repo != "" {
		return filepath.Join(k.Config.ReposBaseDir, org, repo, ".repo_cache")
	}
	// Fallback for workspaces outside the managed roots (e.g. --in): assume the
	// cache is a sibling, matching the historical layout.
	return filepath.Join(filepath.Dir(workspace), ".repo_cache")
}

func (k Remuda) isTmpWorkspace(workspace string) bool {
	tmp := strings.TrimSpace(k.Config.TmpBaseDir)
	return tmp != "" && pathWithin(tmp, workspace)
}

// ensureNoCrossRootWorkspaceDuplicate rejects ambiguous workspace identities
// where the same org/repo/folder exists under multiple managed roots. Session
// names intentionally remain org/repo/folder, so allowing both roots to contain
// the same relative workspace would make active-session and cleanup lookups
// resolve the wrong path.
func (k Remuda) ensureNoCrossRootWorkspaceDuplicate(workspace string) error {
	org, repo, folder := k.splitWorkspaceAnyRoot(workspace)
	if org == "" || repo == "" || folder == "" {
		return nil
	}

	targetAbs, err := filepath.Abs(workspace)
	if err != nil {
		targetAbs = workspace
	}
	targetAbs = filepath.Clean(targetAbs)

	var conflicts []string
	for _, root := range k.workspaceRoots() {
		if strings.TrimSpace(root) == "" {
			continue
		}
		candidate := filepath.Join(root, org, repo, folder)
		candidateAbs, err := filepath.Abs(candidate)
		if err != nil {
			candidateAbs = candidate
		}
		candidateAbs = filepath.Clean(candidateAbs)
		if candidateAbs == targetAbs {
			continue
		}
		if st, err := os.Stat(candidateAbs); err == nil && st.IsDir() {
			conflicts = append(conflicts, candidateAbs)
		}
	}
	if len(conflicts) == 0 {
		return nil
	}

	return fmt.Errorf(
		"workspace %q is ambiguous across workspace roots; duplicate workspace exists at %s; use a different name or remove the duplicate workspace before creating or resuming",
		path.Join(org, repo, folder),
		strings.Join(conflicts, ", "),
	)
}

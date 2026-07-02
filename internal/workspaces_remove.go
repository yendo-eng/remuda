package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/util"
)

// WorkspacesRemove removes explicitly targeted workspaces.
// It refuses to remove workspaces with active Remuda sessions.
func (k Remuda) WorkspacesRemove(workspaces []string, dryRun bool, force bool) ([]PrunedWorkspace, error) {
	active, err := k.activeWorkspaceSessions()
	if err != nil {
		return nil, err
	}

	logger := k.logger()
	seen := map[string]struct{}{}
	removed := make([]PrunedWorkspace, 0, len(workspaces))
	var failures []string

	for _, workspace := range workspaces {
		workspace = strings.TrimSpace(workspace)
		if workspace == "" {
			failures = append(failures, "workspace target is blank")
			continue
		}

		workspaceAbs, err := filepath.Abs(workspace)
		if err != nil {
			failures = append(failures, fmt.Sprintf("resolve workspace %q: %v", workspace, err))
			continue
		}
		workspaceAbs = filepath.Clean(workspaceAbs)
		if _, ok := seen[workspaceAbs]; ok {
			continue
		}
		seen[workspaceAbs] = struct{}{}

		if err := validateWorkspacePath(k.Config.ReposBaseDir, workspaceAbs); err != nil {
			failures = append(failures, pkgerrors.Wrapf(err, "invalid workspace %q", workspaceAbs).Error())
			continue
		}

		if sessionName, ok := active[workspaceAbs]; ok {
			failures = append(failures, fmt.Sprintf("workspace %q is active (session %q); refuse to remove", workspaceAbs, sessionName))
			continue
		}

		info, err := os.Stat(workspaceAbs)
		if err != nil {
			if os.IsNotExist(err) {
				failures = append(failures, fmt.Sprintf("workspace %q does not exist", workspaceAbs))
				continue
			}
			failures = append(failures, fmt.Sprintf("stat workspace %q: %v", workspaceAbs, err))
			continue
		}
		if !info.IsDir() {
			failures = append(failures, fmt.Sprintf("workspace %q is not a directory", workspaceAbs))
			continue
		}

		bytes, err := util.DirSize(workspaceAbs)
		if err != nil {
			logger.Warn().Err(err).Str("workspace", workspaceAbs).Msg("failed to compute workspace size")
		}

		if err := k.PruneOneSession(workspaceAbs, true, dryRun, force); err != nil {
			failures = append(failures, err.Error())
			continue
		}

		removed = append(removed, PrunedWorkspace{Path: workspaceAbs, Bytes: bytes})
	}

	if len(failures) > 0 {
		return removed, pkgerrors.New(strings.Join(failures, "\n"))
	}

	return removed, nil
}

func (k Remuda) activeWorkspaceSessions() (map[string]string, error) {
	sessions, err := k.Session.List()
	if err != nil {
		return nil, err
	}

	active := map[string]string{}
	for _, s := range sessions {
		if !s.IsRemudaSession() {
			continue
		}

		workspace, err := s.WorkspacePath(k.Config.ReposBaseDir)
		if err != nil {
			continue
		}
		workspaceAbs, err := filepath.Abs(workspace)
		if err != nil {
			workspaceAbs = workspace
		}

		active[filepath.Clean(workspaceAbs)] = s.Name
	}
	return active, nil
}

func (k Remuda) PruneOneSession(
	workspace string,
	clean bool,
	dryRun bool,
	force bool,
) error {
	logger := k.logger()
	if err := validateWorkspacePath(k.Config.ReposBaseDir, workspace); err != nil {
		return pkgerrors.Wrapf(err, "invalid workspace %q", workspace)
	}
	if dryRun {
		return nil
	}

	// Best-effort: remove from git worktrees first, then delete folder.
	// cache: <base>/<org>/<repo>/.repo_cache
	org, repo, _ := util.SplitWorkspacePath(k.Config.ReposBaseDir, workspace)
	if org != "" && isLinkedGitWorktree(workspace) {
		cache := filepath.Join(k.Config.ReposBaseDir, org, repo, ".repo_cache")
		args := []string{workspace}
		if force {
			args = append(args, "--force")
		}
		if err := k.Git.WorktreeRemove(cache, args...); err != nil {
			if force {
				logger.Warn().Err(err).Str("cache", cache).Str("workspace", workspace).Msg("worktree remove failed")
			} else {
				return pkgerrors.Wrapf(err, "remove linked git worktree %q (rerun with --force to override)", workspace)
			}
		}
	}
	if err := os.RemoveAll(workspace); err != nil {
		return pkgerrors.Wrapf(err, "remove workspace %q", workspace)
	}

	return nil
}

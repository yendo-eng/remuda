package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/util"
)

type RenamedWorkspace struct {
	OldPath string
	NewPath string
}

// WorkspacesRename renames an inactive workspace path and its default branch.
// The workspace must be under repos base dir and mapped at depth 3:
// <repos-base>/<org>/<repo>/<workspace>.
func (k Remuda) WorkspacesRename(workspace string, newName string) (RenamedWorkspace, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return RenamedWorkspace{}, errors.New("workspace target is blank")
	}

	reposBaseAbs, err := filepath.Abs(k.Config.ReposBaseDir)
	if err != nil {
		return RenamedWorkspace{}, errors.Wrap(err, "abs repos base dir")
	}
	reposBaseAbs = filepath.Clean(reposBaseAbs)

	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return RenamedWorkspace{}, errors.Wrapf(err, "resolve workspace %q", workspace)
	}
	workspaceAbs = filepath.Clean(workspaceAbs)

	if err := validateWorkspacePath(reposBaseAbs, workspaceAbs); err != nil {
		return RenamedWorkspace{}, errors.Wrapf(err, "invalid workspace %q", workspaceAbs)
	}

	active, err := k.activeWorkspaceSessions()
	if err != nil {
		return RenamedWorkspace{}, err
	}
	if sessionName, ok := active[workspaceAbs]; ok {
		return RenamedWorkspace{}, errors.Errorf(
			"workspace %q is active (session %q); refuse to rename",
			workspaceAbs,
			sessionName,
		)
	}

	info, err := os.Stat(workspaceAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return RenamedWorkspace{}, errors.Errorf("workspace %q does not exist", workspaceAbs)
		}
		return RenamedWorkspace{}, errors.Wrapf(err, "stat workspace %q", workspaceAbs)
	}
	if !info.IsDir() {
		return RenamedWorkspace{}, errors.Errorf("workspace %q is not a directory", workspaceAbs)
	}

	org, repo, oldName := util.SplitWorkspacePath(reposBaseAbs, workspaceAbs)
	if org == "" || repo == "" || oldName == "" {
		return RenamedWorkspace{}, errors.Errorf("invalid workspace %q", workspaceAbs)
	}

	newName = strings.TrimSpace(newName)
	if err := validateWorkspaceRenameName(newName); err != nil {
		return RenamedWorkspace{}, err
	}
	if newName == oldName {
		return RenamedWorkspace{}, errors.New("new name must differ from current workspace name")
	}

	repoDir := filepath.Join(reposBaseAbs, org, repo)
	cacheDir := filepath.Join(repoDir, ".repo_cache")
	newWorkspaceAbs := filepath.Join(repoDir, newName)

	if err := validateWorkspacePath(reposBaseAbs, newWorkspaceAbs); err != nil {
		return RenamedWorkspace{}, errors.Wrapf(err, "invalid rename target %q", newWorkspaceAbs)
	}
	if err := workspaceRenameCollisionCheck(workspaceAbs, newWorkspaceAbs); err != nil {
		return RenamedWorkspace{}, err
	}

	isLinked := isLinkedGitWorktree(workspaceAbs)

	if err := withRepoMutationLock(repoDir, func() error {
		if err := workspaceRenameCollisionCheck(workspaceAbs, newWorkspaceAbs); err != nil {
			return err
		}

		if err := moveWorkspaceForRename(k.Git, cacheDir, workspaceAbs, newWorkspaceAbs, isLinked); err != nil {
			return errors.Wrapf(err, "move workspace %q -> %q", workspaceAbs, newWorkspaceAbs)
		}

		if err := renameWorkspaceBranch(k.Git, newWorkspaceAbs, oldName, newName); err != nil {
			rollbackErr := moveWorkspaceForRename(k.Git, cacheDir, newWorkspaceAbs, workspaceAbs, isLinked)
			if rollbackErr != nil {
				return errors.Wrapf(
					err,
					"rename branch %q -> %q; additionally, rollback workspace move failed: %v",
					oldName,
					newName,
					rollbackErr,
				)
			}
			return errors.Wrapf(err, "rename branch %q -> %q", oldName, newName)
		}

		return nil
	}); err != nil {
		return RenamedWorkspace{}, err
	}

	return RenamedWorkspace{
		OldPath: workspaceAbs,
		NewPath: newWorkspaceAbs,
	}, nil
}

func validateWorkspaceRenameName(newName string) error {
	name := strings.TrimSpace(newName)
	if name == "" {
		return errors.New("new-name cannot be blank")
	}
	if strings.Contains(name, "/") || strings.Contains(name, string(filepath.Separator)) {
		return errors.Errorf("invalid new-name %q: must be a single workspace name", newName)
	}
	if name == "." || name == ".." {
		return errors.Errorf("invalid new-name %q", newName)
	}
	return nil
}

func workspaceRenameCollisionCheck(oldWorkspace, newWorkspace string) error {
	if oldWorkspace == newWorkspace {
		return errors.New("new name must differ from current workspace name")
	}

	info, err := os.Stat(newWorkspace)
	if err == nil {
		if info.IsDir() {
			return errors.Errorf("workspace %q already exists", newWorkspace)
		}
		return errors.Errorf("rename target %q already exists and is not a directory", newWorkspace)
	}
	if !os.IsNotExist(err) {
		return errors.Wrapf(err, "stat rename target %q", newWorkspace)
	}

	return nil
}

func moveWorkspaceForRename(g git.Git, cacheDir, src, dst string, linked bool) error {
	if linked {
		return g.WorktreeMove(cacheDir, src, dst)
	}
	return os.Rename(src, dst)
}

func renameWorkspaceBranch(g git.Git, workspaceAbs, oldBranch, newBranch string) error {
	if !git.LocalBranchExists(g, workspaceAbs, oldBranch) {
		return fmt.Errorf("workspace branch %q does not exist in %q", oldBranch, workspaceAbs)
	}
	if git.LocalBranchExists(g, workspaceAbs, newBranch) {
		return fmt.Errorf("workspace branch %q already exists in %q", newBranch, workspaceAbs)
	}

	return g.Branch(workspaceAbs, "-m", oldBranch, newBranch)
}

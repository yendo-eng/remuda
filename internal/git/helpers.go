package git

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func WorktreeAdd(
	git Git,
	repoDir, dst string,
	force bool,
	args ...string,
) error {
	// Ensure the destination's parent directories exist so the command does
	// not fail with "not a directory" errors when the path is nested.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	abs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	if force {
		args = append(args, "--force")
	}

	return git.WorktreeAdd(repoDir, abs, args...)
}

// WorktreeRemove removes a worktree located at dst from the repository at
// repoDir. Without force, git refuses to remove a worktree with local changes.
//
// git also refuses to remove a locked worktree, so the lock is lifted first.
// It is put back when git refuses an unforced removal, since the worktree is
// then left as it was. In between, while git checks the worktree for local
// changes, a `git worktree prune` run inside a container would delete its
// record. After a failed forced removal, the worktree stays unlocked: it is
// meant to go, and git never prunes a locked one.
func WorktreeRemove(git Git, repoDir, dst string, force bool) error {
	abs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	lockReason, locked, err := worktreeLockReason(abs)
	if err != nil {
		return err
	}
	if locked {
		if err := git.WorktreeUnlock(repoDir, abs); err != nil {
			return err
		}
	}

	args := []string{abs}
	if force {
		args = append(args, "--force")
	}
	removeErr := git.WorktreeRemove(repoDir, args...)
	if removeErr != nil && locked && !force {
		if err := git.WorktreeLock(repoDir, abs, lockReason); err != nil {
			return pkgerrors.Wrapf(removeErr, "additionally, relocking worktree: %s", err.Error())
		}
	}
	return removeErr
}

// worktreeLockReason reports whether the linked worktree at dir is locked, and
// why. It follows the worktree's .git file itself rather than asking git, so a
// worktree whose record is already gone reads as unlocked, and removing it
// fails with git's own error.
func worktreeLockReason(dir string) (string, bool, error) {
	dotGit, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return "", false, err
	}
	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(dotGit)), "gitdir: ")
	if !ok {
		return "", false, pkgerrors.Errorf("%s is not a linked worktree", dir)
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}

	//nolint:gosec // G703: gitDir comes from the worktree's .git file, the same file git reads to run any command in the worktree.
	reason, err := os.ReadFile(filepath.Join(gitDir, "locked"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(reason)), true, nil
}

func CheckoutOrCreateBranch(logger zerolog.Logger, git Git, path, branch string) error {
	// Prefer a clean, single attempt based on what exists to avoid noisy
	// failures printed by git during probing.
	if LocalBranchExists(git, path, branch) {
		err := git.Checkout(path, branch)
		return pkgerrors.Wrapf(err, "checkout '%s'", branch)
	}

	if RemoteBranchExists(git, path, branch) {
		err := git.Checkout(path, "-b", branch, "--track", fmt.Sprintf("origin/%s", branch))
		return pkgerrors.Wrapf(err, "checkout tracking branch '%s'", branch)
	}

	// Fallback: create a new local branch from current HEAD without upstream.
	err := git.Checkout(path, "-b", branch)
	return pkgerrors.Wrapf(err, "create new branch '%s'", branch)
}

// LocalBranchExists returns true if refs/heads/<branch> exists in the repo
// at path.
func LocalBranchExists(git Git, path, branch string) bool {
	return git.ShowRef(path, fmt.Sprintf("refs/heads/%s", branch), "--verify", "--quiet") == nil
}

// RemoteBranchExists returns true if refs/remotes/origin/<branch> exists.
func RemoteBranchExists(git Git, path, branch string) bool {
	return git.ShowRef(path, fmt.Sprintf("refs/remotes/origin/%s", branch), "--verify", "--quiet") == nil
}

// HasUpstream returns true if the current branch has an upstream set.
func HasUpstream(git Git, path string) bool {
	_, err := git.RevParse(path, "--abbrev-ref", "--symbolic-full-name", "@{u}")
	return err == nil
}

// SetUpstream attempts to set origin/<branch> as upstream for <branch>.
func SetUpstream(git Git, path, branch string) error {
	return git.Branch(path, "--set-upstream-to", fmt.Sprintf("origin/%s", branch), branch)
}

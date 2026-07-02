package git

import (
	"fmt"
	"os"
	"path/filepath"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func WorktreeAdd(
	git Git,
	repoDir, dst string,
	force bool,
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

	args := []string{}
	if force {
		args = append(args, "--force")
	}

	return git.WorktreeAdd(repoDir, abs, args...)
}

// WorktreeRemove removes a worktree located at dst from the repository at
// repoDir. Any failure in removing the worktree is ignored by the caller when
// used for best-effort cleanup.
func WorktreeRemove(git Git, repoDir, dst string) error {
	abs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	return git.WorktreeRemove(repoDir, "--force", abs)
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

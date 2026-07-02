package internal

import (
	"errors"

	"io/fs"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/util"
)

type CloneCommand struct {
	// The name to give the cloned workspace (also the branch name).
	Name string

	// Optional branch name override. When empty, defaults to the workspace
	// base name (same as Name). When set, the workspace folder remains
	// Name, but git checkouts and upstream configuration operate on Branch.
	Branch string

	// The URL of the git repository to clone.
	RepoURL string

	// If true, skip running clone hooks.
	SkipCloneHooks bool

	// If true, replace existing workspace if it exists.
	Force bool

	// If true, clone the entire repo into the workspace instead of adding a
	// linked worktree that shares objects with the cache.
	FullClone bool

	// If true, skip refreshing the cache before cloning. Use for offline
	// development or to speed up big clone batches.
	SkipCacheRefresh bool
}

// Returns the path to the cloned directory on success.
func (k Remuda) Clone(
	cmd CloneCommand,
) (string, error) {
	logger := k.logger()
	repoURL := strings.TrimSpace(cmd.RepoURL)
	org, repo, err := github.ParseRepo(repoURL)
	if err != nil {
		return "", err
	}

	baseDir := filepath.Join(k.Config.ReposBaseDir, org, repo)
	cacheDir := filepath.Join(baseDir, ".repo_cache")
	// Owner: rwx, Group: rx, Others: rx
	const dirPermissions = fs.FileMode(0o755)
	if err := os.MkdirAll(baseDir, dirPermissions); err != nil {
		return "", pkgerrors.Wrap(err, "creating base directory")
	}

	// Workspace folder equals provided name
	baseName := filepath.Base(cmd.Name)
	branchName := strings.TrimSpace(cmd.Branch)
	if branchName == "" {
		branchName = baseName
	}
	target := filepath.Join(baseDir, baseName)

	logger.Info().
		Str("target", target).
		Bool("force", cmd.Force).
		Bool("full_clone", cmd.FullClone).
		Msg("cloning into directory")

	if err := withRepoMutationLock(baseDir, func() error {
		// Ensure cache present/updated.
		if cmd.SkipCacheRefresh {
			logger.Info().Str("cacheDir", cacheDir).Msg("skipping repo cache refresh")
		} else if _, err := os.Stat(cacheDir); errors.Is(err, fs.ErrNotExist) {
			logger.Info().Str("cacheDir", cacheDir).Msg("creating repo cache")
			if err := k.Git.Clone(repoURL, cacheDir); err != nil {
				return err
			}
		} else {
			logger.Info().Str("cacheDir", cacheDir).Msg("updating repo cache")
			if err := k.Git.Pull(cacheDir); err != nil {
				return err
			}
		}

		// Error on collision, or bulldoze the folder if --force is set.
		if cmd.Force {
			logger.Info().Str("target", target).Msg("force removing existing workspace")
			if err := k.PruneOneSession(target, true, false, true); err != nil {
				return pkgerrors.Wrap(err, "removing existing workspace")
			}
		}

		if cmd.FullClone {
			if err := os.MkdirAll(filepath.Dir(target), dirPermissions); err != nil {
				return pkgerrors.Wrap(err, "creating workspace parent")
			}
			if err := util.CopyDir(cacheDir, target); err != nil {
				_ = os.RemoveAll(target)
				return pkgerrors.Wrap(err, "copying repo cache")
			}
			if err := removeCopiedWorktrees(target); err != nil {
				_ = os.RemoveAll(target)
				return pkgerrors.Wrap(err, "cleaning copied worktrees")
			}

			// The cache directory may contain local, uncommitted changes. Since a
			// full clone uses a filesystem copy of the cache, those modifications can
			// leak into the new workspace and cause subsequent operations (e.g.
			// `gh pr checkout`) to fail due to a dirty working tree. Ensure the
			// freshly copied repository is clean before proceeding.
			// Best‑effort: if cleanup fails, fall back to returning the error so the
			// caller gets a clear failure instead of later, confusing errors.
			if err := util.RunCmdWithLogger(logger, "git", "-C", target, "reset", "--hard"); err != nil {
				_ = os.RemoveAll(target)
				return pkgerrors.Wrap(err, "reset copied repo to HEAD")
			}
			if err := util.RunCmdWithLogger(logger, "git", "-C", target, "clean", "-fdx"); err != nil {
				_ = os.RemoveAll(target)
				return pkgerrors.Wrap(err, "clean copied repo working tree")
			}
		} else {
			// Instead of copying the entire cached repository, create a linked worktree.
			// This is much faster and avoids wasting disk space while still giving the
			// caller an independent working directory.
			if err := git.WorktreeAdd(k.Git, cacheDir, target, cmd.Force); err != nil {
				return pkgerrors.Wrap(err, "adding git worktree")
			}
		}

		return nil
	}); err != nil {
		return "", err
	}

	// Checkout/create the desired branch (defaults to workspace name when not overridden).
	if err := git.CheckoutOrCreateBranch(logger, k.Git, target, branchName); err != nil {
		// Attempt to clean up the partially-created worktree so a broken entry
		// does not linger in the repository. If that removal itself fails we
		// surface both errors.
		if cleanErr := cleanupWorkspace(k.Git, baseDir, cacheDir, target, cmd.FullClone); cleanErr != nil {
			return "", pkgerrors.Wrapf(err, "checking out branch; additionally, cleaning worktree: %s", cleanErr.Error())
		}
		return "", pkgerrors.Wrap(err, "checking out branch")
	}

	// If there's an upstream, pull fast-forward. If not, try to set tracking to
	// origin/<branch> when that remote branch exists; otherwise skip pulling.
	if !git.HasUpstream(k.Git, target) && !cmd.SkipCacheRefresh {
		if git.RemoteBranchExists(k.Git, target, branchName) {
			err := git.SetUpstream(k.Git, target, branchName)
			if err != nil {
				logger.Warn().Err(err).Msg("setting upstream after clone")
			}
		}
	}
	if git.HasUpstream(k.Git, target) && !cmd.SkipCacheRefresh {
		if err := k.Git.Pull(target); err != nil {
			logger.Warn().Err(err).Msg("git pull after clone")
		}
	} else {
		logger.Debug().
			Str("branch", branchName).
			Msg("skipping pull due to no upstream or explicit refresh skip requested")
	}

	if !cmd.SkipCloneHooks {
		if err := k.CloneHooks.RunCloneHooks(CloneHookContext{
			RepoURL:     repoURL,
			Org:         org,
			Repo:        repo,
			CacheDir:    cacheDir,
			WorktreeDir: target,
			Env:         k.envProvider(),
			Logger:      &logger,
		}); err != nil {
			if cleanErr := cleanupWorkspace(k.Git, baseDir, cacheDir, target, cmd.FullClone); cleanErr != nil {
				return "", pkgerrors.Wrapf(err, "running clone hooks; additionally, cleaning worktree: %s", cleanErr.Error())
			}
			return "", pkgerrors.Wrap(err, "running clone hooks")
		}
	} else {
		logger.Info().
			Str("org", org).
			Str("repo", repo).
			Str("worktree", target).
			Msg("skipping clone hooks (--no-clone-hooks)")
	}

	return target, nil
}

func cleanupWorkspace(g git.Git, baseDir, cacheDir, target string, fullClone bool) error {
	if fullClone {
		return os.RemoveAll(target)
	}

	return withRepoMutationLock(baseDir, func() error {
		return git.WorktreeRemove(g, cacheDir, target)
	})
}

func removeCopiedWorktrees(target string) error {
	worktreesDir := filepath.Join(target, ".git", "worktrees")
	if _, err := os.Stat(worktreesDir); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.RemoveAll(worktreesDir)
}

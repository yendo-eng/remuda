package e2e_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
)

func TestWorkspacesRemove(t *testing.T) {
	t.Parallel()

	t.Run("dry-run with absolute path", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		workspace := filepath.Join(baseDir, org, repo, "dry-run-absolute")

		h.RunOK("clone", "--name", "dry-run-absolute", "--repo-url", remoteURL)

		res := h.RunOK("workspaces", "remove", "--dry-run", workspace)
		require.Equal(t, fmt.Sprintf("would remove %s\n", workspace), res.Stdout)
		require.DirExists(t, workspace)
	})

	t.Run("removes linked worktree via identifier", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)

		h.RunOK("clone", "--name", "linked-remove", "--repo-url", remoteURL)
		workspace := filepath.Join(baseDir, org, repo, "linked-remove")
		cacheDir := filepath.Join(baseDir, org, repo, ".repo_cache")

		before := testutils.RunGit(t, cacheDir, "worktree", "list", "--porcelain")
		require.Contains(t, before, workspace)

		target := path.Join(org, repo, "linked-remove")
		res := h.RunOK("workspaces", "remove", target)
		require.Equal(t, fmt.Sprintf("removed %s\n", workspace), res.Stdout)
		require.NoDirExists(t, workspace)

		after := testutils.RunGit(t, cacheDir, "worktree", "list", "--porcelain")
		require.NotContains(t, after, workspace)
	})

	t.Run("removes full clone directly", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		workspace := filepath.Join(baseDir, org, repo, "full-clone-remove")

		h.RunOK("clone", "--name", "full-clone-remove", "--repo-url", remoteURL, "--full-clone")

		res := h.RunOK("workspaces", "remove", workspace)
		require.Equal(t, fmt.Sprintf("removed %s\n", workspace), res.Stdout)
		require.NotContains(t, res.Stderr, "worktree remove failed")
		require.NoDirExists(t, workspace)
	})

	t.Run("fails for active session target", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		workspace := filepath.Join(baseDir, org, repo, "active-target")

		h.RunOK("vibe", "--name", "active-target", "--repo-url", remoteURL)

		res := h.Run("workspaces", "remove", workspace)
		require.ErrorContains(t, res.Err, "is active")
		require.DirExists(t, workspace)
	})

	t.Run("fails for untracked linked worktree target without force", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		workspace := filepath.Join(baseDir, org, repo, "dirty-target")

		h.RunOK("clone", "--name", "dirty-target", "--repo-url", remoteURL)
		require.NoError(t, os.WriteFile(filepath.Join(workspace, "untracked.txt"), []byte("keep me"), 0o644))

		res := h.Run("workspaces", "remove", workspace)
		require.ErrorContains(t, res.Err, "rerun with --force")
		require.DirExists(t, workspace)
	})

	t.Run("force removes untracked linked worktree target", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		workspace := filepath.Join(baseDir, org, repo, "force-dirty-target")

		h.RunOK("clone", "--name", "force-dirty-target", "--repo-url", remoteURL)
		require.NoError(t, os.WriteFile(filepath.Join(workspace, "untracked.txt"), []byte("force"), 0o644))

		res := h.RunOK("workspaces", "remove", "--force", workspace)
		require.Equal(t, fmt.Sprintf("removed %s\n", workspace), res.Stdout)
		require.NoDirExists(t, workspace)
	})

	t.Run("protects repo cache from removal", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)

		h.RunOK("clone", "--name", "seed", "--repo-url", remoteURL)

		cacheDir := filepath.Join(baseDir, org, repo, ".repo_cache")
		res := h.Run("workspaces", "remove", cacheDir)
		require.ErrorContains(t, res.Err, "workspace folder is not a prunable workspace")
		require.DirExists(t, cacheDir)
	})

	t.Run("removes multiple identifier targets", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)

		h.RunOK("clone", "--name", "target-one", "--repo-url", remoteURL)
		h.RunOK("clone", "--name", "target-two", "--repo-url", remoteURL)

		workspaceOne := filepath.Join(baseDir, org, repo, "target-one")
		workspaceTwo := filepath.Join(baseDir, org, repo, "target-two")

		targetOne := path.Join(org, repo, "target-one")
		targetTwo := path.Join(org, repo, "target-two")

		res := h.RunOK("workspaces", "remove", targetOne, targetTwo)
		require.Equal(
			t,
			fmt.Sprintf("removed %s\nremoved %s\n", workspaceOne, workspaceTwo),
			res.Stdout,
		)
		require.NoDirExists(t, workspaceOne)
		require.NoDirExists(t, workspaceTwo)
	})
}

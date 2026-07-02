package e2e_test

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
)

func TestWorkspacesRename(t *testing.T) {
	t.Parallel()

	t.Run("renames inactive linked workspace by identifier and branch", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		cacheDir := filepath.Join(baseDir, org, repo, ".repo_cache")
		oldName := "rename-source"
		newName := "rename-dest"
		oldPath := filepath.Join(baseDir, org, repo, oldName)
		newPath := filepath.Join(baseDir, org, repo, newName)
		target := path.Join(org, repo, oldName)

		h.RunOK("clone", "--name", oldName, "--repo-url", remoteURL)

		res := h.RunOK("workspaces", "rename", target, newName)
		require.Equal(t, fmt.Sprintf("renamed %s -> %s\n", oldPath, newPath), res.Stdout)
		require.NoDirExists(t, oldPath)
		require.DirExists(t, newPath)

		worktreeList := testutils.RunGit(t, cacheDir, "worktree", "list", "--porcelain")
		require.Contains(t, worktreeList, newPath)
		require.NotContains(t, worktreeList, oldPath)

		currentBranch := strings.TrimSpace(testutils.RunGit(t, newPath, "rev-parse", "--abbrev-ref", "HEAD"))
		require.Equal(t, newName, currentBranch)

		branches := testutils.RunGit(t, newPath, "branch", "--list")
		require.Contains(t, branches, newName)
		require.NotContains(t, branches, oldName)
	})

	t.Run("renames inactive workspace by absolute path", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		oldName := "abs-rename-source"
		newName := "abs-rename-dest"
		oldPath := filepath.Join(baseDir, org, repo, oldName)
		newPath := filepath.Join(baseDir, org, repo, newName)

		h.RunOK("clone", "--name", oldName, "--repo-url", remoteURL)

		res := h.RunOK("workspaces", "rename", oldPath, newName)
		require.Equal(t, fmt.Sprintf("renamed %s -> %s\n", oldPath, newPath), res.Stdout)
		require.NoDirExists(t, oldPath)
		require.DirExists(t, newPath)
	})

	t.Run("fails for active session target", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		oldName := "active-rename-source"
		newName := "active-rename-dest"
		oldPath := filepath.Join(baseDir, org, repo, oldName)
		newPath := filepath.Join(baseDir, org, repo, newName)

		h.RunOK("vibe", "--name", oldName, "--repo-url", remoteURL)

		res := h.Run("workspaces", "rename", oldPath, newName)
		require.ErrorContains(t, res.Err, "is active")
		require.DirExists(t, oldPath)
		require.NoDirExists(t, newPath)
	})

	t.Run("fails when rename target already exists", func(t *testing.T) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		baseDir := h.RemudaConfig.ReposBaseDir
		org, repo, _ := github.ParseRepo(remoteURL)
		oldName := "collision-source"
		newName := "collision-target"
		oldPath := filepath.Join(baseDir, org, repo, oldName)
		newPath := filepath.Join(baseDir, org, repo, newName)
		target := path.Join(org, repo, oldName)

		h.RunOK("clone", "--name", oldName, "--repo-url", remoteURL)
		h.RunOK("clone", "--name", newName, "--repo-url", remoteURL)

		res := h.Run("workspaces", "rename", target, newName)
		require.ErrorContains(t, res.Err, "already exists")
		require.DirExists(t, oldPath)
		require.DirExists(t, newPath)
	})
}

package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
)

// --tmp places the worktree under the configured OS-temp root while the
// persistent .repo_cache stays under the repos base dir.
func TestCloneTmpPlacesWorktreeUnderTempRoot(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBase := filepath.Join(t.TempDir(), "repos")
	tmpBase := filepath.Join(t.TempDir(), "tmp-repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{
		ReposBaseDir: reposBase,
		TmpBaseDir:   tmpBase,
	}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	workspace, err := h.Remuda.Clone(internal.CloneCommand{
		Name:    "wk",
		RepoURL: remoteURL,
		Tmp:     true,
	})
	require.NoError(t, err)

	// Worktree lives under the temp root, not the repos base dir.
	require.Equal(t, filepath.Join(tmpBase, org, repo, "wk"), workspace)
	require.DirExists(t, workspace)
	require.NoDirExists(t, filepath.Join(reposBase, org, repo, "wk"))

	// Persistent cache stays under the repos base dir.
	require.DirExists(t, filepath.Join(reposBase, org, repo, ".repo_cache"))

	// The temp checkout is a linked worktree (.git is a file, not a dir).
	info, err := os.Stat(filepath.Join(workspace, ".git"))
	require.NoError(t, err)
	require.False(t, info.IsDir(), "--tmp should use the linked-worktree model")
}

// --tmp forces the linked-worktree model even when --full-clone is requested.
func TestCloneTmpForcesLinkedWorktreeOverFullClone(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBase := filepath.Join(t.TempDir(), "repos")
	tmpBase := filepath.Join(t.TempDir(), "tmp-repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{
		ReposBaseDir: reposBase,
		TmpBaseDir:   tmpBase,
	}))

	workspace, err := h.Remuda.Clone(internal.CloneCommand{
		Name:      "wk",
		RepoURL:   remoteURL,
		Tmp:       true,
		FullClone: true,
	})
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(workspace, ".git"))
	require.NoError(t, err)
	require.False(t, info.IsDir(), "--tmp must override --full-clone and use a linked worktree")
}

// vibe --tmp routes the worktree under the temp root end to end.
func TestVibeTmpPlacesWorktreeUnderTempRoot(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBase := filepath.Join(t.TempDir(), "repos")
	tmpBase := filepath.Join(t.TempDir(), "tmp-repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{
		ReposBaseDir: reposBase,
		TmpBaseDir:   tmpBase,
	}))
	h.SetEnv("REMUDA_CONTAINER", "false")

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	h.RunOK("vibe", "--name", "wk", "--repo-url", remoteURL, "--tmp")

	require.DirExists(t, filepath.Join(tmpBase, org, repo, "wk"))
	require.NoDirExists(t, filepath.Join(reposBase, org, repo, "wk"))
	require.DirExists(t, filepath.Join(reposBase, org, repo, ".repo_cache"))
}

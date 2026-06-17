package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
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

func TestCloneRejectsCrossRootDuplicateWorkspaceName(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBase := filepath.Join(t.TempDir(), "repos")
	tmpBase := filepath.Join(t.TempDir(), "tmp-repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{
		ReposBaseDir: reposBase,
		TmpBaseDir:   tmpBase,
	}))

	_, err := h.Remuda.Clone(internal.CloneCommand{
		Name:    "wk",
		RepoURL: remoteURL,
	})
	require.NoError(t, err)

	_, err = h.Remuda.Clone(internal.CloneCommand{
		Name:    "wk",
		RepoURL: remoteURL,
		Tmp:     true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "ambiguous across workspace roots")
	require.ErrorContains(t, err, filepath.Join(reposBase))
}

func TestSessionResumeRejectsCrossRootDuplicateWorkspaceName(t *testing.T) {
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
	persistentPath := filepath.Join(reposBase, org, repo, "wk")
	tmpPath := filepath.Join(tmpBase, org, repo, "wk")
	require.NoError(t, os.MkdirAll(persistentPath, 0o755))
	require.NoError(t, os.MkdirAll(tmpPath, 0o755))

	res := h.Run("session", "resume", tmpPath)
	require.Error(t, res.Err, res.String())
	require.ErrorContains(t, res.Err, "ambiguous across workspace roots")
	require.ErrorContains(t, res.Err, persistentPath)
}

// Temp workspaces are hidden from workspaces list / session inactive by default
// and surfaced with --include-tmp.
func TestTmpWorkspacesHiddenUnlessIncludeTmp(t *testing.T) {
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

	persistentPath := filepath.Join(reposBase, org, repo, "persistent")
	tmpPath := filepath.Join(tmpBase, org, repo, "throwaway")

	// One persistent and one --tmp session, both made inactive.
	h.RunOK("vibe", "--name", "persistent", "--repo-url", remoteURL)
	h.RunOK("vibe", "--name", "throwaway", "--repo-url", remoteURL, "--tmp")
	h.RunOK("session", "kill", "--name", session.SessionNameFromWorkspaceName(persistentPath))
	h.RunOK("session", "kill", "--name", session.SessionNameFromWorkspaceName(tmpPath))

	// Default: temp workspace hidden.
	res := h.RunOK("workspaces", "list")
	lines := nonEmptyOutputLines(res.Stdout)
	require.Contains(t, lines, persistentPath)
	require.NotContains(t, lines, tmpPath)

	// With --include-tmp: temp workspace surfaced.
	res = h.RunOK("workspaces", "list", "--include-tmp")
	lines = nonEmptyOutputLines(res.Stdout)
	require.Contains(t, lines, persistentPath)
	require.Contains(t, lines, tmpPath)

	// session inactive mirrors the same behavior.
	res = h.RunOK("session", "inactive")
	require.NotContains(t, nonEmptyOutputLines(res.Stdout), tmpPath)
	res = h.RunOK("session", "inactive", "--include-tmp")
	require.Contains(t, nonEmptyOutputLines(res.Stdout), tmpPath)
}

// Active temp workspaces show in `workspaces list --active` (and the plain
// listing) without --include-tmp, matching `session list`; inactive temp
// workspaces stay hidden until --include-tmp.
func TestActiveTmpWorkspaceShownWithoutIncludeTmp(t *testing.T) {
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
	activeTmp := filepath.Join(tmpBase, org, repo, "live")
	inactiveTmp := filepath.Join(tmpBase, org, repo, "dead")

	// One live temp session, one temp session killed (inactive but still on disk).
	h.RunOK("vibe", "--name", "live", "--repo-url", remoteURL, "--tmp")
	h.RunOK("vibe", "--name", "dead", "--repo-url", remoteURL, "--tmp")
	h.RunOK("session", "kill", "--name", session.SessionNameFromWorkspaceName(inactiveTmp))

	// --active shows the live temp workspace without --include-tmp.
	active := h.RunOK("workspaces", "list", "--active")
	require.Contains(t, nonEmptyOutputLines(active.Stdout), activeTmp)
	require.NotContains(t, nonEmptyOutputLines(active.Stdout), inactiveTmp)

	// Plain listing includes the active temp workspace but still hides the
	// inactive one until --include-tmp.
	plain := h.RunOK("workspaces", "list")
	require.Contains(t, nonEmptyOutputLines(plain.Stdout), activeTmp)
	require.NotContains(t, nonEmptyOutputLines(plain.Stdout), inactiveTmp)

	withTmp := h.RunOK("workspaces", "list", "--include-tmp")
	require.Contains(t, nonEmptyOutputLines(withTmp.Stdout), activeTmp)
	require.Contains(t, nonEmptyOutputLines(withTmp.Stdout), inactiveTmp)
}

// A still-present temp workspace can be resumed by path.
func TestResumeTmpWorkspaceByPath(t *testing.T) {
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
	tmpPath := filepath.Join(tmpBase, org, repo, "throwaway")

	// Create the temp worktree and then kill the session so it goes inactive but
	// the directory remains on disk.
	h.RunOK("vibe", "--name", "throwaway", "--repo-url", remoteURL, "--tmp")
	h.RunOK("session", "kill", "--name", session.SessionNameFromWorkspaceName(tmpPath))

	// Resume by explicit path should be accepted (validated against the temp root).
	res := h.RunOK("session", "resume", tmpPath)
	require.NoError(t, res.Err, res.String())

	// It now shows as an active temp workspace.
	active := h.RunOK("workspaces", "list", "--include-tmp", "--active")
	require.Contains(t, nonEmptyOutputLines(active.Stdout), tmpPath)
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

	// An active temp session shows in session list like any other.
	sessionName := session.SessionNameFromWorkspaceName(filepath.Join(tmpBase, org, repo, "wk"))
	res := h.RunOK("session", "list")
	require.Contains(t, nonEmptyOutputLines(res.Stdout), sessionName)
}

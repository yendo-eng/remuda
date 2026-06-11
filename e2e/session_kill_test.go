package e2e_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionKill(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T, gh github.GitHub) (*testutils.Harness, string) {
		var opts []testutils.HarnessOption
		if gh != nil {
			opts = append(opts, testutils.WithGitHub(gh))
		}

		h := testutils.NewHarness(t, opts...)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		return h, remoteURL
	}

	t.Run("errors when session does not exist", func(t *testing.T) {
		h, _ := setup(t, nil)

		res := h.Run("session", "kill", "--name=nonexistent")
		require.ErrorContains(t, res.Err, "can't find session: nonexistent")
	})

	t.Run("kills existing session", func(t *testing.T) {
		h, remoteURL := setup(t, nil)

		baseDir := h.RemudaConfig.ReposBaseDir
		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		// First, create a session
		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		// Now, kill the session
		h.RunOK("session", "kill", "--name", sessionName)

		// Workspace is still there since we didn't pass --cleanup
		require.DirExists(t, workspacePath)
	})

	t.Run("prefers --name over --pick", func(t *testing.T) {
		h, remoteURL := setup(t, nil)

		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(h.RemudaConfig.ReposBaseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		h.RunOK("session", "kill", "--name", sessionName, "--pick")
	})

	t.Run("kills and cleans up existing session", func(t *testing.T) {
		h, remoteURL := setup(t, nil)

		baseDir := h.RemudaConfig.ReposBaseDir
		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		// First, create a session
		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		// Now, kill the session with --cleanup
		h.RunOK("session", "kill", "--name", sessionName, "--cleanup")

		// Workspace should be gone
		require.NoDirExists(t, workspacePath)
	})

	t.Run("kills, closes PR, and cleans up existing session", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		// First, create a session
		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		// Now, kill the session with --cleanup and --close-pr
		h.RunOK("session", "kill", "--name", sessionName, "--cleanup", "--close-pr")

		// Workspace should be gone and PR close attempted
		require.NoDirExists(t, workspacePath)
		require.Equal(t, []string{workspacePath}, mockGitHub.ClosedWorkspaces)
		require.Equal(t, []string{""}, mockGitHub.ClosedComments)
	})

	t.Run("kills, closes PR with comment, and cleans up existing session", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		h.RunOK("session", "kill", "--name", sessionName, "--cleanup", "--close-pr=closing this PR from remuda")

		require.NoDirExists(t, workspacePath)
		require.Equal(t, []string{workspacePath}, mockGitHub.ClosedWorkspaces)
		require.Equal(t, []string{"closing this PR from remuda"}, mockGitHub.ClosedComments)
	})

	t.Run("merges PR before killing session", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		binDir := t.TempDir()
		brPath := filepath.Join(binDir, "br")
		err := os.WriteFile(brPath, []byte("#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$1\" == \"close\" ]]; then\n  exit 0\nfi\necho \"unexpected br args: $*\" >&2\nexit 1\n"), 0o755)
		require.NoError(t, err)
		h.SetEnv("PATH", binDir+string(os.PathListSeparator)+h.Getenv("PATH"))

		res := h.RunOK("session", "kill", "--name", sessionName, "--merge")
		require.Contains(t, res.Stdout, "Closed beads issue")

		require.Equal(t, []string{workspacePath}, mockGitHub.MergedWorkspaces)
		require.Equal(t, [][]string{{"--rebase"}}, mockGitHub.MergedFlags)
		require.Empty(t, mockGitHub.ClosedWorkspaces)
		require.DirExists(t, workspacePath)

		mockSessionMgr, ok := h.Session.(*testutils.MockSessionManager)
		require.True(t, ok)
		_, findErr := mockSessionMgr.Find(sessionName)
		require.ErrorIs(t, findErr, session.ErrSessionNotFound)
	})

	t.Run("closes beads issue without merge when --close-bd is set", func(t *testing.T) {
		h, remoteURL := setup(t, nil)

		baseDir := h.RemudaConfig.ReposBaseDir
		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		binDir := t.TempDir()
		brPath := filepath.Join(binDir, "br")
		err := os.WriteFile(brPath, []byte("#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$1\" != \"close\" ]]; then\n  echo \"unexpected br args: $*\" >&2\n  exit 1\nfi\nif [[ \"$2\" != \"test-session\" ]]; then\n  echo \"unexpected issue id: $2\" >&2\n  exit 1\nfi\n"), 0o755)
		require.NoError(t, err)
		h.SetEnv("PATH", binDir+string(os.PathListSeparator)+h.Getenv("PATH"))

		res := h.RunOK("session", "kill", "--name", sessionName, "--close-bd")
		require.Contains(t, res.Stdout, "Closed beads issue")

		require.DirExists(t, workspacePath)

		mockSessionMgr, ok := h.Session.(*testutils.MockSessionManager)
		require.True(t, ok)
		_, findErr := mockSessionMgr.Find(sessionName)
		require.ErrorIs(t, findErr, session.ErrSessionNotFound)
	})

	t.Run("does not kill session when merge fails", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{MergeErr: errors.New("merge failed")}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		res := h.Run("session", "kill", "--name", sessionName, "--merge")
		require.ErrorContains(t, res.Err, "merge failed")

		mockSessionMgr, ok := h.Session.(*testutils.MockSessionManager)
		require.True(t, ok)
		_, findErr := mockSessionMgr.Find(sessionName)
		require.NoError(t, findErr)
		require.DirExists(t, workspacePath)
		require.Equal(t, []string{workspacePath}, mockGitHub.MergedWorkspaces)
		require.Empty(t, mockGitHub.ClosedWorkspaces)
	})

	t.Run("merges PR with flags from config defaults", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		writeConfigFile(t, h, `
version: 1
defaults:
  merge:
    gh_flags:
      - --squash
      - --delete-branch
`)

		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)
		h.RunOK("session", "kill", "--name", sessionName, "--merge")

		require.Equal(t, []string{workspacePath}, mockGitHub.MergedWorkspaces)
		require.Equal(t, [][]string{{"--squash", "--delete-branch"}}, mockGitHub.MergedFlags)
	})

	t.Run("merge flags from CLI replace config defaults", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		writeConfigFile(t, h, `
version: 1
defaults:
  merge:
    gh_flags:
      - --squash
      - --delete-branch
`)

		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)
		h.RunOK(
			"session", "kill", "--name", sessionName, "--merge",
			"--merge-flag=--merge",
			"--merge-flag=--auto",
		)

		require.Equal(t, []string{workspacePath}, mockGitHub.MergedWorkspaces)
		require.Equal(t, [][]string{{"--merge", "--auto"}}, mockGitHub.MergedFlags)
	})

	t.Run("merges PR with flags from per-repo config defaults", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		org, repo, _ := github.ParseRepo(remoteURL)
		writeConfigFile(t, h, fmt.Sprintf(`
version: 1
defaults:
  merge:
    gh_flags:
      - --squash
per_repo:
  "%s/%s":
    defaults:
      merge:
        gh_flags:
          - --rebase
          - --admin
`, org, repo))

		name := "test-session"
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)
		h.RunOK("session", "kill", "--name", sessionName, "--merge")

		require.Equal(t, []string{workspacePath}, mockGitHub.MergedWorkspaces)
		require.Equal(t, [][]string{{"--rebase", "--admin"}}, mockGitHub.MergedFlags)
	})

	t.Run("merge flags from CLI replace per-repo defaults", func(t *testing.T) {
		mockGitHub := &testutils.MockGitHub{}
		h, remoteURL := setup(t, mockGitHub)
		baseDir := h.RemudaConfig.ReposBaseDir
		mockGitHub.RepoURL = remoteURL

		org, repo, _ := github.ParseRepo(remoteURL)
		writeConfigFile(t, h, fmt.Sprintf(`
version: 1
defaults:
  merge:
    gh_flags:
      - --rebase
per_repo:
  "%s/%s":
    defaults:
      merge:
        gh_flags:
          - --rebase
          - --admin
`, org, repo))

		name := "test-session"
		workspacePath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)
		h.RunOK(
			"session", "kill", "--name", sessionName, "--merge",
			"--merge-flag=--merge",
			"--merge-flag=--auto",
		)

		require.Equal(t, []string{workspacePath}, mockGitHub.MergedWorkspaces)
		require.Equal(t, [][]string{{"--merge", "--auto"}}, mockGitHub.MergedFlags)
	})
}

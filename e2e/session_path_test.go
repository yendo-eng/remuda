package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionPath(t *testing.T) {
	t.Parallel()
	t.Run("prints workspace path", func(t *testing.T) {
		h := testutils.NewHarness(t)
		remoteURL := testutils.InitTestRemote(t)

		h.RunOK(
			"vibe",
			"--name", "feature",
			"--repo-url", remoteURL,
			"--agent-cmd", "echo ",
			"--no-container",
			"prompt",
		)

		org, repo, err := github.ParseRepo(remoteURL)
		require.NoError(t, err)
		workspacePath := filepath.Join(h.RemudaConfig.ReposBaseDir, org, repo, "feature")
		require.DirExists(t, workspacePath)
		sessionName := session.SessionNameFromWorkspaceName(workspacePath)

		res := h.RunOK("session", "path", "--name", sessionName)
		require.Equal(t, workspacePath+"\n", res.Stdout)
	})

	t.Run("errors when session missing", func(t *testing.T) {
		h := testutils.NewHarness(t)

		res := h.Run("session", "path", "--name=org/repo/missing")
		require.ErrorContains(t, res.Err, "session \"org/repo/missing\" not found")
	})
}

func TestSessionPathHandlesDotUnderscore(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	baseDir := h.RemudaConfig.ReposBaseDir
	org, repo := "acme", "remuda"
	// Real workspace directory uses dots
	workspaceFolder := "remuda-5.5-config-validation"
	realPath := filepath.Join(baseDir, org, repo, workspaceFolder)
	require.NoError(t, os.MkdirAll(realPath, 0o755))

	// Session name uses underscores (tmux behavior)
	sessionFolder := "remuda-5_5-config-validation"
	sessionName := fmt.Sprintf("%s/%s/%s", org, repo, sessionFolder)

	sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
	require.True(t, ok)
	require.NoError(t, sessionMgr.Start(sessionName, "echo"))

	res := h.RunOK("session", "path", "--name", sessionName)
	require.Equal(t, realPath+"\n", res.Stdout)
}

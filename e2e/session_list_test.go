package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionList_PrintsNamesOnly(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	remoteURL := testutils.InitTestRemote(t)

	h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--agent-cmd", "echo ",
		"--no-container",
		"prompt",
	)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	workspacePath := filepath.Join(h.RemudaConfig.ReposBaseDir, org, repo, "wk")
	sessionName := session.SessionNameFromWorkspaceName(workspacePath)

	sessionMgr, ok := h.Session.(*testutils.MockSessionManager)
	require.True(t, ok)
	require.NotNil(t, sessionMgr.FindSession(sessionName), "expected session to be registered")

	listRes := h.RunOK("session", "list")
	require.Equal(t, sessionName+"\n", listRes.Stdout)
}

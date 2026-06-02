package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestVibeRemoteControl_ClaudeUsesSessionName(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	sessionMgr := &testutils.MockSessionManager{}

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithSessionManager(sessionMgr),
	)

	h.RunOK(
		"vibe",
		"--name", "wk-remote",
		"--repo-url", remoteURL,
		"--agent", "claude",
		"--no-container",
		"--remote",
		"hello",
	)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	workspace := filepath.Join(runDir, org, repo, "wk-remote")
	expectedSession := session.SessionNameFromWorkspaceName(workspace)

	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded, "expected detached session to be created")
	require.Contains(t, recorded.CommandRan, "claude --remote-control '"+expectedSession+"' 'hello'")
}

func TestVibeRemoteControl_NonClaudeWarnsAndDoesNotChangeLauncher(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	sessionMgr := &testutils.MockSessionManager{}

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithSessionManager(sessionMgr),
	)

	res := h.RunOK(
		"vibe",
		"--name", "wk-remote-codex",
		"--repo-url", remoteURL,
		"--agent", "codex",
		"--no-container",
		"--remote",
		"hello",
	)
	require.Contains(t, res.Stderr, "remote control is not supported for this agent; ignoring --remote")

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	workspace := filepath.Join(runDir, org, repo, "wk-remote-codex")
	expectedSession := session.SessionNameFromWorkspaceName(workspace)

	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded, "expected detached session to be created")
	require.Contains(t, recorded.CommandRan, "codex")
	require.NotContains(t, recorded.CommandRan, "--remote-control")
}

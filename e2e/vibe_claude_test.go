package e2e_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestVibeLaunchesClaudeWithoutContainer(t *testing.T) {
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
		"--name", "wk",
		"--repo-url", remoteURL,
		"--agent", "claude",
		"--no-container",
		"hello",
	)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	workspace := filepath.Join(runDir, org, repo, "wk")
	testutils.RequireDirExists(t, workspace)

	expectedSession := session.SessionNameFromWorkspaceName(workspace)
	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded, "expected detached session to be created")
	require.Contains(t, recorded.CommandRan, fmt.Sprintf("cd '%s'", workspace))
	require.NotContains(t, recorded.CommandRan, "REMUDA_AGENT=")
	value, ok := sessionEnvValue(recorded.StartEnv, "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "claude", value)
	require.Contains(t, recorded.CommandRan, "claude 'hello'")
	require.NotContains(t, recorded.CommandRan, "claude --model")
	require.NotContains(t, recorded.CommandRan, "docker run")
	require.Contains(t, recorded.CommandRan, "; sleep 3600")
}

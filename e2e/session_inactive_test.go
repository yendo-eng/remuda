package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionInactive(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*testutils.Harness, string) {
		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		return h, remoteURL
	}

	t.Run("no inactive workspaces", func(t *testing.T) {
		h, _ := setup(t)

		res := h.RunOK("session", "inactive")
		require.Empty(t, res.Stdout)
	})

	t.Run("prints inactive workspace paths", func(t *testing.T) {
		h, remoteURL := setup(t)

		baseDir := h.RemudaConfig.ReposBaseDir
		name := "test-session"
		org, repo, _ := github.ParseRepo(remoteURL)
		wsPath := filepath.Join(baseDir, org, repo, name)
		sessionName := session.SessionNameFromWorkspaceName(wsPath)

		// Create a session and workspace.
		h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

		// Kill the session so the workspace is considered inactive.
		h.RunOK("session", "kill", "--name", sessionName)

		res := h.RunOK("session", "inactive")
		require.Equal(t, wsPath+"\n", res.Stdout)
	})
}

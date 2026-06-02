package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
)

// Verifies that --branch overrides the default branch derived from --name.
func TestVibeBranchOverride(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	args := []string{
		"vibe",
		"--name", "wk",
		"--branch", "feature/x",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	}

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}))
	h.RunOK(args...)

	workspace := filepath.Join(runDir, org, repo, "wk")
	branch := testutils.RunGit(t, workspace, "rev-parse", "--abbrev-ref", "HEAD")
	require.Equal(t, "feature/x\n", branch)
}

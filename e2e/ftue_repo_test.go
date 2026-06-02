package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
)

func TestCloneWithoutTTYUsesConfiguredDefaultAlias(t *testing.T) {
	// Not parallel: this test mutates global repo aliases via config resolution.
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	config := strings.Join([]string{
		"version: 1",
		"repos:",
		"  default_repo: widgets",
		"  aliases:",
		"    widgets: " + remoteURL,
		"",
	}, "\n")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o644))

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}))
	h.SetEnv("REMUDA_CONFIG", configPath)
	t.Cleanup(github.ResetRepoAliases)

	res := h.Run("clone", "--name", "wk")
	require.NoError(t, res.Err, res.String())

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	expectedPath := filepath.Join(runDir, org, repo, "wk")
	require.Equal(t, expectedPath, strings.TrimSpace(res.Stdout))
}

func TestCloneWithoutTTYAndNoRepoConfigReturnsActionableError(t *testing.T) {
	t.Parallel()
	runDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}))

	res := h.Run("clone", "--name", "wk")
	require.ErrorContains(t, res.Err, "repository is not configured")
}

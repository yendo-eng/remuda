package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
)

func TestCloneWithoutTTYUsesConfiguredDefaultAlias(t *testing.T) {
	// Not parallel: this test mutates global repo aliases via config resolution.
	remoteURL := testutils.InitTestRemote(t)

	config := strings.Join([]string{
		"version: 1",
		"repos:",
		"  default_repo: widgets",
		"  aliases:",
		"    widgets: " + remoteURL,
		"",
	}, "\n")
	configPath := writeTempConfigFile(t, config)

	h := testutils.NewHarness(t)
	h.SetEnv("REMUDA_CONFIG", configPath)
	t.Cleanup(github.ResetRepoAliases)

	res := h.Run("clone", "--name", "wk")
	require.NoError(t, res.Err, res.String())

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	expectedPath := filepath.Join(h.ReposBaseDir, org, repo, "wk")
	require.Equal(t, expectedPath, strings.TrimSpace(res.Stdout))
}

func TestCloneWithoutTTYAndNoRepoConfigReturnsActionableError(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)

	res := h.Run("clone", "--name", "wk")
	require.ErrorContains(t, res.Err, "repository is not configured")
}

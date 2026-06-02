package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/util"
)

// Regression test: when performing a full clone (copying the cache), ensure
// that the resulting workspace is clean even if the cache has local changes.
func TestFullCloneProducesCleanWorkingTree(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")

	k := internal.NewRemuda(
		internal.Config{ReposBaseDir: baseRoot},
		git.NewShellGit(),
		&testutils.MockSessionManager{},
		nil,
		nil,
		nil,
	)

	// Seed the cache by cloning once.
	require.NoError(t, cli.Run(cli.NewContext(t.Context(), k), []string{"clone", "--name", "seed", "--repo-url", remoteURL}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	cacheDir := filepath.Join(baseRoot, org, repo, ".repo_cache")

	// Make the cache dirty with both tracked and untracked changes.
	// Modify README.md (tracked) and add an untracked file.
	cmd := util.Cmd("bash", "-lc", "echo dirty >> README.md")
	cmd.Dir = cacheDir
	require.NoError(t, testutils.ApplyE2EEnvIsolationToCmd(cmd, testutils.ProcessEnvMap(), nil))
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "untracked.tmp"), []byte("x"), 0o644))
	// Sanity check: cache shows modifications.
	status := testutils.RunGit(t, cacheDir, "status", "--porcelain")
	require.NotEqual(t, "", status)

	// Now perform a full clone into a fresh workspace.
	ctx := cli.NewContext(t.Context(), k)
	args := []string{"clone", "--name", "wk-clean", "--repo-url", remoteURL, "--full-clone"}
	require.NoError(t, cli.Run(ctx, args))

	wkPath := filepath.Join(baseRoot, org, repo, "wk-clean")
	// Workspace should be clean.
	out := testutils.RunGit(t, wkPath, "status", "--porcelain")
	require.Equal(t, "", out)
}

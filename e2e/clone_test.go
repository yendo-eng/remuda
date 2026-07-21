package e2e_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
)

// `remuda clone` should error if there is already a workspace with the given
// name.
func TestRemudaCollisionErrors(t *testing.T) {
	t.Parallel()
	t.Run("without --force", func(t *testing.T) {
		remoteURL := testutils.InitTestRemote(t)
		runDir := t.TempDir()

		name := "wk"

		h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}))

		args := []string{"clone", "--name", name, "--repo-url", remoteURL}

		// First clone succeeds
		res := h.Run(args...)
		require.NoError(t, res.Err, res.String())

		// Second clone with same name should fail, and the underlying git
		// fatal error should be visible instead of a bare "exit status 128".
		res = h.Run(args...)
		require.Error(t, res.Err, "expected collision to error")
		require.ErrorContains(t, res.Err, "fatal:")
		require.ErrorContains(t, res.Err, "already exists")
	})

	t.Run("with --force", func(t *testing.T) {
		remoteURL := testutils.InitTestRemote(t)
		runDir := t.TempDir()

		name := "wk"

		h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}))

		args := []string{"clone", "--name", name, "--repo-url", remoteURL, "--force"}

		// First clone succeeds
		res := h.Run(args...)
		require.NoError(t, res.Err, res.String())

		// Second clone with same name should succeed due to --force.
		res = h.Run(args...)
		var pathErr *os.PathError
		if errors.As(res.Err, &pathErr) {
			t.Fatalf("remuda clone failed due to filesystem error: %v", pathErr)
		}
		require.NoError(t, res.Err, res.String())
	})
}

// The workspace folder name and the checked out branch should be the same.
func TestRemudaBranchEqualsName(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposDir}))

	org, repo, _ := github.ParseRepo(remoteURL)

	name := "feature_xyz"
	args := []string{"clone", "--name", name, "--repo-url", remoteURL}
	res := h.Run(args...)
	require.NoError(t, res.Err, res.String())

	dir := filepath.Join(reposDir, org, repo, name)
	testutils.RequireDirExists(t, dir)

	out := testutils.RunGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
	gotBranch := strings.TrimSpace(out)
	require.Equal(t, name, gotBranch, "expected branch to equal workspace name")
}

func TestClonePlacesWorkspaceUnderConfiguredBaseDir(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "vibing")

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
		testutils.WithCloneHooks(internal.NewCloneHookRegistry()),
	)

	workspace, err := h.Remuda.Clone(internal.CloneCommand{
		Name:    "wk",
		RepoURL: remoteURL,
	})
	require.NoError(t, err)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	expectedBase := filepath.Join(baseRoot, org, repo)
	require.Equal(t, filepath.Join(expectedBase, "wk"), workspace)
	require.DirExists(t, filepath.Join(expectedBase, ".repo_cache"))
}

func TestCloneAcceptsRegisteredExperimentFlag(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposDir}))

	res := h.Run("clone", "--experiments", "use-prompts-context-wrapper", "--name", "wk", "--repo-url", remoteURL)
	require.NoError(t, res.Err, res.String())
}

func TestCloneRejectsUnknownExperimentFlag(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposDir}))

	res := h.Run("clone", "--experiments", "not-real", "--name", "wk", "--repo-url", remoteURL)
	require.ErrorContains(t, res.Err, `--experiments: unknown experiment "not-real"`)
}

func TestCloneCreatesBranchMatchingBaseName(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "vibing")

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
		testutils.WithCloneHooks(internal.NewCloneHookRegistry()),
	)

	name := "sessions/dev/alice"
	workspace, err := h.Remuda.Clone(internal.CloneCommand{
		Name:    name,
		RepoURL: remoteURL,
	})
	require.NoError(t, err)

	branch := strings.TrimSpace(testutils.RunGit(t, workspace, "rev-parse", "--abbrev-ref", "HEAD"))
	require.Equal(t, filepath.Base(name), branch)
}

func TestCloneBranchOverride(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "vibing")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	// Clone with name wk but checkout feature/xyz
	args := []string{"clone", "--name", "wk", "--branch", "feature/xyz", "--repo-url", remoteURL}
	res := h.Run(args...)
	require.NoError(t, res.Err, res.String())

	workspace := filepath.Join(baseRoot, org, repo, "wk")
	out := testutils.RunGit(t, workspace, "rev-parse", "--abbrev-ref", "HEAD")
	require.Equal(t, "feature/xyz\n", out)
}

func TestCloneRunsRegisteredHooks(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "vibing")
	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	// Register a clone hook that runs for our test repo to verify hooks run.
	ran := false
	hook := internal.NewCloneHook("test-hook", func(ctx internal.CloneHookContext) error {
		ran = true
		return nil
	})
	registry := internal.NewCloneHookRegistry()
	registry.Register(org, repo, hook)

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
		testutils.WithCloneHooks(registry),
	)

	// Clone a completely different repo to ensure the hook does not run.
	otherRemote := testutils.InitTestRemote(t)
	args := []string{"clone", "--name", "wk1", "--repo-url", otherRemote}
	res := h.Run(args...)
	require.NoError(t, res.Err, res.String())

	require.False(t, ran, "hook should not run for other repo")

	// Now clone our target repo and verify the hook runs.
	args = []string{"clone", "--name", "wk2", "--repo-url", remoteURL}
	res = h.Run(args...)
	require.NoError(t, res.Err, res.String())
	require.True(t, ran, "hook did not run")
}

func TestCloneHookFailureCleansWorktree(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	// Install a hook that always fails.
	registry := internal.NewCloneHookRegistry()
	registry.Register(org, repo,
		internal.NewCloneHook("fail", func(ctx internal.CloneHookContext) error {
			require.DirExists(t, ctx.WorktreeDir)
			return errors.New("boom")
		}),
	)

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
		testutils.WithCloneHooks(registry),
	)

	// Attempt to clone, which should fail due to the hook.
	args := []string{"clone", "--name", "wk", "--repo-url", remoteURL}
	res := h.Run(args...)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "hook fail failed")

	// Worktree should be cleaned up on hook failure.
	require.NoDirExists(t, filepath.Join(baseRoot, org, repo, "wk"))
}

func TestSkipCloneHooksOption(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	// Register a hook that should not run when --skip-clone-hooks is used.
	ran := false
	registry := internal.NewCloneHookRegistry()
	registry.Register(org, repo,
		internal.NewCloneHook("test-hook", func(ctx internal.CloneHookContext) error {
			ran = true
			return nil
		}),
	)

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
		testutils.WithCloneHooks(registry),
	)

	// Clone with --no-clone-hooks flag should not run hooks.
	args := []string{"clone", "--name", "wk", "--repo-url", remoteURL, "--no-clone-hooks"}
	res := h.Run(args...)
	require.NoError(t, res.Err, res.String())

	require.False(t, ran, "hook should not run when --no-clone-hooks is used")

	// Verify the workspace was still created successfully.
	require.DirExists(t, filepath.Join(baseRoot, org, repo, "wk"))
}

func TestFullCloneCreatesStandaloneRepository(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	// Baseline: standard clone should create a worktree with .git as a file pointing to cache.
	h.RunOK("clone", "--name", "wk-worktree", "--repo-url", remoteURL)
	gitPath := filepath.Join(baseRoot, org, repo, "wk-worktree", ".git")
	info, err := os.Stat(gitPath)
	require.NoError(t, err)
	require.False(t, info.IsDir(), ".git should be a file when using worktrees")

	// With --full-clone the workspace should contain a standalone repo (.git directory).
	h.RunOK("clone", "--name", "wk-full", "--repo-url", remoteURL, "--full-clone")
	fullGitPath := filepath.Join(baseRoot, org, repo, "wk-full", ".git")
	fullInfo, err := os.Stat(fullGitPath)
	require.NoError(t, err)
	require.True(t, fullInfo.IsDir(), ".git should be a directory when --full-clone is used")
}

// The cow-clone experiment only changes how bytes get from the cache to the
// workspace, and it falls back to a plain copy on filesystems without
// copy-on-write — so the workspace must come out identical either way.
func TestFullCloneWithCoWExperiment(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	h.RunOK("clone", "--name", "wk-cow", "--repo-url", remoteURL, "--full-clone", "--experiments", "cow-clone")

	wkPath := filepath.Join(baseRoot, org, repo, "wk-cow")
	gitInfo, err := os.Stat(filepath.Join(wkPath, ".git"))
	require.NoError(t, err)
	require.True(t, gitInfo.IsDir(), ".git should be a directory when --full-clone is used")
	require.FileExists(t, filepath.Join(wkPath, "README.md"))
	require.Equal(t, "", testutils.RunGit(t, wkPath, "status", "--porcelain"))
}

func TestCloneConcurrentSameRepo(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}))

	const cloneCount = 8

	type cloneResult struct {
		workspace string
		err       error
	}
	results := make([]cloneResult, cloneCount)

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(cloneCount)

	for i := 0; i < cloneCount; i++ {
		go func(i int) {
			defer wg.Done()
			<-start

			workspace, err := h.Remuda.Clone(internal.CloneCommand{
				Name:    fmt.Sprintf("wk-%d", i),
				RepoURL: remoteURL,
			})
			results[i] = cloneResult{
				workspace: workspace,
				err:       err,
			}
		}(i)
	}

	close(start)
	wg.Wait()

	for i := 0; i < cloneCount; i++ {
		require.NoError(t, results[i].err, "clone %d failed", i)
		require.DirExists(t, results[i].workspace)
	}

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(baseRoot, org, repo, ".repo_cache"))
}

func TestCloneSkipCacheRefresh(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")

	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	cacheDir := filepath.Join(baseRoot, org, repo, ".repo_cache")

	// Seed the cache with an initial clone.
	res := h.Run("clone", "--name", "seed", "--repo-url", remoteURL)
	require.NoError(t, res.Err, res.String())
	preSHA := strings.TrimSpace(testutils.RunGit(t, cacheDir, "rev-parse", "HEAD"))

	// Push a new commit straight to the bare remote so a refresh would
	// observably advance the cache.
	newSHA := pushCommitToRemote(t, remoteURL, "new.txt", "hello again")
	require.NotEqual(t, preSHA, newSHA, "test setup: new commit should differ from initial HEAD")

	// Clone again with --skip-cache-refresh. The cache must not advance and
	// the new commit's objects must not be present in the workspace (which
	// shares its object store with the cache).
	res = h.Run("clone", "--name", "skip", "--repo-url", remoteURL, "--skip-cache-refresh")
	require.NoError(t, res.Err, res.String())

	postSHA := strings.TrimSpace(testutils.RunGit(t, cacheDir, "rev-parse", "HEAD"))
	require.Equal(t, preSHA, postSHA, "cache HEAD advanced — refresh ran despite --skip-cache-refresh")

	skipWorkspace := filepath.Join(baseRoot, org, repo, "skip")
	reachable := testutils.RunGit(t, skipWorkspace, "rev-list", "--all")
	require.NotContains(t, reachable, newSHA,
		"new commit is reachable from workspace — cache was refreshed despite --skip-cache-refresh")

	// Sanity: a clone without the flag picks up the new commit, proving the
	// remote really did move forward (i.e. the negative assertion above isn't
	// passing because the test setup failed to push).
	res = h.Run("clone", "--name", "refresh", "--repo-url", remoteURL)
	require.NoError(t, res.Err, res.String())
	refreshedSHA := strings.TrimSpace(testutils.RunGit(t, cacheDir, "rev-parse", "HEAD"))
	require.Equal(t, newSHA, refreshedSHA, "cache HEAD did not advance after a normal clone")
}

// pushCommitToRemote clones the bare remote into a fresh workdir, writes a
// file, commits, pushes back, and returns the new HEAD SHA on main.
func pushCommitToRemote(t *testing.T, remoteURL, filename, contents string) string {
	t.Helper()
	work := t.TempDir()
	testutils.RunGit(t, work, "clone", remoteURL, ".")
	testutils.RunGit(t, work, "config", "user.email", "test@example.com")
	testutils.RunGit(t, work, "config", "user.name", "Test User")
	testutils.RunGit(t, work, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(work, filename), []byte(contents), 0o644))
	testutils.RunGit(t, work, "add", filename)
	testutils.RunGit(t, work, "commit", "-m", "second commit")
	testutils.RunGit(t, work, "push", "origin", "main")
	return strings.TrimSpace(testutils.RunGit(t, work, "rev-parse", "HEAD"))
}

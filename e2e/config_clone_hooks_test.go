package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
)

const configCloneHookMarker = ".config-clone-hook-marker"
const configCloneHookBranchMarker = ".config-clone-hook-branch"

func writePerRepoCloneHookConfig(t *testing.T, h *testutils.Harness, slug string) {
	t.Helper()
	writePerRepoCloneHookConfigWithScript(t, h, slug, fmt.Sprintf("echo hook-ran > %s", configCloneHookMarker))
}

func writePerRepoCloneHookConfigWithScript(t *testing.T, h *testutils.Harness, slug, script string) {
	t.Helper()
	writeConfigFile(t, h, fmt.Sprintf(`
version: 1
per_repo:
  %q:
    clone_hooks:
      - argv: ["/bin/sh", "-c", %q]
`, slug, script))
}

func seedReviewBranch(t *testing.T, h *testutils.Harness, remoteURL, org, repo, branch string) {
	t.Helper()
	h.RunOK("clone", "--repo-url", remoteURL, "--name", "seed", "--no-clone-hooks")
	cacheDir := filepath.Join(h.ReposBaseDir, org, repo, ".repo_cache")
	require.NoError(t, h.Remuda.Git.Branch(cacheDir, branch))
	testutils.RunGit(t, cacheDir, "push", "origin", branch)
}

func TestConfigCloneHooksRunForCloneCommand(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBaseDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposBaseDir}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	writePerRepoCloneHookConfig(t, h, org+"/"+repo)

	res := h.RunOK("clone", "--name", "wk", "--repo-url", remoteURL)
	workspace := strings.TrimSpace(res.Stdout)
	require.FileExists(t, filepath.Join(workspace, configCloneHookMarker))
}

func TestConfigCloneHooksRunForVibeCommand(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBaseDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposBaseDir}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	writePerRepoCloneHookConfig(t, h, org+"/"+repo)

	h.RunOK(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	)
	require.FileExists(t, filepath.Join(reposBaseDir, org, repo, "wk", configCloneHookMarker))
}

func TestConfigCloneHooksRunForVibeCheckCommand(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBaseDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposBaseDir}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	writePerRepoCloneHookConfig(t, h, org+"/"+repo)

	branch := "review-branch"
	seedReviewBranch(t, h, remoteURL, org, repo, branch)

	h.RunOK("vibe-check", "--name", "review", "--repo-url", remoteURL, "--agent-cmd", "true", branch)
	require.FileExists(t, filepath.Join(reposBaseDir, org, repo, "review", configCloneHookMarker))
}

func TestNoCloneHooksSkipsConfigCloneHooksForVibeCheck(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBaseDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposBaseDir}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	writePerRepoCloneHookConfig(t, h, org+"/"+repo)

	branch := "skip-hooks-branch"
	seedReviewBranch(t, h, remoteURL, org, repo, branch)

	h.RunOK(
		"vibe-check",
		"--name", "review-skip",
		"--repo-url", remoteURL,
		"--agent-cmd", "true",
		"--no-clone-hooks",
		branch,
	)
	require.NoFileExists(t, filepath.Join(reposBaseDir, org, repo, "review-skip", configCloneHookMarker))
}

func TestVibeCheckCloneHooksRunOnReviewedBranch(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBaseDir := t.TempDir()
	h := testutils.NewHarness(t, testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposBaseDir}))

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	writePerRepoCloneHookConfigWithScript(t, h, org+"/"+repo, fmt.Sprintf("git branch --show-current > %s", configCloneHookBranchMarker))

	branch := "review-ref"
	seedReviewBranch(t, h, remoteURL, org, repo, branch)

	h.RunOK("vibe-check", "--name", "review-branch", "--repo-url", remoteURL, "--agent-cmd", "true", branch)

	data, err := os.ReadFile(filepath.Join(reposBaseDir, org, repo, "review-branch", configCloneHookBranchMarker))
	require.NoError(t, err)
	require.Equal(t, branch, strings.TrimSpace(string(data)))
}

func TestVibeCheckPRCloneHooksRunOnPRHeadBranch(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	reposBaseDir := t.TempDir()

	const headBranch = "pr-head-ref"
	mockGitHub := &testutils.MockGitHub{
		RepoURL: remoteURL,
		FakePRData: `{
			"author": {"id": "MDQ6VXNlcjE2MDc0MDkx","is_bot": false,"login": "alex","name": "Alex Example"},
			"baseRefName": "master",
			"body": "test",
			"headRefName": "` + headBranch + `",
			"labels": [],
			"number": 1,
			"title": "pr title",
			"url": "` + remoteURL + `/pull/1"
		}`,
	}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposBaseDir}),
		testutils.WithGitHub(mockGitHub),
	)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	writePerRepoCloneHookConfigWithScript(t, h, org+"/"+repo, fmt.Sprintf("git branch --show-current > %s", configCloneHookBranchMarker))
	seedReviewBranch(t, h, remoteURL, org, repo, headBranch)

	h.RunOK("vibe-check", "--name", "review-pr", "--repo-url", remoteURL, "--agent-cmd", "true", "--pr", "1")

	data, err := os.ReadFile(filepath.Join(reposBaseDir, org, repo, "review-pr", configCloneHookBranchMarker))
	require.NoError(t, err)
	require.Equal(t, headBranch, strings.TrimSpace(string(data)))
}

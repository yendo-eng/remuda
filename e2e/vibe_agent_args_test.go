package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

func TestVibeAgentArgs_ConfigPerRepoAndCLIApplyWithShellQuoting(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent_args:
    codex:
      - --global-arg
per_repo:
  owner/repo:
    defaults:
      agent_args:
        codex:
          - --repo-arg
`), 0o644))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_CONFIG", configPath)

	h.RunOK(
		"vibe",
		"--in", workspace,
		"--agent", "codex",
		"--model", "gpt-5",
		"--agent-arg=--cli,a",
		"--agent-arg=--cli space",
		"--agent-arg=--cli$(echo hi);true",
		"prompt",
	)

	recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded, "expected detached session to be created")
	require.NotContains(t, recorded.CommandRan, shellutil.SingleQuote("--global-arg"))

	expectedArgs := shellutil.SingleQuote("--repo-arg") + " " +
		shellutil.SingleQuote("--cli,a") + " " +
		shellutil.SingleQuote("--cli space") + " " +
		shellutil.SingleQuote("--cli$(echo hi);true")
	require.Contains(t, recorded.CommandRan, expectedArgs+" -- 'prompt'")
	require.NotContains(t, recorded.CommandRan, shellutil.SingleQuote("--cli")+" "+shellutil.SingleQuote("a"))
}

func TestVibeAgentArgs_IgnoredWhenAgentCmdIsSet(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "owner", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent_args:
    codex:
      - --from-config
`), 0o644))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_CONFIG", configPath)

	h.RunOK(
		"vibe",
		"--in", workspace,
		"--agent-cmd", "echo",
		"--agent-arg=--from-cli,a",
		"prompt",
	)

	recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded, "expected detached session to be created")
	require.Contains(t, recorded.CommandRan, "&& echo 'prompt'")
	require.NotContains(t, recorded.CommandRan, shellutil.SingleQuote("--from-config"))
	require.NotContains(t, recorded.CommandRan, shellutil.SingleQuote("--from-cli,a"))
}

func TestVibeCheckAgentArgs_ConfigAndCLIApplyWithShellQuoting(t *testing.T) {
	t.Parallel()

	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")
	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  agent_args:
    codex:
      - --cfg-check
`), 0o644))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{}),
		testutils.WithGitHub(&testutils.MockGitHub{RepoURL: remoteURL}),
	)
	h.SetEnv("REMUDA_CONFIG", configPath)

	h.RunOK("clone", "--repo-url", remoteURL, "--name", "initial")

	cacheDir := filepath.Join(baseRoot, org, repo, ".repo_cache")
	require.NoError(t, h.Remuda.Git.Branch(cacheDir, "branch-under-review"))
	testutils.RunGit(t, cacheDir, "push", "origin", "branch-under-review")

	h.RunOK(
		"vibe-check",
		"--repo-url", remoteURL,
		"--agent", "codex",
		"--model", "gpt-5",
		"--agent-arg=--cli-check,a",
		"--agent-arg=--cli-check space",
		"branch-under-review",
	)

	expectedSession := filepath.Join(org, repo, "branch-under-review-code-review")
	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded, "expected detached review session to be created")

	expectedArgs := shellutil.SingleQuote("--cfg-check") + " " +
		shellutil.SingleQuote("--cli-check,a") + " " +
		shellutil.SingleQuote("--cli-check space")
	require.Contains(t, recorded.CommandRan, expectedArgs+" -- 'Pull Request Review")
	require.NotContains(t, recorded.CommandRan, shellutil.SingleQuote("--cli-check")+" "+shellutil.SingleQuote("a"))
}

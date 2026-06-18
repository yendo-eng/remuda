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
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestVibeContainerFailsWhenDockerUnavailable(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithJira(jira.Mock{}),
		testutils.WithDocker(&docker.Mock{Running: false}),
	)

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--container",
		"--container-name", "ghcr.io/acme/vibe-dev:latest",
		"--agent-cmd", "true",
		"--no-tmux",
		"prompt",
	}

	res := h.Run(args...)
	require.ErrorContains(t, res.Err, "docker is not running")
}

func TestVibeContainerRequiresExplicitImage(t *testing.T) {
	t.Parallel()
	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "org", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)

	res := h.Run(
		"vibe",
		"--in", workspace,
		"--container",
		"prompt",
	)
	require.Error(t, res.Err)
	require.ErrorContains(t, res.Err, "container mode requires an explicit image")

	sessions, err := sessionMgr.List()
	require.NoError(t, err)
	require.Empty(t, sessions)
}

func TestVibeContainerUsesConfiguredImageWithoutContainerNameFlag(t *testing.T) {
	t.Parallel()
	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "org", "repo", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  container:
    image: ghcr.io/acme/vibe-dev:latest
`), 0o644))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_CONFIG", configPath)
	h.SetEnv("GH_TOKEN", "test-token")

	h.RunOK(
		"vibe",
		"--in", workspace,
		"--container",
		"prompt",
	)

	expectedSession := session.SessionNameFromWorkspaceName(workspace)
	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded)
	require.Contains(t, recorded.CommandRan, "ghcr.io/acme/vibe-dev:latest")
}

func TestVibeContainerPerRepoBeadsDirOverridesHostEnv(t *testing.T) {
	t.Parallel()
	runDir := t.TempDir()
	workspace := filepath.Join(runDir, "yendo-eng", "remuda", "wk")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
per_repo:
  yendo-eng/remuda:
    defaults:
      container:
        opts:
          - "-v /host/.beads:/workspaces/.beads-issues/.beads"
          - "-e BEADS_DIR=/workspaces/.beads-issues/.beads"
`), 0o644))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_CONFIG", configPath)
	h.SetEnv("BEADS_DIR", "/host/system/beads")
	h.SetEnv("GH_TOKEN", "test-token")
	h.SetEnv("SSH_AUTH_SOCK", "")

	h.RunOK(
		"vibe",
		"--in", workspace,
		"--container",
		"--container-name", "ghcr.io/acme/vibe-dev:latest",
		"--agent-cmd", "true",
		"prompt",
	)

	recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded)
	require.Contains(t, recorded.CommandRan, "-v /host/.beads:/workspaces/.beads-issues/.beads")
	require.Contains(t, recorded.CommandRan, "-e BEADS_DIR=/workspaces/.beads-issues/.beads")
	require.NotContains(t, recorded.CommandRan, " -e BEADS_DIR ")
	require.NotContains(t, recorded.CommandRan, "/host/system/beads")
}

func TestVibeAutoGeneratesWorkspaceName(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
		testutils.WithGitHub(&testutils.MockGitHub{RepoURL: remoteURL}),
	)

	h.SetEnv("REMUDA_LLM_PROVIDER", "local")
	h.SetEnv("OPENAI_API_KEY", "")
	h.SetEnv("REMUDA_OPENAI_API_KEY", "")

	const prompt = "hello world"
	args := []string{
		"vibe",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		prompt,
	}

	h.RunOK(args...)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	const expectedName = "hello-world"
	expectedWorkspace := filepath.Join(runDir, org, repo, expectedName)
	testutils.RequireDirExists(t, expectedWorkspace)

	gotBranch := strings.TrimSpace(testutils.RunGit(t, expectedWorkspace, "branch", "--show-current"))
	require.Equal(t, expectedName, gotBranch, "expected branch to equal workspace name")
}

func TestVibeAutoGeneratesWorkspaceNameFromStdinPrompt(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
		testutils.WithGitHub(&testutils.MockGitHub{RepoURL: remoteURL}),
		testutils.WithStdin(strings.NewReader("implement pagination\n")),
	)

	h.SetEnv("REMUDA_LLM_PROVIDER", "local")
	h.SetEnv("OPENAI_API_KEY", "")
	h.SetEnv("REMUDA_OPENAI_API_KEY", "")

	args := []string{
		"vibe",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"-",
	}

	res := h.Run(args...)
	require.NoError(t, res.Err, res.String())

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	const expectedName = "implement-pagination"
	expectedWorkspace := filepath.Join(runDir, org, repo, expectedName)
	testutils.RequireDirExists(t, expectedWorkspace)

	gotBranch := strings.TrimSpace(testutils.RunGit(t, expectedWorkspace, "branch", "--show-current"))
	require.Equal(t, expectedName, gotBranch, "expected branch to equal workspace name")
}

func TestVibeAutoGeneratesWorkspaceNameFromFirstJiraTicket(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
		testutils.WithGitHub(&testutils.MockGitHub{RepoURL: remoteURL}),
		testutils.WithJira(jira.Mock{
			Tickets: map[string]string{
				"RBL-1234": "RBL-1234: Fix login timeout handling\nStatus: Open",
				"RBL-9999": "RBL-9999: Secondary ticket context",
			},
		}),
	)

	res := h.RunOK(
		"vibe",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo",
		"--jira", "rbl-1234",
		"--jira", "RBL-9999",
		"implement fix",
	)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	const expectedName = "RBL-1234-fix-login-timeout-handling"
	expectedWorkspace := filepath.Join(runDir, org, repo, expectedName)
	testutils.RequireDirExists(t, expectedWorkspace)

	gotBranch := strings.TrimSpace(testutils.RunGit(t, expectedWorkspace, "branch", "--show-current"))
	require.Equal(t, expectedName, gotBranch, "expected branch to equal workspace name")

	firstIdx := strings.Index(res.Stdout, "---------- Ticket RBL-1234 ----------")
	secondIdx := strings.Index(res.Stdout, "---------- Ticket RBL-9999 ----------")
	require.NotEqual(t, -1, firstIdx, "expected first jira ticket context in output")
	require.NotEqual(t, -1, secondIdx, "expected second jira ticket context in output")
	require.Less(t, firstIdx, secondIdx, "expected jira context order to match input order")
}

func TestVibeAutoGeneratesWorkspaceNameFromJiraFallsBackToKeyWhenSummaryMissing(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
		testutils.WithGitHub(&testutils.MockGitHub{RepoURL: remoteURL}),
		testutils.WithJira(jira.Mock{
			Tickets: map[string]string{
				"RBL-1234": "RBL-1234: (no summary)\nStatus: Open",
			},
		}),
	)

	h.RunOK(
		"vibe",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"--jira", "RBL-1234",
		"implement fix",
	)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	const expectedName = "RBL-1234"
	expectedWorkspace := filepath.Join(runDir, org, repo, expectedName)
	testutils.RequireDirExists(t, expectedWorkspace)

	gotBranch := strings.TrimSpace(testutils.RunGit(t, expectedWorkspace, "branch", "--show-current"))
	require.Equal(t, expectedName, gotBranch, "expected branch to equal workspace name")
}

func TestVibeAutoGeneratesWorkspaceNameFromJiraErrorsWhenFirstTicketMissing(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
		testutils.WithGitHub(&testutils.MockGitHub{RepoURL: remoteURL}),
		testutils.WithJira(jira.Mock{
			Tickets: map[string]string{
				"RBL-9999": "RBL-9999: Secondary ticket context",
			},
		}),
	)

	res := h.Run(
		"vibe",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"--jira", "RBL-1234",
		"--jira", "RBL-9999",
		"implement fix",
	)
	require.Error(t, res.Err)
	require.ErrorContains(t, res.Err, "get ticket RBL-1234")
}

func TestVibeContextEngineering(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	const threadURL = "https://example.slack.com/threads/1234567890"
	const issueSlug = "acme/project"
	const issueNumber = "123"
	issueKey := issueSlug + "|" + issueNumber
	issueURL := "https://github.com/" + issueSlug + "/issues/" + issueNumber

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithJira(jira.Mock{Tickets: map[string]string{"ABC-123": "product requirements here"}}),
		testutils.WithDocker(&docker.Mock{Running: true}),
		testutils.WithGitHub(&testutils.MockGitHub{
			RepoURL: remoteURL,
			Issues: map[string]*github.Issue{
				issueKey: {
					Number: 123,
					Title:  "Resolve login flake",
					Body:   "Sessions expire too early",
					State:  "open",
					URL:    issueURL,
					Author: github.IssueActor{Login: "octocat"},
					Labels: []github.IssueLabel{{Name: "bug"}},
				},
			},
		}),
		testutils.WithSlack(testutils.MockSlack{Threads: map[string]string{threadURL: "slack messages lol"}}),
	)

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--jira", "ABC-123",
		"--slack-thread", threadURL,
		"--gh-issue", issueSlug + "#" + issueNumber,
		"--agent-cmd", "echo ",
		"prompt",
	}

	res := h.RunOK(args...)
	outStr := res.Stdout
	// errStr := stderr.String()
	expectedJira := "---------- Ticket ABC-123 ----------\nproduct requirements here\n"
	expectedSlack := "---------- Slack Thread " + threadURL + " ----------\nslack messages lol\n"
	issueBlock := "---------- GitHub Issue " + issueSlug + "#" + issueNumber + " ----------\n" +
		"Title: Resolve login flake\n" +
		"State: OPEN\n" +
		"Author: octocat\n" +
		"Labels: bug\n" +
		"URL: " + issueURL + "\n" +
		"Body:\nSessions expire too early\n"
	require.Contains(t, outStr, expectedJira)
	require.Contains(t, outStr, expectedSlack)
	require.Contains(t, outStr, issueBlock)
	jiraIdx := strings.Index(outStr, expectedJira)
	slackIdx := strings.Index(outStr, expectedSlack)
	issueIdx := strings.Index(outStr, issueBlock)
	require.True(t, jiraIdx >= 0 && slackIdx > jiraIdx && issueIdx > slackIdx, "context blocks out of order")
	require.True(t, strings.HasSuffix(outStr, "\nprompt\n"))
}

func TestVibeUseBuiltInPrompts(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"--use", "small-commits",
		"implement caching",
	}

	res := h.RunOK(args...)
	outStr := res.Stdout
	require.Contains(t, outStr, "Please work in small, verifiable steps. Use git to manage your changes.")
	require.Contains(t, outStr, "\n\nimplement caching\n")
	firstPrompt := strings.Index(outStr, "Please work in small, verifiable steps.")
	userPrompt := strings.Index(outStr, "implement caching")
	require.NotEqual(t, -1, firstPrompt)
	require.NotEqual(t, -1, userPrompt)
	require.Less(t, firstPrompt, userPrompt)
}

func TestVibeNoUseExcludesDefaultPrompts(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_USE_PROMPTS", "make-pr,small-commits")

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"--no-use", "make-pr",
		"implement caching",
	}

	res := h.RunOK(args...)
	outStr := res.Stdout
	require.Contains(t, outStr, "Please work in small, verifiable steps. Use git to manage your changes.")
	require.NotContains(t, outStr, "gh pr create")
	require.Contains(t, outStr, "\n\nimplement caching\n")
}

func TestVibeNoUseExcludesExplicitPrompts(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"--use", "small-commits",
		"--use", "make-pr",
		"--no-use", "make-pr",
		"implement caching",
	}

	res := h.RunOK(args...)
	outStr := res.Stdout
	require.Contains(t, outStr, "Please work in small, verifiable steps. Use git to manage your changes.")
	require.NotContains(t, outStr, "gh pr create")
	require.Contains(t, outStr, "\n\nimplement caching\n")
}

func TestVibeNoUseAllowsEmptyPrompt(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_USE_PROMPTS", "make-pr")

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"--no-use", "make-pr",
	}

	res := h.Run(args...)
	require.NoError(t, res.Err, res.String())
}

func TestVibeDoesNotAutoConfigureBeads(t *testing.T) {
	t.Parallel()
	remoteURL := func() string {
		t.Helper()

		temp := t.TempDir()
		remotePath := filepath.Join(temp, "remote.git")
		testutils.RunGit(t, temp, "init", "--bare", remotePath)

		workDir := filepath.Join(temp, "work")
		testutils.RunGit(t, temp, "clone", remotePath, workDir)

		// Configure git user for commits.
		testutils.RunGit(t, workDir, "config", "user.email", "test@example.com")
		testutils.RunGit(t, workDir, "config", "user.name", "Test User")
		testutils.RunGit(t, workDir, "config", "commit.gpgsign", "false")

		require.NoError(t, os.WriteFile(filepath.Join(workDir, "README.md"), []byte("hello"), 0o644))

		require.NoError(t, os.MkdirAll(filepath.Join(workDir, ".beads"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(workDir, ".beads", "config.yaml"), []byte("sync-branch: beads-sync\n"), 0o644))

		testutils.RunGit(t, workDir, "add", "README.md", ".beads/config.yaml")
		testutils.RunGit(t, workDir, "commit", "-m", "initial commit")
		testutils.RunGit(t, workDir, "branch", "-M", "main")
		testutils.RunGit(t, workDir, "push", "-u", "origin", "main")

		// Ensure bare repo's HEAD points to main to avoid git pull errors on master.
		testutils.RunGit(t, ".", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")
		return remotePath
	}()
	runDir := t.TempDir()

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("BEADS_DIR", "/tmp/explicit-beads")

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	beadsWorktree := filepath.Join(runDir, org, repo, ".beads_worktree")

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", `bash -lc "echo $BEADS_DIR"`,
		"prompt",
	}

	res := h.RunOK(args...)
	require.Contains(t, res.Stdout, "/tmp/explicit-beads\n")
	require.NoDirExists(t, beadsWorktree)
}

func TestVibeUseCustomPrompts(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	customDir := t.TempDir()
	content := "please end your work by telling a joke"
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "tell-jokes"), []byte(content), 0o644))

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_PROMPTS_DIR", customDir)

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"--use", "tell-jokes",
		"implement caching",
	}

	res := h.RunOK(args...)
	outStr := res.Stdout
	require.Contains(t, outStr, content)
	require.Contains(t, outStr, "\n\nimplement caching\n")
}

func TestVibeUsePromptPrefersCustomOverride(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	customDir := t.TempDir()
	content := "OVERRIDE small commits"
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "small-commits"), []byte(content), 0o644))

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetEnv("REMUDA_PROMPTS_DIR", customDir)

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "echo ",
		"--use", "small-commits",
		"implement caching",
	}

	res := h.RunOK(args...)
	outStr := res.Stdout
	require.Contains(t, outStr, content)
	require.NotContains(t, outStr, "Please work in small, verifiable steps. Use git to manage your changes.")
	require.Contains(t, outStr, "\n\nimplement caching\n")
}

func TestVibeRunsCloneHooks(t *testing.T) {
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
	h.SetEnv("REMUDA_CONTAINER", "false")

	// Vibe in a completely different repo to ensure the hook does not run.
	otherRemote := testutils.InitTestRemote(t)
	args := []string{"vibe", "--name", "wk1", "--repo-url", otherRemote}
	h.RunOK(args...)

	require.False(t, ran, "hook should not run for other repo")

	// Now vibe in our target repo and verify the hook runs.
	args = []string{"vibe", "--name", "wk2", "--repo-url", remoteURL}
	h.RunOK(args...)
	require.True(t, ran, "hook did not run")
}

func TestVibeForceReplacesExistingWorkspace(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	workspacePath := filepath.Join(runDir, org, repo, "wk")

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	}

	// First run creates the workspace successfully.
	h.RunOK(args...)
	testutils.RequireDirExists(t, workspacePath)
	markerFile := filepath.Join(workspacePath, "force-marker.txt")
	require.NoError(t, os.WriteFile(markerFile, []byte("vibe"), 0o644))
	require.FileExists(t, markerFile)

	// Second run with --force should replace the existing workspace instead of failing.
	forceArgs := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-tmux",
		"--no-container",
		"--agent-cmd", "true",
		"--force",
		"prompt",
	}
	h.RunOK(forceArgs...)
	testutils.RequireDirExists(t, workspacePath)
	require.NoFileExists(t, markerFile)
}

func TestVibeForceKillsExistingSession(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	runDir := t.TempDir()
	sessionMgr := &testutils.MockSessionManager{}

	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: runDir}),
		testutils.WithSessionManager(sessionMgr),
	)
	h.SetEnv("REMUDA_CONTAINER", "false")

	args := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	}

	h.RunOK(args...)
	sessions, err := sessionMgr.List()
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	forceArgs := []string{
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--no-container",
		"--agent-cmd", "true",
		"--force",
		"prompt",
	}

	h.RunOK(forceArgs...)
	sessions, err = sessionMgr.List()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
}

func TestVibeLaunchesInExistingWorkspace(t *testing.T) {
	t.Parallel()
	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "org", "repo", "wk-existing")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	relPath := "." + string(os.PathSeparator) + filepath.Join("org", "repo", "wk-existing")

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetWorkingDir(workspaceRoot)
	h.SetEnv("REMUDA_MODEL", "")

	args := []string{
		"vibe",
		"--in", relPath,
		"--no-container",
		"--agent-cmd", "true",
		"prompt",
	}

	h.RunOK(args...)

	expectedSession := session.SessionNameFromWorkspaceName(workspace)
	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded, "expected detached session to be created")
	// Verify the command contains expected components
	require.NotContains(t, recorded.CommandRan, "export BD_ACTOR=")
	require.Contains(t, recorded.CommandRan, fmt.Sprintf("cd '%s'", workspace))
	require.NotContains(t, recorded.CommandRan, "REMUDA_AGENT=")
	require.Contains(t, recorded.CommandRan, "true 'prompt'")
	value, ok := sessionEnvValue(recorded.StartEnv, "BD_ACTOR")
	require.True(t, ok)
	require.Equal(t, expectedSession, value)
	value, ok = sessionEnvValue(recorded.StartEnv, "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "custom", value)
	// Verify crash recovery sleep is appended to detached sessions
	require.Contains(t, recorded.CommandRan, "; sleep 3600")
}

func TestVibeClaudeContainerComposesHermeticDockerCommand(t *testing.T) {
	t.Parallel()
	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "org", "repo", "wk-claude-container")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetWorkingDir(workspaceRoot)
	h.SetEnv("GH_TOKEN", "gh-token")
	h.SetEnv("SSH_AUTH_SOCK", "")

	claudeDir := filepath.Join(h.HomeDir, ".claude")
	claudeJSON := filepath.Join(h.HomeDir, ".claude.json")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))
	require.NoError(t, os.WriteFile(claudeJSON, []byte("{}"), 0o600))

	args := []string{
		"vibe",
		"--in", workspace,
		"--container",
		"--container-name", "ghcr.io/acme/vibe-dev:latest",
		"--agent", "claude",
		"--model", "claude-sonnet-4",
		"--reasoning-level", "high",
		"--yolo",
		"verify claude container integration",
	}
	h.RunOK(args...)

	expectedSession := session.SessionNameFromWorkspaceName(workspace)
	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded, "expected detached session to be created")

	expectedContainer := docker.ContainerNameFromSession(expectedSession)
	containerWS := docker.ContainerWorkspacePath(workspace)

	require.NotContains(t, recorded.CommandRan, "REMUDA_AGENT=")
	require.NotContains(t, recorded.CommandRan, "REMUDA_MODEL=")
	value, ok := sessionEnvValue(recorded.StartEnv, "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "claude", value)
	value, ok = sessionEnvValue(recorded.StartEnv, "REMUDA_MODEL")
	require.True(t, ok)
	require.Equal(t, "claude-sonnet-4", value)
	require.Contains(t, recorded.CommandRan, "docker run --rm -it")
	require.Contains(t, recorded.CommandRan, "--name "+expectedContainer)
	require.Contains(t, recorded.CommandRan, fmt.Sprintf("-v %q", workspace+":"+containerWS))
	require.Contains(t, recorded.CommandRan, "-w "+containerWS)
	require.NotContains(t, recorded.CommandRan, "-w /workspace ")
	require.Contains(t, recorded.CommandRan, "-e OPENAI_API_KEY")
	require.Contains(t, recorded.CommandRan, "-e GH_TOKEN")
	require.Contains(t, recorded.CommandRan, "-e GITHUB_TOKEN")
	require.Contains(t, recorded.CommandRan, "-e ANTHROPIC_API_KEY")
	require.Contains(t, recorded.CommandRan, fmt.Sprintf("-v %q:%q:rw", claudeDir, "/root/.claude"))
	require.Contains(t, recorded.CommandRan, fmt.Sprintf("-v %q:%q:rw", claudeJSON, "/root/.claude.json"))
	require.Contains(
		t,
		recorded.CommandRan,
		"claude --model ",
	)
	require.Contains(t, recorded.CommandRan, "claude-sonnet-4")
	require.Contains(t, recorded.CommandRan, "--dangerously-skip-permissions")
	require.Contains(t, recorded.CommandRan, "--effort ")
	require.Contains(t, recorded.CommandRan, "verify claude container integration")
}

func TestVibeOmittedPromptDoesNotAppendEmptyPromptArg(t *testing.T) {
	t.Parallel()
	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "org", "repo", "wk-existing")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	relPath := "." + string(os.PathSeparator) + filepath.Join("org", "repo", "wk-existing")

	sessionMgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithSessionManager(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetWorkingDir(workspaceRoot)
	h.SetEnv("REMUDA_MODEL", "")

	args := []string{
		"vibe",
		"--in", relPath,
		"--no-container",
		"--agent-cmd", "true",
	}

	h.RunOK(args...)

	expectedSession := session.SessionNameFromWorkspaceName(workspace)
	recorded := sessionMgr.FindSession(expectedSession)
	require.NotNil(t, recorded, "expected detached session to be created")
	require.NotContains(t, recorded.CommandRan, "REMUDA_AGENT=")
	value, ok := sessionEnvValue(recorded.StartEnv, "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "custom", value)
	require.Contains(t, recorded.CommandRan, "true")
	require.NotContains(t, recorded.CommandRan, "true ''")
}

package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

func TestVibeCheck(t *testing.T) {
	t.Parallel()
	t.Run("checks out and prepares a PR branch", func(t *testing.T) {
		remoteURL := testutils.InitTestRemote(t)
		baseRoot := filepath.Join(t.TempDir(), "repos")
		org, repo, err := github.ParseRepo(remoteURL)
		require.NoError(t, err)

		sess := &testutils.MockSessionManager{}
		mockGitHub := &testutils.MockGitHub{
			RepoURL: remoteURL,
			FakePRData: `{
				"author": {
					"id": "MDQ6VXNlcjE2MDc0MDkx",
					"is_bot": false,
					"login": "alex",
					"name": "Alex Example"
				},
				"baseRefName": "master",
				"body": "test",
				"headRefName": "branch-under-review",
				"labels": [],
				"number": 1,
				"title": "pr title",
				"url": "` + remoteURL + `/pull/1"
			}`,
		}
		h := testutils.NewHarness(t,
			testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
			testutils.WithSessionManager(sess),
			testutils.WithJira(jira.Mock{}),
			testutils.WithDocker(&docker.Mock{}),
			testutils.WithGitHub(mockGitHub),
		)

		// First clone the repo to create the local cache.
		h.RunOK("clone", "--repo-url", remoteURL, "--name", "initial")

		// Create a branch in the remote repo by going to the repo cache and using git.
		cacheDir := filepath.Join(baseRoot, org, repo, ".repo_cache")
		require.NoError(t, h.Remuda.Git.Branch(cacheDir, "branch-under-review"))
		testutils.RunGit(t, cacheDir, "push", "origin", "branch-under-review")

		// Now review the branch with vibe check.
		h.RunOK("vibe-check", "--repo-url", remoteURL, "--pr", "1")

		// The session should be created with the correct name
		expectedSessionName := filepath.Join(org, repo, "branch-under-review-code-review")
		sessions, err := sess.List()
		require.NoError(t, err)
		require.Truef(t, slices.ContainsFunc(sessions, func(s session.SessionInfo) bool {
			return s.Name == expectedSessionName
		}), "expected session %q to exist", expectedSessionName)

		// The workspace should be created with the correct path and branch.
		workspacePath := filepath.Join(baseRoot, expectedSessionName)
		require.DirExists(t, workspacePath)
		currentBranch := testutils.RunGit(t, workspacePath, "branch", "--show-current")
		require.Equal(t, "branch-under-review\n", currentBranch)

		gitDirInfo, err := os.Stat(filepath.Join(workspacePath, ".git"))
		require.NoError(t, err)
		require.True(t, gitDirInfo.IsDir(), "vibe check should default to a full clone")

		worktreesPath := filepath.Join(workspacePath, ".git", "worktrees")
		_, err = os.Stat(worktreesPath)
		require.ErrorIs(t, err, os.ErrNotExist, "copied full clone should not retain worktree metadata")
	})

	t.Run("defaults to reviewing a branch name", func(t *testing.T) {
		remoteURL := testutils.InitTestRemote(t)
		baseRoot := filepath.Join(t.TempDir(), "repos")
		org, repo, err := github.ParseRepo(remoteURL)
		require.NoError(t, err)

		sess := &testutils.MockSessionManager{}
		h := testutils.NewHarness(t,
			testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
			testutils.WithSessionManager(sess),
			testutils.WithJira(jira.Mock{}),
			testutils.WithDocker(&docker.Mock{}),
			testutils.WithGitHub(&testutils.MockGitHub{RepoURL: remoteURL}),
		)

		// First clone the repo to create the local cache.
		h.RunOK("clone", "--repo-url", remoteURL, "--name", "initial")

		// Create a branch in the remote repo by going to the repo cache and using git.
		cacheDir := filepath.Join(baseRoot, org, repo, ".repo_cache")
		require.NoError(t, h.Remuda.Git.Branch(cacheDir, "branch-under-review"))
		testutils.RunGit(t, cacheDir, "push", "origin", "branch-under-review")

		// Review the branch without --pr.
		h.RunOK("vibe-check", "--repo-url", remoteURL, "branch-under-review")

		expectedSessionName := filepath.Join(org, repo, "branch-under-review-code-review")
		sessions, err := sess.List()
		require.NoError(t, err)
		require.Truef(t, slices.ContainsFunc(sessions, func(s session.SessionInfo) bool {
			return s.Name == expectedSessionName
		}), "expected session %q to exist", expectedSessionName)

		workspacePath := filepath.Join(baseRoot, expectedSessionName)
		require.DirExists(t, workspacePath)
		currentBranch := testutils.RunGit(t, workspacePath, "branch", "--show-current")
		require.Equal(t, "branch-under-review\n", currentBranch)

		foundSession := sess.FindSession(expectedSessionName)
		require.NotNil(t, foundSession)
		prompt := extractPromptFromCommand(t, foundSession.CommandRan)
		require.Contains(t, prompt, "Pull Request Review")
		require.Contains(t, prompt, "branch-under-review")
		// Branch mode: prompt teaches the agent to discover the default base.
		require.Contains(t, prompt, "git symbolic-ref")
		require.Contains(t, prompt, "git diff")
	})

	t.Run("respects --no-full-clone override", func(t *testing.T) {
		remoteURL := testutils.InitTestRemote(t)
		baseRoot := filepath.Join(t.TempDir(), "repos")
		org, repo, err := github.ParseRepo(remoteURL)
		require.NoError(t, err)

		sess := &testutils.MockSessionManager{}
		mockGitHub := &testutils.MockGitHub{
			RepoURL: remoteURL,
			FakePRData: `{
				"author": {
					"id": "MDQ6VXNlcjE2MDc0MDkx",
					"is_bot": false,
					"login": "alex",
					"name": "Alex Example"
				},
				"baseRefName": "master",
				"body": "test",
				"headRefName": "worktree-review-branch",
				"labels": [],
				"number": 1,
				"title": "pr title",
				"url": "` + remoteURL + `/pull/1"
			}`,
		}
		h := testutils.NewHarness(t,
			testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
			testutils.WithSessionManager(sess),
			testutils.WithJira(jira.Mock{}),
			testutils.WithDocker(&docker.Mock{}),
			testutils.WithGitHub(mockGitHub),
		)

		h.RunOK("clone", "--repo-url", remoteURL, "--name", "initial")

		cacheDir := filepath.Join(baseRoot, org, repo, ".repo_cache")
		require.NoError(t, h.Remuda.Git.Branch(cacheDir, "worktree-review-branch"))
		testutils.RunGit(t, cacheDir, "push", "origin", "worktree-review-branch")

		h.RunOK("vibe-check", "--repo-url", remoteURL, "--name", "worktree-review", "--no-full-clone", "--pr", "1")

		workspacePath := filepath.Join(baseRoot, org, repo, "worktree-review")
		require.DirExists(t, workspacePath)
		gitPath := filepath.Join(workspacePath, ".git")
		gitInfo, err := os.Stat(gitPath)
		require.NoError(t, err)
		require.False(t, gitInfo.IsDir(), "--no-full-clone should create a worktree (.git file)")
	})
}

func TestVibeCheckContextEngineering(t *testing.T) {
	t.Parallel()
	remoteURL := testutils.InitTestRemote(t)
	baseRoot := filepath.Join(t.TempDir(), "repos")
	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	const (
		branchToReview = "branch-with-context"
		jiraID         = "ABC-123"
		jiraBody       = "product requirements here"
		slackThreadURL = "https://example.slack.com/archives/C1234567/p1234567890"
		slackBody      = "slack messages lol"
		issueSlug      = "acme/checkout"
		issueNumber    = "77"
		issueURL       = "https://github.com/" + issueSlug + "/issues/" + issueNumber
	)

	sess := &testutils.MockSessionManager{}
	mockGitHub := &testutils.MockGitHub{
		RepoURL: remoteURL,
		FakePRData: fmt.Sprintf(`{
			"author": {
				"id": "MDQ6VXNlcjE2MDc0MDkx",
				"is_bot": false,
				"login": "alex",
				"name": "Alex Example"
			},
			"baseRefName": "main",
			"body": "test",
			"headRefName": "%s",
			"labels": [],
			"number": 1,
			"title": "pr title",
			"url": "%s/pull/1"
		}`, branchToReview, remoteURL),
		Issues: map[string]*github.Issue{
			issueSlug + "|" + issueNumber: {
				Number: 77,
				Title:  "Tighten checkout logging",
				Body:   "Logs are missing user ID",
				State:  "closed",
				URL:    issueURL,
				Author: github.IssueActor{Login: "qa-bot"},
				Labels: []github.IssueLabel{{Name: "observability"}},
			},
		},
	}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: baseRoot}),
		testutils.WithSessionManager(sess),
		testutils.WithJira(jira.Mock{Tickets: map[string]string{jiraID: jiraBody}}),
		testutils.WithDocker(&docker.Mock{}),
		testutils.WithGitHub(mockGitHub),
		testutils.WithSlack(testutils.MockSlack{Threads: map[string]string{slackThreadURL: slackBody}}),
	)

	h.RunOK("clone", "--repo-url", remoteURL, "--name", "initial")

	cacheDir := filepath.Join(baseRoot, org, repo, ".repo_cache")
	require.NoError(t, h.Remuda.Git.Branch(cacheDir, branchToReview))
	testutils.RunGit(t, cacheDir, "push", "origin", branchToReview)

	args := []string{
		"vibe-check",
		"--repo-url", remoteURL,
		"--name", "ctx-review",
		"--jira", jiraID,
		"--slack-thread", slackThreadURL,
		"--gh-issue", issueSlug + "#" + issueNumber,
		"--pr", "1",
	}
	h.RunOK(args...)

	expectedSessionName := filepath.Join(org, repo, "ctx-review")
	sessions, err := sess.List()
	require.NoError(t, err)
	require.Truef(t, slices.ContainsFunc(sessions, func(s session.SessionInfo) bool {
		return s.Name == expectedSessionName
	}), "expected session %q to exist", expectedSessionName)

	foundSession := sess.FindSession(expectedSessionName)
	require.NotNil(t, foundSession)
	prompt := extractPromptFromCommand(t, foundSession.CommandRan)

	expectedJira := "---------- Ticket " + jiraID + " ----------\n" + jiraBody + "\n"
	expectedSlack := "---------- Slack Thread " + slackThreadURL + " ----------\n" + slackBody + "\n"
	expectedIssue := "---------- GitHub Issue " + issueSlug + "#" + issueNumber + " ----------\n" +
		"Title: Tighten checkout logging\n" +
		"State: CLOSED\n" +
		"Author: qa-bot\n" +
		"Labels: observability\n" +
		"URL: " + issueURL + "\n" +
		"Body:\nLogs are missing user ID\n\n"
	require.Contains(t, prompt, expectedJira)
	require.Contains(t, prompt, expectedSlack)
	require.Contains(t, prompt, expectedIssue)
	jiraIdx := strings.Index(prompt, expectedJira)
	slackIdx := strings.Index(prompt, expectedSlack)
	issueIdx := strings.Index(prompt, expectedIssue)
	require.True(t, jiraIdx >= 0 && slackIdx > jiraIdx && issueIdx > slackIdx, "context blocks out of order")
	require.Contains(t, prompt, "Pull Request Review")
	// PR mode: base branch is known, so the prompt names it directly.
	require.Contains(t, prompt, branchToReview)
	require.Contains(t, prompt, "git diff origin/main...HEAD")
}

// extractPromptFromCommand pulls the agent prompt back out of the captured
// shell command. The capture is of the form `cd <ws> && env... <agent> '<prompt>'`,
// with single quotes inside the prompt escaped as `'\''`.
func extractPromptFromCommand(t *testing.T, command string) string {
	t.Helper()
	parts := strings.SplitN(command, " && ", 2)
	require.Len(t, parts, 2, "expected cd && agent form, got: %q", command)
	agentCommand := parts[1]

	start := strings.Index(agentCommand, " '")
	require.NotEqual(t, -1, start, "prompt opening quote not found in %q", agentCommand)
	start += 2
	end := strings.LastIndex(agentCommand, "'")
	require.NotEqual(t, -1, end, "prompt closing quote not found in %q", agentCommand)
	return strings.ReplaceAll(agentCommand[start:end], "'\\''", "'")
}

package e2e_test

import (
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

func TestUsePromptPosition(t *testing.T) {
	t.Parallel()

	commands := []struct {
		name       string
		mainMarker string
		run        func(*testing.T, string, string) string
	}{
		{name: "vibe", mainMarker: "MAIN PROMPT", run: runVibeWithUsePromptPosition},
		{name: "vibe-check", mainMarker: "Pull Request Review", run: runVibeCheckWithUsePromptPosition},
		{name: "session-resume", mainMarker: "MAIN PROMPT", run: runSessionResumeWithUsePromptPosition},
	}
	for _, command := range commands {
		command := command
		t.Run(command.name, func(t *testing.T) {
			t.Parallel()
			for _, position := range []string{"before", "after"} {
				position := position
				t.Run(position, func(t *testing.T) {
					t.Parallel()
					prompt := command.run(t, position, "MAIN PROMPT")
					assertUsePromptPosition(t, prompt, command.mainMarker, position)
				})
			}
		})
	}
}

func TestSessionResumeUsePromptsContextWrapperExperiment(t *testing.T) {
	t.Parallel()

	prompt := runSessionResumeWithExperiments(t, "MAIN PROMPT", "use-prompts-context-wrapper")
	require.Contains(t, prompt, "<context>\nKeep scope tight and touch only what the request requires.")
	require.Contains(t, prompt, "</context>")
	assertUsePromptPosition(t, prompt, "MAIN PROMPT", "before")
}

func runVibeWithUsePromptPosition(t *testing.T, position, prompt string) string {
	t.Helper()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "vibe")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	mgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: base}),
		testutils.WithSessionManager(mgr),
		testutils.WithJira(jira.Mock{Tickets: map[string]string{"ABC-1": "reference context"}}),
		testutils.WithDocker(&docker.Mock{Running: false}),
	)

	args := []string{"vibe", "--in", workspace, "--no-container", "--use", "minimal-change", "--jira", "ABC-1"}
	if position == "after" {
		args = append(args, "--use-position", position)
	}
	args = append(args, prompt)
	h.RunOK(args...)

	recorded := mgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded)
	return extractPromptFromCommand(t, recorded.CommandRan)
}

func runVibeCheckWithUsePromptPosition(t *testing.T, position, prompt string) string {
	t.Helper()

	remoteURL := testutils.InitTestRemote(t)
	base := filepath.Join(t.TempDir(), "repos")
	branch := "branch-under-review"
	mgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: base}),
		testutils.WithSessionManager(mgr),
		testutils.WithJira(jira.Mock{Tickets: map[string]string{"ABC-1": "reference context"}}),
		testutils.WithDocker(&docker.Mock{Running: false}),
	)
	h.RunOK("clone", "--repo-url", remoteURL, "--name", "initial")
	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	cacheDir := filepath.Join(base, org, repo, ".repo_cache")
	require.NoError(t, h.Remuda.Git.Branch(cacheDir, branch))
	testutils.RunGit(t, cacheDir, "push", "origin", branch)

	args := []string{"vibe-check", "--repo-url", remoteURL, "--name", "review", "--use", "minimal-change", "--jira", "ABC-1"}
	if position == "after" {
		args = append(args, "--use-position", position)
	}
	args = append(args, branch)
	h.RunOK(args...)

	recorded := mgr.FindSession(filepath.Join(org, repo, "review"))
	require.NotNil(t, recorded)
	return extractPromptFromCommand(t, recorded.CommandRan)
}

func runSessionResumeWithUsePromptPosition(t *testing.T, position, prompt string) string {
	t.Helper()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "resume")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))
	mgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: base}),
		testutils.WithSessionManager(mgr),
		testutils.WithJira(jira.Mock{Tickets: map[string]string{"ABC-1": "reference context"}}),
		testutils.WithDocker(&docker.Mock{Running: false}),
	)

	args := []string{"session", "resume", "--no-container", "--use", "minimal-change", "--jira", "ABC-1"}
	if position == "after" {
		args = append(args, "--use-position", position)
	}
	args = append(args, workspace, prompt)
	h.RunOK(args...)

	recorded := mgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded)
	return extractPromptFromCommand(t, recorded.CommandRan)
}

func runSessionResumeWithExperiments(t *testing.T, prompt string, experiments ...string) string {
	t.Helper()

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "resume")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))
	mgr := &testutils.MockSessionManager{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: base}),
		testutils.WithSessionManager(mgr),
		testutils.WithJira(jira.Mock{Tickets: map[string]string{"ABC-1": "reference context"}}),
		testutils.WithDocker(&docker.Mock{Running: false}),
	)

	args := []string{"session", "resume", "--no-container", "--use", "minimal-change", "--jira", "ABC-1"}
	if len(experiments) > 0 {
		args = append(args, "--experiments", strings.Join(experiments, ","))
	}
	args = append(args, workspace, prompt)
	h.RunOK(args...)

	recorded := mgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded)
	return extractPromptFromCommand(t, recorded.CommandRan)
}

func assertUsePromptPosition(t *testing.T, prompt, main, position string) {
	t.Helper()
	usePrompt := "Keep scope tight and touch only what the request requires."
	reference := "---------- Ticket ABC-1 ----------"
	useIndex := strings.Index(prompt, usePrompt)
	referenceIndex := strings.Index(prompt, reference)
	mainIndex := strings.Index(prompt, main)
	require.GreaterOrEqual(t, useIndex, 0)
	require.GreaterOrEqual(t, referenceIndex, 0)
	require.GreaterOrEqual(t, mainIndex, 0)
	if position == "after" {
		require.Less(t, referenceIndex, mainIndex)
		require.Less(t, mainIndex, useIndex)
		return
	}
	require.Less(t, useIndex, referenceIndex)
	require.Less(t, referenceIndex, mainIndex)
}

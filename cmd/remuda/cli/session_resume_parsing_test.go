package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestSessionResumeCmdParse_WithWorkspaceDir(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{"session", "resume", "/tmp/workspace"})
	require.NoError(t, err)
}

func TestSessionResumeCmdParse_WithWorkspaceDirAndPrompt(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{"session", "resume", "/tmp/workspace", "continue with tests"})
	require.NoError(t, err)
	require.Equal(t, "/tmp/workspace", parsed.Session.Resume.WorkspaceDir)
	require.Equal(t, "continue with tests", parsed.Session.Resume.Prompt)
}

func TestSessionResumeCmdParse_WithPick(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{"session", "resume", "--pick"})
	require.NoError(t, err)
}

func TestSessionResumeCmdParse_WithPickAndPrompt(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{"session", "resume", "--pick", "continue with tests"})
	require.Error(t, err)
}

func TestSessionResumeCmdParse_RequiresExactlyOneMode(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{"session", "resume"})
	require.Error(t, err)

	_, err = parser.Parse([]string{"session", "resume", "/tmp/workspace", "--pick"})
	require.Error(t, err)
}

func TestSessionResumeCmdParse_RejectsBlankWorkspaceDir(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{"session", "resume", "   "})
	require.Error(t, err)
}

func TestSessionResumeCmdParse_NegatableYoloFlag(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap{"REMUDA_YOLO": "true"}
	parser, parsed, _ := newParserWithEnv(t, env)

	_, err := parser.Parse([]string{"session", "resume", "/tmp/workspace", "--no-yolo"})
	require.NoError(t, err)
	require.False(t, parsed.Session.Resume.Yolo)
}

func TestSessionResumeCmdParse_AgentAndContextFlags(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{
		"session", "resume",
		"--agent", "claude",
		"--model", "claude-sonnet-4.6",
		"--reasoning-level", "high",
		"--agent-cmd", "claude --continue",
		"--use", "small-commits",
		"--no-use", "make-pr",
		"--jira", "ABC-123",
		"--gh-issue", "https://github.com/acme/repo/issues/42",
		"--openai-api-key", "sk-test",
		"/tmp/workspace",
		"prompt",
	})
	require.NoError(t, err)
	require.Equal(t, "claude", parsed.Session.Resume.Agent)
	require.Equal(t, "claude-sonnet-4.6", parsed.Session.Resume.Model)
	require.Equal(t, "high", parsed.Session.Resume.ReasoningLevel)
	require.Equal(t, "claude --continue", parsed.Session.Resume.AgentCmd)
	require.Equal(t, []string{"ABC-123"}, parsed.Session.Resume.Jira)
	require.Equal(t, []string{"https://github.com/acme/repo/issues/42"}, parsed.Session.Resume.GitHubIssue)
	require.Equal(t, "sk-test", parsed.Session.Resume.OpenAIAPIKey)
	require.Equal(t, "prompt", parsed.Session.Resume.Prompt)
}

func TestSessionResumeCmdParse_CloneFlagsAreRejected(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, nil)

	_, err := parser.Parse([]string{"session", "resume", "--repo", "owner/repo", "/tmp/workspace"})
	require.Error(t, err)

	_, err = parser.Parse([]string{"session", "resume", "--repo-url", "https://github.com/owner/repo.git", "/tmp/workspace"})
	require.Error(t, err)

	_, err = parser.Parse([]string{"session", "resume", "--full-clone", "/tmp/workspace"})
	require.Error(t, err)

	_, err = parser.Parse([]string{"session", "resume", "--no-clone-hooks", "/tmp/workspace"})
	require.Error(t, err)
}

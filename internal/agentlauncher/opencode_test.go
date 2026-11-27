package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenCodeLauncher_Command_DefaultModel(t *testing.T) {
	l, resolved, err := Parse(string(AgentOpenCode), "", false)
	require.NoError(t, err)
	require.Equal(t, DefaultModel(string(AgentOpenCode)), resolved)
	require.Equal(t, "opencode --prompt 'line 1\nline 2'", l.Command("line 1\nline 2"))
}

func TestOpenCodeLauncher_Command_AgentDefaultOmitsModelFlag(t *testing.T) {
	l, resolved, err := Parse(string(AgentOpenCode), ModelAgentDefault, false)
	require.NoError(t, err)
	require.Equal(t, ModelAgentDefault, resolved)
	require.Equal(t, "opencode --prompt 'line 1\nline 2'", l.Command("line 1\nline 2"))
	require.Empty(t, l.Environment())
}

func TestOpenCodeLauncher_EnvironmentContainsJSONModelConfig(t *testing.T) {
	t.Parallel()

	l := OpenCode("openai/gpt-5#variant")
	require.Equal(t, map[string]string{
		"OPENCODE_CONFIG_CONTENT": `{"model":"openai/gpt-5#variant"}`,
	}, l.Environment())
}

func TestOpenCodeLauncher_Command_WithModelAndQuotes(t *testing.T) {
	l := OpenCode("openai/gpt-5#variant")
	got := l.Command("don't stop")
	require.Equal(t, "opencode --prompt 'don'\\''t stop'", got)
}

func TestOpenCodeLauncher_Command_ExtraArgsBeforePrompt(t *testing.T) {
	l := OpenCode("gpt-4o")
	got := l.Command("hello", "--foo", "--bar")
	require.Equal(t, "opencode '--foo' '--bar' --prompt 'hello'", got)
}

func TestOpenCodeLauncher_Arguments(t *testing.T) {
	l := OpenCode("openai/gpt-5")
	require.Equal(t, []string{
		"--foo",
		"--prompt", "fix it",
	}, l.Arguments("fix it", "--foo"))
}

package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenCodeLauncher_Command_DefaultModel(t *testing.T) {
	l, resolved, err := Parse(string(AgentOpenCode), "", false)
	require.NoError(t, err)
	require.Equal(t, DefaultModel(string(AgentOpenCode)), resolved)
	require.Equal(
		t,
		"opencode --prompt 'line 1\nline 2' --model '"+DefaultModel(string(AgentOpenCode))+"'",
		l.Command("line 1\nline 2"),
	)
}

func TestOpenCodeLauncher_Command_AgentDefaultOmitsModelFlag(t *testing.T) {
	l, resolved, err := Parse(string(AgentOpenCode), ModelAgentDefault, false)
	require.NoError(t, err)
	require.Equal(t, ModelAgentDefault, resolved)
	require.Equal(t, "opencode --prompt 'line 1\nline 2'", l.Command("line 1\nline 2"))
}

func TestOpenCodeLauncher_Command_WithModelAndQuotes(t *testing.T) {
	l := OpenCode("gpt-4o")
	got := l.Command("don't stop")
	require.Equal(t, "opencode --prompt 'don'\\''t stop' --model 'gpt-4o'", got)
}

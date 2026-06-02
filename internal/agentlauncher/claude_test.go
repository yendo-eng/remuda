package agentlauncher

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeLauncher_Command_IncludesPromptWhenNonEmpty(t *testing.T) {
	l := Claude("", false, "")
	require.Equal(t, "claude 'line 1\nline 2'", l.Command("line 1\nline 2"))
}

func TestClaudeLauncher_Command_OmitsPromptWhenEmptyOrWhitespace(t *testing.T) {
	l := Claude("", false, "")
	for _, prompt := range []string{"", " ", "\n\t"} {
		require.Equal(t, "claude", l.Command(prompt))
	}
}

func TestClaudeLauncher_Command_DefaultModelIncludesEffort(t *testing.T) {
	l, resolved, err := ParseWithReasoning(string(AgentClaude), "", "high", false)
	require.NoError(t, err)
	require.Equal(t, "", resolved)
	cmd := l.Command("line 1\nline 2")
	require.Contains(t, cmd, "claude")
	require.Contains(t, cmd, "--effort 'high'")
	require.NotContains(t, cmd, "--model ")
	require.True(t, strings.HasSuffix(cmd, "'line 1\nline 2'"))
}

func TestClaudeLauncher_Command_WithModelYoloAndQuoteEscaping(t *testing.T) {
	l := Claude("claude-opus-4.1", true, "medium")
	cmd := l.Command("don't stop")
	require.Contains(t, cmd, "--model 'claude-opus-4.1'")
	require.Contains(t, cmd, "--dangerously-skip-permissions")
	require.Contains(t, cmd, "--effort 'medium'")
	require.True(t, strings.HasSuffix(cmd, "'don'\\''t stop'"))
}

func TestClaudeLauncher_Command_AgentDefaultOmitsModelFlag(t *testing.T) {
	l, resolved, err := ParseWithReasoning(string(AgentClaude), ModelAgentDefault, "low", false)
	require.NoError(t, err)
	require.Equal(t, ModelAgentDefault, resolved)
	cmd := l.Command("do stuff")
	require.NotContains(t, cmd, "--model")
	require.Contains(t, cmd, "--effort 'low'")
}

func TestClaudeLauncher_Command_EmptyPromptOmitsPositionalArg(t *testing.T) {
	l := Claude("claude-sonnet-4", false, "high")
	withPrompt := l.Command("write tests")
	withoutPrompt := l.Command("")
	require.Contains(t, withPrompt, "'write tests'")
	require.Contains(t, withoutPrompt, "--model 'claude-sonnet-4'")
	require.Contains(t, withoutPrompt, "--effort 'high'")
	require.NotContains(t, withoutPrompt, "'write tests'")
}

func TestClaudeLauncher_Command_EmptyReasoningOmitsEffort(t *testing.T) {
	l := Claude("claude-sonnet-4", false, "")
	cmd := l.Command("do stuff")
	require.Contains(t, cmd, "--model 'claude-sonnet-4'")
	require.NotContains(t, cmd, "--effort")
	require.True(t, strings.HasSuffix(cmd, "'do stuff'"))
}

func TestClaudeLauncher_Command_WithRemoteControl(t *testing.T) {
	l := Claude("claude-sonnet-4", false, "")
	withRemote, supported := l.WithRemoteControl("")
	require.True(t, supported)
	cmd := withRemote.Command("do stuff")
	require.Contains(t, cmd, "claude --model 'claude-sonnet-4' --remote-control -- 'do stuff'")
	require.NotContains(t, cmd, "--remote-control 'do stuff'")
}

func TestClaudeLauncher_Command_WithRemoteControlSession(t *testing.T) {
	l := Claude("", false, "")
	withRemote, supported := l.WithRemoteControl("my-session")
	require.True(t, supported)
	require.Contains(t, withRemote.Command("do stuff"), "claude --remote-control 'my-session' 'do stuff'")
}

func TestClaudeLauncher_Metadata(t *testing.T) {
	l := Claude("", false, "")
	require.Equal(t, "claude", l.Name())
	require.Equal(t, "", l.MinimumVersion())
	require.Equal(t, []string{"sonnet", "opus", "claude-sonnet-4-6", "claude-opus-4-7", "claude-opus-4-8"}, l.SupportedModels())
}

func TestClaudeLauncher_Command_ModelIsShellQuoted(t *testing.T) {
	l := Claude("foo'bar; rm -rf /", false, "")
	cmd := l.Command("do stuff")
	require.Contains(t, cmd, "--model 'foo'\\''bar; rm -rf /'")
}

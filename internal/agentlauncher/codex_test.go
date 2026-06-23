package agentlauncher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexLauncher_SupportedModels_ContainsGPT54(t *testing.T) {
	models := Codex("", false, "").SupportedModels()
	require.Contains(t, models, "gpt-5.5")
	require.Contains(t, models, "gpt-5.4")
}

func TestCodexLauncher_Command_DashPrefixedPromptUsesArgumentTerminator(t *testing.T) {
	l := Codex("", false, "")

	for _, prompt := range []string{"-", "--name"} {
		t.Run(prompt, func(t *testing.T) {
			cmd := l.Command(prompt)
			require.Contains(t, cmd, " -- '"+prompt+"'")
		})
	}
}

func TestCodexLauncher_Command_MultiLineDashedHeaderPromptUsesArgumentTerminator(t *testing.T) {
	l := Codex("", false, "")
	prompt := "---------- Ticket RBL-123 ----------\n\nSummarize the current branch."
	cmd := l.Command(prompt)

	require.Contains(t, cmd, " -- '"+prompt+"'")
}

func TestCodexLauncher_WithRemoteControl_IsUnsupported(t *testing.T) {
	l := Codex("", false, "")
	got, supported := l.WithRemoteControl("session-1")
	require.False(t, supported)
	require.Equal(t, l.Command("prompt"), got.Command("prompt"))
}

func TestCodexLauncher_Command_ExtraArgsBeforePrompt(t *testing.T) {
	l := Codex("gpt-5.5", false, "")
	cmd := l.Command("hello", "--foo", "--bar")
	require.Contains(t, cmd, "--model 'gpt-5.5' --foo --bar -- 'hello'")
}
